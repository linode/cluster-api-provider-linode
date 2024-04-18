package mocktest

import (
	"errors"
	"strings"
)

// A container for mock calls and a function for asserting results.
type path struct {
	// Store as pointers so each path can check if it was invoked
	once  []*once
	calls []call
	result
}

// Describe generates a string of all nodes belonging to a test path.
func (p path) Describe() string {
	text := make([]string, 0, len(p.once)+len(p.calls)+1)
	for _, o := range p.once {
		if !o.described {
			text = append(text, o.text)
			o.described = true
		}
	}
	for _, c := range p.calls {
		text = append(text, c.text)
	}
	text = append(text, p.result.text)
	return strings.Join(text, " > ")
}

// Paths declares one or more test paths with mock clients.
// It traverses each node and their children, returning a list of permutations,
// each representing a different test path as specified and evaluated in order.
func Paths(nodes ...node) []path {
	if len(nodes) == 0 {
		return nil
	}

	staged, committed := rPaths(nil, nil, nodes)
	if len(staged) > 0 {
		panic(errors.New("unresolved path detected"))
	}

	return committed
}

func rPaths(staged, committed []path, each []node) (st, com []path) {
	if len(each) == 0 {
		return staged, committed
	}

	// Get the current node to add to staged/committed.
	head, tail := each[0], each[1:]

	// If there are no open paths, make a new path.
	if len(staged) == 0 {
		staged = append(staged, path{})
	}

	// Add to staged/committed.
	staged, committed = head.update(staged, committed)

	return rPaths(staged, committed, tail)
}
