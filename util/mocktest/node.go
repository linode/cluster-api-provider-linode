package mocktest

func Mock(opts ...MockerOption) mocker {
	m := mocker{}
	for _, apply := range opts {
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

type MockerOption func(m *mocker)

func Message(msg string) MockerOption {
	return func(m *mocker) {
		m.msg = msg
	}
}

func Calls(calls any) MockerOption {
	return func(m *mocker) {
		if m.call != nil {
			panic("attempted Mock with multiple Calls")
		}
		m.call = calls
	}
}

func End() MockerOption {
	return func(m *mocker) {
		m.end = true
	}
}

type mocker struct {
	msg  string
	call any
	fail bool
	end  bool
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
