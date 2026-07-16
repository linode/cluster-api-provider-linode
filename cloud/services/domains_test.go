package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v12/pkg/dns"
	"github.com/linode/linodego/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	kcpv1beta2 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
)

var unhealthyStatus = &clusterv1.MachineStatus{
	Conditions: []metav1.Condition{
		{
			Type:   kcpv1beta2.KubeadmControlPlaneMachineAPIServerPodHealthyCondition,
			Status: metav1.ConditionTrue,
		},
		{
			Type:   kcpv1beta2.KubeadmControlPlaneMachineControllerManagerPodHealthyCondition,
			Status: metav1.ConditionTrue,
		},
		{
			Type:   kcpv1beta2.KubeadmControlPlaneMachineSchedulerPodHealthyCondition,
			Status: metav1.ConditionTrue,
		},
		{
			Type:   kcpv1beta2.KubeadmControlPlaneMachineEtcdPodHealthyCondition,
			Status: metav1.ConditionTrue,
		},
		{
			Type:   kcpv1beta2.KubeadmControlPlaneMachineEtcdMemberHealthyCondition,
			Status: metav1.ConditionFalse,
		},
		{
			Type:   clusterv1.ReadyCondition,
			Status: metav1.ConditionTrue,
		},
	},
}

func TestAddIPToEdgeDNS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		clusterScope    *scope.ClusterScope
		expects         func(*mock.MockAkamClient)
		expectK8sClient func(*mock.MockK8sClient)
		expectedError   error
	}{
		{
			name: "Success - If DNS Provider is akamai",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "akafn.com",
							DNSUniqueIdentifier: "test-hash",
							DNSProvider:         "akamai",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockAkamClient) {
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Not Found")).AnyTimes()
				mockClient.EXPECT().CreateRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Failure if Machine control plane components are unhealthy (etcd)",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "akafn.com",
							DNSUniqueIdentifier: "test-hash",
							DNSProvider:         "akamai",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: clusterv1.GroupVersion.String(),
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: clusterv1.GroupVersion.String(),
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockAkamClient) {
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Not Found")).AnyTimes()
			},
			expectedError: util.ErrReconcileAgain,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				machineOwnerReferences := []metav1.OwnerReference{{
					APIVersion: kcpv1beta2.GroupVersion.String(),
					Kind:       "KubeadmControlPlane",
					Name:       "test-cluster-cp",
					UID:        "test-kcp-uid",
					Controller: ptr.To(true),
				}}
				machineStatus := unhealthyStatus
				mockCAPIMachine(mockK8sClient, machineOwnerReferences, machineStatus)
				kcp := &kcpv1beta2.KubeadmControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster-cp",
						UID:  "test-kcp-uid",
					},
					Spec: kcpv1beta2.KubeadmControlPlaneSpec{
						KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
							ClusterConfiguration: bootstrapv1.ClusterConfiguration{
								Etcd: bootstrapv1.Etcd{},
							},
						},
					},
				}
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: "default", Name: "test-cluster-cp"}, kcp).Return(nil).AnyTimes()
			},
		},
		{
			name: "Faiure - Error in creating records",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "akafn.com",
							DNSUniqueIdentifier: "test-hash",
							DNSProvider:         "akamai",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockAkamClient) {
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Not Found")).AnyTimes()
				mockClient.EXPECT().CreateRecord(gomock.Any(), gomock.Any()).Return(fmt.Errorf("create record failed")).AnyTimes()
			},
			expectedError: fmt.Errorf("create record failed"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAkamClient := mock.NewMockAkamClient(ctrl)
			testcase.clusterScope.AkamaiDomainsClient = mockAkamClient
			testcase.expects(mockAkamClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(t.Context(), testcase.clusterScope, "create")
			if testcase.expectedError != nil {
				require.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
			if testcase.name == "Failure if Machine control plane components are unhealthy (etcd)" {
				// set everything to healthy and make sure it succeeds
				for i := range unhealthyStatus.Conditions {
					unhealthyStatus.Conditions[i].Status = metav1.ConditionTrue
				}
				mockAkamClient.EXPECT().CreateRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				require.NoError(t, EnsureDNSEntries(t.Context(), testcase.clusterScope, "create"))
			}
		})
	}
}

func TestRemoveIPFromEdgeDNS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		listOfIPS       []string
		expectedList    []string
		clusterScope    *scope.ClusterScope
		expects         func(*mock.MockAkamClient)
		expectK8sClient func(*mock.MockK8sClient)
		expectedError   error
	}{
		{
			name: "Success - If DNS Provider is akamai",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "akafn.com",
							DNSUniqueIdentifier: "test-hash",
							DNSProvider:         "akamai",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			listOfIPS: []string{"10.10.10.10", "10.10.10.11", "10.10.10.12"},
			expects: func(mockClient *mock.MockAkamClient) {
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any()).Return(&dns.GetRecordResponse{
					Name:       "test-machine",
					RecordType: "A",
					TTL:        30,
					Target:     []string{"10.10.10.10"},
				}, nil).AnyTimes()
				mockClient.EXPECT().UpdateRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockClient.EXPECT().DeleteRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectedList:  []string{"10.10.10.10", "10.10.10.12"},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Failure - API Error",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "akafn.com",
							DNSUniqueIdentifier: "test-hash",
							DNSProvider:         "akamai",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			listOfIPS: []string{"10.10.10.10", "10.10.10.11", "10.10.10.12"},
			expects: func(mockClient *mock.MockAkamClient) {
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API Down")).AnyTimes()
				mockClient.EXPECT().DeleteRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: fmt.Errorf("API Down"),
			expectedList:  []string{"10.10.10.10", "10.10.10.12"},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			MockAkamClient := mock.NewMockAkamClient(ctrl)
			testcase.clusterScope.AkamaiDomainsClient = MockAkamClient
			testcase.expects(MockAkamClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(t.Context(), testcase.clusterScope, "delete")
			if err != nil || testcase.expectedError != nil {
				require.ErrorContains(t, err, testcase.expectedError.Error())
			}
			assert.Equal(t, testcase.expectedList, removeElement(testcase.listOfIPS, "10.10.10.11"))
		})
	}
}

func TestAddIPToDNS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		clusterScope         *scope.ClusterScope
		expects              func(*mock.MockLinodeClient)
		expectK8sClient      func(*mock.MockK8sClient)
		expectedDomainRecord *linodego.DomainRecord
		expectedError        error
	}{
		{name: "Skip - If a CAPI machine is deleted, don't add its IP to the Domain but include other machines",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							// This machine's CAPI owner is deleted
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-deleted-machine",
								UID:  "test-uid-1",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-deleted-machine",
										UID:        "test-uid-1",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
								},
							},
						},
						{
							// This machine's CAPI owner is NOT deleted and should have DNS entries
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-active-machine",
								UID:  "test-uid-2",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-active-machine",
										UID:        "test-uid-2",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://456"),
								InstanceID: ptr.To(456),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.20.20.20",
									},
								},
							},
						},
						{
							// This machine's CAPI owner is NOT ready, and should NOT have DNS entries
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-not-ready-machine",
								UID:  "test-uid-2",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-not-ready-machine",
										UID:        "test-uid-3",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://4567"),
								InstanceID: ptr.To(456),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.20.20.22",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				// The code path should still call ListDomains
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()

				// Must mock ListDomainRecords
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{}, nil).AnyTimes()

				// Expect CreateDomainRecord to be called for the active machine's IP (10.20.20.20)
				// but NOT for the deleted machine's IP (10.10.10.10)
				mockClient.EXPECT().CreateDomainRecord(gomock.Any(), gomock.Any(), gomock.Eq(linodego.DomainRecordCreateOptions{
					Type:   "A",
					Name:   "test-cluster-test-hash",
					Target: "10.20.20.20",
					TTLSec: 30,
				})).Return(&linodego.DomainRecord{
					ID:     1234,
					Type:   "A",
					Name:   "test-cluster",
					Target: "10.20.20.20",
					TTLSec: 30,
				}, nil).AnyTimes()
				mockClient.EXPECT().CreateDomainRecord(gomock.Any(), gomock.Any(), gomock.Eq(linodego.DomainRecordCreateOptions{
					Type:   "TXT",
					Name:   "test-cluster-test-hash",
					Target: "test-cluster",
					TTLSec: 30,
				})).Return(&linodego.DomainRecord{
					ID:     1234,
					Type:   "TXT",
					Name:   "test-cluster",
					Target: "test-cluster",
					TTLSec: 30,
				}, nil).AnyTimes()

				// Make sure there's no expectation for the deleted machine's IP
				// We don't need an explicit negative expectation since the mock
				// will fail if any unexpected calls are made
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()

				// Mock the Get call for GetOwnerMachine to handle both machines
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						// Set the Machine fields based on the machine name
						machine, ok := obj.(*clusterv1.Machine)
						if ok {
							switch key.Name {
							case "test-deleted-machine":
								// Set up as a deleted machine
								machine.Name = "test-deleted-machine"
								machine.Namespace = "default"
								// Set DeletionTimestamp to indicate the machine is being deleted
								deletionTime := metav1.Now()
								machine.DeletionTimestamp = &deletionTime
								machine.UID = "test-uid-1"
							case "test-active-machine":
								// Set up as an active machine
								machine.Name = "test-active-machine"
								machine.Namespace = "default"
								machine.UID = "test-uid-2"
								machine.DeletionTimestamp = nil
								machine.Status.Conditions = []metav1.Condition{
									{
										Type:   clusterv1.ReadyCondition,
										Status: metav1.ConditionTrue,
									},
								}
							case "test-not-ready-machine":
								// Set up as a not-ready machine (skipping conditions)
								machine.Name = "test-not-ready-machine"
								machine.Namespace = "default"
								machine.UID = "test-uid-3"
								machine.DeletionTimestamp = nil
							}
						}
						return nil
					}).AnyTimes()
			},
		},
		{
			name: "Success - If the machine is a control plane node, add the IP to the Domain",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{}, nil).AnyTimes()
				mockClient.EXPECT().CreateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.DomainRecord{
					ID:     1234,
					Type:   "A",
					Name:   "test-cluster",
					TTLSec: 30,
				}, nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Success - use custom dnsttlsec",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
							DNSTTLSec:           100,
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{}, nil).AnyTimes()
				mockClient.EXPECT().CreateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.DomainRecord{
					ID:     1234,
					Type:   "A",
					Name:   "test-cluster",
					TTLSec: 100,
				}, nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Error - CreateDomainRecord() returns an error",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{}, nil).AnyTimes()
				mockClient.EXPECT().CreateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed to create domain record of type A")).AnyTimes()
			},
			expectedError: fmt.Errorf("failed to create domain record of type A"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Success - If the machine is a control plane node and record already exists, leave it alone",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{
					{
						ID:     1234,
						Type:   "A",
						Name:   "test-cluster",
						TTLSec: 30,
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().DeleteDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Failure - Failed to get domain records",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("api error")).AnyTimes()
			},
			expectedError: fmt.Errorf("api error"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Error - no public ip set",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: nil,
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{
					{
						ID:     1234,
						Type:   "A",
						Name:   "test-cluster",
						TTLSec: 30,
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().DeleteDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Error - no domain found when creating",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "test.net",
					},
				}, nil).AnyTimes()
			},
			expectedError: fmt.Errorf("domain lkedevs.net not found in list of domains owned by this account"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
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
			MockLinodeDomainsClient := mock.NewMockLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.clusterScope.LinodeDomainsClient = MockLinodeClient

			testcase.expects(MockLinodeClient)
			testcase.expects(MockLinodeDomainsClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(t.Context(), testcase.clusterScope, "create")
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

func TestEnsureLinodeDNSDeletesRecordForDeletingCAPIMachine(t *testing.T) {
	t.Parallel()

	clusterScope := &scope.ClusterScope{
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
					LoadBalancerType:    "dns",
					DNSRootDomain:       "lkedevs.net",
					DNSUniqueIdentifier: "test-hash",
				},
			},
		},
		LinodeMachines: infrav1alpha2.LinodeMachineList{
			Items: []infrav1alpha2.LinodeMachine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "deleting-machine",
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Machine",
							Name:       "deleting-machine",
							UID:        "deleting-machine-uid",
						}},
					},
					Status: infrav1alpha2.LinodeMachineStatus{
						Addresses: []clusterv1.MachineAddress{{
							Type:    clusterv1.MachineExternalIP,
							Address: "10.10.10.10",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "active-machine",
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Machine",
							Name:       "active-machine",
							UID:        "active-machine-uid",
						}},
					},
					Status: infrav1alpha2.LinodeMachineStatus{
						Addresses: []clusterv1.MachineAddress{{
							Type:    clusterv1.MachineExternalIP,
							Address: "10.20.20.20",
						}},
					},
				},
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDNSClient := mock.NewMockLinodeClient(ctrl)
	clusterScope.LinodeDomainsClient = mockDNSClient
	mockDNSClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{{
		ID:     1,
		Domain: "lkedevs.net",
	}}, nil)
	mockDNSClient.EXPECT().ListDomainRecords(gomock.Any(), 1, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int, opts *linodego.ListOptions) ([]linodego.DomainRecord, error) {
			if strings.Contains(opts.Filter, `"name":"test-cluster-test-hash"`) && !strings.Contains(opts.Filter, `"target"`) {
				return []linodego.DomainRecord{
					{
						ID:     100,
						Type:   linodego.RecordTypeA,
						Name:   "test-cluster-test-hash",
						Target: "10.10.10.10",
					},
					{
						ID:     101,
						Type:   linodego.RecordTypeA,
						Name:   "test-cluster-test-hash",
						Target: "10.20.20.20",
					},
					{
						ID:     102,
						Type:   linodego.RecordTypeTXT,
						Name:   "test-cluster-test-hash",
						Target: "test-cluster",
					},
				}, nil
			}
			return []linodego.DomainRecord{{ID: 101}}, nil
		}).AnyTimes()
	mockDNSClient.EXPECT().DeleteDomainRecord(gomock.Any(), 1, 100).Return(nil)

	mockK8sClient := mock.NewMockK8sClient(ctrl)
	clusterScope.Client = mockK8sClient
	mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
	mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			machine, ok := obj.(*clusterv1.Machine)
			if !ok {
				return nil
			}

			machine.Name = key.Name
			machine.Namespace = key.Namespace
			switch key.Name {
			case "deleting-machine":
				deletionTime := metav1.Now()
				machine.DeletionTimestamp = &deletionTime
				machine.UID = "deleting-machine-uid"
			case "active-machine":
				machine.UID = "active-machine-uid"
				machine.Status.Conditions = []metav1.Condition{{
					Type:   clusterv1.ReadyCondition,
					Status: metav1.ConditionTrue,
				}}
			}
			return nil
		}).AnyTimes()

	require.NoError(t, EnsureDNSEntries(t.Context(), clusterScope, "create"))
}

func TestDeleteIPFromDNS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		clusterScope    *scope.ClusterScope
		expects         func(*mock.MockLinodeClient)
		expectK8sClient func(*mock.MockK8sClient)
		expectedError   error
	}{
		{
			name: "Success - Deleted the record",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{
					{
						ID:     1234,
						Type:   "A",
						Name:   "test-cluster",
						TTLSec: 30,
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().DeleteDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Failure - Deleting the record fails",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{
					{
						ID:     1234,
						Type:   "A",
						Name:   "test-cluster",
						TTLSec: 30,
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().DeleteDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to delete record")).AnyTimes()
			},
			expectedError: fmt.Errorf("failed to delete record"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Error - failed to get machine",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{
					{
						ID:     1234,
						Type:   "A",
						Name:   "test-cluster",
						TTLSec: 30,
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().DeleteDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Error - failure in getting domain",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot get the domain from the api")).AnyTimes()
			},
			expectedError: fmt.Errorf("cannot get the domain from the api"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Error - no domain found when deleting",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "test.net",
					},
				}, nil).AnyTimes()
			},
			expectedError: fmt.Errorf("domain lkedevs.net not found in list of domains owned by this account"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
			},
		},
		{
			name: "Error - error listing domains when deleting",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachines: infrav1alpha2.LinodeMachineList{
					Items: []infrav1alpha2.LinodeMachine{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-machine",
								UID:  "test-uid",
								OwnerReferences: []metav1.OwnerReference{
									{
										APIVersion: "cluster.x-k8s.io/v1beta1",
										Kind:       "Machine",
										Name:       "test-machine",
										UID:        "test-uid",
									},
								},
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
							Status: infrav1alpha2.LinodeMachineStatus{
								Addresses: []clusterv1.MachineAddress{
									{
										Type:    "ExternalIP",
										Address: "10.10.10.10",
									},
									{
										Type:    "ExternalIP",
										Address: "fd00::",
									},
								},
							},
						},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("api error")).AnyTimes()
			},
			expectedError: fmt.Errorf("api error"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockCAPIMachine(mockK8sClient, nil, nil)
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
			MockLinodeDomainsClient := mock.NewMockLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.clusterScope.LinodeDomainsClient = MockLinodeClient

			testcase.expects(MockLinodeClient)
			testcase.expects(MockLinodeDomainsClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(t.Context(), testcase.clusterScope, "delete")
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

// mockCAPIMachine sets up the k8s client mock to return a CAPI machine for GetOwnerMachine
func mockCAPIMachine(mockK8sClient *mock.MockK8sClient, ownerRefs []metav1.OwnerReference, machineStatus *clusterv1.MachineStatus) {
	mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
	// Mock the Get call for GetOwnerMachine to return a CAPI machine
	mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			// Set the Machine fields to make it not deleted
			machine, ok := obj.(*clusterv1.Machine)
			if ok {
				machine.Name = "test-machine"
				machine.Namespace = "default"
				machine.DeletionTimestamp = nil
				machine.UID = "test-uid"
				machine.OwnerReferences = ownerRefs
				if machineStatus != nil {
					machine.SetConditions(machineStatus.Conditions)
				}
			}
			return nil
		}).AnyTimes()
}
