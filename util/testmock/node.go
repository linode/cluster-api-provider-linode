package testmock

type entry struct {
	text   string
	calls  []entryCall
	result entryCall
}

type entryCall struct {
	text  string
	value any
}

type action func(m *entry)

func Case(text string, actions ...action) entry {
	ent := entry{text: text}
	for _, do := range actions {
		do(&ent)
	}

	return ent
}

type fork struct {
	text    string
	entries []entry
}

func Either(text string, entries ...entry) fork {
	return fork{
		text:    text,
		entries: entries,
	}
}

type node interface {
	impl()
}

func (entry) impl() {}
func (fork) impl()  {}

func Mock(text string, call any) action {
	return func(m *entry) {
		m.calls = append(m.calls, entryCall{text, call})
	}
}

func Result(text string, result any) action {
	return func(m *entry) {
		if m.result.value != nil {
			panic("attempted Case with multiple Result")
		}
		m.result.text = text
		m.result.value = result
	}
}
