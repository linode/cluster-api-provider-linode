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
)

// LinodeClusterTemplateSpec defines the desired state of LinodeClusterTemplate
type LinodeClusterTemplateSpec struct {
	Template LinodeClusterTemplateResource `json:"template"`
}

// LinodeClusterTemplateResource describes the data needed to create a LinodeCluster from a template.
type LinodeClusterTemplateResource struct {
	Spec LinodeClusterSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodeclustertemplates,scope=Namespaced,categories=cluster-api,shortName=lct

// LinodeClusterTemplate is the Schema for the linodeclustertemplates API
type LinodeClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LinodeClusterTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// LinodeClusterTemplateList contains a list of LinodeClusterTemplate
type LinodeClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeClusterTemplate{}, &LinodeClusterTemplateList{})
}
