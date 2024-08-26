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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// FirewallFinalizer allows ReconcileLinodeFirewall to clean up Linode resources associated
	// with LinodeFirewall before removing it from the apiserver.
	FirewallFinalizer = "linodefirewall.infrastructure.cluster.x-k8s.io"
)

// LinodeFirewallSpec defines the desired state of LinodeFirewall
type LinodeFirewallSpec struct {
	// +optional
	FirewallID *int `json:"firewallID,omitempty"`

	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// +optional
	InboundRules []FirewallRule `json:"inboundRules,omitempty"`

	// InboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined.
	// +kubebuilder:validation:Enum=ACCEPT;DROP
	// +kubebuilder:default=ACCEPT
	// +optional
	InboundPolicy string `json:"inboundPolicy,omitempty"`

	// +optional
	OutboundRules []FirewallRule `json:"outboundRules,omitempty"`

	// OutboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined.
	// +kubebuilder:validation:Enum=ACCEPT;DROP
	// +kubebuilder:default=ACCEPT
	// +optional
	OutboundPolicy string `json:"outboundPolicy,omitempty"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this Firewall. If not
	// supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`
}

type FirewallRule struct {
	Action      string                   `json:"action"`
	Label       string                   `json:"label"`
	Description string                   `json:"description,omitempty"`
	Ports       string                   `json:"ports,omitempty"`
	Protocol    linodego.NetworkProtocol `json:"protocol"`
	Addresses   *NetworkAddresses        `json:"addresses"`
}

// NetworkAddresses holds a list of IPv4 and IPv6 addresses
// We don't use linodego here since kubebuilder can't generate DeepCopyInto
// for linodego.NetworkAddresses
type NetworkAddresses struct {
	IPv4 *[]string `json:"ipv4,omitempty"`
	IPv6 *[]string `json:"ipv6,omitempty"`
}

// LinodeFirewallStatus defines the observed state of LinodeFirewall
type LinodeFirewallStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Firewall and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Firewall's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Firewalls
	// can be added as events to the Firewall object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *FirewallStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Firewall and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Firewall's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Firewalls
	// can be added as events to the Firewall object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the LinodeFirewall.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodefirewalls,scope=Namespaced,categories=cluster-api,shortName=lfw
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Firewall is ready"
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"
// +kubebuilder:storageversion

// LinodeFirewall is the Schema for the linodefirewalls API
type LinodeFirewall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeFirewallSpec   `json:"spec,omitempty"`
	Status LinodeFirewallStatus `json:"status,omitempty"`
}

func (lfw *LinodeFirewall) GetConditions() clusterv1.Conditions {
	return lfw.Status.Conditions
}

func (lfw *LinodeFirewall) SetConditions(conditions clusterv1.Conditions) {
	lfw.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// LinodeFirewallList contains a list of LinodeFirewall
type LinodeFirewallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeFirewall `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeFirewall{}, &LinodeFirewallList{})
}

// FirewallStatusError defines errors states for Firewall objects.
type FirewallStatusError string

const (
	// CreateFirewallError indicates that an error was encountered
	// when trying to create the Firewall.
	CreateFirewallError FirewallStatusError = "CreateError"

	// UpdateFirewallError indicates that an error was encountered
	// when trying to update the Firewall.
	UpdateFirewallError FirewallStatusError = "UpdateError"

	// DeleteFirewallError indicates that an error was encountered
	// when trying to delete the Firewall.
	DeleteFirewallError FirewallStatusError = "DeleteError"
)
