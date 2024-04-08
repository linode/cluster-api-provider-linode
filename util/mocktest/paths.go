package mocktest

import (
	"strings"

	"github.com/linode/cluster-api-provider-linode/mock"
)

type Path struct {
	Message string
	Fail    bool
	Calls   []mocker
}

func (p Path) Run(client any) {
	switch c := client.(type) {
	case *mock.MockLinodeMachineClient:
		for _, m := range p.Calls {
			fn := m.call.(func(*mock.MockLinodeMachineClient))
			fn(c)
		}
	case *mock.MockLinodeInstanceClient:
		for _, m := range p.Calls {
			fn := m.call.(func(*mock.MockLinodeInstanceClient))
			fn(c)
		}
	case *mock.MockLinodeVPCClient:
		for _, m := range p.Calls {
			fn := m.call.(func(*mock.MockLinodeVPCClient))
			fn(c)
		}
	case *mock.MockLinodeNodeBalancerClient:
		for _, m := range p.Calls {
			fn := m.call.(func(*mock.MockLinodeNodeBalancerClient))
			fn(c)
		}
	case *mock.MockLinodeObjectStorageClient:
		for _, m := range p.Calls {
			fn := m.call.(func(*mock.MockLinodeObjectStorageClient))
			fn(c)
		}
	case *mock.MockK8sClient:
		for _, m := range p.Calls {
			fn := m.call.(func(*mock.MockK8sClient))
			fn(c)
		}
	default:
		panic("Path.Run invoked with unknown mock client")
	}
}

func Paths(name string, nodes ...node) []Path {
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
	var desc []string
	for _, n := range nodes {
		if n.msg != "" {
			desc = append(desc, n.msg)
		}
	}

	pth := Path{Calls: nodes}
	pth.Message = strings.Join(desc, " ")

	if nodes[len(nodes)-1].fail {
		pth.Fail = true
	}

	return pth
}

func paths(nodes []node) [][]mocker {
	if len(nodes) == 0 {
		return nil
	}

	each := [][]mocker{{}}

	for _, n := range nodes {
		switch impl := n.(type) {
		case mocker:
			each[0] = append(each[0], impl)

		case fork:
			impl.fail.fail = true
			each = append(each, append(each[0], impl.fail))
			each[0] = append(each[0], impl.pass)
		}
	}

	return each
}
