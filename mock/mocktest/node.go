package mocktest

import (
	"bytes"
	"context"
	"fmt"

	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/record"

	"github.com/linode/cluster-api-provider-linode/mock"
)

// Mock holds configuration for a single test path.
type Mock struct {
	gomock.TestReporter
	mock.MockClients

	recorder *record.FakeRecorder
	logs     *bytes.Buffer
}

// Events returns a channel for receiving event strings for a single test path.
func (m Mock) Events() <-chan string {
	return m.recorder.Events
}

// Logs returns a string of all log output written during a single test path.
func (m Mock) Logs() string {
	return m.logs.String()
}

// Call declares a function for mocking method calls on a single mock client.
func Call(text string, does func(context.Context, Mock)) call {
	return call{
		text: fmt.Sprintf("Call(%s)", text),
		does: does,
	}
}

// Result terminates a test path with a function that tests the effects of mocked method calls.
func Result(text string, does func(context.Context, Mock)) result {
	return result{
		text: fmt.Sprintf("Result(%s)", text),
		does: does,
	}
}

// Once declares a function that runs one time when executing all test paths.
// It is triggered at the beginning of the leftmost test path where it is inserted.
func Once(text string, does func(context.Context)) once {
	return once{
		text: fmt.Sprintf("Once(%s)", text),
		does: does,
	}
}

// Case declares both a Mock and a Result for terminating a test path.
func Case(c call, r result) leaf {
	return leaf{c, r}
}

// Either declares multiple nodes that fork out into unique test paths.
func Either(nodes ...node) fork {
	return nodes
}

// Common interface for defining permutations of test paths as a tree.
type node interface {
	update(staged, committed []path) (st, com []path)
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

// Contains a function for an event trigger that runs once.
type once struct {
	text      string
	does      func(context.Context)
	described bool
	ran       bool
}

// Adds once to the first staged path.
// It will only be invoked once in the first path to be evaluated.
func (o once) update(staged, committed []path) (st, com []path) {
	staged[0].once = append(staged[0].once, &o)

	return staged, committed
}

// Contains both a function for mocking calls and a result to end a path.
type leaf struct {
	call
	result
}

// Commits each staged path with the leaf's call and result.
func (l leaf) update(staged, committed []path) (st, com []path) {
	for _, pth := range staged {
		newCalls := make([]call, len(pth.calls), len(pth.calls)+1)
		copy(newCalls, pth.calls)
		committed = append(committed, path{
			once:   pth.once,
			calls:  append(newCalls, l.call),
			result: l.result,
		})
	}

	staged = []path{}

	return staged, committed
}

// A container for defining nodes that fork out into new test paths.
type fork []node

// Generates new permutations of each staged path with each node in the fork.
// Each node in the fork should never occur on the same path.
func (f fork) update(staged, committed []path) (st, com []path) {
	var permutations []path

	for _, pth := range staged {
		for _, fi := range f {
			var localPerms []path
			localPerms, committed = fi.update([]path{pth}, committed)
			permutations = append(permutations, localPerms...)
		}
	}

	return permutations, committed
}

// A container for describing and holding a function.
type fn struct {
	text string
	does func(context.Context, Mock)
}
