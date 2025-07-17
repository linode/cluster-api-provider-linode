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
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// VPCFinalizer allows ReconcileLinodeVPC to clean up Linode resources associated
	// with LinodeVPC before removing it from the apiserver.
	VPCFinalizer = "linodevpc.infrastructure.cluster.x-k8s.io"
)

// LinodeVPCSpec defines the desired state of LinodeVPC
type LinodeVPCSpec struct {
	// +optional
	VPCID *int `json:"vpcID,omitempty"`
	// +optional
	Description string `json:"description,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Region string `json:"region"`
	// +optional
	// IPv6 is a list of IPv6 ranges allocated to the VPC.
	// Once ranges are allocated based on the IPv6Range field, they will be
	// added to this field.
	IPv6 []linodego.VPCIPv6Range `json:"ipv6,omitempty"`
	// +optional
	// IPv6Range is a list of IPv6 ranges to allocate to the VPC.
	// If not specified, the VPC will not have an IPv6 range allocated.
	// Once ranges are allocated, they will be added to the IPv6 field.
	IPv6Range []VPCCreateOptionsIPv6 `json:"ipv6Range,omitempty"`
	// +optional
	Subnets []VPCSubnetCreateOptions `json:"subnets,omitempty"`

	// Retain allows you to keep the VPC after the LinodeVPC object is deleted.
	// This is useful if you want to use an existing VPC that was not created by this controller.
	// If set to true, the controller will not delete the VPC resource in Linode.
	// Defaults to false.
	// +optional
	// +kubebuilder:default=false
	Retain bool `json:"retain,omitempty"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this VPC. If not
	// supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`
}

// VPCCreateOptionsIPv6 defines the options for creating an IPv6 range in a VPC.
// It's copied from linodego.VPCCreateOptionsIPv6 and should be kept in sync.
// Values supported by the linode API should be used here.
// See https://techdocs.akamai.com/linode-api/reference/post-vpc for more details.
type VPCCreateOptionsIPv6 struct {
	// Range is the IPv6 prefix for the VPC.
	Range *string `json:"range,omitempty"`
	// IPv6 inventory from which the VPC prefix should be allocated.
	AllocationClass *string `json:"allocation_class,omitempty"`
}

// VPCSubnetCreateOptions defines subnet options
type VPCSubnetCreateOptions struct {
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Label string `json:"label,omitempty"`
	// +optional
	IPv4 string `json:"ipv4,omitempty"`
	// +optional
	// IPv6 is a list of IPv6 ranges allocated to the subnet.
	// Once ranges are allocated based on the IPv6Range field, they will be
	// added to this field.
	IPv6 []linodego.VPCIPv6Range `json:"ipv6,omitempty"`
	// +optional
	// IPv6Range is a list of IPv6 ranges to allocate to the subnet.
	// If not specified, the subnet will not have an IPv6 range allocated.
	// Once ranges are allocated, they will be added to the IPv6 field.
	IPv6Range []VPCSubnetCreateOptionsIPv6 `json:"ipv6Range,omitempty"`
	// SubnetID is subnet id for the subnet
	// +optional
	SubnetID int `json:"subnetID,omitempty"`
	// Retain allows you to keep the Subnet after the LinodeVPC object is deleted.
	// This is only applicable when the parent VPC has retain set to true.
	// +optional
	// +kubebuilder:default=false
	Retain bool `json:"retain,omitempty"`
}

// VPCSubnetCreateOptionsIPv6 defines the options for creating an IPv6 range in a VPC subnet.
// It's copied from linodego.VPCSubnetCreateOptionsIPv6 and should be kept in sync.
// Values supported by the linode API should be used here.
// See https://techdocs.akamai.com/linode-api/reference/post-vpc-subnet for more details.
type VPCSubnetCreateOptionsIPv6 struct {
	Range *string `json:"range,omitempty"`
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
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodevpcs,scope=Namespaced,categories=cluster-api,shortName=lvpc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="VPC is ready"
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"
// +kubebuilder:storageversion

// LinodeVPC is the Schema for the linodemachines API
type LinodeVPC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeVPCSpec   `json:"spec,omitempty"`
	Status LinodeVPCStatus `json:"status,omitempty"`
}

func (lv *LinodeVPC) GetConditions() []metav1.Condition {
	for i := range lv.Status.Conditions {
		if lv.Status.Conditions[i].Reason == "" {
			lv.Status.Conditions[i].Reason = DefaultConditionReason
		}
	}
	return lv.Status.Conditions
}

func (lv *LinodeVPC) SetConditions(conditions []metav1.Condition) {
	lv.Status.Conditions = conditions
}

func (lv *LinodeVPC) GetV1Beta2Conditions() []metav1.Condition {
	return lv.GetConditions()
}

func (lv *LinodeVPC) SetV1Beta2Conditions(conditions []metav1.Condition) {
	lv.SetConditions(conditions)
}

// +kubebuilder:object:root=true

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
