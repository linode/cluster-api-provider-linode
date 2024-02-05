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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
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
	FailureReason *errors.ClusterStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the LinodeCluster and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the LinodeCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodeclusters,scope=Namespaced,categories=cluster-api,shortName=lc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this LinodeCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for Linode instances"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.ControlPlaneEndpoint",description="API Endpoint",priority=1

// LinodeCluster is the Schema for the linodeclusters API
type LinodeCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeClusterSpec   `json:"spec,omitempty"`
	Status LinodeClusterStatus `json:"status,omitempty"`
}

func (lm *LinodeCluster) GetConditions() clusterv1.Conditions {
	return lm.Status.Conditions
}

func (lm *LinodeCluster) SetConditions(conditions clusterv1.Conditions) {
	lm.Status.Conditions = conditions
}

// NetworkSpec encapsulates Linode networking resources.
type NetworkSpec struct {
	// LoadBalancerType is the type of load balancer to use, defaults to NodeBalancer if not otherwise set
	// +kubebuilder:validation:Enum=NodeBalancer
	// +optional
	LoadBalancerType string `json:"loadBalancerType,omitempty"`
	// LoadBalancerPort used by the api server. It must be valid ports range (1-65535). If omitted, default value is 6443.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	LoadBalancerPort int `json:"loadBalancerPort,omitempty"`
	// NodeBalancerID is the id of api server NodeBalancer.
	// +optional
	NodeBalancerID int `json:"nodeBalancerID,omitempty"`
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
