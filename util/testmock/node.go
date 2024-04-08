package testmock

type entry struct {
	text       string
	called     any
	calledText string
	result     any
	resultText string
}

func If(text string, events ...Event) entry {
	m := entry{text: text}
	for _, add := range events {
		add(&m)
	}

	return m
}

type fork []entry

func Either(entries ...entry) fork {
	return entries
}

type node interface {
	impl()
}

func (entry) impl() {}
func (fork) impl()  {}

type Event func(m *entry)

func Called(text string, called any) Event {
	return func(m *entry) {
		if m.called != nil {
			panic("attempted If with multiple Called")
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
