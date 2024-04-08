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

func TestMock(t *testing.T) {
	t.Parallel()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = Describe("mock", func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	paths := Paths(
		If("reconcile",
			Mock("fetch object", func(c *mock.MockK8sClient) {
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}),
		),
		Either("create",
			If("success",
				Mock("server 200", func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
				}),
				Then("no error", func(ctx context.Context, client *mock.MockLinodeMachineClient, kClient *mock.MockK8sClient) {
					Expect(contrivedCalls(ctx, client, kClient)).To(Succeed())
				}),
			),
			If("error",
				Mock("server 500", func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("server error"))
				}),
				Then("error", func(ctx context.Context, client *mock.MockLinodeMachineClient, kClient *mock.MockK8sClient) {
					Expect(contrivedCalls(ctx, client, kClient)).NotTo(Succeed())
				}),
			),
		),
	)

	for _, path := range paths {
		It(path.Text, func(ctx SpecContext) {
			path.Run(ctx, mock.NewMockLinodeMachineClient(mockCtrl), mock.NewMockK8sClient(mockCtrl))
		})
	}
})

func TestPaths(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    []node
		output   [][]entry
		panicErr error
	}{
		{
			name: "entry",
			input: []node{
				entry{result: true},
			},
			output: [][]entry{
				{entry{result: true}},
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
					entry{result: true},
					entry{result: true},
					entry{result: true},
				},
			},
			output: [][]entry{
				{entry{result: true}},
				{entry{result: true}},
				{entry{result: true}},
			},
		},
		{
			name: "open fork",
			input: []node{
				fork{
					entry{result: true},
					entry{},
				},
			},
			panicErr: errors.New("unresolved path at index 0"),
		},
		{
			name: "split",
			input: []node{
				entry{called: 0},
				fork{
					entry{called: 1},
					entry{called: 2},
					entry{called: 3},
				},
				entry{result: true},
			},
			output: [][]entry{
				{
					entry{called: 0},
					entry{called: 1},
					entry{result: true},
				},
				{
					entry{called: 0},
					entry{called: 2},
					entry{result: true},
				},
				{
					entry{called: 0},
					entry{called: 3},
					entry{result: true},
				},
			},
		},
		{
			name: "partial early closed fork",
			input: []node{
				entry{called: 0},
				fork{
					entry{called: 1},
					entry{called: 2, result: true},
				},
				entry{result: true},
			},
			output: [][]entry{
				{
					entry{called: 0},
					entry{called: 1},
					entry{result: true},
				},
				{
					entry{called: 0},
					entry{called: 2, result: true},
				},
			},
		},
		{
			name: "ordering",
			input: []node{
				fork{
					entry{called: 0, result: true},
					entry{called: 1},
				},
				fork{
					entry{called: 2, result: true},
					entry{called: 3, result: true},
				},
			},
			output: [][]entry{
				{
					{called: 0, result: true},
				},
				{
					{called: 1},
					{called: 2, result: true},
				},
				{
					{called: 1},
					{called: 3, result: true},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.panicErr != nil {
				assert.PanicsWithError(t, tc.panicErr.Error(), func() {
					paths(tc.input)
				})
				return
			}

			actual := paths(tc.input)
			require.Len(t, actual, len(tc.output))
			assert.Equal(t, tc.output, actual)
		})
	}
}

func contrivedCalls(ctx context.Context, lc scope.LinodeMachineClient, kc scope.K8sClient) error {
	GinkgoHelper()

	err := kc.Get(ctx, client.ObjectKey{}, &infrav1alpha1.LinodeMachine{})
	if err != nil {
		return err
	}

	_, err = lc.CreateInstance(ctx, linodego.InstanceCreateOptions{})
	if err != nil {
		return err
	}

	return nil
}
