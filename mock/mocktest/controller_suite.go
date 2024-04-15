package mocktest

import (
	"bytes"
	"errors"

	"github.com/go-logr/logr"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/onsi/ginkgo/v2"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type ctlrSuite struct {
	clients  []mock.MockClient
	recorder *record.FakeRecorder
	logger   logr.Logger
	logs     *bytes.Buffer
}

// NewControllerTestSuite creates a test suite for a controller.
// It generates new mock clients for each test path it runs.
func NewControllerTestSuite(clients ...mock.MockClient) *ctlrSuite {
	if len(clients) == 0 {
		panic(errors.New("unable to run tests without clients"))
	}

	c := ctlrSuite{
		clients: clients,
		// Create a recorder with a buffered channel for consuming event strings.
		recorder: record.NewFakeRecorder(50),
		logs:     &bytes.Buffer{},
	}

	// Create a logger that writes to both GinkgoWriter and the local logs buffer
	c.logger = zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.WriteTo(c.logs))

	return &c
}

// Recorder returns a *FakeRecorder for recording events published in a reconcile loop.
// Events can be consumed within test paths by receiving from a MockContext.Events() channel.
func (c *ctlrSuite) Recorder() *record.FakeRecorder {
	return c.recorder
}

// Logger returns a logr.Logger for capturing logs written during a reconcile loop.
// Log output can be read within test paths by calling MockContext.Logs().
func (c *ctlrSuite) Logger() logr.Logger {
	return c.logger
}

// Run executes Ginkgo test specs for each computed test path.
// It manages mock client instantiation, events, and logging.
func (c *ctlrSuite) Run(paths []path) {
	for _, path := range paths {
		ginkgo.It(path.Describe(), func(ctx ginkgo.SpecContext) {
			mockCtrl := gomock.NewController(ginkgo.GinkgoT())
			defer mockCtrl.Finish()

			mockCtx := MockContext{
				Context:      ctx,
				TestReporter: mockCtrl.T,
				recorder:     c.recorder,
				logs:         c.logs,
			}

			for _, client := range c.clients {
				mockCtx.MockClients.Build(client, mockCtrl)
			}

			path.Run(mockCtx)

			// Flush the channel if any events were not consumed.
			for len(c.recorder.Events) > 0 {
				<-c.recorder.Events
			}

			// Flush the logs buffer for each test run
			c.logs.Reset()
		})
	}
}
