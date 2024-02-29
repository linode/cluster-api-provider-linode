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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LinodeObjectStorageBucketSpec defines the desired state of LinodeObjectStorageBucket
type LinodeObjectStorageBucketSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the bucket. It must be unique in the Object Storage cluster.
	// If not specified, one will be generated using the UID assigned by Kubernetes to the resource.
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	Label *string `json:"label,omitempty"`

	// The ID of the Object Storage cluster for the bucket.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Cluster string `json:"cluster"`

	// A reference to a Secret that contains the credentials to use for provisioning the bucket.
	// If not supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *corev1.SecretReference `json:"credentialsRef"`

	// May be modified to trigger rotations of access keys created for the bucket.
	// +optional
	// +kubebuilder:default=0
	KeyGeneration *int `json:"keyGeneration,omitempty"`
}

// LinodeObjectStorageBucketStatus defines the observed state of LinodeObjectStorageBucket
type LinodeObjectStorageBucketStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Denotes that the bucket has been provisioned along with access keys.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// Will be set in the event that there is a terminal problem
	// reconciling the Object Storage Bucket and will contain a verbose string
	// suitable for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Current service state of the LinodeObjectStorageBucket.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// The hostname assigned to the bucket.
	// +optional
	Hostname *string `json:"hostname,omitempty"`

	// The creation timestamp for the underlying bucket.
	// +optional
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// Tracks the last known value of .spec.keyGeneration.
	// +optional
	LastKeyGeneration *int `json:"lastKeyGeneration,omitempty"`

	// The name of the Secret containing access keys for the bucket.
	// +optional
	KeySecretName *string `json:"keySecretName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=linodeobjectstoragebuckets,scope=Namespaced,shortName=lobj
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Label",type="string",JSONPath=".spec.label",description="The name of the bucket"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.cluster",description="The ID of the Object Storage cluster for the bucket"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Bucket and keys have been provisioned"

// LinodeObjectStorageBucket is the Schema for the linodeobjectstoragebuckets API
type LinodeObjectStorageBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinodeObjectStorageBucketSpec   `json:"spec,omitempty"`
	Status LinodeObjectStorageBucketStatus `json:"status,omitempty"`
}

func (b *LinodeObjectStorageBucket) GetConditions() clusterv1.Conditions {
	return b.Status.Conditions
}

func (b *LinodeObjectStorageBucket) SetConditions(conditions clusterv1.Conditions) {
	b.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// LinodeObjectStorageBucketList contains a list of LinodeObjectStorageBucket
type LinodeObjectStorageBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinodeObjectStorageBucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinodeObjectStorageBucket{}, &LinodeObjectStorageBucketList{})
}
