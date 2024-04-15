package mocktest

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUsage(t *testing.T) {
	t.Parallel()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = Describe("k8s client", Label("k8sclient"), func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	for _, path := range Paths(
		Mock("fetch object", func(ctx MockContext) {
			ctx.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		Result("no error", func(ctx MockContext) {
			Expect(contrivedCalls(ctx)).To(Succeed())
		}),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			path.Run(MockContext{
				Context:      ctx,
				TestReporter: GinkgoT(),
				MockClients: mock.MockClients{
					K8sClient: mock.NewMockK8sClient(mockCtrl),
				},
			})
		})
	}
})

var _ = Describe("multiple clients", Label("multiple"), func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	for _, path := range Paths(
		Mock("read object", func(ctx MockContext) {
			ctx.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		Either(
			Case(
				Mock("underlying exists", func(ctx MockContext) {
					ctx.MachineClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
				}),
				Result("no error", func(ctx MockContext) {
					Expect(contrivedCalls(ctx)).To(Succeed())
				}),
			),
			Case(
				Mock("underlying does not exist", func(ctx MockContext) {
					ctx.MachineClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("404"))
				}),
				Result("error", func(ctx MockContext) {
					Expect(contrivedCalls(ctx)).NotTo(Succeed())
				}),
			),
		),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			path.Run(MockContext{
				Context:      ctx,
				TestReporter: GinkgoT(),
				MockClients: mock.MockClients{
					MachineClient: mock.NewMockLinodeMachineClient(mockCtrl),
					K8sClient:     mock.NewMockK8sClient(mockCtrl),
				},
			})
		})
	}
})

func contrivedCalls(ctx MockContext) error {
	GinkgoHelper()

	err := ctx.K8sClient.Get(ctx, client.ObjectKey{}, &infrav1alpha1.LinodeMachine{})
	if err != nil {
		return err
	}

	if ctx.MachineClient != nil {
		_, err = ctx.MachineClient.CreateInstance(ctx, linodego.InstanceCreateOptions{})
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
				call{text: "0"},
				result{text: "0"},
			},
			output: []path{
				{
					calls:  []call{{text: "0"}},
					result: result{text: "0"},
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
				call{text: "0"},
				fork{
					call{text: "1"},
					leaf{call{text: "1"}, result{text: "1"}},
				},
			},
			panicErr: errors.New("unresolved path at index 1"),
		},
		{
			name: "split",
			input: []node{
				call{text: "0"},
				fork{
					call{text: "1"},
					call{text: "2"},
				},
				result{text: "4"},
			},
			output: []path{
				{
					calls: []call{
						{text: "0"},
						{text: "1"},
					},
					result: result{text: "4"},
				},
				{
					calls: []call{
						{text: "0"},
						{text: "2"},
					},
					result: result{text: "4"},
				},
			},
		},
		{
			name: "close order",
			input: []node{
				call{text: "0"},
				fork{
					call{text: "1"},
					result{text: "2"},
				},
				result{text: "3"},
			},
			output: []path{
				{
					calls: []call{
						{text: "0"},
					},
					result: result{text: "2"},
				},
				{
					calls: []call{
						{text: "0"},
						{text: "1"},
					},
					result: result{text: "3"},
				},
			},
		},
		{
			name: "path order",
			input: []node{
				fork{
					leaf{call{text: "0"}, result{text: "0"}},
					call{text: "1"},
				},
				fork{
					leaf{call{text: "2"}, result{text: "2"}},
					leaf{call{text: "3"}, result{text: "3"}},
				},
			},
			output: []path{
				{
					calls:  []call{{text: "0"}},
					result: result{text: "0"},
				},
				{
					calls: []call{
						{text: "1"},
						{text: "2"},
					},
					result: result{text: "2"},
				},
				{
					calls: []call{
						{text: "1"},
						{text: "3"},
					},
					result: result{text: "3"},
				},
			},
		},
		{
			name: "once",
			input: []node{
				once{text: "0"},
				fork{
					leaf{call{text: "1"}, result{text: "1"}},
					call{text: "1"},
				},
				fork{
					leaf{call{text: "2"}, result{text: "2"}},
					call{text: "2"},
				},
				result{text: "3"},
				once{text: "4"},
				fork{
					leaf{call{text: "5"}, result{text: "5"}},
					call{text: "5"},
				},
				fork{
					leaf{call{text: "6"}, result{text: "6"}},
					leaf{call{text: "7"}, result{text: "7"}},
				},
			},
			output: []path{
				{
					once:   []*once{{text: "0"}},
					calls:  []call{{text: "1"}},
					result: result{text: "1"},
				},
				{
					once: []*once{{text: "0"}},
					calls: []call{
						{text: "1"},
						{text: "2"},
					},
					result: result{text: "2"},
				},
				{
					once: []*once{{text: "0"}},
					calls: []call{
						{text: "1"},
						{text: "2"},
					},
					result: result{text: "3"},
				},
				{
					once:   []*once{{text: "4"}},
					calls:  []call{{text: "5"}},
					result: result{text: "5"},
				},
				{
					once: []*once{{text: "4"}},
					calls: []call{
						{text: "5"},
						{text: "6"},
					},
					result: result{text: "6"},
				},
				{
					once: []*once{{text: "4"}},
					calls: []call{
						{text: "5"},
						{text: "7"},
					},
					result: result{text: "7"},
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
