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

	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

// ConvertTo converts this LinodeObjectStorageBucket to the Hub version (v1alpha2).
func (src *LinodeObjectStorageBucket) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*infrastructurev1alpha2.LinodeObjectStorageBucket)
	if !ok {
		return errors.New("failed to convert LinodeObjectStorageBucket version from v1alpha1 to v1alpha2")
	}

	if err := Convert_v1alpha1_LinodeObjectStorageBucket_To_v1alpha2_LinodeObjectStorageBucket(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data from annotations
	restored := &LinodeObjectStorageBucket{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *LinodeObjectStorageBucket) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*infrastructurev1alpha2.LinodeObjectStorageBucket)
	if !ok {
		return errors.New("failed to convert LinodeObjectStorageBucket version from v1alpha2 to v1alpha1")
	}

	if err := Convert_v1alpha2_LinodeObjectStorageBucket_To_v1alpha1_LinodeObjectStorageBucket(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}
