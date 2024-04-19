package mocktest

import (
	"bytes"
	"errors"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/linode/cluster-api-provider-linode/mock"
)

const recorderBufferSize = 20

type ctlrSuite struct {
	suite

	ginkgoT  ginkgo.FullGinkgoTInterface
	recorder *record.FakeRecorder
	logger   logr.Logger
	logs     *bytes.Buffer
}

// NewControllerTestSuite creates a test suite for a controller.
// It generates new mock clients for each test path it runs.
func NewControllerTestSuite(ginkgoT ginkgo.FullGinkgoTInterface, clients ...mock.MockClient) *ctlrSuite {
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

// Run executes Ginkgo test specs for each computed test path.
// It manages mock client instantiation, events, and logging.
func (cs *ctlrSuite) Run(paths []path) {
	for _, path := range paths {
		ginkgo.It(path.Describe(), func(ctx ginkgo.SpecContext) {
			cs.suite.run(cs.ginkgoT, ctx, path, func(mck *Mock) {
				mck.recorder = cs.recorder
				mck.logs = cs.logs
			})

			// Flush the channel if any events were not consumed.
			for len(cs.recorder.Events) > 0 {
				<-cs.recorder.Events
			}

			// Flush the logs buffer for each test run
			cs.logs.Reset()
		})
	}
}
