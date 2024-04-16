package mocktest

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/linode/cluster-api-provider-linode/mock"
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

func (s *suite) Run(ctx context.Context, t *testing.T, paths []path) {
	for _, path := range paths {
		t.Run(path.Describe(), func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			m := Mock{
				TestReporter: mockCtrl.T,
			}

			for _, client := range s.clients {
				m.MockClients.Build(client, mockCtrl)
			}

			path.Run(ctx, m)
		})
	}
}
