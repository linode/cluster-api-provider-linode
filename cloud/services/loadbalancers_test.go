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

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestEnsureNodeBalancer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		clusterScope         *scope.ClusterScope
		expects              func(*mock.MockLinodeClient)
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
			expects: func(mockClient *mock.MockLinodeClient) {
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
			expects: func(mockClient *mock.MockLinodeClient) {
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
			expects: func(mockClient *mock.MockLinodeClient) {
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Unable to create NodeBalancer"))
			},
			expectedError: fmt.Errorf("Unable to create NodeBalancer"),
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

			got, err := ensureNodeBalancer(context.Background(), testcase.clusterScope, logr.Discard())
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
				assert.Equal(t, testcase.expectedNodeBalancer, got)
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

			got, err := ensureNodeBalancerConfigs(context.Background(), testcase.clusterScope, logr.Discard())
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
		machineScope    *scope.MachineScope
		expectedError   error
		expects         func(*mock.MockLinodeClient)
		expectK8sClient func(*mock.MockK8sClient)
	}{
		{
			name: "Error - ApiserverNodeBalancerConfigID is not set",
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
					},
				},
			},
			expectedError: fmt.Errorf("nil NodeBalancer Config ID"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil)
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
					},
				},
			},
			expectedError: fmt.Errorf("no private IP address"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{},
					},
				}, nil)
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
					},
				},
			},
			expectedError: fmt.Errorf("could not get instance IP addresses"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("could not get instance IP addresses"))
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
			testcase.machineScope.LinodeClient = MockLinodeClient
			testcase.expects(MockLinodeClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.machineScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := AddNodeToNB(context.Background(), logr.Discard(), testcase.machineScope, 123)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

func TestAddNodeToNBFullWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		machineScope    *scope.MachineScope
		expectedError   error
		expects         func(*mock.MockLinodeClient)
		expectK8sClient func(*mock.MockK8sClient)
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
			expects: func(*mock.MockLinodeClient) {},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil)
				mockClient.EXPECT().CreateNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(&linodego.NodeBalancerNode{}, nil)
			},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
					},
				},
			},
			expectedError: fmt.Errorf("could not create node balancer node"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil)
				mockClient.EXPECT().CreateNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("could not create node balancer node"))
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
			testcase.machineScope.LinodeClient = MockLinodeClient
			testcase.expects(MockLinodeClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.machineScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := AddNodeToNB(context.Background(), logr.Discard(), testcase.machineScope, 123)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

func TestDeleteNodeFromNB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		machineScope    *scope.MachineScope
		expectedError   error
		expects         func(*mock.MockLinodeClient)
		expectK8sClient func(*mock.MockK8sClient)
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
			expects: func(*mock.MockLinodeClient) {},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
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
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{
						ProviderID: ptr.To("linode://123"),
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
			testcase.machineScope.LinodeClient = MockLinodeClient
			testcase.expects(MockLinodeClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.machineScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := DeleteNodeFromNB(context.Background(), logr.Discard(), testcase.machineScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}
