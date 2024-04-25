package mocktest

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/linode/cluster-api-provider-linode/mock"
)

type suite struct {
	clients    []mock.MockClient
	beforeEach []fn
	afterEach  []fn
}

func (s *suite) BeforeEach(action func(context.Context, Mock)) {
	s.beforeEach = append(s.beforeEach, fn{
		text: "BeforeEach",
		does: action,
	})
}

func (s *suite) AfterEach(action func(context.Context, Mock)) {
	s.afterEach = append(s.afterEach, fn{
		text: "AfterEach",
		does: action,
	})
}

type mockOpt func(*Mock)

func (s *suite) run(t gomock.TestHelper, ctx context.Context, pth path, mockOpts ...mockOpt) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mck := Mock{
		TestReporter: t,
	}

	for _, client := range s.clients {
		mck.MockClients.Build(client, mockCtrl)
	}

	for _, opt := range mockOpts {
		opt(&mck)
	}

	for _, fun := range s.beforeEach {
		evalFn(ctx, mck, fun)
	}

	pth.run(ctx, mck)

	for _, fun := range s.afterEach {
		evalFn(ctx, mck, fun)
	}

	// If a recorder is configured and events were not consumed, flush the channel
	if mck.recorder != nil {
		for len(mck.recorder.Events) > 0 {
			<-mck.recorder.Events
		}
	}
}

type standardSuite struct {
	suite

	t *testing.T
}

// NewSuite creates a test suite using Go's standard testing library.
// It generates new mock clients for each test path it runs.
func NewSuite(t *testing.T, clients ...mock.MockClient) *standardSuite {
	t.Helper()

	if len(clients) == 0 {
		panic(errors.New("unable to run tests without clients"))
	}

	return &standardSuite{
		suite: suite{clients: clients},
		t:     t,
	}
}

// Run calls t.Run for each computed test path.
func (ss *standardSuite) Run(nodes ...node) {
	pths := mkPaths(nodes...)

	for _, pth := range pths {
		ss.t.Run(pth.describe(), func(t *testing.T) {
			t.Parallel()

			ss.suite.run(t, context.Background(), pth)
		})
	}
}

const recorderBufferSize = 20

type ctlrSuite struct {
	suite

	ginkgoT ginkgo.FullGinkgoTInterface
}

// NewControllerSuite creates a test suite for a controller.
// It generates new mock clients for each test path it runs.
func NewControllerSuite(ginkgoT ginkgo.FullGinkgoTInterface, clients ...mock.MockClient) *ctlrSuite {
	if len(clients) == 0 {
		panic(errors.New("unable to run tests without clients"))
	}

	return &ctlrSuite{
		suite:   suite{clients: clients},
		ginkgoT: ginkgoT,
	}
}

// Run executes Ginkgo test specs for each computed test path.
// It manages mock client instantiation, events, and logging.
func (cs *ctlrSuite) Run(nodes ...node) {
	pths := mkPaths(nodes...)

	for _, pth := range pths {
		ginkgo.It(pth.describe(), func(ctx ginkgo.SpecContext) {
			cs.suite.run(cs.ginkgoT, ctx, pth, func(mck *Mock) {
				// Create a recorder with a buffered channel for consuming event strings.
				mck.recorder = record.NewFakeRecorder(recorderBufferSize)
				// Create a logger that writes to both GinkgoWriter and the local logs buffer
				mck.logs = &bytes.Buffer{}
				mck.logger = zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.WriteTo(mck.logs))
			})
		})
	}
}
