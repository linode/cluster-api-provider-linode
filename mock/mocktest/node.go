package mocktest

import (
	"context"
)

// Common interface for defining permutations of test paths as a tree.
type node interface {
	update(staged, committed []path) (st, com []path)
}

// A container for describing and holding a function.
type fn struct {
	text string
	does func(context.Context, Mock)
	ran  bool
}

// Call declares a function for mocking method calls on a single mock client.
func Call(text string, does func(context.Context, Mock)) call {
	return call{
		text: text,
		does: does,
	}
}

// Contains a function for mocking method calls on a single mock client.
type call fn

// Adds the call to each staged path.
func (c call) update(staged, committed []path) (st, com []path) {
	for idx, pth := range staged {
		newCalls := make([]call, len(pth.calls), len(pth.calls)+1)
		copy(newCalls, pth.calls)
		staged[idx] = path{
			once:  pth.once,
			calls: append(newCalls, c),
		}
	}

	return staged, committed
}

// Result terminates a test path with a function that tests the effects of mocked method calls.
func Result(text string, does func(context.Context, Mock)) result {
	return result{
		text: text,
		does: does,
	}
}

// Contains a function that tests the effects of mocked method calls.
type result fn

// Commits each staged path with the result.
func (r result) update(staged, committed []path) (st, com []path) {
	for idx := range staged {
		staged[idx].result = r
	}

	committed = append(committed, staged...)
	staged = []path{}

	return staged, committed
}

// Once declares a function that runs one time when executing all test paths.
// It is triggered at the beginning of the leftmost test path where it is inserted.
func Once(text string, does func(context.Context, Mock)) once {
	return once{
		text: text,
		does: does,
	}
}

// Contains a function that will only run once.
type once fn

// Adds once to the first staged path.
// It will only be invoked once in the first path to be evaluated.
func (o once) update(staged, committed []path) (st, com []path) {
	if len(staged) > 0 {
		staged[0].once = append(staged[0].once, &o)
	}

	return staged, committed
}

// Path declares a sequence of nodes belonging to the same test path.
func Path(nodes ...node) allOf {
	return nodes
}

// A container for defining nodes added to the same test path.
type allOf []node

// Adds all nodes to each staged path, committing paths whenever a result is included.
func (a allOf) update(staged, committed []path) (st, com []path) {
	for _, impl := range a {
		staged, committed = impl.update(staged, committed)
	}

	return staged, committed
}

// Either declares multiple nodes that fork out into unique test paths.
func Either(nodes ...node) oneOf {
	return nodes
}

// A container for defining nodes that fork out into unique test paths.
type oneOf []node

// Generates new permutations of each staged path with each node.
// Each node should never occur on the same path.
func (o oneOf) update(staged, committed []path) (st, com []path) {
	var permutations []path

	for _, pth := range staged {
		for _, impl := range o {
			var localPerms []path
			localPerms, committed = impl.update([]path{pth}, committed)
			permutations = append(permutations, localPerms...)
		}
	}

	return permutations, committed
}
