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
	// bucketName is the name of the bucket to grant access to.
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +required
	BucketName string `json:"bucketName,omitempty"`

	// permissions is the permissions to grant to the bucket.
	// +kubebuilder:validation:Enum=read_only;read_write
	// +required
	Permissions string `json:"permissions,omitempty"`

	// region is the region of the bucket.
	// +kubebuilder:validation:MinLength=1
	// +required
	Region string `json:"region,omitempty"`
}

type GeneratedSecret struct {
	// name of the generated Secret. If not set, the name is formatted as "{name-of-obj-key}-obj-key".
	// +optional
	Name string `json:"name,omitempty"`

	// namespace for the generated Secret. If not set, defaults to the namespace of the LinodeObjectStorageKey.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// type of the generated Secret.
	// +kubebuilder:validation:Enum=Opaque;addons.cluster.x-k8s.io/resource-set
	// +kubebuilder:default=Opaque
	// +optional
	Type corev1.SecretType `json:"type,omitempty"`

	// format of the data stored in the generated Secret.
	// It supports Go template syntax and interpolating the following values: .AccessKey .SecretKey .BucketName .BucketEndpoint .S3Endpoint
	// If no format is supplied, then a generic one is used containing the values specified.
	// +optional
	Format map[string]string `json:"format,omitempty"`
}

// LinodeObjectStorageKeySpec defines the desired state of LinodeObjectStorageKey
type LinodeObjectStorageKeySpec struct {
	// bucketAccess is the list of object storage bucket labels which can be accessed using the key
	// +kubebuilder:validation:MinItems=1
	// +required
	// +listType=atomic
	BucketAccess []BucketAccessRef `json:"bucketAccess,omitempty"`

	// credentialsRef is a reference to a Secret that contains the credentials to use for generating access keys.
	// If not supplied, then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`

	// keyGeneration may be modified to trigger a rotation of the access key.
	// +kubebuilder:default=0
	// +optional
	KeyGeneration *int `json:"keyGeneration,omitempty"`

	// generatedSecret configures the Secret to generate containing access key details.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +required
	GeneratedSecret `json:"generatedSecret"`

	// secretType instructs the controller what type of secret to generate containing access key details.
	//
	// Deprecated: secretType is no longer supported, Use generatedSecret.type.
	//
	// +kubebuilder:validation:Enum=Opaque;addons.cluster.x-k8s.io/resource-set
	// +kubebuilder:deprecatedversion:warning="secretType deprecated by generatedSecret.type"
	// +optional
	SecretType corev1.SecretType `json:"secretType,omitempty"`

	// secretDataFormat instructs the controller how to format the data stored in the secret containing access key details.
	//
	// Deprecated: secretDataFormat is no longer supported, please use generatedSecret.format.
	//
	// +kubebuilder:deprecatedversion:warning="secretDataFormat deprecated by generatedSecret.format"
	// +optional
	SecretDataFormat map[string]string `json:"secretDataFormat,omitempty"`
}

// LinodeObjectStorageKeyStatus defines the observed state of LinodeObjectStorageKey
type LinodeObjectStorageKeyStatus struct {
	// conditions define the current service state of the LinodeObjectStorageKey.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// ready denotes that the key has been provisioned.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready,omitempty"`

	// failureMessage will be set in the event that there is a terminal problem
	// reconciling the Object Storage Key and will contain a verbose string
	// suitable for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// creationTime specifies the creation timestamp for the secret.
	// +optional
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// lastKeyGeneration tracks the last known value of .spec.keyGeneration.
	// +optional
	LastKeyGeneration *int `json:"lastKeyGeneration,omitempty"`

	// accessKeyRef stores the ID for Object Storage key provisioned.
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
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the LinodeObjectStorageKey.
	// +required
	Spec LinodeObjectStorageKeySpec `json:"spec,omitzero,omitempty"`

	// status is the observed state of the LinodeObjectStorageKey.
	// +optional
	Status LinodeObjectStorageKeyStatus `json:"status,omitempty"`
}

func (key *LinodeObjectStorageKey) SetCondition(cond metav1.Condition) {
	if cond.LastTransitionTime.IsZero() {
		cond.LastTransitionTime = metav1.Now()
	}
	for i := range key.Status.Conditions {
		if key.Status.Conditions[i].Type == cond.Type {
			key.Status.Conditions[i] = cond

			return
		}
	}
	key.Status.Conditions = append(key.Status.Conditions, cond)
}

func (key *LinodeObjectStorageKey) GetCondition(condType string) *metav1.Condition {
	for i := range key.Status.Conditions {
		if key.Status.Conditions[i].Type == condType {
			return &key.Status.Conditions[i]
		}
	}

	return nil
}

// LinodeObjectStorageKeyList contains a list of LinodeObjectStorageKey
// +kubebuilder:object:root=true
type LinodeObjectStorageKeyList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items represent the list of LinodeObjectStorageKey objects.
	Items []LinodeObjectStorageKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeObjectStorageKey{}, &LinodeObjectStorageKeyList{})
}
