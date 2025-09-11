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
)

const (
	// MachineFinalizer allows ReconcileLinodeMachine to clean up Linode resources associated
	// with LinodeMachine before removing it from the apiserver.
	MachineFinalizer       = "linodemachine.infrastructure.cluster.x-k8s.io"
	DefaultConditionReason = "None"
)

// LinodeMachineSpec defines the desired state of LinodeMachine
type LinodeMachineSpec struct {
	// providerID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`
	// instanceID is the Linode instance ID for this machine.
	// +optional
	// +kubebuilder:deprecatedversion:warning="ProviderID deprecates InstanceID"
	InstanceID *int `json:"instanceID,omitempty"`

	// region is the Linode region to create the instance in.
	// +kubebuilder:validation:MinLength=1
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Region string `json:"region,omitempty"`

	// type is the Linode instance type to create.
	// +kubebuilder:validation:MinLength=1
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Type string `json:"type,omitempty"`

	// group is the Linode group to create the instance in.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Group string `json:"group,omitempty"`

	// rootPass is the root password for the instance.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	RootPass string `json:"rootPass,omitempty"`

	// authorizedKeys is a list of SSH public keys to add to the instance.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +listType=set
	AuthorizedKeys []string `json:"authorizedKeys,omitempty"`

	// authorizedUsers is a list of usernames to add to the instance.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +listType=set
	AuthorizedUsers []string `json:"authorizedUsers,omitempty"`

	// backupID is the ID of the backup to restore the instance from.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	BackupID int `json:"backupID,omitempty"`

	// image is the Linode image to use for the instance.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Image string `json:"image,omitempty"`

	// interfaces is a list of legacy network interfaces to use for the instance.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +listType=atomic
	Interfaces []InstanceConfigInterfaceCreateOptions `json:"interfaces,omitempty"`

	// linodeInterfaces is a list of Linode network interfaces to use for the instance. Requires Linode Interfaces beta opt-in to use.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:object:generate=true
	// +listType=atomic
	LinodeInterfaces []LinodeInterfaceCreateOptions `json:"linodeInterfaces,omitempty"`

	// backupsEnabled is a boolean indicating whether backups should be enabled for the instance.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	BackupsEnabled bool `json:"backupsEnabled,omitempty"`

	// privateIP is a boolean indicating whether the instance should have a private IP address.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	PrivateIP *bool `json:"privateIP,omitempty"`

	// tags is a list of tags to apply to the Linode instance.
	// +optional
	// +listType=set
	Tags []string `json:"tags,omitempty"`

	// firewallID is the id of the cloud firewall to apply to the Linode Instance
	// +optional
	FirewallID int `json:"firewallID,omitempty"`

	// osDisk is a configuration for the root disk that includes the OS,
	// if not specified, this defaults to whatever space is not taken up by the DataDisks
	// +optional
	OSDisk *InstanceDisk `json:"osDisk,omitempty"`

	// dataDisks is a map of any additional disks to add to an instance,
	// The sum of these disks + the OSDisk must not be more than allowed on a linodes plan
	// +optional
	DataDisks *InstanceDisks `json:"dataDisks,omitempty"`

	// diskEncryption determines if the disks of the instance should be encrypted. The default is disabled.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:Enum=enabled;disabled
	DiskEncryption string `json:"diskEncryption,omitempty"`

	// credentialsRef is a reference to a Secret that contains the credentials
	// to use for provisioning this machine. If not supplied then these
	// credentials will be used in-order:
	//   1. LinodeMachine
	//   2. Owner LinodeCluster
	//   3. Controller
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`

	// configuration is the Akamai instance configuration OS,
	// if not specified, this defaults to the default configuration associated to the instance.
	// +optional
	Configuration *InstanceConfiguration `json:"configuration,omitempty"`

	// placementGroupRef is a reference to a placement group object. This makes the linode to be launched in that specific group.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	PlacementGroupRef *corev1.ObjectReference `json:"placementGroupRef,omitempty"`

	// firewallRef is a reference to a firewall object. This makes the linode use the specified firewall.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	FirewallRef *corev1.ObjectReference `json:"firewallRef,omitempty"`

	// vpcRef is a reference to a LinodeVPC resource. If specified, this takes precedence over
	// the cluster-level VPC configuration for multi-region support.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	VPCRef *corev1.ObjectReference `json:"vpcRef,omitempty"`

	// vpcID is the ID of an existing VPC in Linode. This allows using a VPC that is not managed by CAPL.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	VPCID *int `json:"vpcID,omitempty"`

	// ipv6Options defines the IPv6 options for the instance.
	// If not specified, IPv6 ranges won't be allocated to instance.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	IPv6Options *IPv6CreateOptions `json:"ipv6Options,omitempty"`

	// networkHelper is an option usually enabled on account level. It helps configure networking automatically for instances.
	// You can use this to enable/disable the network helper for a specific instance.
	// For more information, see https://techdocs.akamai.com/cloud-computing/docs/automatically-configure-networking
	// Defaults to true.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	NetworkHelper *bool `json:"networkHelper,omitempty"`

	// interfaceGeneration is the generation of the interface to use for the cluster's
	// nodes in interface / linodeInterface are not specified for a LinodeMachine.
	// If not set, defaults to "legacy_config".
	// +optional
	// +kubebuilder:validation:Enum=legacy_config;linode
	// +kubebuilder:default=legacy_config
	InterfaceGeneration linodego.InterfaceGeneration `json:"interfaceGeneration,omitempty"`
}

// IPv6CreateOptions defines the IPv6 options for the instance.
type IPv6CreateOptions struct {

	// enableSLAAC is an option to enable SLAAC (Stateless Address Autoconfiguration) for the instance.
	// This is useful for IPv6 addresses, allowing the instance to automatically configure its own IPv6 address.
	// Defaults to false.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	EnableSLAAC *bool `json:"enableSLAAC,omitempty"`

	// enableRanges is an option to enable IPv6 ranges for the instance.
	// If set to true, the instance will have a range of IPv6 addresses.
	// This is useful for instances that require multiple IPv6 addresses.
	// Defaults to false.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	EnableRanges *bool `json:"enableRanges,omitempty"`

	// isPublicIPv6 is an option to enable public IPv6 for the instance.
	// If set to true, the instance will have a publicly routable IPv6 range.
	// Defaults to false.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	IsPublicIPv6 *bool `json:"isPublicIPv6,omitempty"`
}

type InstanceDisks struct {
	// sdb is a disk for the instance.
	// +optional
	SDB *InstanceDisk `json:"sdb,omitempty"`
	// sdc is a disk for the instance.
	// +optional
	SDC *InstanceDisk `json:"sdc,omitempty"`
	// sdd is a disk for the instance.
	// +optional
	SDD *InstanceDisk `json:"sdd,omitempty"`
	// sde is a disk for the instance.
	// +optional
	SDE *InstanceDisk `json:"sde,omitempty"`
	// sdf is a disk for the instance.
	// +optional
	SDF *InstanceDisk `json:"sdf,omitempty"`
	// sdg is a disk for the instance.
	// +optional
	SDG *InstanceDisk `json:"sdg,omitempty"`
	// sdh is a disk for the instance.
	// +optional
	SDH *InstanceDisk `json:"sdh,omitempty"`
}

// InstanceDisk defines a list of disks to use for an instance
type InstanceDisk struct {
	// diskID is the linode assigned ID of the disk.
	// +optional
	DiskID int `json:"diskID,omitempty"`

	// size of the disk in resource.Quantity notation.
	// +required
	Size resource.Quantity `json:"size,omitempty"`

	// label for the instance disk, if nothing is provided, it will match the device name.
	// +optional
	Label string `json:"label,omitempty"`

	// filesystem of disk to provision, the default disk filesystem is "ext4".
	// +optional
	// +kubebuilder:validation:Enum=raw;swap;ext3;ext4;initrd
	Filesystem string `json:"filesystem,omitempty"`
}

// InstanceMetadataOptions defines metadata of instance
type InstanceMetadataOptions struct {
	// userData expects a Base64-encoded string.
	// +optional
	UserData string `json:"userData,omitempty"`
}

// InstanceConfiguration defines the instance configuration
type InstanceConfiguration struct {
	// kernel is a Kernel ID to boot a Linode with. (e.g linode/latest-64bit).
	// +optional
	Kernel string `json:"kernel,omitempty"`
}

// InstanceConfigInterfaceCreateOptions defines network interface config
type InstanceConfigInterfaceCreateOptions struct {
	// ipamAddress is the IP address to assign to the interface.
	// +optional
	IPAMAddress string `json:"ipamAddress,omitempty"`

	// label is the label of the interface.
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Label string `json:"label,omitempty"`

	// purpose is the purpose of the interface.
	// +optional
	Purpose linodego.ConfigInterfacePurpose `json:"purpose,omitempty"`

	// primary is a boolean indicating whether the interface is primary.
	// +optional
	Primary bool `json:"primary,omitempty"`

	// subnetId is the ID of the subnet to use for the interface.
	// +optional
	SubnetID *int `json:"subnetId,omitempty"`

	// ipv4 is the IPv4 configuration for the interface.
	// +optional
	IPv4 *VPCIPv4 `json:"ipv4,omitempty"`

	// ipRanges is a list of IPv4 ranges to assign to the interface.
	// +optional
	// +listType=set
	IPRanges []string `json:"ipRanges,omitempty"`
}

// LinodeInterfaceCreateOptions defines the linode network interface config
type LinodeInterfaceCreateOptions struct {
	// firewall_id is the ID of the firewall to use for the interface.
	// +optional
	FirewallID *int `json:"firewall_id,omitempty"`

	// default_route is the default route for the interface.
	// +optional
	DefaultRoute *InterfaceDefaultRoute `json:"default_route,omitempty"`

	// public is the public interface configuration for the interface.
	// +optional
	Public *PublicInterfaceCreateOptions `json:"public,omitempty"`

	// vpc is the VPC interface configuration for the interface.
	// +optional
	VPC *VPCInterfaceCreateOptions `json:"vpc,omitempty"`

	// vlan is the VLAN interface configuration for the interface.
	// +optional
	VLAN *VLANInterface `json:"vlan,omitempty"`
}

// InterfaceDefaultRoute defines the default IPv4 and IPv6 routes for an interface
type InterfaceDefaultRoute struct {
	// ipv4 is the IPv4 default route for the interface.
	// +optional
	IPv4 *bool `json:"ipv4,omitempty"`

	// ipv6 is the IPv6 default route for the interface.
	// +optional
	IPv6 *bool `json:"ipv6,omitempty"`
}

// PublicInterfaceCreateOptions defines the IPv4 and IPv6 public interface create options
type PublicInterfaceCreateOptions struct {
	// ipv4 is the IPv4 configuration for the public interface.
	// +optional
	IPv4 *PublicInterfaceIPv4CreateOptions `json:"ipv4,omitempty"`

	// ipv6 is the IPv6 configuration for the public interface.
	// +optional
	IPv6 *PublicInterfaceIPv6CreateOptions `json:"ipv6,omitempty"`
}

// PublicInterfaceIPv4CreateOptions defines the PublicInterfaceIPv4AddressCreateOptions for addresses
type PublicInterfaceIPv4CreateOptions struct {
	// addresses is the IPv4 addresses for the public interface.
	// +optional
	// +listType=map
	// +listMapKey=address
	Addresses []PublicInterfaceIPv4AddressCreateOptions `json:"addresses,omitempty"`
}

// PublicInterfaceIPv4AddressCreateOptions defines the public IPv4 address and whether it is primary
type PublicInterfaceIPv4AddressCreateOptions struct {
	// address is the IPv4 address for the public interface.
	// +kubebuilder:validation:MinLength=1
	// +required
	Address string `json:"address,omitempty"`

	// primary is a boolean indicating whether the address is primary.
	// +optional
	Primary *bool `json:"primary,omitempty"`
}

// PublicInterfaceIPv6CreateOptions defines the PublicInterfaceIPv6RangeCreateOptions
type PublicInterfaceIPv6CreateOptions struct {
	// ranges is the IPv6 ranges for the public interface.
	// +optional
	// +listType=map
	// +listMapKey=range
	Ranges []PublicInterfaceIPv6RangeCreateOptions `json:"ranges,omitempty"`
}

// PublicInterfaceIPv6RangeCreateOptions defines the IPv6 range for a public interface
type PublicInterfaceIPv6RangeCreateOptions struct {
	// range is the IPv6 range for the public interface.
	// +kubebuilder:validation:MinLength=1
	// +required
	Range string `json:"range,omitempty"`
}

// VPCInterfaceCreateOptions defines the VPC interface configuration for an instance
type VPCInterfaceCreateOptions struct {
	// subnet_id is the ID of the subnet to use for the interface.
	// +kubebuilder:validation:Minimum=1
	// +required
	SubnetID int `json:"subnet_id,omitempty"`

	// ipv4 is the IPv4 configuration for the interface.
	// +optional
	IPv4 *VPCInterfaceIPv4CreateOptions `json:"ipv4,omitempty"`

	// ipv6 is the IPv6 configuration for the interface.
	// +optional
	IPv6 *VPCInterfaceIPv6CreateOptions `json:"ipv6,omitempty"`
}

// VPCInterfaceIPv6CreateOptions defines the IPv6 configuration for a VPC interface
type VPCInterfaceIPv6CreateOptions struct {
	// slaac is the IPv6 SLAAC configuration for the interface.
	// +optional
	// +listType=map
	// +listMapKey=range
	SLAAC []VPCInterfaceIPv6SLAACCreateOptions `json:"slaac,omitempty"`

	// ranges is the IPv6 ranges for the interface.
	// +optional
	// +listType=map
	// +listMapKey=range
	Ranges []VPCInterfaceIPv6RangeCreateOptions `json:"ranges,omitempty"`

	// is_public is a boolean indicating whether the interface is public.
	// +required
	IsPublic *bool `json:"is_public,omitempty"`
}

// VPCInterfaceIPv6SLAACCreateOptions defines the Range for IPv6 SLAAC
type VPCInterfaceIPv6SLAACCreateOptions struct {
	// range is the IPv6 range for the interface.
	// +kubebuilder:validation:MinLength=1
	// +required
	Range string `json:"range,omitempty"`
}

// VPCInterfaceIPv6RangeCreateOptions defines the IPv6 range for a VPC interface
type VPCInterfaceIPv6RangeCreateOptions struct {
	// range is the IPv6 range for the interface.
	// +kubebuilder:validation:MinLength=1
	// +required
	Range string `json:"range,omitempty"`
}

// VPCInterfaceIPv4CreateOptions defines the IPv4 address and range configuration for a VPC interface
type VPCInterfaceIPv4CreateOptions struct {
	// addresses is the IPv4 addresses for the interface.
	// +optional
	// +listType=map
	// +listMapKey=address
	Addresses []VPCInterfaceIPv4AddressCreateOptions `json:"addresses,omitempty"`

	// ranges is the IPv4 ranges for the interface.
	// +optional
	// +listType=map
	// +listMapKey=range
	Ranges []VPCInterfaceIPv4RangeCreateOptions `json:"ranges,omitempty"`
}

// VPCInterfaceIPv4AddressCreateOptions defines the IPv4 configuration for a VPC interface
type VPCInterfaceIPv4AddressCreateOptions struct {
	// address is the IPv4 address for the interface.
	// +kubebuilder:validation:MinLength=1
	// +required
	Address string `json:"address,omitempty"`

	// primary is a boolean indicating whether the address is primary.
	// +optional
	Primary *bool `json:"primary,omitempty"`

	// nat_1_1_address is the NAT 1:1 address for the interface.
	// +optional
	NAT1To1Address *string `json:"nat_1_1_address,omitempty"`
}

// VPCInterfaceIPv4RangeCreateOptions defines the IPv4 range for a VPC interface
type VPCInterfaceIPv4RangeCreateOptions struct {
	// range is the IPv4 range for the interface.
	// +kubebuilder:validation:MinLength=1
	// +required
	Range string `json:"range,omitempty"`
}

// VLANInterface defines the VLAN interface configuration for an instance
type VLANInterface struct {
	// vlan_label is the label of the VLAN.
	// +kubebuilder:validation:MinLength=1
	// +required
	VLANLabel string `json:"vlan_label,omitempty"`

	// ipam_address is the IP address to assign to the interface.
	// +optional
	IPAMAddress *string `json:"ipam_address,omitempty"`
}

// VPCIPv4 defines VPC IPV4 settings
type VPCIPv4 struct {
	// vpc is the ID of the VPC to use for the interface.
	// +optional
	VPC string `json:"vpc,omitempty"`

	// nat1to1 is the NAT 1:1 address for the interface.
	// +optional
	NAT1To1 string `json:"nat1to1,omitempty"`
}

// LinodeMachineStatus defines the observed state of LinodeMachine
type LinodeMachineStatus struct {
	// conditions define the current service state of the LinodeMachine.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// addresses contains the Linode instance associated addresses.
	// +optional
	// +listType=map
	// +listMapKey=address
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// cloudinitMetadataSupport determines whether to use cloud-init or not.
	// Deprecated: Stackscript no longer in use, so this field is not used.
	// +kubebuilder:deprecatedversion:warning="CloudinitMetadataSupport is deprecated"
	// +optional
	// +kubebuilder:default=true
	CloudinitMetadataSupport bool `json:"cloudinitMetadataSupport,omitempty"`

	// instanceState is the state of the Linode instance for this machine.
	// +optional
	InstanceState *linodego.InstanceStatus `json:"instanceState,omitempty"`

	// failureReason will be set in the event that there is a terminal problem
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
	FailureReason *string `json:"failureReason,omitempty"`

	// failureMessage will be set in the event that there is a terminal problem
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

	// tags are the tags applied to the Linode Machine.
	// +optional
	// +listType=set
	Tags []string `json:"tags,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=linodemachines,scope=Namespaced,categories=cluster-api,shortName=lm
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this LinodeMachine belongs"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.instanceState",description="Linode instance state"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this LinodeMachine"

// LinodeMachine is the Schema for the linodemachines API
type LinodeMachine struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the specification of desired behavior for the LinodeMachine.
	// +required
	Spec LinodeMachineSpec `json:"spec,omitzero,omitempty"`

	// status defines the observed state of LinodeMachine.
	// +optional
	Status LinodeMachineStatus `json:"status,omitempty"`
}

func (lm *LinodeMachine) SetCondition(cond metav1.Condition) {
	if cond.LastTransitionTime.IsZero() {
		cond.LastTransitionTime = metav1.Now()
	}
	for i := range lm.Status.Conditions {
		if lm.Status.Conditions[i].Type == cond.Type {
			lm.Status.Conditions[i] = cond

			return
		}
	}
	lm.Status.Conditions = append(lm.Status.Conditions, cond)
}

func (lm *LinodeMachine) GetCondition(condType string) *metav1.Condition {
	for i := range lm.Status.Conditions {
		if lm.Status.Conditions[i].Type == condType {
			return &lm.Status.Conditions[i]
		}
	}

	return nil
}

func (lm *LinodeMachine) DeleteCondition(condType string) {
	for i := range lm.Status.Conditions {
		if lm.Status.Conditions[i].Type == condType {
			lm.Status.Conditions = append(lm.Status.Conditions[:i], lm.Status.Conditions[i+1:]...)
		}
	}
}

func (lm *LinodeMachine) IsPaused() bool {
	for i := range lm.Status.Conditions {
		if lm.Status.Conditions[i].Type == ConditionPaused {
			return lm.Status.Conditions[i].Status == metav1.ConditionTrue
		}
	}
	return false
}

// +kubebuilder:object:root=true

// LinodeMachineList contains a list of LinodeMachine
type LinodeMachineList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	// items is a list of LinodeMachine.
	Items []LinodeMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeMachine{}, &LinodeMachineList{})
}
