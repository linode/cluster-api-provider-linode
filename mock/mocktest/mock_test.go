package mocktest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventsWithoutRecorder(t *testing.T) {
	t.Parallel()

	mck := Mock{}
	assert.Panics(t, func() {
		mck.Events()
	})
}

func TestLogsWithoutLogger(t *testing.T) {
	t.Parallel()

	mck := Mock{}
	assert.Panics(t, func() {
		mck.Logs()
	})
}
