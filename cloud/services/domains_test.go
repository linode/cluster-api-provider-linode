package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
)

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
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Not Found")).AnyTimes()
				mockClient.EXPECT().CreateRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("Not Found")).AnyTimes()
				mockClient.EXPECT().CreateRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("create record failed")).AnyTimes()
			},
			expectedError: fmt.Errorf("create record failed"),
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

			MockAkamClient := mock.NewMockAkamClient(ctrl)
			testcase.clusterScope.AkamaiDomainsClient = MockAkamClient
			testcase.expects(MockAkamClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(context.Background(), testcase.clusterScope, "create")
			if testcase.expectedError != nil {
				require.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NoError(t, err)
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
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&dns.RecordBody{
					Name:       "test-machine",
					RecordType: "A",
					TTL:        30,
					Target:     []string{"10.10.10.10"},
				}, nil).AnyTimes()
				mockClient.EXPECT().UpdateRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockClient.EXPECT().DeleteRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil,
			expectedList:  []string{"10.10.10.10", "10.10.10.12"},
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockClient.EXPECT().GetRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("API Down")).AnyTimes()
				mockClient.EXPECT().DeleteRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: fmt.Errorf("API Down"),
			expectedList:  []string{"10.10.10.10", "10.10.10.12"},
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

			MockAkamClient := mock.NewMockAkamClient(ctrl)
			testcase.clusterScope.AkamaiDomainsClient = MockAkamClient
			testcase.expects(MockAkamClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(context.Background(), testcase.clusterScope, "delete")
			if err != nil || testcase.expectedError != nil {
				require.ErrorContains(t, err, testcase.expectedError.Error())
			}
			assert.EqualValues(t, testcase.expectedList, removeElement(testcase.listOfIPS, "10.10.10.11"))
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
			},
			expectedError: nil,
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
			},
			expectedError: fmt.Errorf("dnsEntries are empty"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
			MockLinodeDomainsClient := mock.NewMockLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.clusterScope.LinodeDomainsClient = MockLinodeClient

			testcase.expects(MockLinodeClient)
			testcase.expects(MockLinodeDomainsClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(context.Background(), testcase.clusterScope, "create")
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
			},
		},
		{
			name: "Error - failed to get machine ip",
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
							},
							Spec: infrav1alpha2.LinodeMachineSpec{
								ProviderID: ptr.To("linode://123"),
								InstanceID: ptr.To(123),
							},
						},
					},
				},
			},
			expects:       func(mockClient *mock.MockLinodeClient) {},
			expectedError: fmt.Errorf("dnsEntries are empty"),
			expectK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
				mockK8sClient.EXPECT().Scheme().Return(nil).AnyTimes()
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
			MockLinodeDomainsClient := mock.NewMockLinodeClient(ctrl)

			testcase.clusterScope.LinodeClient = MockLinodeClient
			testcase.clusterScope.LinodeDomainsClient = MockLinodeClient

			testcase.expects(MockLinodeClient)
			testcase.expects(MockLinodeDomainsClient)

			MockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.clusterScope.Client = MockK8sClient
			testcase.expectK8sClient(MockK8sClient)

			err := EnsureDNSEntries(context.Background(), testcase.clusterScope, "delete")
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}
