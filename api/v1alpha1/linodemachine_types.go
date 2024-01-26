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
	"github.com/linode/linodego"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LinodeMachineSpec defines the desired state of LinodeMachine
type LinodeMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`
	// InstanceID is the Linode instance ID for this machine.
	// +optional
	InstanceID *int `json:"instanceID,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Region string `json:"region"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Type string `json:"type"`
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	Label string `json:"label,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Group string `json:"group,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	RootPass string `json:"rootPass,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	AuthorizedKeys []string `json:"authorizedKeys,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	AuthorizedUsers []string `json:"authorizedUsers,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	StackScriptID int `json:"stackscriptId,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	StackScriptData map[string]string `json:"stackscriptData,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	BackupID int `json:"backupId,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Interfaces []InstanceConfigInterfaceCreateOptions `json:"interfaces,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	BackupsEnabled bool `json:"backupsEnabled,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	PrivateIP bool `json:"privateIp,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Tags []string `json:"tags,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	Metadata *InstanceMetadataOptions `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	FirewallID int `json:"firewallId,omitempty"`
}

// InstanceMetadataOptions defines metadata of instance
type InstanceMetadataOptions struct {
	// UserData expects a Base64-encoded string
	UserData string `json:"userData,omitempty"`
}

// InstanceConfigInterfaceCreateOptions defines network interface config
type InstanceConfigInterfaceCreateOptions struct {
	IPAMAddress string `json:"ipamAddress,omitempty"`
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Label   string                          `json:"label,omitempty"`
	Purpose linodego.ConfigInterfacePurpose `json:"purpose,omitempty"`
	Primary bool                            `json:"primary,omitempty"`
	// +optional
	SubnetID *int `json:"subnetId,omitempty"`
	// +optional
	IPv4     *VPCIPv4 `json:"ipv4,omitempty"`
	IPRanges []string `json:"ipRanges,omitempty"`
}

// VPCIPv4 defines VPC IPV4 settings
type VPCIPv4 struct {
	VPC     string `json:"vpc,omitempty"`
	NAT1To1 string `json:"nat1to1,omitempty"`
}

// LinodeMachineStatus defines the observed state of LinodeMachine
type LinodeMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// Addresses contains the Linode instance associated addresses.
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// InstanceState is the state of the Linode instance for this machine.
	// +optional
	InstanceState *linodego.InstanceStatus `json:"instanceState,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the LinodeMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodemachines,scope=Namespaced,categories=cluster-api,shortName=lm
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this LinodeMachine belongs"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.instanceState",description="Linode instance state"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="InstanceID",type="string",JSONPath=".spec.providerID",description="Linode instance ID"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this LinodeMachine"

// LinodeMachine is the Schema for the linodemachines API
type LinodeMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeMachineSpec   `json:"spec,omitempty"`
	Status LinodeMachineStatus `json:"status,omitempty"`
}

func (lm *LinodeMachine) GetConditions() clusterv1.Conditions {
	return lm.Status.Conditions
}

func (lm *LinodeMachine) SetConditions(conditions clusterv1.Conditions) {
	lm.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// LinodeMachineList contains a list of LinodeMachine
type LinodeMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeMachine{}, &LinodeMachineList{})
}
