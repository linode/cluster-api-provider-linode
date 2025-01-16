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
	// ObjectStorageKeyFinalizer allows ReconcileLinodeObjectStorageKey to clean up Linode resources associated
	// with LinodeObjectStorageKey before removing it from the apiserver.
	ObjectStorageKeyFinalizer = "linodeobjectstoragekey.infrastructure.cluster.x-k8s.io"
)

type BucketAccessRef struct {
	BucketName  string `json:"bucketName"`
	Permissions string `json:"permissions"`
	Region      string `json:"region"`
}

type GeneratedSecret struct {
	// The name of the generated Secret. If not set, the name is formatted as "{name-of-obj-key}-obj-key".
	// +optional
	Name string `json:"name,omitempty"`
	// The namespace for the generated Secret. If not set, defaults to the namespace of the LinodeObjectStorageKey.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// The type of the generated Secret.
	// +kubebuilder:validation:Enum=Opaque;addons.cluster.x-k8s.io/resource-set
	// +kubebuilder:default=Opaque
	// +optional
	Type corev1.SecretType `json:"type,omitempty"`
	// How to format the data stored in the generated Secret.
	// It supports Go template syntax and interpolating the following values: .AccessKey, .SecretKey .BucketName .BucketEndpoint .S3Endpoint
	// If no format is supplied then a generic one is used containing the values specified.
	// +optional
	Format map[string]string `json:"format,omitempty"`
}

// LinodeObjectStorageKeySpec defines the desired state of LinodeObjectStorageKey
type LinodeObjectStorageKeySpec struct {
	// BucketAccess is the list of object storage bucket labels which can be accessed using the key
	// +kubebuilder:validation:MinItems=1
	BucketAccess []BucketAccessRef `json:"bucketAccess"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for generating access keys.
	// If not supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`

	// KeyGeneration may be modified to trigger a rotation of the access key.
	// +kubebuilder:default=0
	KeyGeneration int `json:"keyGeneration"`

	// GeneratedSecret configures the Secret to generate containing access key details.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	GeneratedSecret `json:"generatedSecret"`

	// SecretType instructs the controller what type of secret to generate containing access key details.
	// Deprecated: Use generatedSecret.type.
	// +kubebuilder:validation:Enum=Opaque;addons.cluster.x-k8s.io/resource-set
	// +kubebuilder:deprecatedversion:warning="secretType deprecated by generatedSecret.type"
	// +optional
	SecretType corev1.SecretType `json:"secretType,omitempty"`

	// SecretDataFormat instructs the controller how to format the data stored in the secret containing access key details.
	// Deprecated: Use generatedSecret.format.
	// +kubebuilder:deprecatedversion:warning="secretDataFormat deprecated by generatedSecret.format"
	// +optional
	SecretDataFormat map[string]string `json:"secretDataFormat,omitempty"`
}

// LinodeObjectStorageKeyStatus defines the observed state of LinodeObjectStorageKey
type LinodeObjectStorageKeyStatus struct {
	// Ready denotes that the key has been provisioned.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Object Storage Key and will contain a verbose string
	// suitable for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions specify the service state of the LinodeObjectStorageKey.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// CreationTime specifies the creation timestamp for the secret.
	// +optional
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// LastKeyGeneration tracks the last known value of .spec.keyGeneration.
	// +optional
	LastKeyGeneration *int `json:"lastKeyGeneration,omitempty"`

	// AccessKeyRef stores the ID for Object Storage key provisioned.
	// +optional
	AccessKeyRef *int `json:"accessKeyRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodeobjectstoragekeys,scope=Namespaced,categories=cluster-api,shortName=lobjkey
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.accessKeyRef",description="The ID assigned to the access key"
// +kubebuilder:printcolumn:name="Secret",type="string",JSONPath=".status.secretName",description="The name of the Secret containing access key data"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Whether the access key is synced in the Linode API"

// LinodeObjectStorageKey is the Schema for the linodeobjectstoragekeys API
type LinodeObjectStorageKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeObjectStorageKeySpec   `json:"spec,omitempty"`
	Status LinodeObjectStorageKeyStatus `json:"status,omitempty"`
}

func (losk *LinodeObjectStorageKey) GetConditions() []metav1.Condition {
	for i := range losk.Status.Conditions {
		if losk.Status.Conditions[i].Reason == "" {
			losk.Status.Conditions[i].Reason = DefaultConditionReason
		}
	}
	return losk.Status.Conditions
}

func (losk *LinodeObjectStorageKey) SetConditions(conditions []metav1.Condition) {
	losk.Status.Conditions = conditions
}

func (losk *LinodeObjectStorageKey) GetV1Beta2Conditions() []metav1.Condition {
	return losk.GetConditions()
}

func (losk *LinodeObjectStorageKey) SetV1Beta2Conditions(conditions []metav1.Condition) {
	losk.SetConditions(conditions)
}

// +kubebuilder:object:root=true

// LinodeObjectStorageKeyList contains a list of LinodeObjectStorageKey
type LinodeObjectStorageKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeObjectStorageKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeObjectStorageKey{}, &LinodeObjectStorageKeyList{})
}
