package mocktest

import (
	"bytes"
	"strings"

	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/record"

	"github.com/linode/cluster-api-provider-linode/mock"
)

// Mock holds configuration for a single test path.
type Mock struct {
	gomock.TestReporter
	mock.MockClients

	endOfPath bool
	recorder  *record.FakeRecorder
	events    string
	logs      *bytes.Buffer
}

// Events a string of all recorded events for a single test path.
func (m *Mock) Events() string {
	if m.recorder == nil {
		panic("unexpected call to Events() outside of a ControllerTestSuite")
	}

	if !m.endOfPath {
		panic("unexpected call to Events() prior to Result node")
	}

	if m.events != "" {
		return m.events
	}

	var sb strings.Builder
	for len(m.recorder.Events) > 0 {
		sb.WriteString(<-m.recorder.Events)
	}

	m.events = sb.String()

	return m.events
}

// Logs returns a string of all log output written during a single test path.
func (m *Mock) Logs() string {
	if m.logs == nil {
		panic("unexpected call to Events() outside of a ControllerTestSuite")
	}

	if !m.endOfPath {
		panic("unexpected call to Events() prior to Result node")
	}

	return m.logs.String()
}
