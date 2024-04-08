package mocktest

import (
	"context"
	"strings"

	"github.com/linode/cluster-api-provider-linode/mock"
)

type Path struct {
	Message string

	calls   []mocker
	asserts []any
}

func (p Path) Run(ctx context.Context, client any) {
	switch c := client.(type) {
	case *mock.MockLinodeMachineClient:
		for _, m := range p.calls {
			fn := m.call.(func(*mock.MockLinodeMachineClient))
			fn(c)
		}
		for _, a := range p.asserts {
			fn := a.(func(context.Context, *Path, *mock.MockLinodeMachineClient))
			fn(ctx, &p, c)
		}
	case *mock.MockLinodeInstanceClient:
		for _, m := range p.calls {
			fn := m.call.(func(*mock.MockLinodeInstanceClient))
			fn(c)
		}
		for _, a := range p.asserts {
			fn := a.(func(context.Context, *Path, *mock.MockLinodeInstanceClient))
			fn(ctx, &p, c)
		}
	case *mock.MockLinodeVPCClient:
		for _, m := range p.calls {
			fn := m.call.(func(*mock.MockLinodeVPCClient))
			fn(c)
		}
		for _, a := range p.asserts {
			fn := a.(func(context.Context, *Path, *mock.MockLinodeVPCClient))
			fn(ctx, &p, c)
		}
	case *mock.MockLinodeNodeBalancerClient:
		for _, m := range p.calls {
			fn := m.call.(func(*mock.MockLinodeNodeBalancerClient))
			fn(c)
		}
		for _, a := range p.asserts {
			fn := a.(func(context.Context, *Path, *mock.MockLinodeNodeBalancerClient))
			fn(ctx, &p, c)
		}
	case *mock.MockLinodeObjectStorageClient:
		for _, m := range p.calls {
			fn := m.call.(func(*mock.MockLinodeObjectStorageClient))
			fn(c)
		}
		for _, a := range p.asserts {
			fn := a.(func(context.Context, *Path, *mock.MockLinodeObjectStorageClient))
			fn(ctx, &p, c)
		}
	case *mock.MockK8sClient:
		for _, m := range p.calls {
			fn := m.call.(func(*mock.MockK8sClient))
			fn(c)
		}
		for _, a := range p.asserts {
			fn := a.(func(context.Context, *Path, *mock.MockK8sClient))
			fn(ctx, &p, c)
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

func createPath(nodes []mocker) Path {
	pth := Path{calls: nodes}

	var desc []string
	for _, n := range nodes {
		if n.message != "" {
			desc = append(desc, n.message)
		}
		if n.asserts != nil {
			pth.asserts = append(pth.asserts, n.asserts)
		}
	}

	pth.Message = strings.Join(desc, " ")

	return pth
}

func paths(nodes []node) [][]mocker {
	if len(nodes) == 0 {
		return nil
	}

	each := [][]mocker{{}}

	var currPath int

	for i, n := range nodes {
		switch impl := n.(type) {
		case mocker:
			each[currPath] = append(each[currPath], impl)
			if impl.asserts != nil && i < len(nodes)-1 {
				each = append(each, []mocker{})
				currPath = len(each) - 1
			}

		case fork:
			impl.fail.fail = true
			each = append(each, append(each[0], impl.fail))
			each[0] = append(each[0], impl.pass)
		}
	}

	return each
}
