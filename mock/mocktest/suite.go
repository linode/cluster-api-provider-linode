package mocktest

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-logr/logr"
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

func (s *suite) run(t gomock.TestHelper, ctx context.Context, pth path) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mck := Mock{
		TestReporter: t,
	}

	for _, client := range s.clients {
		mck.MockClients.Build(client, mockCtrl)
	}

	for _, fun := range s.beforeEach {
		evalFn(ctx, mck, fun)
	}

	pth.Run(ctx, mck)

	for _, fun := range s.afterEach {
		evalFn(ctx, mck, fun)
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
func (ss *standardSuite) Run(pths []path) {
	for _, pth := range pths {
		ss.t.Run(pth.Describe(), func(t *testing.T) {
			t.Parallel()

			ss.suite.run(t, context.Background(), pth)
		})
	}
}

const recorderBufferSize = 20

type ctlrSuite struct {
	suite

	ginkgoT  ginkgo.FullGinkgoTInterface
	recorder *record.FakeRecorder
	events   string
	logger   logr.Logger
	logs     *bytes.Buffer
}

// NewControllerSuite creates a test suite for a controller.
// It generates new mock clients for each test path it runs.
func NewControllerSuite(ginkgoT ginkgo.FullGinkgoTInterface, clients ...mock.MockClient) *ctlrSuite {
	if len(clients) == 0 {
		panic(errors.New("unable to run tests without clients"))
	}

	logs := bytes.Buffer{}

	return &ctlrSuite{
		suite:   suite{clients: clients},
		ginkgoT: ginkgoT,
		// Create a recorder with a buffered channel for consuming event strings.
		recorder: record.NewFakeRecorder(recorderBufferSize),
		// Create a logger that writes to both GinkgoWriter and the local logs buffer
		logger: zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.WriteTo(&logs)),
		logs:   &logs,
	}
}

// Recorder returns a *FakeRecorder for recording events published in a reconcile loop.
// Events can be consumed within test paths by receiving from a MockContext.Events() channel.
func (cs *ctlrSuite) Recorder() *record.FakeRecorder {
	return cs.recorder
}

// Logger returns a logr.Logger for capturing logs written during a reconcile loop.
// Log output can be read within test paths by calling MockContext.Logs().
func (cs *ctlrSuite) Logger() logr.Logger {
	return cs.logger
}

// Events returns a string of all recorded events for a single test path.
func (cs *ctlrSuite) Events() string {
	var strBuilder strings.Builder
	for len(cs.recorder.Events) > 0 {
		strBuilder.WriteString(<-cs.recorder.Events)
	}

	cs.events += strBuilder.String()

	return cs.events
}

// Logs returns a string of all log output written during a single test path.
func (cs *ctlrSuite) Logs() string {
	return cs.logs.String()
}

// Run executes Ginkgo test specs for each computed test path.
// It manages mock client instantiation, events, and logging.
func (cs *ctlrSuite) Run(pths []path) {
	for _, pth := range pths {
		ginkgo.It(pth.Describe(), func(ctx ginkgo.SpecContext) {
			cs.suite.run(cs.ginkgoT, ctx, pth)

			// Flush the channel if any events were not consumed
			// i.e. Events was never called
			for len(cs.recorder.Events) > 0 {
				<-cs.recorder.Events
			}

			// Flush the logs buffer
			cs.logs.Reset()
		})
	}
}
