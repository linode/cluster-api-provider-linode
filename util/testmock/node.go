package testmock

import "fmt"

type entry struct {
	text       string
	calledText string
	resultText string
	called     any
	result     any
}

func If(text string, events ...Event) entry {
	ent := entry{text: text}
	for _, apply := range events {
		apply(&ent)
	}

	return ent
}

type fork []entry

func Either(text string, entries ...entry) fork {
	for _, entry := range entries {
		entry.text = fmt.Sprintf("%s %s", text, entry.text)
	}
	return entries
}

type node interface {
	impl()
}

func (entry) impl() {}
func (fork) impl()  {}

type Event func(m *entry)

func Mock(text string, called any) Event {
	return func(m *entry) {
		if m.called != nil {
			panic("attempted If with multiple Mock")
		}
		m.called = called
		m.calledText = text
	}
}

func Then(text string, result any) Event {
	return func(m *entry) {
		m.result = result
		m.resultText = text
	}
}
