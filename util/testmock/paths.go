package testmock

import (
	"context"
	"fmt"
	"strings"

	"github.com/linode/cluster-api-provider-linode/mock"
)

type Path struct {
	Text string

	calls   []any
	results []any
}

func (p Path) Run(ctx context.Context, client any) {
	switch c := client.(type) {
	case *mock.MockLinodeMachineClient:
		for _, call := range p.calls {
			fn := call.(func(*mock.MockLinodeMachineClient))
			fn(c)
		}
		for _, a := range p.results {
			fn := a.(func(context.Context, *mock.MockLinodeMachineClient))
			fn(ctx, c)
		}
	case *mock.MockLinodeInstanceClient:
		for _, call := range p.calls {
			fn := call.(func(*mock.MockLinodeInstanceClient))
			fn(c)
		}
		for _, a := range p.results {
			fn := a.(func(context.Context, *mock.MockLinodeInstanceClient))
			fn(ctx, c)
		}
	case *mock.MockLinodeVPCClient:
		for _, call := range p.calls {
			fn := call.(func(*mock.MockLinodeVPCClient))
			fn(c)
		}
		for _, a := range p.results {
			fn := a.(func(context.Context, *mock.MockLinodeVPCClient))
			fn(ctx, c)
		}
	case *mock.MockLinodeNodeBalancerClient:
		for _, call := range p.calls {
			fn := call.(func(*mock.MockLinodeNodeBalancerClient))
			fn(c)
		}
		for _, a := range p.results {
			fn := a.(func(context.Context, *mock.MockLinodeNodeBalancerClient))
			fn(ctx, c)
		}
	case *mock.MockLinodeObjectStorageClient:
		for _, call := range p.calls {
			fn := call.(func(*mock.MockLinodeObjectStorageClient))
			fn(c)
		}
		for _, a := range p.results {
			fn := a.(func(context.Context, *mock.MockLinodeObjectStorageClient))
			fn(ctx, c)
		}
	case *mock.MockK8sClient:
		for _, call := range p.calls {
			fn := call.(func(*mock.MockK8sClient))
			fn(c)
		}
		for _, a := range p.results {
			fn := a.(func(context.Context, *mock.MockK8sClient))
			fn(ctx, c)
		}
	default:
		panic("Path.Run invoked with unknown mock client")
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
	for _, n := range nodes {
		if n.text != "" {
			text = append(text, n.text)
		}
		if n.called != nil {
			pth.calls = append(pth.calls, n.called)
			if n.calledText != "" {
				text = append(text, n.calledText)
			}
		}
		if n.result != nil {
			pth.results = append(pth.results, n.result)
			if n.resultText != "" {
				text = append(text, n.resultText)
			}
		}
	}

	pth.Text = strings.Join(text, " > ")

	return pth
}
