package mocktest

import (
	"fmt"
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
// New permutations are defined by
// 1. Terminal nodes i.e. entries that have a function for asserting results.
// 2. Nodes belonging to a fork i.e. multiple entries non-occurring on the same path.
func Paths(nodes ...node) []path {
	if len(nodes) == 0 {
		return nil
	}

	tmp := []path{}
	final := []path{}

	for idx, n := range nodes {
		// If all paths are closed, make a new path
		if len(tmp) == 0 {
			tmp = append(tmp, path{})
		}

		switch impl := n.(type) {
		// A once node should only be added to the first path.
		// It will only invoked once in the first path evaluated.
		case once:
			tmp[0].once = append(tmp[0].once, &impl)

		// A call node should be appended to all open paths.
		case call:
			// Add new entry to each open path
			for j := range tmp {
				tmp[j].calls = append(tmp[j].calls, impl)
			}

			// Panic if any paths are open at the end
			if idx == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", idx))
			}

			// A result node should terminate all open paths.
		case result:
			// Close all open paths
			for j := range tmp {
				tmp[j].result = impl
			}

			// Commit all closed paths
			final = append(final, tmp...)
			tmp = nil

		// A leaf node contains both a call node and a result node.
		// The call is appended to all open paths, and then immediately closed with the result.
		case leaf:
			// Add new entry to each open path and close it
			for j := range tmp {
				tmp[j].calls = append(tmp[j].calls, impl.call)
				tmp[j].result = impl.result
			}

			// Commit all closed paths
			final = append(final, tmp...)
			tmp = nil

		// A fork node is a list of call or leaf nodes that should not occur on the same path.
		case fork:
			var newTmp []path
			var open bool

			// Make new version of each open path with each new entry
			for _, pth := range tmp {
				for _, fi := range impl {
					switch forkImpl := fi.(type) {
					case once:
						open = true
						newTmp = append(newTmp, path{
							once:  append(pth.once, &forkImpl),
							calls: pth.calls,
						})

					case call:
						open = true
						// Duplicate open paths with the new entry
						newTmp = append(newTmp, path{
							once:  pth.once,
							calls: append(pth.calls, forkImpl),
						})

					case result:
						final = append(final, path{
							once:   pth.once,
							calls:  pth.calls,
							result: forkImpl,
						})

					// A leaf in a fork is terminal
					case leaf:
						// Avoid mutating pth.calls slice by allocating new slice.
						// Calling append(pth.calls, ...) shares state.
						newCalls := make([]call, 0, len(pth.calls))
						newCalls = append(newCalls, pth.calls...)
						final = append(final, path{
							once:   pth.once,
							calls:  append(newCalls, forkImpl.call),
							result: forkImpl.result,
						})
					}
				}
			}

			tmp = newTmp

			// Panic if any paths are open at the end
			if open && idx == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", idx))
			}
		}
	}

	return final
}
