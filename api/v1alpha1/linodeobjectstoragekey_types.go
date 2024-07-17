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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ObjectStorageKeyFinalizer allows ReconcileLinodeObjectStorageKey to clean up Linode resources associated
	// with LinodeObjectStorageKey before removing it from the apiserver.
	ObjectStorageKeyFinalizer = "linodeobjectstoragekey.infrastructure.cluster.x-k8s.io"
)

type LinodeObjStorageBucket struct {
	Name       string `json:"name"`
	Cluster    string `json:"cluster"`
	Permission string `json:"permission"`
}

// LinodeObjectStorageKeySpec defines the desired state of LinodeObjectStorageKey
type LinodeObjectStorageKeySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Buckets is the list of object storage bucket labels which can be accessed using the key
	Buckets []LinodeObjStorageBucket `json:"buckets"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for generating access keys.
	// If not supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef"`

	// KeyGeneration may be modified to trigger rotation of access key.
	// +optional
	// +kubebuilder:default=0
	KeyGeneration *int `json:"keyGeneration,omitempty"`
}

// LinodeObjectStorageKeyStatus defines the observed state of LinodeObjectStorageKey
type LinodeObjectStorageKeyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

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
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// CreationTime specifies the creation timestamp for the secret.
	// +optional
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// LastKeyGeneration tracks the last known value of .spec.keyGeneration.
	// +optional
	LastKeyGeneration *int `json:"lastKeyGeneration,omitempty"`

	// AccessKeyRefs stores ID for Object Storage key provisioned.
	// +optional
	AccessKeyRef []int `json:"accessKeyRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LinodeObjectStorageKey is the Schema for the linodeobjectstoragekeys API
type LinodeObjectStorageKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeObjectStorageKeySpec   `json:"spec,omitempty"`
	Status LinodeObjectStorageKeyStatus `json:"status,omitempty"`
}

func (b *LinodeObjectStorageKey) GetConditions() clusterv1.Conditions {
	return b.Status.Conditions
}

func (b *LinodeObjectStorageKey) SetConditions(conditions clusterv1.Conditions) {
	b.Status.Conditions = conditions
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