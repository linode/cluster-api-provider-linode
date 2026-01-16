package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
)

func TestGetIPPortCombo(t *testing.T) {
	t.Parallel()

	// Define test cases
	tests := []struct {
		name          string
		clusterScope  *scope.ClusterScope
		expectedCombo []string
	}{
		{
			name: "Default port with no VPC and only private IPs",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							// Default port (6443) will be used
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.100",
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{"192.168.128.100:6443"},
		},
		{
			name: "Custom port with no VPC and only private IPs",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							ApiserverLoadBalancerPort: 8443,
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.100",
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{"192.168.128.100:8443"},
		},
		{
			name: "With VPC and VPC IPs available",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							EnableVPCBackends:            true,
							NodeBalancerBackendIPv4Range: "10.0.0.0/24",
						},
						VPCRef: &corev1.ObjectReference{
							Name: "test-vpc",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "10.0.0.100", // VPC IP
									},
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.100", // Private IP
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{"10.0.0.100:6443"},
		},
		{
			name: "With VPC but no VPC IPs, should fall back to private IPs",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							EnableVPCBackends:            true,
							NodeBalancerBackendIPv4Range: "10.0.0.0/24",
						},
						VPCRef: &corev1.ObjectReference{
							Name: "test-vpc",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									// No VPC IP (10.0.0.x), just private IP
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.100",
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{"192.168.128.100:6443"},
		},
		{
			name: "With additional ports",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port: 8080,
								},
								{
									Port: 9090,
								},
							},
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.100",
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{"192.168.128.100:6443", "192.168.128.100:8080", "192.168.128.100:9090"},
		},
		{
			name: "With VPC IPs and additional ports",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							EnableVPCBackends:            true,
							NodeBalancerBackendIPv4Range: "10.0.0.0/24",
							AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
								{
									Port: 8080,
								},
							},
						},
						VPCRef: &corev1.ObjectReference{
							Name: "test-vpc",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "10.0.0.100", // VPC IP
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{"10.0.0.100:6443", "10.0.0.100:8080"},
		},
		{
			name: "Multiple machines with mix of VPC and private IPs",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							EnableVPCBackends:            true,
							NodeBalancerBackendIPv4Range: "10.0.0.0/24",
						},
						VPCRef: &corev1.ObjectReference{
							Name: "test-vpc",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							// Machine 1 with VPC IP
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "10.0.0.100", // VPC IP
									},
								},
							},
						},
						{
							// Machine 2 with only private IP
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.101", // Private IP
									},
								},
							},
						},
						{
							// Machine 3 with both IPs
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "10.0.0.102", // VPC IP
									},
									{
										Type:    clusterv1.MachineInternalIP,
										Address: "192.168.128.102", // Private IP
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{"10.0.0.100:6443", "192.168.128.101:6443", "10.0.0.102:6443"},
		},
		{
			name: "No internal IPs available",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    clusterv1.MachineExternalIP, // Not an internal IP
										Address: "203.0.113.100",
									},
								},
							},
						},
					},
				},
			},
			expectedCombo: []string{}, // No IPs should match
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			// Call the function
			result := getIPPortCombo(testcase.clusterScope)

			// Verify the results
			assert.ElementsMatch(t, testcase.expectedCombo, result)
		})
	}
}

func TestAddMachineToLB(t *testing.T) {
	t.Parallel()

	// Set up the mock controller
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Define test cases
	tests := []struct {
		name                string
		clusterScope        *scope.ClusterScope
		setupMocks          func(*mock.MockLinodeClient, *mock.MockAkamClient, *mock.MockK8sClient)
		expectedError       bool
		expectedErrorString string
	}{
		{
			name: "External load balancer type should return nil",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							LoadBalancerType: lbTypeExternal,
						},
					},
				},
			},
			setupMocks: func(mockLinodeClient *mock.MockLinodeClient, mockDNSClient *mock.MockAkamClient, mockK8sClient *mock.MockK8sClient) {
			},
			expectedError: false,
		},
		{
			name: "NodeBalancer without IDs should set type to external",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							LoadBalancerType: lbTypeNB,
							// NodeBalancerID and ApiserverNodeBalancerConfigID are nil
						},
					},
				},
			},
			setupMocks: func(mockLinodeClient *mock.MockLinodeClient, mockDNSClient *mock.MockAkamClient, mockK8sClient *mock.MockK8sClient) {
			},
			expectedError: false,
		},
		{
			name: "NodeBalancer with IDs should list nodes",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							LoadBalancerType:              lbTypeNB,
							NodeBalancerID:                util.Pointer(12345),
							ApiserverNodeBalancerConfigID: util.Pointer(67890),
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{},
				},
			},
			setupMocks: func(mockLinodeClient *mock.MockLinodeClient, mockDNSClient *mock.MockAkamClient, mockK8sClient *mock.MockK8sClient) {
				mockLinodeClient.EXPECT().
					ListNodeBalancerNodes(
						gomock.Any(),
						12345,
						67890,
						gomock.Any(),
					).
					Return([]linodego.NodeBalancerNode{}, nil)
			},
			expectedError: false,
		},
		{
			name: "Error listing NodeBalancer nodes",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							LoadBalancerType:              lbTypeNB,
							NodeBalancerID:                util.Pointer(12345),
							ApiserverNodeBalancerConfigID: util.Pointer(67890),
						},
					},
				},
			},
			setupMocks: func(mockLinodeClient *mock.MockLinodeClient, mockDNSClient *mock.MockAkamClient, mockK8sClient *mock.MockK8sClient) {
				mockLinodeClient.EXPECT().
					ListNodeBalancerNodes(
						gomock.Any(),
						12345,
						67890,
						gomock.Any(),
					).
					Return(nil, fmt.Errorf("API error"))
			},
			expectedError:       true,
			expectedErrorString: "API error",
		},
		{
			name: "machines ready for DNS load balancer type",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							LoadBalancerType: lbTypeDNS,
							DNSProvider:      "akamai",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-machine-1",
								CreationTimestamp: metav1.Time{Time: time.Now()},
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{},
								Ready:     true,
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-machine-2",
								CreationTimestamp: metav1.Time{Time: time.Now()},
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{},
								Ready:     true,
							},
						},
					},
				},
			},
			setupMocks: func(mockLinodeClient *mock.MockLinodeClient, mockDNSClient *mock.MockAkamClient, mockK8sClient *mock.MockK8sClient) {
				mockDNSClient.EXPECT().GetRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&dns.RecordBody{}, nil).AnyTimes()
				mockDNSClient.EXPECT().DeleteRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockDNSClient.EXPECT().UpdateRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: false,
		},
		{
			name: "machine not ready yet for DNS load balancer type",
			clusterScope: &scope.ClusterScope{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: defaultNamespace,
					},
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							LoadBalancerType: lbTypeDNS,
							DNSProvider:      "akamai",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-machine-1",
								Namespace:         defaultNamespace,
								CreationTimestamp: metav1.Time{Time: time.Now()},
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: clusterv1.GroupVersion.String(),
										Kind:       "Machine",
										Name:       "test-machine-1",
									},
								},
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{},
								Ready:     true,
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-machine-2",
								Namespace:         defaultNamespace,
								CreationTimestamp: metav1.Time{Time: time.Now()},
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: clusterv1.GroupVersion.String(),
										Kind:       "Machine",
										Name:       "test-machine-2",
									},
								},
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{},
								Ready:     false,
							},
						},
					},
				},
			},
			setupMocks: func(mockLinodeClient *mock.MockLinodeClient, mockDNSClient *mock.MockAkamClient, mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				machine1 := &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-machine-1",
						Namespace: defaultNamespace,
					},
					Status: clusterv1.MachineStatus{
						Conditions: []metav1.Condition{
							{
								Type:   clusterv1.ReadyCondition,
								Status: metav1.ConditionTrue,
							},
						},
					},
				}
				err := mockK8sClient.Create(context.Background(), machine1)
				if err != nil {
					return
				}
				machine2 := &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-machine-2",
						Namespace: defaultNamespace,
					},
					Status: clusterv1.MachineStatus{
						Conditions: []metav1.Condition{
							{
								Type:   clusterv1.ReadyCondition,
								Status: metav1.ConditionFalse,
							},
						},
					},
				}
				err = mockK8sClient.Create(context.Background(), machine2)
				if err != nil {
					return
				}
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: "test-machine-1", Namespace: defaultNamespace}, gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: "test-machine-2", Namespace: defaultNamespace}, gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError:       true,
			expectedErrorString: util.ErrReconcileAgain.Error(),
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			// Create mock clients
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			mockK8sClient := mock.NewMockK8sClient(mockCtrl)
			mockAkamClient := mock.NewMockAkamClient(mockCtrl)

			// Set up the mocks
			testcase.setupMocks(mockLinodeClient, mockAkamClient, mockK8sClient)

			// Set the mock clients in the scope
			testcase.clusterScope.LinodeClient = mockLinodeClient
			testcase.clusterScope.Client = mockK8sClient
			testcase.clusterScope.AkamaiDomainsClient = mockAkamClient

			// Create a context with logger
			ctx := t.Context()
			logger := testr.New(t)
			ctx = logr.NewContext(ctx, logger)

			// Call the function
			err := addMachineToLB(ctx, testcase.clusterScope)

			// Verify the results
			if testcase.expectedError {
				require.Error(t, err)
				if testcase.expectedErrorString != "" {
					assert.Contains(t, err.Error(), testcase.expectedErrorString)
				}
			} else {
				require.NoError(t, err)
				// For the NodeBalancer without IDs case, verify that LoadBalancerType was changed to lbTypeExternal
				if testcase.name == "NodeBalancer without IDs should set type to lbTypeExternal" {
					assert.Equal(t, lbTypeExternal, testcase.clusterScope.LinodeCluster.Spec.Network.LoadBalancerType)
				}
			}
		})
	}
}

func TestLinodeMachineToLinodeCluster(t *testing.T) {
	t.Parallel()

	// Define test cases
	tests := []struct {
		name             string
		setupMockClient  func(*mock.MockK8sClient)
		inputObject      client.Object
		expectedRequests []ctrl.Request
	}{
		{
			name: "Non-LinodeMachine object should return nil",
			setupMockClient: func(mockClient *mock.MockK8sClient) {
				// No mocks needed for this case
			},
			inputObject:      &corev1.Pod{},
			expectedRequests: nil,
		},
		{
			name: "LinodeMachine without owner reference should return nil",
			setupMockClient: func(mockClient *mock.MockK8sClient) {
				// For a machine without owner reference, the GetOwnerMachine function will try to get the owner machine
				// but will fail because there's no owner reference
				mockClient.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&clusterv1.Machine{})).
					Return(fmt.Errorf("machine not found")).
					AnyTimes()
			},
			inputObject: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-without-owner",
					Namespace: "default",
				},
			},
			expectedRequests: nil,
		},
		{
			name: "LinodeMachine with owner reference that is not a control plane should return nil",
			setupMockClient: func(mockClient *mock.MockK8sClient) {
				mockClient.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&clusterv1.Machine{})).
					SetArg(2, clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "worker-machine",
							Namespace: "default",
							Labels:    map[string]string{},
						},
					}).
					Return(nil)
			},
			inputObject: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-machine",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "worker-machine",
							UID:        "abc-123",
						},
					},
				},
			},
			expectedRequests: nil,
		},
		{
			name: "Control plane LinodeMachine but LinodeCluster lookup fails",
			setupMockClient: func(mockClient *mock.MockK8sClient) {
				// First Get call for the Machine owner
				mockClient.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&clusterv1.Machine{})).
					SetArg(2, clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "control-plane-machine",
							Namespace: "default",
							Labels: map[string]string{
								clusterv1.MachineControlPlaneLabel: "true",
								clusterv1.ClusterNameLabel:         "test-cluster",
							},
						},
					}).
					Return(nil)

				// Second Get call for the LinodeCluster
				mockClient.EXPECT().
					Get(gomock.Any(), types.NamespacedName{
						Name:      "test-cluster",
						Namespace: "default",
					}, gomock.AssignableToTypeOf(&infrav1alpha2.LinodeCluster{})).
					Return(fmt.Errorf("LinodeCluster not found"))
			},
			inputObject: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "control-plane-machine",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "control-plane-machine",
							UID:        "abc-123",
						},
					},
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "test-cluster",
					},
				},
			},
			expectedRequests: nil,
		},
		{
			name: "Control plane LinodeMachine with successful LinodeCluster lookup",
			setupMockClient: func(mockClient *mock.MockK8sClient) {
				// First Get call for the Machine owner
				mockClient.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&clusterv1.Machine{})).
					SetArg(2, clusterv1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "control-plane-machine",
							Namespace: "default",
							Labels: map[string]string{
								clusterv1.MachineControlPlaneLabel: "true",
								clusterv1.ClusterNameLabel:         "test-cluster",
							},
						},
					}).
					Return(nil)

				// Second Get call for the LinodeCluster
				mockClient.EXPECT().
					Get(gomock.Any(), types.NamespacedName{
						Name:      "test-cluster",
						Namespace: "default",
					}, gomock.AssignableToTypeOf(&infrav1alpha2.LinodeCluster{})).
					DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *infrav1alpha2.LinodeCluster, _ ...client.GetOption) error {
						obj.Name = "test-cluster"
						obj.Namespace = "default"
						return nil
					})
			},
			inputObject: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "control-plane-machine",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "control-plane-machine",
							UID:        "abc-123",
						},
					},
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "test-cluster",
					},
				},
			},
			expectedRequests: []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "test-cluster",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			// Set up the mock controller for each test case
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			// Create a test logger
			logger := testr.New(t)

			// Create a mock client
			mockClient := mock.NewMockK8sClient(mockCtrl)
			testcase.setupMockClient(mockClient)

			// Create the MapFunc
			mapFunc := linodeMachineToLinodeCluster(mockClient, logger)

			// Call the function with the test input
			ctx := t.Context()
			result := mapFunc(ctx, testcase.inputObject)

			// Verify the results
			assert.Len(t, result, len(testcase.expectedRequests))
			if len(testcase.expectedRequests) > 0 {
				assert.Equal(t, testcase.expectedRequests, result)
			}
		})
	}
}
