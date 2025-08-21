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

type ObjectStorageACL string

// ObjectStorageACL options represent the access control level of a bucket.
const (
	ACLPrivate           ObjectStorageACL = "private"
	ACLPublicRead        ObjectStorageACL = "public-read"
	ACLAuthenticatedRead ObjectStorageACL = "authenticated-read"
	ACLPublicReadWrite   ObjectStorageACL = "public-read-write"

	BucketFinalizer = "linodeobjectstoragebucket.infrastructure.cluster.x-k8s.io"
)

// LinodeObjectStorageBucketSpec defines the desired state of LinodeObjectStorageBucket
type LinodeObjectStorageBucketSpec struct {

	// region is the ID of the Object Storage region for the bucket.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +required
	Region string `json:"region"`

	// acl sets the Access Control Level of the bucket using a canned ACL string
	// +optional
	// +kubebuilder:default=private
	// +kubebuilder:validation:Enum=private;public-read;authenticated-read;public-read-write
	ACL ObjectStorageACL `json:"acl,omitempty"`

	// corsEnabled enables for all origins in the bucket .If set to false, CORS is disabled for all origins in the bucket
	// +optional
	// +kubebuilder:default=true
	CorsEnabled bool `json:"corsEnabled,omitempty"`

	// credentialsRef is a reference to a Secret that contains the credentials to use for provisioning the bucket.
	// If not supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef,omitempty"`

	// accessKeyRef is a reference to a LinodeObjectStorageBucketKey for the bucket.
	// +optional
	AccessKeyRef *corev1.ObjectReference `json:"accessKeyRef,omitempty"`

	// forceDeleteBucket enables the object storage bucket used to be deleted even if it contains objects.
	// +optional
	ForceDeleteBucket bool `json:"forceDeleteBucket,omitempty"`
}

// LinodeObjectStorageBucketStatus defines the observed state of LinodeObjectStorageBucket
type LinodeObjectStorageBucketStatus struct {
	// ready denotes that the bucket has been provisioned along with access keys.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready,omitempty"`

	// failureMessage will be set in the event that there is a terminal problem
	// reconciling the Object Storage Bucket and will contain a verbose string
	// suitable for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// conditions specify the service state of the LinodeObjectStorageBucket.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// hostname is the address assigned to the bucket.
	// +optional
	Hostname *string `json:"hostname,omitempty"`

	// creationTime specifies the creation timestamp for the bucket.
	// +optional
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=linodeobjectstoragebuckets,scope=Namespaced,categories=cluster-api,shortName=lobj
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="clusterctl.cluster.x-k8s.io/move-hierarchy=true"
// +kubebuilder:printcolumn:name="Label",type="string",JSONPath=".spec.label",description="The name of the bucket"
// +kubebuilder:printcolumn:name="Region",type="string",JSONPath=".spec.region",description="The ID of the Object Storage region for the bucket"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Bucket and keys have been provisioned"

// LinodeObjectStorageBucket is the Schema for the linodeobjectstoragebuckets API
type LinodeObjectStorageBucket struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the LinodeObjectStorageBucket.
	// +optional
	Spec LinodeObjectStorageBucketSpec `json:"spec,omitempty"`

	// status is the observed state of the LinodeObjectStorageBucket.
	// +optional
	Status LinodeObjectStorageBucketStatus `json:"status,omitempty"`
}

func (losb *LinodeObjectStorageBucket) GetConditions() []metav1.Condition {
	for i := range losb.Status.Conditions {
		if losb.Status.Conditions[i].Reason == "" {
			losb.Status.Conditions[i].Reason = DefaultConditionReason
		}
	}
	return losb.Status.Conditions
}

func (losb *LinodeObjectStorageBucket) SetConditions(conditions []metav1.Condition) {
	losb.Status.Conditions = conditions
}

func (losb *LinodeObjectStorageBucket) GetV1Beta2Conditions() []metav1.Condition {
	return losb.GetConditions()
}

func (losb *LinodeObjectStorageBucket) SetV1Beta2Conditions(conditions []metav1.Condition) {
	losb.SetConditions(conditions)
}

// +kubebuilder:object:root=true

// LinodeObjectStorageBucketList contains a list of LinodeObjectStorageBucket
type LinodeObjectStorageBucketList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of LinodeObjectStorageBucket.
	// +optional
	Items []LinodeObjectStorageBucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeObjectStorageBucket{}, &LinodeObjectStorageBucketList{})
}
