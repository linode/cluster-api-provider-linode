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

// FirewallRuleSpec defines the desired state of FirewallRule.
type FirewallRuleSpec struct {
	// action is the action to take when the rule matches.
	Action string `json:"action"`
	// label is the label of the rule.
	Label string `json:"label"`
	// description is the description of the rule.
	Description string `json:"description,omitempty"`
	// ports is the ports to apply the rule to.
	Ports string `json:"ports,omitempty"`
	// protocol is the protocol to apply the rule to.
	// +kubebuilder:validation:Enum=TCP;UDP;ICMP;IPENCAP
	Protocol linodego.NetworkProtocol `json:"protocol"`
	// addresses is a list of addresses to apply the rule to.
	Addresses *NetworkAddresses `json:"addresses,omitempty"`
	// addressSetRefs is a list of references to AddressSets as an alternative to
	// using Addresses but can be used in conjunction with it.
	AddressSetRefs []*corev1.ObjectReference `json:"addressSetRefs,omitempty"`
}

// NetworkAddresses holds a list of IPv4 and IPv6 addresses.
// We don't use linodego here since kubebuilder can't generate DeepCopyInto
// for linodego.NetworkAddresses
type NetworkAddresses struct {
	// ipv4 defines a list of IPv4 address strings.
	IPv4 *[]string `json:"ipv4,omitempty"`
	// ipv6 defines a list of IPv6 address strings.
	IPv6 *[]string `json:"ipv6,omitempty"`
}

// FirewallRuleStatus defines the observed state of FirewallRule.
type FirewallRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=firewallrules,scope=Namespaced,categories=cluster-api,shortName=fwr
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"

// FirewallRule is the Schema for the firewallrules API
type FirewallRule struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// spec is the desired state of the FirewallRule.
	Spec FirewallRuleSpec `json:"spec,omitempty"`
	// status is the observed state of the FirewallRule.
	Status FirewallRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FirewallRuleList contains a list of FirewallRule
type FirewallRuleList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	// items is a list of FirewallRule.
	Items []FirewallRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FirewallRule{}, &FirewallRuleList{})
}
