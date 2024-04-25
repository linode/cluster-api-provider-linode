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

	for _, pth := range mkPaths(
		Once("setup", func(_ context.Context, _ Mock) {}),
		Call("fetch object", func(ctx context.Context, mck Mock) {
			mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		Result("no error", func(ctx context.Context, mck Mock) {
			Expect(contrivedCalls(ctx, mck)).To(Succeed())
		}),
	) {
		It(pth.describe(), func(ctx SpecContext) {
			pth.run(ctx, Mock{
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

	for _, pth := range mkPaths(
		Call("read object", func(ctx context.Context, mck Mock) {
			mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}),
		OneOf(
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
		It(pth.describe(), func(ctx SpecContext) {
			pth.run(ctx, Mock{
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
			name:     "empty",
			input:    []node{},
			output:   paths{},
			describe: []string{},
		},
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
					allOf{call{text: "1"}},
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
					allOf{call{text: "1"}},
					allOf{call{text: "2"}},
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
					allOf{oneOf{
						allOf{call{text: "0"}},
						allOf{oneOf{
							allOf{call{text: "1"}},
							allOf{call{text: "2"}},
						}},
					}},
					allOf{oneOf{
						allOf{call{text: "3"}},
						allOf{oneOf{
							allOf{call{text: "4"}},
							allOf{call{text: "5"}},
						}},
					}},
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
					allOf{call{text: "1"}},
					allOf{result{text: "2"}},
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
					allOf{call{text: "1"}},
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
					allOf{call{text: "1"}},
				},
				oneOf{
					allOf{call{text: "2"}, result{text: "2"}},
					allOf{call{text: "2"}},
				},
				result{text: "3"},
				once{text: "4"},
				oneOf{
					allOf{call{text: "5"}, result{text: "5"}},
					allOf{call{text: "5"}},
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
					allOf{call{text: "mock2"}},
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
							allOf{call{text: "able to be created"}},
							allOf{
								call{text: "not able to be created"},
								result{text: "error"},
							},
						},
					},
					allOf{call{text: "instance exists but is offline"}},
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
					mkPaths(testCase.input...)
				})
				return
			}

			actual := mkPaths(testCase.input...)
			assert.Equal(t, testCase.output, actual)
			assert.Equal(t, testCase.describe, actual.describe())
			assert.Equal(t, DescribePaths(testCase.input...), actual.describe())
		})
	}
}

func TestRunWithoutTestReporter(t *testing.T) {
	t.Parallel()

	pth := path{}
	assert.Panics(t, func() {
		pth.run(context.Background(), Mock{})
	})
}

func TestEvalOnceOnlyCallsOnce(t *testing.T) {
	t.Parallel()

	var toggle bool

	onceFn := once{does: func(_ context.Context, _ Mock) {
		toggle = !toggle
	}}

	ctx := context.Background()
	mck := Mock{}
	evalOnce(ctx, mck, &onceFn)
	evalOnce(ctx, mck, &onceFn)

	assert.True(t, toggle)
}
