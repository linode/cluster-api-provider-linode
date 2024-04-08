package testmock

func If(text string, events ...Event) entry {
	m := entry{text: text}
	for _, add := range events {
		add(&m)
	}

	return m
}

func Either(left, right entry) fork {
	return fork{left, right}
}

type Event func(m *entry)

func Called(called any) Event {
	return func(m *entry) {
		if m.called != nil {
			panic("attempted If with multiple Called")
		}
		m.called = called
	}
}

func Then(result any) Event {
	return func(m *entry) {
		m.result = result
	}
}

type entry struct {
	text   string
	called any
	result any
}

type fork struct {
	left  entry
	right entry
}

type node interface {
	impl()
}

func (entry) impl() {}
func (fork) impl()  {}
