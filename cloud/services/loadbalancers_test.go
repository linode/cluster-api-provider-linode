package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
)

func TestEnsureNodeBalancer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		clusterScope         *scope.ClusterScope
		expects              func(*mock.MockLinodeClient, *mock.MockK8sClient)
		expectedNodeBalancer *linodego.NodeBalancer
		expectedError        error
	}{
		{
			name: "Success - Create NodeBalancer",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 DefaultKonnectivityLBPort,
									NodeBalancerConfigID: ptr.To(1234),
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockClient.EXPECT().GetNodeBalancer(gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancer{
					ID: 1234,
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID: 1234,
			},
		},
		{
			name: "Success - Get NodeBalancers returns one nodebalancer and we return that",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockClient.EXPECT().GetNodeBalancer(gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancer{
					ID:    1234,
					Label: ptr.To("test"),
					Tags:  []string{"test-uid"},
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID:    1234,
				Label: ptr.To("test"),
				Tags:  []string{"test-uid"},
			},
		},
		{
			name: "Error - Get NodeBalancer returns an error",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockClient.EXPECT().GetNodeBalancer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Unable to get NodeBalancer"))
			},
			expectedError: fmt.Errorf("Unable to get NodeBalancer"),
		},
		{
			name: "Error - Create NodeBalancer returns an error",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Unable to create NodeBalancer"))
			},
			expectedError: fmt.Errorf("Unable to create NodeBalancer"),
		},
		{
			name: "Success - Create NodeBalancer with FirewallRef",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						UID:       "test-uid",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Region: "us-east",
						NodeBalancerFirewallRef: &corev1.ObjectReference{
							Name:      "test-firewall",
							Namespace: "default",
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				// Mock K8s client Get call for FirewallRef
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					// Check type assertion
					firewall, ok := obj.(*infrav1alpha2.LinodeFirewall)
					if !ok {
						return fmt.Errorf("expected *infrav1alpha2.LinodeFirewall, got %T", obj)
					}
					// Set the FirewallID in the mock response
					firewall.Spec.FirewallID = util.Pointer(5678)
					return nil
				})

				// Mock GetFirewall call
				mockClient.EXPECT().GetFirewall(gomock.Any(), 5678).Return(&linodego.Firewall{
					ID: 5678,
				}, nil)

				// Mock CreateNodeBalancer call
				mockClient.EXPECT().CreateNodeBalancer(
					gomock.Any(),
					gomock.Eq(linodego.NodeBalancerCreateOptions{
						Label:      util.Pointer("test-cluster"),
						Region:     "us-east",
						Tags:       []string{"test-uid"},
						FirewallID: 5678,
					}),
				).Return(&linodego.NodeBalancer{
					ID:    1234,
					Label: util.Pointer("test-cluster"),
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID:    1234,
				Label: util.Pointer("test-cluster"),
			},
			expectedError: nil,
		},
		{
			name: "Success - Create NodeBalancer with direct FirewallID",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Region: "us-east",
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerFirewallID: util.Pointer(5678),
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockClient.EXPECT().GetFirewall(gomock.Any(), 5678).Return(&linodego.Firewall{
					ID: 5678,
				}, nil)
				mockClient.EXPECT().CreateNodeBalancer(gomock.Any(), linodego.NodeBalancerCreateOptions{
					Label:      util.Pointer("test-cluster"),
					Region:     "us-east",
					Tags:       []string{"test-uid"},
					FirewallID: 5678,
				}).Return(&linodego.NodeBalancer{
					ID:    1234,
					Label: util.Pointer("test-cluster"),
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID:    1234,
				Label: util.Pointer("test-cluster"),
			},
		},
		{
			name: "Error - FirewallRef not found",
			clusterScope: &scope.ClusterScope{
				Client: mock.NewMockK8sClient(nil),
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						UID:       "test-uid",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						NodeBalancerFirewallRef: &corev1.ObjectReference{
							Name:      "non-existent-firewall",
							Namespace: "default",
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed to fetch LinodeFirewall"))
			},
			expectedError: fmt.Errorf("Failed to fetch LinodeFirewall"),
		},
		{
			name: "Error - Direct FirewallID not found",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerFirewallID: util.Pointer(9999),
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockClient.EXPECT().GetFirewall(gomock.Any(), 9999).Return(nil, fmt.Errorf("Firewall not found"))
			},
			expectedError: fmt.Errorf("Firewall not found"),
		},
		{
			name: "Success - Create NodeBalancer in VPC",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						UID:       "test-uid",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Region: "us-east",
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerBackendIPv4Range: "10.0.0.0/24",
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				// Mock K8s client Get call for VPC
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
					if !ok {
						return fmt.Errorf("expected *infrav1alpha2.LinodeVPC, got %T", obj)
					}
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label:    "subnet-1",
							SubnetID: 1001,
						},
					}
					return nil
				})

				// Mock CreateNodeBalancer call with VPC options
				mockClient.EXPECT().CreateNodeBalancer(
					gomock.Any(),
					gomock.Eq(linodego.NodeBalancerCreateOptions{
						Label:  util.Pointer("test-cluster"),
						Region: "us-east",
						Tags:   []string{"test-uid"},
						VPCs: []linodego.NodeBalancerVPCOptions{
							{
								IPv4Range: "10.0.0.0/24",
								SubnetID:  1001,
							},
						},
					}),
				).Return(&linodego.NodeBalancer{
					ID:    1234,
					Label: util.Pointer("test-cluster"),
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID:    1234,
				Label: util.Pointer("test-cluster"),
			},
		},
		{
			name: "Error - Failed to get subnet ID for VPC",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						UID:       "test-uid",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Region: "us-east",
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerBackendIPv4Range: "10.0.0.0/24",
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				// Mock K8s client Get call for VPC with error
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(fmt.Errorf("Failed to fetch LinodeVPC"))
			},
			expectedError: fmt.Errorf("Failed to fetch LinodeVPC"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockLinodeClient := mock.NewMockLinodeClient(ctrl)
			MockK8sClient := mock.NewMockK8sClient(ctrl)

			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.clusterScope.Client = MockK8sClient

			testcase.expects(MockLinodeClient, MockK8sClient)

			got, err := EnsureNodeBalancer(context.Background(), testcase.clusterScope, logr.Discard())
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
				assert.Equal(t, testcase.expectedNodeBalancer, got)
			}
		})
	}
}

func TestGetSubnetID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		clusterScope  *scope.ClusterScope
		expects       func(*mock.MockK8sClient)
		expectedID    int
		expectedError string
	}{
		{
			name: "Success - Get first subnet when no subnet name specified",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
					},
				},
			},
			expects: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
					if !ok {
						return fmt.Errorf("expected *infrav1alpha2.LinodeVPC, got %T", obj)
					}
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label:    "subnet-1",
							SubnetID: 1001,
						},
						{
							Label:    "subnet-2",
							SubnetID: 1002,
						},
					}
					return nil
				})
			},
			expectedID: 1001,
		},
		{
			name: "Success - Get subnet by name",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
						Network: infrav1alpha2.NetworkSpec{
							SubnetName: "subnet-2",
						},
					},
				},
			},
			expects: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
					if !ok {
						return fmt.Errorf("expected *infrav1alpha2.LinodeVPC, got %T", obj)
					}
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label:    "subnet-1",
							SubnetID: 1001,
						},
						{
							Label:    "subnet-2",
							SubnetID: 1002,
						},
					}
					return nil
				})
			},
			expectedID: 1002,
		},
		{
			name: "Error - Failed to fetch VPC",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "non-existent-vpc",
							Namespace: "default",
						},
					},
				},
			},
			expects: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(fmt.Errorf("Failed to fetch LinodeVPC"))
			},
			expectedError: "Failed to fetch LinodeVPC",
		},
		{
			name: "Error - No subnets in VPC",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
					},
				},
			},
			expects: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
					if !ok {
						return fmt.Errorf("expected *infrav1alpha2.LinodeVPC, got %T", obj)
					}
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{} // Empty subnets
					return nil
				})
			},
			expectedError: "No subnets found in LinodeVPC",
		},
		{
			name: "Error - Subnet with specific name not found",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
						Network: infrav1alpha2.NetworkSpec{
							SubnetName: "non-existent-subnet",
						},
					},
				},
			},
			expects: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
					if !ok {
						return fmt.Errorf("expected *infrav1alpha2.LinodeVPC, got %T", obj)
					}
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label:    "subnet-1",
							SubnetID: 1001,
						},
						{
							Label:    "subnet-2",
							SubnetID: 1002,
						},
					}
					return nil
				})
			},
			expectedError: "subnet with label non-existent-subnet not found in VPC",
		},
		{
			name: "Error - Selected subnet ID is 0",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
					},
				},
			},
			expects: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
					if !ok {
						return fmt.Errorf("expected *infrav1alpha2.LinodeVPC, got %T", obj)
					}
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label:    "subnet-1",
							SubnetID: 0, // Invalid subnet ID
						},
					}
					return nil
				})
			},
			expectedError: "selected subnet ID is 0",
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = mockK8sClient

			testcase.expects(mockK8sClient)

			got, err := getSubnetID(context.Background(), testcase.clusterScope, logr.Discard())
			if testcase.expectedError != "" {
				require.ErrorContains(t, err, testcase.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testcase.expectedID, got)
			}
		})
	}
}

func TestProcessAndCreateNodeBalancerNodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		ipAddress         string
		clusterScope      *scope.ClusterScope
		nodeBalancerNodes []linodego.NodeBalancerNode
		subnetID          int
		expects           func(*mock.MockLinodeClient)
		expectedError     string
	}{
		{
			name:      "Success - Create node with standard port only",
			ipAddress: "192.168.1.10",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(123),
							ApiserverNodeBalancerConfigID: ptr.To(456),
							ApiserverLoadBalancerPort:     6443,
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			nodeBalancerNodes: []linodego.NodeBalancerNode{},
			subnetID:          0,
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancerNode(
					gomock.Any(),
					123,
					456,
					linodego.NodeBalancerNodeCreateOptions{
						Label:   "test-cluster",
						Address: "192.168.1.10:6443",
						Mode:    linodego.ModeAccept,
					},
				).Return(&linodego.NodeBalancerNode{}, nil)
			},
		},
		{
			name:      "Success - Create node with standard and additional ports",
			ipAddress: "192.168.1.10",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(123),
							ApiserverNodeBalancerConfigID: ptr.To(456),
							ApiserverLoadBalancerPort:     6443,
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 8132,
									NodeBalancerConfigID: ptr.To(789),
								},
							},
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			nodeBalancerNodes: []linodego.NodeBalancerNode{},
			subnetID:          0,
			expects: func(mockClient *mock.MockLinodeClient) {
				// Expect call for standard port
				mockClient.EXPECT().CreateNodeBalancerNode(
					gomock.Any(),
					123,
					456,
					linodego.NodeBalancerNodeCreateOptions{
						Label:   "test-cluster",
						Address: "192.168.1.10:6443",
						Mode:    linodego.ModeAccept,
					},
				).Return(&linodego.NodeBalancerNode{}, nil)

				// Expect call for additional port
				mockClient.EXPECT().CreateNodeBalancerNode(
					gomock.Any(),
					123,
					789,
					linodego.NodeBalancerNodeCreateOptions{
						Label:   "test-cluster",
						Address: "192.168.1.10:8132",
						Mode:    linodego.ModeAccept,
					},
				).Return(&linodego.NodeBalancerNode{}, nil)
			},
		},
		{
			name:      "Success - Node already exists for standard port",
			ipAddress: "192.168.1.10",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(123),
							ApiserverNodeBalancerConfigID: ptr.To(456),
							ApiserverLoadBalancerPort:     6443,
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			nodeBalancerNodes: []linodego.NodeBalancerNode{
				{
					ID:      789,
					Address: "192.168.1.10:6443", // Node with this address already exists
					Label:   "test-cluster",
				},
			},
			subnetID: 0,
			expects: func(mockClient *mock.MockLinodeClient) {
				// No API calls expected as node already exists
			},
		},
		{
			name:      "Success - Create node with SubnetID",
			ipAddress: "192.168.1.10",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(123),
							ApiserverNodeBalancerConfigID: ptr.To(456),
							ApiserverLoadBalancerPort:     6443,
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			nodeBalancerNodes: []linodego.NodeBalancerNode{},
			subnetID:          1001, // Subnet ID is set
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancerNode(
					gomock.Any(),
					123,
					456,
					linodego.NodeBalancerNodeCreateOptions{
						Label:    "test-cluster",
						Address:  "192.168.1.10:6443",
						Mode:     linodego.ModeAccept,
						SubnetID: 1001,
					},
				).Return(&linodego.NodeBalancerNode{}, nil)
			},
		},
		{
			name:      "Error - CreateNodeBalancerNode fails",
			ipAddress: "192.168.1.10",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(123),
							ApiserverNodeBalancerConfigID: ptr.To(456),
							ApiserverLoadBalancerPort:     6443,
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			nodeBalancerNodes: []linodego.NodeBalancerNode{},
			subnetID:          0,
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancerNode(
					gomock.Any(),
					123,
					456,
					gomock.Any(),
				).Return(nil, fmt.Errorf("Failed to create NodeBalancerNode"))
			},
			expectedError: "Failed to create NodeBalancerNode",
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockLinodeClient(ctrl)
			testcase.clusterScope.LinodeClient = mockClient

			testcase.expects(mockClient)

			err := processAndCreateNodeBalancerNodes(
				context.Background(),
				testcase.ipAddress,
				testcase.clusterScope,
				testcase.nodeBalancerNodes,
				testcase.subnetID,
			)

			if testcase.expectedError != "" {
				require.ErrorContains(t, err, testcase.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEnsureNodeBalancerConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		clusterScope    *scope.ClusterScope
		expectedConfigs []*linodego.NodeBalancerConfig
		expectedError   error
		expects         func(*mock.MockLinodeClient)
	}{
		{
			name: "Success - Create NodeBalancerConfig using default LB ports",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expectedConfigs: []*linodego.NodeBalancerConfig{
				{
					Port:           DefaultApiserverLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerConfig{
					Port:           DefaultApiserverLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				}, nil)
			},
		},
		{
			name: "Success - Get NodeBalancerConfig",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(2),
						},
						ControlPlaneEndpoint: clusterv1.APIEndpoint{
							Host: "",
							Port: 0,
						},
					},
				},
			},
			expectedConfigs: []*linodego.NodeBalancerConfig{
				{
					Port:           DefaultApiserverLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
					ID:             2,
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerConfig{
					ID:             2,
					Port:           DefaultApiserverLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				}, nil)
			},
		},
		{
			name: "Success - Create NodeBalancerConfig using assigned LB ports",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:            ptr.To(1234),
							ApiserverLoadBalancerPort: 80,
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 90,
									NodeBalancerConfigID: ptr.To(1234),
								},
							},
						},
					},
				},
			},
			expectedConfigs: []*linodego.NodeBalancerConfig{
				{
					Port:           80,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				},
				{
					Port:           90,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerConfig{
					Port:           80,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				}, nil)
				mockClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerConfig{
					Port:           90,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				}, nil)
			},
		},
		{
			name: "Error - CreateNodeBalancerConfig() returns an error when creating nbconfig for apiserver",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 DefaultKonnectivityLBPort,
									NodeBalancerConfigID: ptr.To(1234),
								},
							},
						},
					},
				},
			},
			expectedConfigs: []*linodego.NodeBalancerConfig{
				{
					Port:           DefaultApiserverLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				},
				{
					Port:           DefaultKonnectivityLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				},
			},
			expectedError: fmt.Errorf("error creating NodeBalancerConfig"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error creating NodeBalancerConfig"))
			},
		},
		{
			name: "Error - CreateNodeBalancerConfig() returns an error when creating nbconfig for konnectivity",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 DefaultKonnectivityLBPort,
									NodeBalancerConfigID: ptr.To(1234),
								},
							},
						},
					},
				},
			},
			expectedConfigs: []*linodego.NodeBalancerConfig{
				{
					Port:           DefaultApiserverLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				},
				{
					Port:           DefaultKonnectivityLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				},
			},
			expectedError: fmt.Errorf("error creating NodeBalancerConfig"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerConfig{
					Port:           DefaultApiserverLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				}, nil)
				mockClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error creating NodeBalancerConfig"))
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockLinodeClient := mock.NewMockLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = MockLinodeClient

			testcase.expects(MockLinodeClient)

			got, err := EnsureNodeBalancerConfigs(context.Background(), testcase.clusterScope, logr.Discard())
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
				assert.Equal(t, testcase.expectedConfigs, got)
			}
		})
	}
}

func TestAddNodeToNBConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		clusterScope    *scope.ClusterScope
		expectedError   error
		expects         func(*mock.MockLinodeClient)
		expectK8sClient func(*mock.MockK8sClient)
	}{
		{
			name: "Error - ApiserverNodeBalancerConfigID is not set",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: nil,
							ApiserverLoadBalancerPort:     DefaultApiserverLBPort,
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("nil NodeBalancer Config ID"),
			expects:       func(mockClient *mock.MockLinodeClient) {},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
		{
			name: "Error - No private IP addresses were set",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(1234),
							ApiserverLoadBalancerPort:     DefaultApiserverLBPort,
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("no private IP address"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancerNode{}, nil).AnyTimes()
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockLinodeClient := mock.NewMockLinodeClient(ctrl)
			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.expects(MockLinodeClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			for _, eachMachine := range testcase.clusterScope.LinodeMachines.Items {
				err := AddNodesToNB(context.Background(), logr.Discard(), testcase.clusterScope, eachMachine, []linodego.NodeBalancerNode{})
				if testcase.expectedError != nil {
					assert.ErrorContains(t, err, testcase.expectedError.Error())
				}
			}
		})
	}
}

func TestAddNodeToNBFullWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		clusterScope    *scope.ClusterScope
		expectedError   error
		expects         func(*mock.MockLinodeClient)
		expectK8sClient func(*mock.MockK8sClient)
	}{
		{
			name: "Success - If the machine is a control plane node, add the node to the NodeBalancer",
			clusterScope: &scope.ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 DefaultKonnectivityLBPort,
									NodeBalancerConfigID: ptr.To(1234),
								},
							},
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancerNode{}, nil).AnyTimes()
				mockClient.EXPECT().CreateNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&linodego.NodeBalancerNode{}, nil)
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
		{
			name: "Error - CreateNodeBalancerNode() returns an error",
			clusterScope: &scope.ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.10",
									},
								},
							},
						},
					},
				},
			},
			expectedError: nil,
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancerNode{}, nil).AnyTimes()
				mockClient.EXPECT().CreateNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
		{
			name: "Success - Prioritizes VPC IP over private IP when NodeBalancerBackendIPv4Range is set",
			clusterScope: &scope.ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						UID:       "test-uid",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
							NodeBalancerBackendIPv4Range:  "10.0.0.0/24",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "10.0.0.5", // VPC IP (not a Linode private IP)
									},
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.10", // Linode private IP
									},
								},
							},
						},
					},
				},
			},
			expectedError: nil,
			expects: func(mockClient *mock.MockLinodeClient) {
				// Linode API calls for node balancer
				mockClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancerNode{}, nil).AnyTimes()
				mockClient.EXPECT().CreateNodeBalancerNode(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Do(func(_ context.Context, _ int, _ int, options linodego.NodeBalancerNodeCreateOptions) {
					// Verify the VPC IP is used
					require.Contains(t, options.Address, "10.0.0.5:")
					require.Equal(t, 5678, options.SubnetID)
				}).Return(&linodego.NodeBalancerNode{}, nil).AnyTimes()
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClientGetForVPC(t, mockK8sClient, false)
			},
		},
		{
			name: "Error - getSubnetID() fails when VPCRef is set",
			clusterScope: &scope.ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						UID:       "test-uid",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
							NodeBalancerBackendIPv4Range:  "10.0.0.0/24",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "10.0.0.5", // VPC IP
									},
								},
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("Failed to fetch LinodeVPC"),
			expects: func(mockClient *mock.MockLinodeClient) {
				// We shouldn't get to any Linode API calls as the K8s Get will fail
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClientGetForVPC(t, mockK8sClient, true)
			},
		},
		{
			name: "Success - Falls back to private IP when no VPC IP is found",
			clusterScope: &scope.ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						UID:       "test-uid",
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCRef: &corev1.ObjectReference{
							Name:      "test-vpc",
							Namespace: "default",
						},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
							NodeBalancerBackendIPv4Range:  "10.0.0.0/24",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.10", // Only Linode private IP, no VPC IP
									},
								},
							},
						},
					},
				},
			},
			expectedError: nil,
			expects: func(mockClient *mock.MockLinodeClient) {
				// Linode API calls for node balancer
				mockClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancerNode{}, nil).AnyTimes()
				mockClient.EXPECT().CreateNodeBalancerNode(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Do(func(_ context.Context, _ int, _ int, options linodego.NodeBalancerNodeCreateOptions) {
					// Verify the private IP is used
					require.Contains(t, options.Address, "192.168.128.10:")
					require.Equal(t, 0, options.SubnetID) // No subnet ID for private IP
				}).Return(&linodego.NodeBalancerNode{}, nil).AnyTimes()
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClientGetForVPC(t, mockK8sClient, false)
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockLinodeClient := mock.NewMockLinodeClient(ctrl)
			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.expects(MockLinodeClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			for _, eachMachine := range testcase.clusterScope.LinodeMachines.Items {
				err := AddNodesToNB(context.Background(), logr.Discard(), testcase.clusterScope, eachMachine, []linodego.NodeBalancerNode{})
				if testcase.expectedError != nil {
					assert.ErrorContains(t, err, testcase.expectedError.Error())
				}
			}
		})
	}
}

func TestDeleteNodeFromNB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		clusterScope    *scope.ClusterScope
		expectedError   error
		expects         func(*mock.MockLinodeClient)
		expectK8sClient func(*mock.MockK8sClient)
	}{
		// TODO: Add test cases.
		{
			name: "NodeBalancer is already deleted",
			clusterScope: &scope.ClusterScope{
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: ""},
					},
				},
			},
			expects: func(*mock.MockLinodeClient) {},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
		{
			name: "Success - Delete Node from NodeBalancer",
			clusterScope: &scope.ClusterScope{
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 DefaultKonnectivityLBPort,
									NodeBalancerConfigID: ptr.To(1234),
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
		{
			name: "Error - Deleting Apiserver Node from NodeBalancer",
			clusterScope: &scope.ClusterScope{
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
						},
					},
				},
			},
			expectedError: fmt.Errorf("error deleting node from NodeBalancer"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("error deleting node from NodeBalancer"))
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
		{
			name: "Error - Deleting Konnectivity Node from NodeBalancer",
			clusterScope: &scope.ClusterScope{
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
						Network: infrav1alpha2.NetworkSpec{
							NodeBalancerID:                ptr.To(1234),
							ApiserverNodeBalancerConfigID: ptr.To(5678),
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port:                 DefaultKonnectivityLBPort,
									NodeBalancerConfigID: ptr.To(1234),
								},
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("error deleting node from NodeBalancer"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("error deleting node from NodeBalancer"))
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockLinodeClient := mock.NewMockLinodeClient(ctrl)
			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.expects(MockLinodeClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := DeleteNodesFromNB(context.Background(), logr.Discard(), testcase.clusterScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

// Create a helper function to mock K8s client Get for VPC
func mockK8sClientGetForVPC(t *testing.T, mockK8sClient *mock.MockK8sClient, shouldFail bool) {
	mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()

	if shouldFail {
		mockK8sClient.EXPECT().Get(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(fmt.Errorf("Failed to fetch LinodeVPC"))
	} else {
		mockK8sClient.EXPECT().Get(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj interface{}, opts ...client.GetOption) error {
			vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
			if !ok {
				return fmt.Errorf("expected *infrav1alpha2.LinodeVPC, got %T", obj)
			}

			// Set the VPC subnets
			vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
				{
					Label:    "test-subnet",
					SubnetID: 5678,
				},
			}
			return nil
		})
	}
}
