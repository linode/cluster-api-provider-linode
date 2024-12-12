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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FirewallRuleSpec defines the desired state of FirewallRule
type FirewallRuleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Action      string `json:"action"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Ports       string `json:"ports,omitempty"`
	// +kubebuilder:validation:Enum=TCP;UDP;ICMP;IPENCAP
	Protocol  linodego.NetworkProtocol `json:"protocol"`
	Addresses *NetworkAddresses        `json:"addresses,omitempty"`
	// AddressSetRefs is a list of references to AddressSets as an alternative to
	// using Addresses but can be used in conjunction with it
	AddressSetRefs []*corev1.ObjectReference `json:"addressSetRefs,omitempty"`
}

// NetworkAddresses holds a list of IPv4 and IPv6 addresses
// We don't use linodego here since kubebuilder can't generate DeepCopyInto
// for linodego.NetworkAddresses
type NetworkAddresses struct {
	IPv4 *[]string `json:"ipv4,omitempty"`
	IPv6 *[]string `json:"ipv6,omitempty"`
}

// FirewallRuleStatus defines the observed state of FirewallRule
type FirewallRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=firewallrules,scope=Namespaced,categories=cluster-api,shortName=fwr
//+kubebuilder:subresource:status
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"

// FirewallRule is the Schema for the firewallrules API
type FirewallRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FirewallRuleSpec   `json:"spec,omitempty"`
	Status FirewallRuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FirewallRuleList contains a list of FirewallRule
type FirewallRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FirewallRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FirewallRule{}, &FirewallRuleList{})
}
