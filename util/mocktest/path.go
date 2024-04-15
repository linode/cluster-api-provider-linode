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
	var text []string
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

	tmp := []path{{}}
	final := []path{}

	for i, n := range nodes {
		switch impl := n.(type) {

		// A once node should only be added to the first path.
		// It will only invoked once in the first path evaluated.
		case once:
			if len(tmp) > 0 {
				tmp[0].once = append(tmp[0].once, &impl)
			} else {
				// If there are no open paths, make a new one
				tmp = append(tmp, path{once: []*once{&impl}})
			}

		// A result node should terminate all open paths.
		case result:
			for j, pth := range tmp {
				if len(pth.calls) == 0 {
					panic(fmt.Errorf("closed path with no mock calls at index %d", i))
				}

				// Close all open paths
				tmp[j].result = impl
			}

			// Commit all closed paths
			final = append(final, tmp...)
			tmp = []path{{}}

		// A call node should be appended to all open paths.
		case call:
			// Add new entry to each open path
			for j, pth := range tmp {
				tmp[j].calls = append(pth.calls, impl)
			}

			// If all paths are closed, make a new path
			if len(tmp) == 0 {
				tmp = append(tmp, path{calls: []call{impl}})
			}

			// Panic if any paths are open at the end
			if i == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", i))
			}

		// A leaf node contains both a call node and a result node.
		// The call is appended to all open paths, and then immediately closed with the result.
		case leaf:
			// Add new entry to each open path and close it
			for j, pth := range tmp {
				tmp[j].calls = append(pth.calls, impl.call)
				tmp[j].result = impl.result
			}

			// If all paths are closed, make a new path and close it
			if len(tmp) == 0 {
				final = append(final, path{
					calls:  []call{impl.call},
					result: impl.result,
				})
			}

			// Commit all closed paths
			final = append(final, tmp...)
			tmp = []path{{}}

		// A fork node is a list of call or leaf nodes that should not occur on the same path.
		case fork:
			var newTmp []path
			var open bool

			// If all paths are closed, make new paths with each new entry
			if len(tmp) == 0 {
				for _, fi := range impl {
					switch forkImpl := fi.(type) {
					case once:
						open = true
						tmp = append(tmp, path{once: []*once{&forkImpl}})
					case call:
						open = true
						tmp = append(tmp, path{calls: []call{forkImpl}})
					case leaf:
						final = append(final, path{
							calls:  []call{forkImpl.call},
							result: forkImpl.result,
						})
					}
				}
			}

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
			if open && i == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", i))
			}
		}
	}

	return final
}
