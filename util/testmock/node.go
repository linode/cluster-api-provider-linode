package testmock

import "fmt"

// Mock declares a function for mocking method calls on a single mock client.
func Mock(text string, value any) call {
	return call{
		text:  fmt.Sprintf("Mock(%s)", text),
		value: value,
	}
}

// Result terminates a code path with a function that tests the effects of mocked method calls.
func Result(text string, value any) result {
	return result{
		text:  fmt.Sprintf("Result(%s)", text),
		value: value,
	}
}

// Once declares a function that runs one time when executing all code paths.
// It is triggered at the beginning of the leftmost code path where it is inserted.
func Once(text string, value any) once {
	return once{
		fn: fn{
			text:  fmt.Sprintf("Once(%s)", text),
			value: value,
		},
	}
}

// Case declares both a Mock and a Result for terminating a code path.
func Case(c call, r result) leaf {
	return leaf{c, r}
}

// Either declares multiple nodes that fork out into unique code paths.
func Either(nodes ...prong) fork {
	return nodes
}

// Common interface for defining permutations of code paths as a tree.
type node interface {
	node()
}

// A container for describing and holding a function.
type fn struct {
	text  string
	value any
}

// Contains a function for mocking method calls on a single mock client.
type call fn

// Contains a function that tests the effects of mocked method calls.
type result fn

// Contains a function for an event trigger that runs once.
type once struct {
	fn
	described bool
	ran       bool
}

type leaf struct {
	call
	result
}

// A container for defining nodes that fork out into new code paths.
type fork []prong

// Common interface for nodes that fork out into new code paths.
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
