package services

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCreateNodeBalancer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		clusterScope *scope.ClusterScope
		want    *linodego.NodeBalancer
		wantErr bool
		expects func(mock *mock.MockLinodeClient)
		expectedNodeBalancer *linodego.NodeBalancer
		expectedError error
	}{
		// TODO: Add test cases.
		{
			name: "Success - Create NodeBalancer",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:   "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{}, nil)
				mock.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancer{
					ID: 1234,
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID: 1234,
			},
			expectedError:        nil,
			
		},
		{
			name: "Success - List NodeBalancers returns one nodebalancer and we return that",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:   "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{
					{
						ID: 1234,
						Label: ptr.To("test"),
						Tags: []string{"test-uid"},
					},
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID: 1234,
				Label: ptr.To("test"),
				Tags: []string{"test-uid"},
			},
			expectedError:        nil,
		},
		{
			name: "Error - List NodeBalancers returns one nodebalancer but there is a nodebalancer conflict",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:   "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{
					{
						ID: 1234,
						Label: ptr.To("test"),
						Tags: []string{"test"},
					},
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID: 1234,
				Label: ptr.To("test"),
				Tags: []string{"test"},
			},
			expectedError: fmt.Errorf("NodeBalancer conflict"),
		},
		{
			name: "Error - List NodeBalancers returns an error",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:   "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Unable to list NodeBalancers"))
			},
			expectedNodeBalancer: nil,
			expectedError: fmt.Errorf("Unable to list NodeBalancers"),
		},
		{
			name: "Error - Create NodeBalancer returns an error",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:   "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{}, nil)
				mock.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Unable to create NodeBalancer"))
			},
			expectedNodeBalancer: nil,
			expectedError: fmt.Errorf("Unable to create NodeBalancer"),
			
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLinodeClient := mock.NewMockLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = mockLinodeClient

			testcase.expects(mockLinodeClient)

			got, err := CreateNodeBalancer(context.Background(), testcase.clusterScope, logr.Discard())
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
				assert.Equal(t, testcase.expectedNodeBalancer, got)
			}
		})
	}
}

func TestCreateNodeBalancerConfig(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx          context.Context
		clusterScope *scope.ClusterScope
		logger       logr.Logger
	}
	tests := []struct {
		name    string
		args    args
		want    *linodego.NodeBalancerConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CreateNodeBalancerConfig(tt.args.ctx, tt.args.clusterScope, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNodeBalancerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateNodeBalancerConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddNodeToNB(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx          context.Context
		logger       logr.Logger
		machineScope *scope.MachineScope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if err := AddNodeToNB(testcase.args.ctx, testcase.args.logger, testcase.args.machineScope); (err != nil) != testcase.wantErr {
				t.Errorf("AddNodeToNB() error = %v, wantErr %v", err, testcase.wantErr)
			}
		})
	}
}

func TestDeleteNodeFromNB(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx          context.Context
		logger       logr.Logger
		machineScope *scope.MachineScope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if err := DeleteNodeFromNB(testcase.args.ctx, testcase.args.logger, testcase.args.machineScope); (err != nil) != testcase.wantErr {
				t.Errorf("DeleteNodeFromNB() error = %v, wantErr %v", err, testcase.wantErr)
			}
		})
	}
}
