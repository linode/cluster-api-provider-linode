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

var _ = Describe("fork", func() {
	paths := Paths(
		Mock(
			Message("list and create"),
			Calls(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().ListInstances(gomock.Any(), gomock.Any()).Return([]linodego.Instance{}, nil)
			}),
		),
		Fork(
			Mock(Message("succeeds"),
				Calls(func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
				}),
				Asserts(func(ctx context.Context, path *Path, client *mock.MockLinodeMachineClient) {
					Expect(clientCalls(ctx, client)).To(Succeed())
				}),
			),
			Mock(Message("fails with server error"),
				Calls(func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("server error"))
				}),
				Asserts(func(ctx context.Context, path *Path, client *mock.MockLinodeMachineClient) {
					Expect(clientCalls(ctx, client)).NotTo(Succeed())
				}),
			),
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
			path.Run(ctx, client)
		})
	}
})

var _ = Describe("end", func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	paths := Paths(
		Mock(
			Message("succeed"),
			Calls(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().ListInstances(gomock.Any(), gomock.Any()).Return([]linodego.Instance{}, nil)
				c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
			}),
			Asserts(func(ctx context.Context, path *Path, client *mock.MockLinodeMachineClient) {
				Expect(clientCalls(ctx, client)).To(Succeed())
			}),
		),
		Mock(
			Message("fail"),
			Calls(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().ListInstances(gomock.Any(), gomock.Any()).Return([]linodego.Instance{}, nil)
				c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("server error"))
			}),
			Asserts(func(ctx context.Context, path *Path, client *mock.MockLinodeMachineClient) {
				Expect(clientCalls(ctx, client)).NotTo(Succeed())
			}),
		),
	)

	for _, path := range paths {
		It(path.Message, func(ctx SpecContext) {
			client := mock.NewMockLinodeMachineClient(mockCtrl)
			path.Run(ctx, client)
		})
	}
})

func clientCalls(ctx context.Context, client scope.LinodeMachineClient) error {
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
