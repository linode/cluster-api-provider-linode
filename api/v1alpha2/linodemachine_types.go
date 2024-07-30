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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows ReconcileLinodeMachine to clean up Linode resources associated
	// with LinodeMachine before removing it from the apiserver.
	MachineFinalizer = "linodemachine.infrastructure.cluster.x-k8s.io"
)

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
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Group string `json:"group,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	RootPass string `json:"rootPass,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	AuthorizedKeys []string `json:"authorizedKeys,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	AuthorizedUsers []string `json:"authorizedUsers,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	BackupID int `json:"backupID,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Interfaces []InstanceConfigInterfaceCreateOptions `json:"interfaces,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	BackupsEnabled bool `json:"backupsEnabled,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	PrivateIP *bool `json:"privateIP,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Tags []string `json:"tags,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	FirewallID int `json:"firewallID,omitempty"`
	// OSDisk is configuration for the root disk that includes the OS,
	// if not specified this defaults to whatever space is not taken up by the DataDisks
	OSDisk *InstanceDisk `json:"osDisk,omitempty"`
	// DataDisks is a map of any additional disks to add to an instance,
	// The sum of these disks + the OSDisk must not be more than allowed on a linodes plan
	DataDisks map[string]*InstanceDisk `json:"dataDisks,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:Enum=enabled;disabled
	// DiskEncryption determines if the disks of the instance should be encrypted.
	DiskEncryption string `json:"diskEncryption,omitempty"`

	// CredentialsRef is a reference to a Secret that contains the credentials
	// to use for provisioning this machine. If not supplied then these
	// credentials will be used in-order:
	//   1. LinodeMachine
	//   2. Owner LinodeCluster
	//   3. Controller
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`

	// Configuration is the Akamai instance configuration OS,
	// if not specified this defaults to the default configuration associated to the instance.
	Configuration *InstanceConfiguration `json:"configuration,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	// PlacementGroupRef is a reference to a placement group object. This makes the linode to be launched in that specific group.
	PlacementGroupRef *corev1.ObjectReference `json:"placementGroupRef,omitempty"`
}

// InstanceDisk defines a list of disks to use for an instance
type InstanceDisk struct {
	// DiskID is the linode assigned ID of the disk
	DiskID int `json:"diskID,omitempty"`
	// Size of the disk in resource.Quantity notation
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`
	// Label for the instance disk, if nothing is provided it will match the device name
	Label string `json:"label,omitempty"`
	// Filesystem of disk to provision, the default disk filesystem is "ext4"
	// +kubebuilder:validation:Enum=raw;swap;ext3;ext4;initrd
	Filesystem string `json:"filesystem,omitempty"`
}

// InstanceMetadataOptions defines metadata of instance
type InstanceMetadataOptions struct {
	// UserData expects a Base64-encoded string
	UserData string `json:"userData,omitempty"`
}

// InstanceConfiguration defines the instance configuration
type InstanceConfiguration struct {
	// Kernel is a Kernel ID to boot a Linode with. (e.g linode/latest-64bit)
	Kernel string `json:"kernel,omitempty"`
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

	// ClusterNetworkSpec is the type of LoadBalancer being used
	ClusterNetworkSpec NetworkSpec `json:"clusterNetworkSpec,omitempty"`

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
// +kubebuilder:storageversion
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

// +kubebuilder:object:root=true

// LinodeMachineList contains a list of LinodeMachine
type LinodeMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeMachine{}, &LinodeMachineList{})
}
