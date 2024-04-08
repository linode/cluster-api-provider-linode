package testmock

import (
	"fmt"
	"strings"
)

type path struct {
	Text   string
	events []event
}

type event struct {
	isResult bool
	text     string
	value    any
}

type paths []path

func Paths(nodes ...node) paths {
	if len(nodes) == 0 {
		return nil
	}

	pths := drawPaths(nodes)
	result := make(paths, len(pths))
	for i, p := range pths {
		result[i] = createPath(p)
	}

	return result
}

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
						fe.text = strings.TrimSpace(fmt.Sprintf("%s: %s", impl.text, fe.text))
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
						fe.text = strings.TrimSpace(fmt.Sprintf("%s: %s", impl.text, fe.text))
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

func createPath(nodes []entry) path {
	pth := path{}

	var text []string

	for _, n := range nodes {
		text = append(text, n.text)
		for _, call := range n.calls {
			pth.events = append(pth.events, event{
				text:  fmt.Sprintf("Case(%s) > Mock(%s)", n.text, call.text),
				value: call.value,
			})
		}
		if n.result.value != nil {
			pth.events = append(pth.events, event{
				isResult: true,
				text:     fmt.Sprintf("Result(%s)", n.result.text),
				value:    n.result.value,
			})
		}
	}

	pth.Text = strings.Join(text, " > ")

	return pth
}
