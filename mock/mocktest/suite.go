package mocktest

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/linode/cluster-api-provider-linode/mock"
)

type suite struct {
	clients    []mock.MockClient
	beforeEach []fn
	afterEach  []fn
	beforeAll  []*once
	afterAll   []*once
}

func (s *suite) BeforeEach(action func(context.Context, Mock)) {
	s.beforeEach = append(s.beforeEach, fn{
		text: "BeforeEach()",
		does: action,
	})
}

func (s *suite) AfterEach(action func(context.Context, Mock)) {
	s.afterEach = append(s.afterEach, fn{
		text: "AfterEach()",
		does: action,
	})
}

func (s *suite) BeforeAll(action func(context.Context, Mock)) {
	s.beforeAll = append(s.beforeAll, &once{
		text: "BeforeAll()",
		does: action,
	})
}

func (s *suite) AfterAll(action func(context.Context, Mock)) {
	s.afterAll = append(s.afterAll, &once{
		text: "AfterAll()",
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

	for _, fun := range s.beforeAll {
		evalOnce(ctx, mck, fun)
	}
	for _, fun := range s.beforeEach {
		evalFn(ctx, mck, fun)
	}

	pth.Run(ctx, mck)

	for _, fun := range s.afterEach {
		evalFn(ctx, mck, fun)
	}
	for _, fun := range s.afterAll {
		evalOnce(ctx, mck, fun)
	}
}

type standardSuite struct {
	suite

	t *testing.T
}

func NewTestSuite(t *testing.T, clients ...mock.MockClient) *standardSuite {
	t.Helper()

	if len(clients) == 0 {
		panic(errors.New("unable to run tests without clients"))
	}

	return &standardSuite{
		suite: suite{clients: clients},
		t:     t,
	}
}

func (ss *standardSuite) Run(paths []path) {
	for _, path := range paths {
		ss.t.Run(path.Describe(), func(t *testing.T) {
			t.Parallel()

			ss.suite.run(t, context.Background(), path)
		})
	}
}
