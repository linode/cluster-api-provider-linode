package testmock

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUsage(t *testing.T) {
	t.Parallel()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = Describe("k8s client", func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	for _, path := range Paths(
		Mock("fetch object", func(c *mock.MockK8sClient) {
			c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		Result("no error", func(ctx context.Context, c *mock.MockK8sClient) {
			Expect(contrivedCalls(ctx, nil, c)).To(Succeed())
		}),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			Run(path, GinkgoT(), ctx, mock.NewMockK8sClient(mockCtrl))
		})
	}
})

var _ = Describe("multiple clients", func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	for _, path := range Paths(
		Mock("read object", func(c *mock.MockK8sClient) {
			c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		Either(
			Case(
				Mock("underlying exists", func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
				}),
				Result("no error", func(ctx context.Context, lc *mock.MockLinodeMachineClient, kc *mock.MockK8sClient) {
					Expect(contrivedCalls(ctx, lc, kc)).To(Succeed())
				}),
			),
			Case(
				Mock("underlying does not exist", func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("404"))
				}),
				Result("error", func(ctx context.Context, lc *mock.MockLinodeMachineClient, kc *mock.MockK8sClient) {
					Expect(contrivedCalls(ctx, lc, kc)).NotTo(Succeed())
				}),
			),
		),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			Run(path, GinkgoT(), ctx, mock.NewMockLinodeMachineClient(mockCtrl), mock.NewMockK8sClient(mockCtrl))
		})
	}
})

func contrivedCalls(ctx context.Context, lc scope.LinodeMachineClient, kc scope.K8sClient) error {
	GinkgoHelper()

	err := kc.Get(ctx, client.ObjectKey{}, &infrav1alpha1.LinodeMachine{})
	if err != nil {
		return err
	}

	if lc != nil {
		_, err = lc.CreateInstance(ctx, linodego.InstanceCreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func TestPaths(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    []node
		output   []path
		panicErr error
	}{
		{
			name: "basic",
			input: []node{
				call{value: 0},
				result{value: 0},
			},
			output: []path{
				{
					calls:  []call{{value: 0}},
					result: result{value: 0},
				},
			},
		},
		{
			name: "open",
			input: []node{
				call{},
			},
			panicErr: errors.New("unresolved path at index 0"),
		},
		{
			name: "open fork",
			input: []node{
				call{value: 0},
				fork{
					call{value: 1},
					leaf{call{value: 1}, result{value: 1}},
				},
			},
			panicErr: errors.New("unresolved path at index 1"),
		},
		{
			name: "split",
			input: []node{
				call{value: 0},
				fork{
					call{value: 1},
					call{value: 2},
				},
				result{value: 4},
			},
			output: []path{
				{
					calls: []call{
						{value: 0},
						{value: 1},
					},
					result: result{value: 4},
				},
				{
					calls: []call{
						{value: 0},
						{value: 2},
					},
					result: result{value: 4},
				},
			},
		},
		{
			name: "close order",
			input: []node{
				call{value: 0},
				fork{
					call{value: 1},
					leaf{call{value: 2}, result{value: 4}},
				},
				result{value: 3},
			},
			output: []path{
				{
					calls: []call{
						{value: 0},
						{value: 2},
					},
					result: result{value: 4},
				},
				{
					calls: []call{
						{value: 0},
						{value: 1},
					},
					result: result{value: 3},
				},
			},
		},
		{
			name: "path order",
			input: []node{
				fork{
					leaf{call{value: 0}, result{value: 0}},
					call{value: 1},
				},
				fork{
					leaf{call{value: 2}, result{value: 2}},
					leaf{call{value: 3}, result{value: 3}},
				},
			},
			output: []path{
				{
					calls:  []call{{value: 0}},
					result: result{value: 0},
				},
				{
					calls: []call{
						{value: 1},
						{value: 2},
					},
					result: result{value: 2},
				},
				{
					calls: []call{
						{value: 1},
						{value: 3},
					},
					result: result{value: 3},
				},
			},
		},
		{
			name: "once",
			input: []node{
				once{fn: fn{value: 0}},
				fork{
					leaf{call{value: 1}, result{value: 1}},
					call{value: 1},
				},
				fork{
					leaf{call{value: 2}, result{value: 2}},
					call{value: 2},
				},
				result{value: 3},
				once{fn: fn{value: 4}},
				fork{
					leaf{call{value: 5}, result{value: 5}},
					call{value: 5},
				},
				fork{
					leaf{call{value: 6}, result{value: 6}},
					leaf{call{value: 7}, result{value: 7}},
				},
			},
			output: []path{
				{
					once:   []*once{{fn: fn{value: 0}}},
					calls:  []call{{value: 1}},
					result: result{value: 1},
				},
				{
					once: []*once{{fn: fn{value: 0}}},
					calls: []call{
						{value: 1},
						{value: 2},
					},
					result: result{value: 2},
				},
				{
					once: []*once{{fn: fn{value: 0}}},
					calls: []call{
						{value: 1},
						{value: 2},
					},
					result: result{value: 3},
				},
				{
					once:   []*once{{fn: fn{value: 4}}},
					calls:  []call{{value: 5}},
					result: result{value: 5},
				},
				{
					once: []*once{{fn: fn{value: 4}}},
					calls: []call{
						{value: 5},
						{value: 6},
					},
					result: result{value: 6},
				},
				{
					once: []*once{{fn: fn{value: 4}}},
					calls: []call{
						{value: 5},
						{value: 7},
					},
					result: result{value: 7},
				},
			},
		},
		{
			name: "no shared state",
			input: []node{
				call{text: "mock1"},
				fork{
					leaf{call{text: "mock1.1"}, result{text: "result1"}},
					call{text: "mock2"},
				},
				call{text: "mock3"},
				fork{
					leaf{call{text: "mock3.1"}, result{text: "result2"}},
					leaf{call{text: "mock3.2"}, result{text: "result3"}},
				},
			},
			output: []path{
				{
					calls: []call{
						{text: "mock1"},
						{text: "mock1.1"},
					},
					result: result{text: "result1"},
				},
				{
					calls: []call{
						{text: "mock1"},
						{text: "mock2"},
						{text: "mock3"},
						{text: "mock3.1"},
					},
					result: result{text: "result2"},
				},
				{
					calls: []call{
						{text: "mock1"},
						{text: "mock2"},
						{text: "mock3"},
						{text: "mock3.2"},
					},
					result: result{text: "result3"},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.panicErr != nil {
				assert.PanicsWithError(t, tc.panicErr.Error(), func() {
					Paths(tc.input...)
				})
				return
			}

			actual := Paths(tc.input...)
			assert.Equal(t, tc.output, actual)
		})
	}
}
