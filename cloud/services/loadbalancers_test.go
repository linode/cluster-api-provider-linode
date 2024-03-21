package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestCreateNodeBalancer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		clusterScope         *scope.ClusterScope
		want                 *linodego.NodeBalancer
		wantErr              bool
		expects              func(mock *mock.MockMachineLinodeClient)
		expectedNodeBalancer *linodego.NodeBalancer
		expectedError        error
	}{
		{
			name: "Success - Create NodeBalancer",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockMachineLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{}, nil)
				mock.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancer{
					ID: 1234,
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID: 1234,
			},
			expectedError: nil,
		},
		{
			name: "Success - List NodeBalancers returns one nodebalancer and we return that",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockMachineLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{
					{
						ID:    1234,
						Label: ptr.To("test"),
						Tags:  []string{"test-uid"},
					},
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID:    1234,
				Label: ptr.To("test"),
				Tags:  []string{"test-uid"},
			},
			expectedError: nil,
		},
		{
			name: "Error - List NodeBalancers returns one nodebalancer but there is a nodebalancer conflict",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockMachineLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{
					{
						ID:    1234,
						Label: ptr.To("test"),
						Tags:  []string{"test"},
					},
				}, nil)
			},
			expectedNodeBalancer: &linodego.NodeBalancer{
				ID:    1234,
				Label: ptr.To("test"),
				Tags:  []string{"test"},
			},
			expectedError: fmt.Errorf("NodeBalancer conflict"),
		},
		{
			name: "Error - List NodeBalancers returns an error",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockMachineLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Unable to list NodeBalancers"))
			},
			expectedNodeBalancer: nil,
			expectedError:        fmt.Errorf("Unable to list NodeBalancers"),
		},
		{
			name: "Error - Create NodeBalancer returns an error",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expects: func(mock *mock.MockMachineLinodeClient) {
				mock.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancer{}, nil)
				mock.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Unable to create NodeBalancer"))
			},
			expectedNodeBalancer: nil,
			expectedError:        fmt.Errorf("Unable to create NodeBalancer"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockMachineLinodeClient := mock.NewMockMachineLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = MockMachineLinodeClient

			testcase.expects(MockMachineLinodeClient)

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

	tests := []struct {
		name           string
		clusterScope   *scope.ClusterScope
		expectedConfig *linodego.NodeBalancerConfig
		expectedError  error
		expects        func(m *mock.MockMachineLinodeClient)
	}{
		{
			name: "Success - Create NodeBalancerConfig using default LB port",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expectedConfig: &linodego.NodeBalancerConfig{
				Port:           defaultLBPort,
				Protocol:       linodego.ProtocolTCP,
				Algorithm:      linodego.AlgorithmRoundRobin,
				Check:          linodego.CheckConnection,
				NodeBalancerID: 1234,
			},
			expectedError: nil,
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerConfig{
					Port:           defaultLBPort,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				}, nil)
			},
		},
		{
			name: "Success - Create NodeBalancerConfig using assigned LB port",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID:   ptr.To(1234),
							LoadBalancerPort: 80,
						},
					},
				},
			},
			expectedConfig: &linodego.NodeBalancerConfig{
				Port:           80,
				Protocol:       linodego.ProtocolTCP,
				Algorithm:      linodego.AlgorithmRoundRobin,
				Check:          linodego.CheckConnection,
				NodeBalancerID: 1234,
			},
			expectedError: nil,
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerConfig{
					Port:           80,
					Protocol:       linodego.ProtocolTCP,
					Algorithm:      linodego.AlgorithmRoundRobin,
					Check:          linodego.CheckConnection,
					NodeBalancerID: 1234,
				}, nil)
			},
		},
		{
			name: "Error - CreateNodeBalancerConfig() returns and error",
			clusterScope: &scope.ClusterScope{
				LinodeClient: nil,
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID: ptr.To(1234),
						},
					},
				},
			},
			expectedConfig: &linodego.NodeBalancerConfig{
				Port:           defaultLBPort,
				Protocol:       linodego.ProtocolTCP,
				Algorithm:      linodego.AlgorithmRoundRobin,
				Check:          linodego.CheckConnection,
				NodeBalancerID: 1234,
			},
			expectedError: fmt.Errorf("error creating NodeBalancerConfig"),
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error creating NodeBalancerConfig"))
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockMachineLinodeClient := mock.NewMockMachineLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = MockMachineLinodeClient

			testcase.expects(MockMachineLinodeClient)

			got, err := CreateNodeBalancerConfig(context.Background(), testcase.clusterScope, logr.Discard())
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
				assert.Equal(t, testcase.expectedConfig, got)
			}
		})
	}
}

func TestAddNodeToNBConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		expectedError error
		expects       func(mock *mock.MockMachineLinodeClient)
	}{
		{
			name: "Error - NodeBalancerConfigID are is set",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID:       ptr.To(1234),
							NodeBalancerConfigID: nil,
							LoadBalancerPort:     6443,
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
			},
			expectedError: fmt.Errorf("nil NodeBalancer Config ID"),
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil)
			},
		},
		{
			name: "Error - No private IP addresses were set",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
			},
			expectedError: fmt.Errorf("no private IP address"),
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{},
					},
				}, nil)
			},
		},
		{
			name: "Error - GetInstanceIPAddresses() returns an error",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
			},
			expectedError: fmt.Errorf("could not get instance IP addresses"),
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("could not get instance IP addresses"))
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockMachineLinodeClient := mock.NewMockMachineLinodeClient(ctrl)

			testcase.machineScope.LinodeClient = MockMachineLinodeClient

			testcase.expects(MockMachineLinodeClient)

			err := AddNodeToNB(context.Background(), logr.Discard(), testcase.machineScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

func TestAddNodeToNBFullWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		expectedError error
		expects       func(mock *mock.MockMachineLinodeClient)
	}{
		{
			name: "If the machine is not a control plane node, do nothing",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
			},
			expectedError: nil,
			expects:       func(m *mock.MockMachineLinodeClient) {},
		},
		{
			name: "Success - If the machine is a control plane node, add the node to the NodeBalancer",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID:       ptr.To(1234),
							NodeBalancerConfigID: ptr.To(5678),
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
			},
			expectedError: nil,
			expects: func(mock *mock.MockMachineLinodeClient) {
				mock.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil)
				mock.EXPECT().CreateNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.NodeBalancerNode{}, nil)
			},
		},
		{
			name: "Error - CreateNodeBalancerNode() returns an error",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID:       ptr.To(1234),
							NodeBalancerConfigID: ptr.To(5678),
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
			},
			expectedError: fmt.Errorf("could not create node balancer node"),
			expects: func(mock *mock.MockMachineLinodeClient) {
				mock.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil)
				mock.EXPECT().CreateNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("could not create node balancer node"))
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockMachineLinodeClient := mock.NewMockMachineLinodeClient(ctrl)

			testcase.machineScope.LinodeClient = MockMachineLinodeClient

			testcase.expects(MockMachineLinodeClient)

			err := AddNodeToNB(context.Background(), logr.Discard(), testcase.machineScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

func TestDeleteNodeFromNB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		wantErr       bool
		expectedError error
		expects       func(mock *mock.MockMachineLinodeClient)
	}{
		// TODO: Add test cases.
		{
			name: "If the machine is not a control plane node, do nothing",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
				},
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
				},
			},
			expectedError: nil,
			expects:       func(m *mock.MockMachineLinodeClient) {},
		},
		{
			name: "NodeBalancer is already deleted",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: ""},
					},
				},
			},
			expectedError: nil,
			expects:       func(m *mock.MockMachineLinodeClient) {},
		},
		{
			name: "Success - Delete Node from NodeBalancer",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID:       ptr.To(1234),
							NodeBalancerConfigID: ptr.To(5678),
						},
					},
				},
			},
			expectedError: nil,
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "Error - Deleting Node from NodeBalancer",
			machineScope: &scope.MachineScope{
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
						Labels: map[string]string{
							clusterv1.MachineControlPlaneLabel: "true",
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{
						InstanceID: ptr.To(123),
					},
				},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeClusterSpec{
						ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
						Network: infrav1alpha1.NetworkSpec{
							NodeBalancerID:       ptr.To(1234),
							NodeBalancerConfigID: ptr.To(5678),
						},
					},
				},
			},
			expectedError: fmt.Errorf("error deleting node from NodeBalancer"),
			expects: func(m *mock.MockMachineLinodeClient) {
				m.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("error deleting node from NodeBalancer"))
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockMachineLinodeClient := mock.NewMockMachineLinodeClient(ctrl)

			testcase.machineScope.LinodeClient = MockMachineLinodeClient

			testcase.expects(MockMachineLinodeClient)

			err := DeleteNodeFromNB(context.Background(), logr.Discard(), testcase.machineScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}
