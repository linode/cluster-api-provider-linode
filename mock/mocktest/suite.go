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

func (s *suite) Run(t *testing.T, paths []path) {
	t.Helper()

	for _, path := range paths {
		t.Run(path.Describe(), func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mck := Mock{
				TestReporter: mockCtrl.T,
			}

			for _, client := range s.clients {
				mck.MockClients.Build(client, mockCtrl)
			}

			path.Run(context.Background(), mck)
		})
	}
}
