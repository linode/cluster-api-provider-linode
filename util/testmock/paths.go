package testmock

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/linode/cluster-api-provider-linode/mock"
)

type Path struct {
	Text    string
	allText string

	linodeCalls   []any
	linodeResults []any
	k8sCalls      []func(*mock.MockK8sClient)
	k8sResults    []func(context.Context, *mock.MockK8sClient)
}

func mockResult[T any](ctx context.Context, result, lc any, kc ...*mock.MockK8sClient) {
	if reflect.TypeOf(result).NumIn() > 2 {
		if len(kc) == 0 {
			panic("detected Then with mock Linode client and MockK8sClient, but no MockK8sClient passed to Run")
		}

		mockFunc := result.(func(context.Context, T, *mock.MockK8sClient))
		mockFunc(ctx, lc.(T), kc[0])
		return
	}

	mockFunc := result.(func(context.Context, T))
	mockFunc(ctx, lc.(T))
}

// TODO: Try to combine linodeCalls and k8sCalls into one to preserve ordering.
// Ideally, try to group and trigger each Event in sequence regardless of type (i.e. [[If, Mock], [If, Mock, Then]])
// Then it should be possible to log events using ginkgo.By or t.Log
func (p Path) Run(ctx context.Context, lc any, kc ...*mock.MockK8sClient) {
	if len(p.k8sCalls) > 0 {
		switch lcType := lc.(type) {
		case *mock.MockK8sClient:
			for _, call := range p.k8sCalls {
				call(lcType)
			}
			for _, result := range p.k8sResults {
				mockResult[*mock.MockK8sClient](ctx, result, lc)
			}
		default:
			// If lc is not MockK8sClient, assume MockK8sClient was passed as a 3rd arg
			if len(kc) == 0 {
				panic("detected Mock with MockK8sClient, but no MockK8sClient passed to Run")
			}
			for _, call := range p.k8sCalls {
				call(kc[0])
			}
			for _, result := range p.k8sResults {
				mockResult[*mock.MockK8sClient](ctx, result, lc, kc...)
			}
		}
	}

	switch lcType := lc.(type) {
	case *mock.MockLinodeMachineClient:
		for _, call := range p.linodeCalls {
			mockCall := call.(func(*mock.MockLinodeMachineClient))
			mockCall(lcType)
		}
		for _, result := range p.linodeResults {
			mockResult[*mock.MockLinodeMachineClient](ctx, result, lc, kc...)
		}
	case *mock.MockLinodeInstanceClient:
		for _, call := range p.linodeCalls {
			mockCall := call.(func(*mock.MockLinodeInstanceClient))
			mockCall(lcType)
		}
		for _, result := range p.linodeResults {
			mockResult[*mock.MockLinodeInstanceClient](ctx, result, lc, kc...)
		}
	case *mock.MockLinodeVPCClient:
		for _, call := range p.linodeCalls {
			mockCall := call.(func(*mock.MockLinodeVPCClient))
			mockCall(lcType)
		}
		for _, result := range p.linodeResults {
			mockResult[*mock.MockLinodeVPCClient](ctx, result, lc, kc...)
		}
	case *mock.MockLinodeNodeBalancerClient:
		for _, call := range p.linodeCalls {
			mockCall := call.(func(*mock.MockLinodeNodeBalancerClient))
			mockCall(lcType)
		}
		for _, result := range p.linodeResults {
			mockResult[*mock.MockLinodeNodeBalancerClient](ctx, result, lc, kc...)
		}
	case *mock.MockLinodeObjectStorageClient:
		for _, call := range p.linodeCalls {
			mockCall := call.(func(*mock.MockLinodeObjectStorageClient))
			mockCall(lcType)
		}
		for _, result := range p.linodeResults {
			mockResult[*mock.MockLinodeObjectStorageClient](ctx, result, lc, kc...)
		}
	default:
	}
}

func Paths(nodes ...node) []Path {
	if len(nodes) == 0 {
		return nil
	}

	pths := paths(nodes)
	each := make([]Path, len(pths))
	for i, p := range pths {
		each[i] = createPath(p)
	}

	return each
}

func paths(nodes []node) [][]entry {
	if len(nodes) == 0 {
		return nil
	}

	var each [][]entry

	for i, n := range nodes {
		switch impl := n.(type) {
		case entry:
			var added bool
			// Add new entry to each unclosed path
			for j, pth := range each {
				if len(pth) == 0 || pth[len(pth)-1].result == nil {
					each[j] = append(each[j], impl)
					added = true
				}
			}

			// If all paths are closed, make a new one
			if !added {
				each = append(each, []entry{impl})
			}

			// Panic if any paths are open at the end
			if impl.result == nil && i == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", i))
			}

		case fork:
			var newEach [][]entry
			var added, open bool
			closed := map[int]struct{}{}
			// Make new version of each unclosed path with each new entry

			// TODO: invert this to loop through each first
			for _, fe := range impl {
				if fe.result == nil {
					open = true
				}
				for j, pth := range each {
					// Duplicate unclosed paths
					if len(pth) == 0 || pth[len(pth)-1].result == nil {
						newPth := append(pth, fe)
						newEach = append(newEach, newPth)
						added = true
					} else if _, ok := closed[j]; !ok {
						// Include closed paths
						newEach = append(newEach, pth)
						closed[j] = struct{}{}
					}
				}
			}

			// If all paths are closed, make new ones
			if !added {
				for _, fe := range impl {
					newEach = append(newEach, []entry{fe})
				}
			}

			// Panic if any paths are open at the end
			if open && i == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", i))
			}
			each = newEach
		}
	}

	return each
}

func createPath(nodes []entry) Path {
	pth := Path{}

	var text []string
	var allText []string

	for _, n := range nodes {
		text = append(text, n.text)
		nodeText := fmt.Sprintf("If(%s)", n.text)
		if n.called != nil {
			switch mockCall := n.called.(type) {
			case func(*mock.MockK8sClient):
				pth.k8sCalls = append(pth.k8sCalls, mockCall)
			default:
				pth.linodeCalls = append(pth.linodeCalls, n.called)
			}
			nodeText += fmt.Sprintf(" > Mock(%s)", n.calledText)
		}
		if n.result != nil {
			switch mockResult := n.result.(type) {
			case func(context.Context, *mock.MockK8sClient):
				pth.k8sResults = append(pth.k8sResults, mockResult)
			default:
				pth.linodeResults = append(pth.linodeResults, n.result)
			}
			nodeText += fmt.Sprintf(" > Then(%s)", n.resultText)
		}
		allText = append(allText, nodeText)
	}

	pth.Text = strings.Join(text, " > ")
	pth.allText = strings.Join(allText, "\n")

	return pth
}
