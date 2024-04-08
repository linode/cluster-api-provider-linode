package mocktest

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMocktest(t *testing.T) {
	t.Parallel()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = Describe("uses each client per path", func() {
	paths := Paths("myFunc",
		Mock(
			Message("creates if needed"),
			Calls(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().ListInstances(gomock.Any(), gomock.Any()).Return([]linodego.Instance{}, nil)
			}),
		),
		Fork(
			Mock(Calls(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
			})),
			Mock(Message("fails if server unavailable"), Calls(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("server unavailable"))
			})),
		),
	)

	var mockCtrl *gomock.Controller
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	for _, path := range paths {
		It(path.Message, func(ctx SpecContext) {
			client := mock.NewMockLinodeMachineClient(mockCtrl)
			path.Run(client)

			err := helper(ctx, client)
			if path.Fail {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
})

func helper(ctx context.Context, client scope.LinodeMachineClient) error {
	GinkgoHelper()

	_, err := client.ListInstances(ctx, &linodego.ListOptions{})
	if err != nil {
		return err
	}

	_, err = client.CreateInstance(ctx, linodego.InstanceCreateOptions{})
	if err != nil {
		return err
	}

	return nil
}
