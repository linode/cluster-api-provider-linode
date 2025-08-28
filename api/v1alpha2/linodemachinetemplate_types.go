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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LinodeMachineTemplateSpec defines the desired state of LinodeMachineTemplate
type LinodeMachineTemplateSpec struct {
	// template defines the specification for a LinodeMachine.
	// +required
	Template LinodeMachineTemplateResource `json:"template"`
}

// LinodeMachineTemplateStatus defines the observed state of LinodeMachineTemplate
// It is used to store the status of the LinodeMachineTemplate, such as tags.
type LinodeMachineTemplateStatus struct {
	// conditions define the current service state of the LinodeMachineTemplate
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// tags that are currently applied to the LinodeMachineTemplate.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// firewallID that is currently applied to the LinodeMachineTemplate.
	// +optional
	FirewallID int `json:"firewallID,omitempty"`
}

// LinodeMachineTemplateResource describes the data needed to create a LinodeMachine from a template.
type LinodeMachineTemplateResource struct {
	// spec is the specification of the desired behavior of the machine.
	// +required
	Spec LinodeMachineSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=linodemachinetemplates,scope=Namespaced,categories=cluster-api,shortName=lmt
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"

// LinodeMachineTemplate is the Schema for the linodemachinetemplates API
type LinodeMachineTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the LinodeMachineTemplate.
	// +optional
	Spec LinodeMachineTemplateSpec `json:"spec,omitempty"`

	// status is the observed state of the LinodeMachineTemplate.
	// +optional
	Status LinodeMachineTemplateStatus `json:"status,omitempty"`
}

func (lmt *LinodeMachineTemplate) SetCondition(cond metav1.Condition) {
	if cond.LastTransitionTime.IsZero() {
		cond.LastTransitionTime = metav1.Now()
	}
	for i := range lmt.Status.Conditions {
		if lmt.Status.Conditions[i].Type == cond.Type {
			lmt.Status.Conditions[i] = cond

			return
		}
	}
	lmt.Status.Conditions = append(lmt.Status.Conditions, cond)
}

// +kubebuilder:object:root=true

// LinodeMachineTemplateList contains a list of LinodeMachineTemplate
type LinodeMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of LinodeMachineTemplate.
	// +optional
	Items []LinodeMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeMachineTemplate{}, &LinodeMachineTemplateList{})
}
