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
	// PlacementGroupFinalizer allows ReconcileLinodePG to clean up Linode resources associated
	// with LinodePlacementGroup before removing it from the apiserver.
	PlacementGroupFinalizer = "linodeplacementgroup.infrastructure.cluster.x-k8s.io"
)

// LinodePlacementGroupSpec defines the desired state of LinodePlacementGroup
type LinodePlacementGroupSpec struct {
	// +optional
	PGID *int `json:"pgID,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Region string `json:"region"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:default=true
	// +optional
	IsStrict bool `json:"isStrict"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:default="anti_affinity:local"
	// +kubebuilder:validation:Enum="anti_affinity:local"
	// +optional
	AffinityType string `json:"affinityType"`
	// TODO: add affinity as a type when available

	// CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this PlacementGroup. If not
	// supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`
}

// LinodePlacementGroupStatus defines the observed state of LinodePlacementGroup
type LinodePlacementGroupStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the PlacementGroup and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the PlacementGroup's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of PlacementGroups
	// can be added as events to the PlacementGroup object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *LinodePlacementGroupStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the PlacementGroup and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the PlacementGroup's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of PlacementGroups
	// can be added as events to the PlacementGroup object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the LinodePlacementGroup.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodeplacementgroups,scope=Namespaced,categories=cluster-api,shortName=lpg
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="PlacementGroup is ready"
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"

// LinodePlacementGroup is the Schema for the linodeplacementgroups API
type LinodePlacementGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodePlacementGroupSpec   `json:"spec,omitempty"`
	Status LinodePlacementGroupStatus `json:"status,omitempty"`
}

func (lm *LinodePlacementGroup) GetConditions() clusterv1.Conditions {
	return lm.Status.Conditions
}

func (lm *LinodePlacementGroup) SetConditions(conditions clusterv1.Conditions) {
	lm.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// LinodePlacementGroupList contains a list of LinodePlacementGroup
type LinodePlacementGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodePlacementGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodePlacementGroup{}, &LinodePlacementGroupList{})
}

// LinodePlacementGroupStatusError defines errors states for PlacementGroup objects.
type LinodePlacementGroupStatusError string

const (
	// CreatePlacementGroupError indicates that an error was encountered
	// when trying to create the PlacementGroup.
	CreatePlacementGroupError LinodePlacementGroupStatusError = "CreateError"

	// DeletePlacementGroupError indicates that an error was encountered
	// when trying to delete the PlacementGroup.
	DeletePlacementGroupError LinodePlacementGroupStatusError = "DeleteError"
)
