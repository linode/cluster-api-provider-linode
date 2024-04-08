package mocktest

func Mock(options ...Option) mocker {
	m := mocker{}
	for _, apply := range options {
		apply(&m)
	}

	if m.call == nil {
		panic("attempted Mock with no Calls")
	}

	return m
}

func Fork(pass, fail mocker) fork {
	return fork{pass, fail}
}

type Option func(m *mocker)

func Message(message string) Option {
	return func(m *mocker) {
		m.message = message
	}
}

func Calls(calls any) Option {
	return func(m *mocker) {
		if m.call != nil {
			panic("attempted Mock with multiple Calls")
		}
		m.call = calls
	}
}

func Asserts(asserts any) Option {
	return func(m *mocker) {
		m.asserts = asserts
	}
}

type mocker struct {
	message string
	call    any
	fail    bool
	asserts any
}

type fork struct {
	pass mocker
	fail mocker
}

type node interface {
	impl()
}

func (mocker) impl() {}
func (fork) impl()   {}
