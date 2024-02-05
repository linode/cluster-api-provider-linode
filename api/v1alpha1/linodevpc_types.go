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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LinodeVPCSpec defines the desired state of LinodeVPC
type LinodeVPCSpec struct {
	// +optional
	VPCID *int `json:"vpcID,omitempty"`

	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	Label string `json:"label,omitempty"`
	// +optional
	Description string `json:"description,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Region string `json:"region"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	Subnets []VPCSubnetCreateOptions `json:"subnets,omitempty"`
}

// VPCSubnetCreateOptions defines subnet options
type VPCSubnetCreateOptions struct {
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Label string `json:"label,omitempty"`
	// +optional
	IPv4 string `json:"ipv4,omitempty"`
}

// LinodeVPCStatus defines the observed state of LinodeVPC
type LinodeVPCStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the VPC and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the VPC's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of VPCs
	// can be added as events to the VPC object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *VPCStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the VPC and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the VPC's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of VPCs
	// can be added as events to the VPC object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the LinodeVPC.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// LinodeVPC is the Schema for the linodemachines API
type LinodeVPC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeVPCSpec   `json:"spec,omitempty"`
	Status LinodeVPCStatus `json:"status,omitempty"`
}

func (lm *LinodeVPC) GetConditions() clusterv1.Conditions {
	return lm.Status.Conditions
}

func (lm *LinodeVPC) SetConditions(conditions clusterv1.Conditions) {
	lm.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// LinodeVPCList contains a list of LinodeVPC
type LinodeVPCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeVPC `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeVPC{}, &LinodeVPCList{})
}

// VPCStatusError defines errors states for VPC objects.
type VPCStatusError string

const (
	// CreateVPCError indicates that an error was encountered
	// when trying to create the VPC.
	CreateVPCError VPCStatusError = "CreateError"

	// UpdateVPCError indicates that an error was encountered
	// when trying to update the VPC.
	UpdateVPCError VPCStatusError = "UpdateError"

	// DeleteVPCError indicates that an error was encountered
	// when trying to delete the VPC.
	DeleteVPCError VPCStatusError = "DeleteError"
)
