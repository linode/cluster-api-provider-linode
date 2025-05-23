/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows ReconcileLinodeCluster to clean up Linode resources associated
	// with LinodeCluster before removing it from the apiserver.
	ClusterFinalizer = "linodecluster.infrastructure.cluster.x-k8s.io"
)

// LinodeClusterSpec defines the desired state of LinodeCluster
type LinodeClusterSpec struct {
	// The Linode Region the LinodeCluster lives in.
	Region string `json:"region"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the LinodeCluster control plane.
	// If ControlPlaneEndpoint is unset then the Nodebalancer ip will be used.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// NetworkSpec encapsulates all things related to Linode network.
	// +optional
	Network NetworkSpec `json:"network"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	VPCRef *corev1.ObjectReference `json:"vpcRef,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// VPCID is the ID of an existing VPC in Linode. This allows using a VPC that is not managed by CAPL.
	// +optional
	VPCID *int `json:"vpcID,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	// NodeBalancerFirewallRef is a reference to a NodeBalancer Firewall object. This makes the linode use the specified NodeBalancer Firewall.
	NodeBalancerFirewallRef *corev1.ObjectReference `json:"nodeBalancerFirewallRef,omitempty"`

	// ObjectStore defines a supporting Object Storage bucket for cluster operations. This is currently used for
	// bootstrapping (e.g. Cloud-init).
	// +optional
	ObjectStore *ObjectStore `json:"objectStore,omitempty"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this cluster. If not
	// supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`
}

// LinodeClusterStatus defines the observed state of LinodeCluster
type LinodeClusterStatus struct {
	// Ready denotes that the cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the LinodeCluster and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the LinodeCluster and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the LinodeCluster.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodeclusters,scope=Namespaced,categories=cluster-api,shortName=lc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this LinodeCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for Linode instances"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.ControlPlaneEndpoint",description="API Endpoint",priority=1
// +kubebuilder:storageversion

// LinodeCluster is the Schema for the linodeclusters API
type LinodeCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeClusterSpec   `json:"spec,omitempty"`
	Status LinodeClusterStatus `json:"status,omitempty"`
}

func (lc *LinodeCluster) GetConditions() []metav1.Condition {
	for i := range lc.Status.Conditions {
		if lc.Status.Conditions[i].Reason == "" {
			lc.Status.Conditions[i].Reason = DefaultConditionReason
		}
	}
	return lc.Status.Conditions
}

func (lc *LinodeCluster) SetConditions(conditions []metav1.Condition) {
	lc.Status.Conditions = conditions
}

// We need V1Beta2Conditions helpers to be able to use the conditions package from cluster-api
func (lc *LinodeCluster) GetV1Beta2Conditions() []metav1.Condition {
	return lc.GetConditions()
}

func (lc *LinodeCluster) SetV1Beta2Conditions(conditions []metav1.Condition) {
	lc.SetConditions(conditions)
}

// NetworkSpec encapsulates Linode networking resources.
type NetworkSpec struct {
	// LoadBalancerType is the type of load balancer to use, defaults to NodeBalancer if not otherwise set
	// +kubebuilder:validation:Enum=NodeBalancer;dns;external
	// +kubebuilder:default=NodeBalancer
	// +optional
	LoadBalancerType string `json:"loadBalancerType,omitempty"`
	// DNSProvider is provider who manages the domain
	// Ignored if the LoadBalancerType is set to anything other than dns
	// If not set, defaults linode dns
	// +kubebuilder:validation:Enum=linode;akamai
	// +optional
	DNSProvider string `json:"dnsProvider,omitempty"`
	// DNSRootDomain is the root domain used to create a DNS entry for the control-plane endpoint
	// Ignored if the LoadBalancerType is set to anything other than dns
	// +optional
	DNSRootDomain string `json:"dnsRootDomain,omitempty"`
	// DNSUniqueIdentifier is the unique identifier for the DNS. This let clusters with the same name have unique
	// DNS record
	// Ignored if the LoadBalancerType is set to anything other than dns
	// If not set, CAPL will create a unique identifier for you
	// +optional
	DNSUniqueIdentifier string `json:"dnsUniqueIdentifier,omitempty"`
	// DNSTTLSec is the TTL for the domain record
	// Ignored if the LoadBalancerType is set to anything other than dns
	// If not set, defaults to 30
	// +optional
	DNSTTLSec int `json:"dnsTTLsec,omitempty"`
	// DNSSubDomainOverride is used to override CAPL's construction of the controlplane endpoint
	// If set, this will override the DNS subdomain from <clustername>-<uniqueid>.<rootdomain> to <overridevalue>.<rootdomain>
	// +optional
	DNSSubDomainOverride string `json:"dnsSubDomainOverride,omitempty"`
	// apiserverLoadBalancerPort used by the api server. It must be valid ports range (1-65535).
	// If omitted, default value is 6443.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	ApiserverLoadBalancerPort int `json:"apiserverLoadBalancerPort,omitempty"`
	// NodeBalancerID is the id of NodeBalancer.
	// +optional
	NodeBalancerID *int `json:"nodeBalancerID,omitempty"`
	// NodeBalancerFirewallID is the id of NodeBalancer Firewall.
	// +optional
	NodeBalancerFirewallID *int `json:"nodeBalancerFirewallID,omitempty"`
	// apiserverNodeBalancerConfigID is the config ID of api server NodeBalancer config.
	// +optional
	ApiserverNodeBalancerConfigID *int `json:"apiserverNodeBalancerConfigID,omitempty"`
	// additionalPorts contains list of ports to be configured with NodeBalancer.
	// +optional
	AdditionalPorts []LinodeNBPortConfig `json:"additionalPorts,omitempty"`
	// subnetName is the name/label of the VPC subnet to be used by the cluster
	// +optional
	SubnetName string `json:"subnetName,omitempty"`
	// UseVlan provisions a cluster that uses VLANs instead of VPCs. IPAM is managed internally.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	UseVlan bool `json:"useVlan,omitempty"`
	// NodeBalancerBackendIPv4Range is the subnet range we want to provide for creating nodebalancer in VPC.
	// example: 10.10.10.0/30
	// +optional
	NodeBalancerBackendIPv4Range string `json:"nodeBalancerBackendIPv4Range,omitempty"`
}

type LinodeNBPortConfig struct {
	// port configured on the NodeBalancer. It must be valid port range (1-65535).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int `json:"port"`
	// nodeBalancerConfigID is the config ID of port's NodeBalancer config.
	// +optional
	NodeBalancerConfigID *int `json:"nodeBalancerConfigID,omitempty"`
}

// ObjectStore defines a supporting Object Storage bucket for cluster operations. This is currently used for
// bootstrapping (e.g. Cloud-init).
type ObjectStore struct {
	// PresignedURLDuration defines the duration for which presigned URLs are valid.
	//
	// This is used to generate presigned URLs for S3 Bucket objects, which are used by
	// control-plane and worker nodes to fetch bootstrap data.
	//
	// +optional
	PresignedURLDuration *metav1.Duration `json:"presignedURLDuration,omitempty"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for accessing the Cluster Object Store.
	CredentialsRef corev1.SecretReference `json:"credentialsRef,omitempty"`
}

// +kubebuilder:object:root=true

// LinodeClusterList contains a list of LinodeCluster
type LinodeClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeCluster{}, &LinodeClusterList{})
}
