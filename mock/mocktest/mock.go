package mocktest

import (
	"bytes"
	"strings"

	"github.com/go-logr/logr"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/events"

	"github.com/linode/cluster-api-provider-linode/mock"
)

// Mock holds configuration for a single test path.
type Mock struct {
	gomock.TestReporter
	mock.MockClients

	recorder *events.FakeRecorder
	events   string
	logger   logr.Logger
	logs     *bytes.Buffer
}

// Recorder returns a *FakeRecorder for recording events published in a reconcile loop.
// Events can be consumed as a single string by calling Events().
func (m *Mock) Recorder() *events.FakeRecorder {
	return m.recorder
}

// Logger returns a logr.Logger for capturing logs written during a reconcile loop.
// Log output can be read as a single string by calling Logs().
func (m *Mock) Logger() logr.Logger {
	return m.logger
}

// Events returns a string of all events currently recorded during path evaluation.
func (m *Mock) Events() string {
	if m.recorder == nil {
		panic("no recorder configured on Mock")
	}

	var strBuilder strings.Builder
	for len(m.recorder.Events) > 0 {
		strBuilder.WriteString(<-m.recorder.Events)
	}

	m.events += strBuilder.String()

	return m.events
}

// Logs returns a string of all log outputs currently written during path evaluation.
func (m *Mock) Logs() string {
	if m.logs == nil {
		panic("no logger configured on Mock")
	}

	return m.logs.String()
}
