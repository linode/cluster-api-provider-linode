package mocktest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"

	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestSuitesNoClients(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { NewSuite(t) })
	assert.Panics(t, func() { NewControllerSuite(ginkgo.GinkgoT()) })
}

func TestSuite(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // these tests should run prior to their nested t.Run
	for _, testCase := range []struct {
		name                  string
		beforeEach, afterEach bool
		expectedCount         int
	}{
		{
			name:          "beforeEach",
			beforeEach:    true,
			expectedCount: 6,
		},
		{
			name:          "afterEach",
			afterEach:     true,
			expectedCount: 6,
		},
		{
			name:          "both",
			beforeEach:    true,
			afterEach:     true,
			expectedCount: 8,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// Create a counter with the expected number of calls.
			// As each call runs, the counter will decrement to 0.
			var counter sync.WaitGroup
			counter.Add(testCase.expectedCount)

			suite := NewSuite(t, mock.MockK8sClient{})
			if testCase.beforeEach {
				suite.BeforeEach(func(_ context.Context, _ Mock) { counter.Done() })
			}
			if testCase.afterEach {
				suite.AfterEach(func(_ context.Context, _ Mock) { counter.Done() })
			}

			suite.Run(Paths(
				Either(
					Call("", func(_ context.Context, _ Mock) { counter.Done() }),
					Call("", func(_ context.Context, _ Mock) { counter.Done() }),
				),
				Result("", func(_ context.Context, _ Mock) { counter.Done() }),
			))

			// Wait until the counter reaches 0, or time out.
			// This runs in a goroutine so the nested t.Runs are scheduled.
			go func() {
				select {
				case <-waitCh(&counter):
					return
				case <-time.NewTimer(time.Second * 5).C:
					assert.Error(t, errors.New(testCase.name))
				}
			}()
		})
	}
}

func waitCh(counter *sync.WaitGroup) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		counter.Wait()
		out <- struct{}{}
	}()
	return out
}
