package testmock

// Case declares a single node belonging to one or more code paths.
func Case(text string, actions ...action) entry {
	ent := entry{Text: text}
	for _, do := range actions {
		do(&ent)
	}

	return ent
}

// Either declares multiple nodes forking out into different code paths.
func Either(text string, entries ...entry) fork {
	return fork{
		text:    text,
		entries: entries,
	}
}

// Mock declares a function for mocking method calls on a single mock client.
func Mock(text string, call any) action {
	return func(m *entry) {
		m.calls = append(m.calls, fn{text, call})
	}
}

// Result terminates a code path with a function that tests the effects of mocked method calls.
func Result(text string, result any) action {
	return func(m *entry) {
		if m.result.value != nil {
			panic("attempted Case with multiple Result")
		}
		m.result.text = text
		m.result.value = result
	}
}

// Common interface for defining permutations of code paths as a tree.
type node interface {
	impl()
}

func (fork) impl()  {}
func (entry) impl() {}

// A container for defining nodes that fork out to new code paths.
type fork struct {
	text    string
	entries []entry
}

// A node that includes functions for mocking.
// The node is considered terminal if result's value is non-nil.
type entry struct {
	Text   string
	calls  []fn
	result fn
}

// A container for describing and holding a function.
type fn struct {
	text  string
	value any
}

// An abstraction for populating code paths.
type action func(n *entry)
