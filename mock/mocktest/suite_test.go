package mocktest

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSuitesNoClients(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { NewSuite(t) })
	assert.Panics(t, func() { NewControllerSuite(GinkgoT()) })
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

			suite.Run(
				OneOf(
					Path(Call("", func(_ context.Context, _ Mock) { counter.Done() })),
					Path(Call("", func(_ context.Context, _ Mock) { counter.Done() })),
				),
				Result("", func(_ context.Context, _ Mock) { counter.Done() }),
			)

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

var _ = Describe("controller suite", Label("suite"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockK8sClient{})

	suite.Run(
		Call("call", func(ctx context.Context, mck Mock) {
			mck.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(nil)
		}),
		Result("result", func(ctx context.Context, mck Mock) {
			mck.recorder.Eventf(nil, nil, "event", "reason", "action", "message")
			err := mck.K8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "myobj"}, nil)
			Expect(err).NotTo(HaveOccurred())
		}),
	)
})

var _ = Describe("controller suite with events/logs", Label("suite"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockK8sClient{})

	suite.Run(
		OneOf(
			Path(Call("call1", func(_ context.Context, mck Mock) {
				mck.Recorder().Eventf(nil, nil, "", "", "", "+")
				mck.Logger().Info("+")
			})),
			Path(Call("call2", func(_ context.Context, mck Mock) {
				mck.Recorder().Eventf(nil, nil, "", "", "", "+")
				mck.Logger().Info("+")
			})),
		),
		Result("result", func(_ context.Context, mck Mock) {
			mck.Recorder().Eventf(nil, nil, "", "", "", "+")
			mck.Logger().Info("+")

			Expect(strings.Count(mck.Events(), "+")).To(Equal(2))
		}),
	)
})
