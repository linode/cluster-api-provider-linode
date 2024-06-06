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

// ConvertTo converts this LinodeCluster to the Hub version (v1alpha2).
func (src *LinodeCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*infrastructurev1alpha2.LinodeCluster)
	if !ok {
		return errors.New("failed to convert LinodeCluster version from v1alpha1 to v1alpha2")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Network = infrastructurev1alpha2.NetworkSpec{
		LoadBalancerType:                 src.Spec.Network.LoadBalancerType,
		ApiserverLoadBalancerPort:        src.Spec.Network.LoadBalancerPort,
		NodeBalancerID:                   src.Spec.Network.NodeBalancerID,
		ApiserverNodeBalancerConfigID:    src.Spec.Network.NodeBalancerConfigID,
		Konnectivity:                     false,
		KonnectivityLoadBalancerPort:     0,
		KonnectivityNodeBalancerConfigID: nil,
	}
	dst.Spec.ControlPlaneEndpoint = src.Spec.ControlPlaneEndpoint
	dst.Spec.Region = src.Spec.Region
	dst.Spec.VPCRef = src.Spec.VPCRef
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = src.Status.FailureReason

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *LinodeCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*infrastructurev1alpha2.LinodeCluster)
	if !ok {
		return errors.New("failed to convert LinodeCluster version from v1alpha2 to v1alpha1")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Network.LoadBalancerPort = src.Spec.Network.ApiserverLoadBalancerPort
	dst.Spec.Network.LoadBalancerType = src.Spec.Network.LoadBalancerType
	dst.Spec.Network.NodeBalancerID = src.Spec.Network.NodeBalancerID
	dst.Spec.Network.NodeBalancerConfigID = src.Spec.Network.ApiserverNodeBalancerConfigID
	dst.Spec.ControlPlaneEndpoint = src.Spec.ControlPlaneEndpoint
	dst.Spec.Region = src.Spec.Region
	dst.Spec.VPCRef = src.Spec.VPCRef
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = src.Status.FailureReason

	return nil
}
