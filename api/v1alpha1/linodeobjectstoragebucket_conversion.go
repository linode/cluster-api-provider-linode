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
	"errors"
	"regexp"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

// ConvertTo converts this LinodeObjectStorageBucket to the Hub version (v1alpha2).
func (src *LinodeObjectStorageBucket) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*infrastructurev1alpha2.LinodeObjectStorageBucket)
	if !ok {
		return errors.New("failed to convert LinodeObjectStorageBucket version from v1alpha1 to v1alpha2")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	cexp := regexp.MustCompile(`^(([[:lower:]]+-)*[[:lower:]]+)-\d+$`)

	// Spec
	dst.Spec.Region = cexp.FindStringSubmatch(src.Spec.Cluster)[1]
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef
	dst.Spec.KeyGeneration = src.Spec.KeyGeneration
	dst.Spec.SecretType = src.Spec.SecretType

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.Hostname = src.Status.Hostname
	dst.Status.CreationTime = src.Status.CreationTime
	dst.Status.LastKeyGeneration = src.Status.LastKeyGeneration
	dst.Status.KeySecretName = src.Status.KeySecretName
	dst.Status.AccessKeyRefs = src.Status.AccessKeyRefs

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *LinodeObjectStorageBucket) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*infrastructurev1alpha2.LinodeObjectStorageBucket)
	if !ok {
		return errors.New("failed to convert LinodeObjectStorageBucket version from v1alpha2 to v1alpha1")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Cluster = src.Spec.Region + "-1"
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef
	dst.Spec.KeyGeneration = src.Spec.KeyGeneration
	dst.Spec.SecretType = src.Spec.SecretType

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.Hostname = src.Status.Hostname
	dst.Status.CreationTime = src.Status.CreationTime
	dst.Status.LastKeyGeneration = src.Status.LastKeyGeneration
	dst.Status.KeySecretName = src.Status.KeySecretName
	dst.Status.AccessKeyRefs = src.Status.AccessKeyRefs

	return nil
}
