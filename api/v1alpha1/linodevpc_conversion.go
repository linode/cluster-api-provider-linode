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

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

// ConvertTo converts this LinodeVPC to the Hub version (v1alpha2).
func (src *LinodeVPC) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*infrastructurev1alpha2.LinodeVPC)
	if !ok {
		return errors.New("failed to convert LinodeVPC version from v1alpha1 to v1alpha2")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Description = src.Spec.Description
	dst.Spec.Region = src.Spec.Region
	if src.Spec.Subnets != nil {
		dst.Spec.Subnets = []infrastructurev1alpha2.VPCSubnetCreateOptions{}
		for _, subnet := range src.Spec.Subnets {
			dst.Spec.Subnets = append(dst.Spec.Subnets, infrastructurev1alpha2.VPCSubnetCreateOptions{
				Label: subnet.Label,
				IPv4:  subnet.IPv4,
			})
		}
	}

	dst.Spec.VPCID = src.Spec.VPCID
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = (*infrastructurev1alpha2.VPCStatusError)(src.Status.FailureReason)

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *LinodeVPC) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*infrastructurev1alpha2.LinodeVPC)
	if !ok {
		return errors.New("failed to convert LinodeVPC version from v1alpha2 to v1alpha1")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Description = src.Spec.Description
	dst.Spec.Region = src.Spec.Region
	if src.Spec.Subnets != nil {
		dst.Spec.Subnets = []VPCSubnetCreateOptions{}
		for _, subnet := range src.Spec.Subnets {
			dst.Spec.Subnets = append(dst.Spec.Subnets, VPCSubnetCreateOptions{
				Label: subnet.Label,
				IPv4:  subnet.IPv4,
			})
		}
	}

	dst.Spec.VPCID = src.Spec.VPCID
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = (*VPCStatusError)(src.Status.FailureReason)

	return nil
}
