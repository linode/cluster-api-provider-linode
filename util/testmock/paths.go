package testmock

import (
	"fmt"
	"strings"
)

// Paths declares one or more code paths to test with mock clients.
func Paths(nodes ...node) []entry {
	if len(nodes) == 0 {
		return nil
	}

	pths := drawPaths(nodes)
	result := make([]entry, len(pths))
	for i, p := range pths {
		result[i] = createPath(p)
	}

	return result
}

// Traverses each node and their children, returning a list of permutations,
// each representing a different code path as specified and evaluated in order.
// New permutations are defined by
//  1. Terminal nodes i.e. entries that have a function for asserting results.
//  2. Nodes belonging to a fork i.e. multiple entries non-occurring on the same path.
func drawPaths(nodes []node) [][]entry {
	if len(nodes) == 0 {
		return nil
	}

	pathGroup := [][]entry{{}}

	for i, n := range nodes {
		switch impl := n.(type) {
		case entry:
			var added bool

			// Add new entry to each unclosed path
			for j, pth := range pathGroup {
				if len(pth) == 0 || pth[len(pth)-1].result.value == nil {
					pathGroup[j] = append(pathGroup[j], impl)
					added = true
				}
			}

			// If all paths are closed, make a new path
			if !added {
				pathGroup = append(pathGroup, []entry{impl})
			}

			// Panic if any paths are open at the end
			if impl.result.value == nil && i == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", i))
			}

		case fork:
			var newPathGroup [][]entry
			var added, open bool

			// Make new version of each unclosed path with each new entry
			for _, pth := range pathGroup {
				var included bool

				for _, fe := range impl.entries {
					if impl.text != "" {
						fe.Text = strings.TrimSpace(fmt.Sprintf("%s: %s", impl.text, fe.Text))
					}

					// If either new entry has no result, path will be open
					if fe.result.value == nil {
						open = true
					}

					// Duplicate unclosed paths with the new entry
					if len(pth) == 0 || pth[len(pth)-1].result.value == nil {
						newPth := append(pth, fe)
						newPathGroup = append(newPathGroup, newPth)
						added = true
					} else if !included {
						// Include closed paths
						newPathGroup = append(newPathGroup, pth)
						included = true
					}
				}
			}

			// If all paths are closed, make new paths
			if !added {
				for _, fe := range impl.entries {
					if impl.text != "" {
						fe.Text = strings.TrimSpace(fmt.Sprintf("%s: %s", impl.text, fe.Text))
					}
					newPathGroup = append(newPathGroup, []entry{fe})
				}
			}

			// Panic if any paths are open at the end
			if open && i == len(nodes)-1 {
				panic(fmt.Errorf("unresolved path at index %d", i))
			}
			pathGroup = newPathGroup
		}
	}

	return pathGroup
}

// Processes the list of entries into a single code path.
func createPath(nodes []entry) entry {
	pth := entry{}

	var text []string

	for _, n := range nodes {
		text = append(text, n.Text)
		for _, call := range n.calls {
			pth.calls = append(pth.calls, fn{
				text:  fmt.Sprintf("Case(%s) > Mock(%s)", n.Text, call.text),
				value: call.value,
			})
		}
		if n.result.value != nil {
			pth.result = fn{
				text:  fmt.Sprintf("Result(%s)", n.result.text),
				value: n.result.value,
			}
		}
	}

	pth.Text = strings.Join(text, " > ")

	return pth
}
