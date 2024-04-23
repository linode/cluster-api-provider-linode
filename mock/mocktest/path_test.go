package mocktest

import (
	"context"
	"errors"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		Call("fetch object", func(ctx context.Context, mck Mock) {
			mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		Result("no error", func(ctx context.Context, mck Mock) {
			Expect(contrivedCalls(ctx, mck)).To(Succeed())
		}),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			path.Run(ctx, Mock{
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
		Call("read object", func(ctx context.Context, mck Mock) {
			mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		Either(
			Path(
				Call("underlying exists", func(ctx context.Context, mck Mock) {
					mck.MachineClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
				}),
				Result("no error", func(ctx context.Context, mck Mock) {
					Expect(contrivedCalls(ctx, mck)).To(Succeed())
				}),
			),
			Path(
				Call("underlying does not exist", func(ctx context.Context, mck Mock) {
					mck.MachineClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("404"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					Expect(contrivedCalls(ctx, mck)).NotTo(Succeed())
				}),
			),
		),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			path.Run(ctx, Mock{
				TestReporter: GinkgoT(),
				MockClients: mock.MockClients{
					MachineClient: mock.NewMockLinodeMachineClient(mockCtrl),
					K8sClient:     mock.NewMockK8sClient(mockCtrl),
				},
			})
		})
	}
})

func contrivedCalls(ctx context.Context, mck Mock) error {
	GinkgoHelper()

	err := mck.K8sClient.Get(ctx, client.ObjectKey{}, &infrav1alpha1.LinodeMachine{})
	if err != nil {
		return err
	}

	if mck.MachineClient != nil {
		_, err = mck.MachineClient.CreateInstance(ctx, linodego.InstanceCreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func TestPaths(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name     string
		input    []node
		output   paths
		describe []string
		panic    bool
	}{
		{
			name: "basic",
			input: []node{
				call{text: "0"},
				result{text: "0"},
			},
			output: paths{
				{
					calls:  []call{{text: "0"}},
					result: result{text: "0"},
				},
			},
			describe: []string{
				"0 > 0",
			},
		},
		{
			name: "open",
			input: []node{
				call{text: "0"},
			},
			panic: true,
		},
		{
			name: "open fork",
			input: []node{
				call{text: "0"},
				oneOf{
					call{text: "1"},
					allOf{call{text: "1"}, result{text: "1"}},
				},
			},
			panic: true,
		},
		{
			name: "split",
			input: []node{
				call{text: "0"},
				oneOf{
					call{text: "1"},
					call{text: "2"},
				},
				result{text: "4"},
			},
			output: paths{
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
			describe: []string{
				"0 > 1 > 4",
				"0 > 2 > 4",
			},
		},
		{
			name: "recursive",
			input: []node{
				oneOf{
					oneOf{
						call{text: "0"},
						oneOf{
							call{text: "1"},
							call{text: "2"},
						},
					},
					oneOf{
						call{text: "3"},
						oneOf{
							call{text: "4"},
							call{text: "5"},
						},
					},
				},
				result{text: "6"},
			},
			output: paths{
				{
					calls: []call{
						{text: "0"},
					},
					result: result{text: "6"},
				},
				{
					calls: []call{
						{text: "1"},
					},
					result: result{text: "6"},
				},
				{
					calls: []call{
						{text: "2"},
					},
					result: result{text: "6"},
				},
				{
					calls: []call{
						{text: "3"},
					},
					result: result{text: "6"},
				},
				{
					calls: []call{
						{text: "4"},
					},
					result: result{text: "6"},
				},
				{
					calls: []call{
						{text: "5"},
					},
					result: result{text: "6"},
				},
			},
			describe: []string{
				"0 > 6",
				"1 > 6",
				"2 > 6",
				"3 > 6",
				"4 > 6",
				"5 > 6",
			},
		},
		{
			name: "close order",
			input: []node{
				call{text: "0"},
				oneOf{
					call{text: "1"},
					result{text: "2"},
				},
				result{text: "3"},
			},
			output: paths{
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
			describe: []string{
				"0 > 2",
				"0 > 1 > 3",
			},
		},
		{
			name: "path order",
			input: []node{
				oneOf{
					allOf{call{text: "0"}, result{text: "0"}},
					call{text: "1"},
				},
				oneOf{
					allOf{call{text: "2"}, result{text: "2"}},
					allOf{call{text: "3"}, result{text: "3"}},
				},
			},
			output: paths{
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
			describe: []string{
				"0 > 0",
				"1 > 2 > 2",
				"1 > 3 > 3",
			},
		},
		{
			name: "once",
			input: []node{
				once{text: "0"},
				oneOf{
					allOf{call{text: "1"}, result{text: "1"}},
					call{text: "1"},
				},
				oneOf{
					allOf{call{text: "2"}, result{text: "2"}},
					call{text: "2"},
				},
				result{text: "3"},
				once{text: "4"},
				oneOf{
					allOf{call{text: "5"}, result{text: "5"}},
					call{text: "5"},
				},
				oneOf{
					allOf{call{text: "6"}, result{text: "6"}},
					allOf{call{text: "7"}, result{text: "7"}},
				},
			},
			output: paths{
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
			describe: []string{
				"0 > 1 > 1",
				"1 > 2 > 2",
				"1 > 2 > 3",
				"4 > 5 > 5",
				"5 > 6 > 6",
				"5 > 7 > 7",
			},
		},
		{
			name: "no shared state",
			input: []node{
				call{text: "mock1"},
				oneOf{
					allOf{call{text: "mock1.1"}, result{text: "result1"}},
					call{text: "mock2"},
				},
				call{text: "mock3"},
				oneOf{
					allOf{call{text: "mock3.1"}, result{text: "result2"}},
					allOf{call{text: "mock3.2"}, result{text: "result3"}},
				},
			},
			output: paths{
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
			describe: []string{
				"mock1 > mock1.1 > result1",
				"mock1 > mock2 > mock3 > mock3.1 > result2",
				"mock1 > mock2 > mock3 > mock3.2 > result3",
			},
		},
		{
			name: "docs",
			input: []node{
				oneOf{
					allOf{
						call{text: "instance exists and is not offline"},
						result{text: "success"},
					},
					allOf{
						call{text: "instance does not exist"},
						oneOf{
							call{text: "able to be created"},
							allOf{
								call{text: "not able to be created"},
								result{text: "error"},
							},
						},
					},
					call{text: "instance exists but is offline"},
				},
				oneOf{
					allOf{
						call{text: "able to boot"},
						result{text: "success"},
					},
					allOf{
						call{text: "not able to boot"},
						result{text: "error"},
					},
				},
			},
			output: paths{
				{
					calls:  []call{{text: "instance exists and is not offline"}},
					result: result{text: "success"},
				},
				{
					calls: []call{
						{text: "instance does not exist"},
						{text: "not able to be created"},
					},
					result: result{text: "error"},
				},
				{
					calls: []call{
						{text: "instance does not exist"},
						{text: "able to be created"},
						{text: "able to boot"},
					},
					result: result{text: "success"},
				},
				{
					calls: []call{
						{text: "instance does not exist"},
						{text: "able to be created"},
						{text: "not able to boot"},
					},
					result: result{text: "error"},
				},
				{
					calls: []call{
						{text: "instance exists but is offline"},
						{text: "able to boot"},
					},
					result: result{text: "success"},
				},
				{
					calls: []call{
						{text: "instance exists but is offline"},
						{text: "not able to boot"},
					},
					result: result{text: "error"},
				},
			},
			describe: []string{
				"instance exists and is not offline > success",
				"instance does not exist > not able to be created > error",
				"instance does not exist > able to be created > able to boot > success",
				"instance does not exist > able to be created > not able to boot > error",
				"instance exists but is offline > able to boot > success",
				"instance exists but is offline > not able to boot > error",
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if testCase.panic {
				assert.Panics(t, func() {
					Paths(testCase.input...)
				})
				return
			}

			actual := Paths(testCase.input...)
			assert.Equal(t, testCase.output, actual)
			assert.Equal(t, testCase.describe, actual.Describe())
		})
	}
}
