package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestAddIPToDNS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		machineScope         *scope.MachineScope
		expects              func(*mock.MockLinodeClient)
		expectedDomainRecord *linodego.DomainRecord
		expectedError        error
	}{
		{
			name: "Success - If the machine is a control plane node, add the IP to the Domain",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
		},
		{
			name: "Success - use custom dnsttlsec",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
							DNSTTLSec:           100,
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
		},
		{
			name: "Error - CreateDomainRecord() returns an error",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
		},
		{
			name: "Success - If the machine is a control plane node and record already exists, update it",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
				mockClient.EXPECT().CreateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				mockClient.EXPECT().UpdateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.DomainRecord{
					ID:     1234,
					Type:   "A",
					Name:   "test-cluster",
					TTLSec: 30,
				}, nil).AnyTimes()
			},
			expectedError: nil,
		},
		{
			name: "Failure - Failed to get domain records",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
					{
						ID:     1,
						Domain: "lkedevs.net",
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("api error")).AnyTimes()
			},
			expectedError: fmt.Errorf("api error"),
		},
		{
			name: "Error - UpdateDomainRecord fails",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
				mockClient.EXPECT().CreateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				mockClient.EXPECT().UpdateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed to update domain record of type A")).AnyTimes()
			},
			expectedError: fmt.Errorf("failed to update domain record of type A"),
		},
		{
			name: "Error - failed to get machine ip",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed to get public ipv4 ip address")).AnyTimes()
			},
			expectedError: fmt.Errorf("failed to get public ipv4 ip address"),
		},
		{
			name: "Error - no SLAAC address for machine ip",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: nil,
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
				mockClient.EXPECT().UpdateDomainRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.DomainRecord{
					ID:     1234,
					Type:   "A",
					Name:   "test-cluster",
					TTLSec: 30,
				}, nil).AnyTimes()
			},
			expectedError: fmt.Errorf("no SLAAC address"),
		},
		{
			name: "Error - no public ip set",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{},
					},
				}, nil).AnyTimes()
			},
			expectedError: fmt.Errorf("no public address"),
		},
		{
			name: "Error - failure in getting domain",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot get the domain from the api")).AnyTimes()
			},
			expectedError: fmt.Errorf("cannot get the domain from the api"),
		},
		{
			name: "Error - no domain found",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{}, nil).AnyTimes()
			},
			expectedError: fmt.Errorf("domain lkedevs.net not found in list of domains owned by this account"),
		},
		{
			name: "Error - no instance id",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machine",
						UID:  "test-uid",
					},
					Spec: infrav1alpha1.LinodeMachineSpec{},
				},
			},
			expects:       func(mockClient *mock.MockLinodeClient) {},
			expectedError: fmt.Errorf("instance ID is nil. cant get machine's public ip"),
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

			testcase.machineScope.LinodeClient = MockLinodeClient
			testcase.machineScope.LinodeDomainsClient = MockLinodeClient

			testcase.expects(MockLinodeClient)
			testcase.expects(MockLinodeDomainsClient)

			err := AddIPToDNS(context.Background(), testcase.machineScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}

func TestDeleteIPFromDNS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		expects       func(*mock.MockLinodeClient)
		expectedError error
	}{
		{
			name: "Success - Deleted the record",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
		},
		{
			name: "Failure - Deleting the record fails",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
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
		},
		{
			name: "Error - failed to get machine ip",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed to get public ipv4 ip address")).AnyTimes()
			},
			expectedError: fmt.Errorf("failed to get public ipv4 ip address"),
		},
		{
			name: "Error - failure in getting domain",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot get the domain from the api")).AnyTimes()
			},
			expectedError: fmt.Errorf("cannot get the domain from the api"),
		},
		{
			name: "Error - no domain found",
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
							LoadBalancerType:    "dns",
							DNSRootDomain:       "lkedevs.net",
							DNSUniqueIdentifier: "test-hash",
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
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), gomock.Any()).Return(&linodego.InstanceIPAddressResponse{
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{
							{
								Address: "1.2.3.4",
							},
						},
					},
				}, nil).AnyTimes()
				mockClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{}, nil).AnyTimes()
			},
			expectedError: fmt.Errorf("domain lkedevs.net not found in list of domains owned by this account"),
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

			testcase.machineScope.LinodeClient = MockLinodeClient
			testcase.machineScope.LinodeDomainsClient = MockLinodeClient

			testcase.expects(MockLinodeClient)
			testcase.expects(MockLinodeDomainsClient)

			err := DeleteIPFromDNS(context.Background(), testcase.machineScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}
