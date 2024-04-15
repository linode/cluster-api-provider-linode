package mocktest

import (
	"context"
	"errors"
	"testing"

	"github.com/linode/cluster-api-provider-linode/mock"
	"go.uber.org/mock/gomock"
)

type suite struct {
	clients []mock.MockClient
}

func NewTestSuite(clients ...mock.MockClient) *suite {
	if len(clients) == 0 {
		panic(errors.New("unable to run tests without clients"))
	}

	return &suite{clients: clients}
}

func (s *suite) Run(t *testing.T, paths []path) {
	t.Parallel()

	for _, path := range paths {
		t.Run(path.Describe(), func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockCtx := MockContext{
				Context:      context.Background(),
				TestReporter: mockCtrl.T,
			}

			for _, client := range s.clients {
				mockCtx.MockClients.Build(client, mockCtrl)
			}

			path.Run(mockCtx)
		})
	}
}
