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
	// FirewallFinalizer allows ReconcileLinodeFirewall to clean up Linode resources associated
	// with LinodeFirewall before removing it from the apiserver.
	FirewallFinalizer = "linodefirewall.infrastructure.cluster.x-k8s.io"
)

// LinodeFirewallSpec defines the desired state of LinodeFirewall
type LinodeFirewallSpec struct {
	// firewallID is the ID of the Firewall.
	// +optional
	FirewallID *int `json:"firewallID,omitempty"`

	// enabled determines if the Firewall is enabled. Defaults to false if not defined.
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// inboundRules is a list of FirewallRules that will be applied to the Firewall.
	// +optional
	InboundRules []FirewallRuleSpec `json:"inboundRules,omitempty"`

	// inboundRuleRefs is a list of references to FirewallRules as an alternative to
	// using InboundRules but can be used in conjunction with it
	// +optional
	InboundRuleRefs []*corev1.ObjectReference `json:"inboundRuleRefs,omitempty"`

	// inboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined.
	// +kubebuilder:validation:Enum=ACCEPT;DROP
	// +kubebuilder:default=ACCEPT
	// +optional
	InboundPolicy string `json:"inboundPolicy,omitempty"`

	// outboundRules is a list of FirewallRules that will be applied to the Firewall.
	// +optional
	OutboundRules []FirewallRuleSpec `json:"outboundRules,omitempty"`

	// outboundRuleRefs is a list of references to FirewallRules as an alternative to
	// using OutboundRules but can be used in conjunction with it
	// +optional
	OutboundRuleRefs []*corev1.ObjectReference `json:"outboundRuleRefs,omitempty"`

	// outboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined.
	// +kubebuilder:validation:Enum=ACCEPT;DROP
	// +kubebuilder:default=ACCEPT
	// +optional
	OutboundPolicy string `json:"outboundPolicy,omitempty"`

	// credentialsRef is a reference to a Secret that contains the credentials to use for provisioning this Firewall. If not
	// supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`
}

// LinodeFirewallStatus defines the observed state of LinodeFirewall
type LinodeFirewallStatus struct {
	// ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// failureReason will be set in the event that there is a terminal problem
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

	// failureMessage will be set in the event that there is a terminal problem
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

	// conditions define the current service state of the LinodeFirewall.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodefirewalls,scope=Namespaced,categories=cluster-api,shortName=lfw
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Firewall is ready"
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"
// +kubebuilder:storageversion

// LinodeFirewall is the Schema for the linodefirewalls API
type LinodeFirewall struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the LinodeFirewall.
	// +optional
	Spec LinodeFirewallSpec `json:"spec,omitempty"`

	// status is the observed state of the LinodeFirewall.
	// +optional
	Status LinodeFirewallStatus `json:"status,omitempty"`
}

func (lfw *LinodeFirewall) GetConditions() []metav1.Condition {
	for i := range lfw.Status.Conditions {
		if lfw.Status.Conditions[i].Reason == "" {
			lfw.Status.Conditions[i].Reason = DefaultConditionReason
		}
	}
	return lfw.Status.Conditions
}

func (lfw *LinodeFirewall) SetConditions(conditions []metav1.Condition) {
	lfw.Status.Conditions = conditions
}

// We need V1Beta2Conditions helpers to be able to use the conditions package from cluster-api
func (lfw *LinodeFirewall) GetV1Beta2Conditions() []metav1.Condition {
	return lfw.GetConditions()
}

func (lfw *LinodeFirewall) SetV1Beta2Conditions(conditions []metav1.Condition) {
	lfw.SetConditions(conditions)
}

// +kubebuilder:object:root=true

// LinodeFirewallList contains a list of LinodeFirewall
type LinodeFirewallList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	// items is a list of LinodeFirewall.
	Items []LinodeFirewall `json:"items"`
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
