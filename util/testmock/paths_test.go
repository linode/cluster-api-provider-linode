package testmock

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPaths(t *testing.T) {
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
		Case("reconcile",
			Mock("fetch object", func(c *mock.MockK8sClient) {
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}),
			Result("no error", func(ctx context.Context, c *mock.MockK8sClient) {
				Expect(contrivedCalls(ctx, nil, c)).To(Succeed())
			}),
		),
	) {
		It(path.Text, func(ctx SpecContext) {
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
		Case("reconcile",
			Mock("fetch object", func(c *mock.MockK8sClient) {
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}),
		),
		Either("create",
			Case("success",
				Mock("server 200", func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
				}),
				Result("no error", func(ctx context.Context, lc *mock.MockLinodeMachineClient, kc *mock.MockK8sClient) {
					Expect(contrivedCalls(ctx, lc, kc)).To(Succeed())
				}),
			),
			Case("error",
				Mock("server 500", func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("server error"))
				}),
				Result("error", func(ctx context.Context, lc *mock.MockLinodeMachineClient, kc *mock.MockK8sClient) {
					Expect(contrivedCalls(ctx, lc, kc)).NotTo(Succeed())
				}),
			),
		),
	) {
		It(path.Text, func(ctx SpecContext) {
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

func TestDrawPaths(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    []node
		output   [][]entry
		panicErr error
	}{
		{
			name: "entry",
			input: []node{
				entry{result: fn{value: 0}},
			},
			output: [][]entry{
				{entry{result: fn{value: 0}}},
			},
		},
		{
			name: "open entry",
			input: []node{
				entry{},
			},
			panicErr: errors.New("unresolved path at index 0"),
		},
		{
			name: "fork",
			input: []node{
				fork{
					entries: []entry{
						{result: fn{value: 0}},
						{result: fn{value: 1}},
						{result: fn{value: 2}},
					},
				},
			},
			output: [][]entry{
				{{result: fn{value: 0}}},
				{{result: fn{value: 1}}},
				{{result: fn{value: 2}}},
			},
		},
		{
			name: "open fork",
			input: []node{
				fork{
					entries: []entry{
						{result: fn{value: 0}},
						{calls: []fn{{value: 1}}},
					},
				},
			},
			panicErr: errors.New("unresolved path at index 0"),
		},
		{
			name: "split",
			input: []node{
				entry{calls: []fn{{value: 0}}},
				fork{
					entries: []entry{
						{calls: []fn{{value: 1}}},
						{calls: []fn{{value: 2}}},
						{calls: []fn{{value: 3}}},
					},
				},
				entry{result: fn{value: 4}},
			},
			output: [][]entry{
				{
					entry{calls: []fn{{value: 0}}},
					entry{calls: []fn{{value: 1}}},
					entry{result: fn{value: 4}},
				},
				{
					entry{calls: []fn{{value: 0}}},
					entry{calls: []fn{{value: 2}}},
					entry{result: fn{value: 4}},
				},
				{
					entry{calls: []fn{{value: 0}}},
					entry{calls: []fn{{value: 3}}},
					entry{result: fn{value: 4}},
				},
			},
		},
		{
			name: "partial early closed fork",
			input: []node{
				entry{calls: []fn{{value: 0}}},
				fork{
					entries: []entry{
						{calls: []fn{{value: 1}}},
						{calls: []fn{{value: 2}}, result: fn{value: 2}},
					},
				},
				entry{result: fn{value: 3}},
			},
			output: [][]entry{
				{
					{calls: []fn{{value: 0}}},
					{calls: []fn{{value: 1}}},
					{result: fn{value: 3}},
				},
				{
					{calls: []fn{{value: 0}}},
					{calls: []fn{{value: 2}}, result: fn{value: 2}},
				},
			},
		},
		{
			name: "ordering",
			input: []node{
				fork{
					entries: []entry{
						{calls: []fn{{value: 0}}, result: fn{value: 0}},
						{calls: []fn{{value: 1}}},
					},
				},
				fork{
					entries: []entry{
						{calls: []fn{{value: 2}}, result: fn{value: 2}},
						{calls: []fn{{value: 3}}, result: fn{value: 3}},
					},
				},
			},
			output: [][]entry{
				{
					{calls: []fn{{value: 0}}, result: fn{value: 0}},
				},
				{
					{calls: []fn{{value: 1}}},
					{calls: []fn{{value: 2}}, result: fn{value: 2}},
				},
				{
					{calls: []fn{{value: 1}}},
					{calls: []fn{{value: 3}}, result: fn{value: 3}},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.panicErr != nil {
				assert.PanicsWithError(t, tc.panicErr.Error(), func() {
					drawPaths(tc.input)
				})
				return
			}

			actual := drawPaths(tc.input)
			require.Len(t, actual, len(tc.output))
			assert.Equal(t, tc.output, actual)
		})
	}
}
