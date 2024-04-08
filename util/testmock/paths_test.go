package testmock

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMock(t *testing.T) {
	t.Parallel()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = Describe("seq", func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	paths := Paths(
		If("succeed",
			Called(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().ListInstances(gomock.Any(), gomock.Any()).Return([]linodego.Instance{}, nil)
				c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
			}),
			Then(func(ctx context.Context, client *mock.MockLinodeMachineClient) {
				Expect(clientCalls(ctx, client)).To(Succeed())
			}),
		),
		If("fail",
			Called(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().ListInstances(gomock.Any(), gomock.Any()).Return([]linodego.Instance{}, nil)
				c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("server error"))
			}),
			Then(func(ctx context.Context, client *mock.MockLinodeMachineClient) {
				Expect(clientCalls(ctx, client)).NotTo(Succeed())
			}),
		),
	)

	for _, path := range paths {
		It(path.Text, func(ctx SpecContext) {
			path.Run(ctx, mock.NewMockLinodeMachineClient(mockCtrl))
		})
	}
})

var _ = Describe("fork", func() {
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	paths := Paths(
		If("list and create",
			Called(func(c *mock.MockLinodeMachineClient) {
				c.EXPECT().ListInstances(gomock.Any(), gomock.Any()).Return([]linodego.Instance{}, nil)
			}),
		),
		Either(
			If("succeeds",
				Called(func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(&linodego.Instance{ID: 1}, nil)
				}),
				Then(func(ctx context.Context, client *mock.MockLinodeMachineClient) {
					Expect(clientCalls(ctx, client)).To(Succeed())
				}),
			),
			If("fails with server error",
				Called(func(c *mock.MockLinodeMachineClient) {
					c.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, errors.New("server error"))
				}),
				Then(func(ctx context.Context, client *mock.MockLinodeMachineClient) {
					Expect(clientCalls(ctx, client)).NotTo(Succeed())
				}),
			),
		),
	)

	for _, path := range paths {
		It(path.Text, func(ctx SpecContext) {
			path.Run(ctx, mock.NewMockLinodeMachineClient(mockCtrl))
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
			name: "early closed entry",
			input: []node{
				entry{result: true},
				entry{},
			},
			panicErr: errors.New("unreachable path beyond index 0"),
		},
		{
			name: "fork",
			input: []node{
				fork{
					entry{result: true},
					entry{result: true},
				},
			},
			output: [][]entry{
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
			name: "early closed fork",
			input: []node{
				fork{
					entry{result: true},
					entry{result: true},
				},
				entry{},
			},
			panicErr: errors.New("unreachable path beyond index 0"),
		},
		{
			name: "split",
			input: []node{
				entry{called: 0},
				fork{
					entry{called: 1},
					entry{called: 2},
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
					entry{text: "bucket and keys do not exist", result: true},
					entry{text: "bucket and keys do exist"},
				},
				fork{
					entry{text: "the secret is deleted", result: true},
					entry{text: "keyGeneration changes", result: true},
				},
			},
			output: [][]entry{
				{
					{text: "bucket and keys do not exist", result: true},
				},
				{
					{text: "bucket and keys do exist"},
					{text: "the secret is deleted", result: true},
				},
				{
					{text: "bucket and keys do exist"},
					{text: "keyGeneration changes", result: true},
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
