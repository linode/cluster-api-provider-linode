package mocktest

import (
	"bytes"

	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/record"

	"github.com/linode/cluster-api-provider-linode/mock"
)

// Mock holds configuration for a single test path.
type Mock struct {
	gomock.TestReporter
	mock.MockClients

	recorder *record.FakeRecorder
	logs     *bytes.Buffer
}

// Events returns a channel for receiving event strings for a single test path.
func (m Mock) Events() <-chan string {
	if m.recorder == nil {
		panic("events are only available in a ControllerTestSuite")
	}

	return m.recorder.Events
}

// Logs returns a string of all log output written during a single test path.
func (m Mock) Logs() string {
	if m.logs == nil {
		panic("logs are only available in a ControllerTestSuite")
	}

	return m.logs.String()
}
