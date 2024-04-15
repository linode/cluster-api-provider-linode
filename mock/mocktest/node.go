package mocktest

import (
	"bytes"
	"context"
	"fmt"

	"github.com/linode/cluster-api-provider-linode/mock"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/record"
)

// MockContext is the context for a single test path.
type MockContext struct {
	context.Context
	gomock.TestReporter
	mock.MockClients

	recorder *record.FakeRecorder
	logs     *bytes.Buffer
}

// Events returns a channel for receiving event strings for a single test path.
func (ctx MockContext) Events() <-chan string {
	return ctx.recorder.Events
}

// Logs returns a string of all log output written during a single test path.
func (ctx MockContext) Logs() string {
	return ctx.logs.String()
}

// Mock declares a function for mocking method calls on a single mock client.
func Mock(text string, do func(MockContext)) call {
	return call{
		text: fmt.Sprintf("Mock(%s)", text),
		do:   do,
	}
}

// Result terminates a test path with a function that tests the effects of mocked method calls.
func Result(text string, do func(MockContext)) result {
	return result{
		text: fmt.Sprintf("Result(%s)", text),
		do:   do,
	}
}

// Once declares a function that runs one time when executing all test paths.
// It is triggered at the beginning of the leftmost test path where it is inserted.
func Once(text string, do func(context.Context)) once {
	return once{
		text: fmt.Sprintf("Once(%s)", text),
		do:   do,
	}
}

// Case declares both a Mock and a Result for terminating a test path.
func Case(c call, r result) leaf {
	return leaf{c, r}
}

// Either declares multiple nodes that fork out into unique test paths.
func Either(nodes ...prong) fork {
	return nodes
}

// Common interface for defining permutations of test paths as a tree.
type node interface {
	node()
}

// A container for describing and holding a function.
type fn struct {
	text string
	do   func(MockContext)
}

// Contains a function for mocking method calls on a single mock client.
type call fn

// Contains a function that tests the effects of mocked method calls.
type result fn

// Contains a function for an event trigger that runs once.
type once struct {
	text      string
	do        func(context.Context)
	described bool
	ran       bool
}

type leaf struct {
	call
	result
}

// A container for defining nodes that fork out into new test paths.
type fork []prong

// Common interface for nodes that fork out into new test paths.
type prong interface {
	prong()
}

func (call) node()   {}
func (result) node() {}
func (once) node()   {}
func (leaf) node()   {}
func (fork) node()   {}

func (call) prong() {}
func (leaf) prong() {}
func (once) prong() {}
