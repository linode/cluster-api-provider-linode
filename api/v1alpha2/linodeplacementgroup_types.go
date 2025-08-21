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
)

const (
	// PlacementGroupFinalizer allows ReconcileLinodePG to clean up Linode resources associated
	// with LinodePlacementGroup before removing it from the apiserver.
	PlacementGroupFinalizer = "linodeplacementgroup.infrastructure.cluster.x-k8s.io"
)

// LinodePlacementGroupSpec defines the desired state of LinodePlacementGroup
type LinodePlacementGroupSpec struct {
	// pgID is the ID of the PlacementGroup.
	// +optional
	PGID *int `json:"pgID,omitempty"`
	// region is the Linode region to create the PlacementGroup in.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +required
	Region string `json:"region"`
	// placementGroupPolicy defines the policy for the PlacementGroup.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:default="strict"
	// +kubebuilder:validation:Enum=strict;flexible
	// +optional
	PlacementGroupPolicy string `json:"placementGroupPolicy"`

	// placementGroupType defines the type of the PlacementGroup.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:default="anti_affinity:local"
	// +kubebuilder:validation:Enum="anti_affinity:local"
	// +optional
	PlacementGroupType string `json:"placementGroupType"`
	// TODO: add affinity as a type when available

	// credentialsRef is a reference to a Secret that contains the credentials to use for provisioning this PlacementGroup.
	// If not supplied, then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`
}

// LinodePlacementGroupStatus defines the observed state of LinodePlacementGroup
type LinodePlacementGroupStatus struct {
	// ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// failureReason will be set in the event that there is a terminal problem
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

	// failureMessage will be set in the event that there is a terminal problem
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

	// conditions defines current service state of the LinodePlacementGroup.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodeplacementgroups,scope=Namespaced,categories=cluster-api,shortName=lpg
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="PlacementGroup is ready"
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"

// LinodePlacementGroup is the Schema for the linodeplacementgroups API
type LinodePlacementGroup struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the LinodePlacementGroup.
	// +optional
	Spec LinodePlacementGroupSpec `json:"spec,omitempty"`

	// status is the observed state of the LinodePlacementGroup.
	// +optional
	Status LinodePlacementGroupStatus `json:"status,omitempty"`
}

func (lpg *LinodePlacementGroup) GetConditions() []metav1.Condition {
	for i := range lpg.Status.Conditions {
		if lpg.Status.Conditions[i].Reason == "" {
			lpg.Status.Conditions[i].Reason = DefaultConditionReason
		}
	}
	return lpg.Status.Conditions
}

func (lpg *LinodePlacementGroup) SetConditions(conditions []metav1.Condition) {
	lpg.Status.Conditions = conditions
}

func (lpg *LinodePlacementGroup) GetV1Beta2Conditions() []metav1.Condition {
	return lpg.GetConditions()
}

func (lpg *LinodePlacementGroup) SetV1Beta2Conditions(conditions []metav1.Condition) {
	lpg.SetConditions(conditions)
}

// +kubebuilder:object:root=true

// LinodePlacementGroupList contains a list of LinodePlacementGroup
type LinodePlacementGroupList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of LinodePlacementGroup.
	Items []LinodePlacementGroup `json:"items"`
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
