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
	"strings"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"

	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

func Convert_v1alpha1_NetworkSpec_To_v1alpha2_NetworkSpec(in *NetworkSpec, out *infrastructurev1alpha2.NetworkSpec, s conversion.Scope) error {
	out.ApiserverNodeBalancerConfigID = in.NodeBalancerConfigID
	out.ApiserverLoadBalancerPort = in.LoadBalancerPort
	out.LoadBalancerType = in.LoadBalancerType
	out.NodeBalancerID = in.NodeBalancerID
	out.AdditionalPorts = make([]infrastructurev1alpha2.LinodeNBPortConfig, 0)
	return nil
}

func Convert_v1alpha2_NetworkSpec_To_v1alpha1_NetworkSpec(in *infrastructurev1alpha2.NetworkSpec, out *NetworkSpec, s conversion.Scope) error {
	out.NodeBalancerConfigID = in.ApiserverNodeBalancerConfigID
	out.LoadBalancerPort = in.ApiserverLoadBalancerPort
	out.LoadBalancerType = in.LoadBalancerType
	out.NodeBalancerID = in.NodeBalancerID
	return nil
}

func Convert_v1alpha2_LinodeMachineSpec_To_v1alpha1_LinodeMachineSpec(in *infrastructurev1alpha2.LinodeMachineSpec, out *LinodeMachineSpec, s conversion.Scope) error {
	// Ok to use the auto-generated conversion function, it simply drops the PlacementGroupRef, and copies everything else
	return autoConvert_v1alpha2_LinodeMachineSpec_To_v1alpha1_LinodeMachineSpec(in, out, s)
}

func Convert_v1alpha2_LinodeMachineStatus_To_v1alpha1_LinodeMachineStatus(in *infrastructurev1alpha2.LinodeMachineStatus, out *LinodeMachineStatus, s conversion.Scope) error {
	// Ok to use the auto-generated conversion function
	return autoConvert_v1alpha2_LinodeMachineStatus_To_v1alpha1_LinodeMachineStatus(in, out, s)
}

func Convert_v1alpha1_LinodeMachineSpec_To_v1alpha2_LinodeMachineSpec(in *LinodeMachineSpec, out *infrastructurev1alpha2.LinodeMachineSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_LinodeMachineSpec_To_v1alpha2_LinodeMachineSpec(in, out, s)
}

func Convert_v1alpha1_LinodeObjectStorageBucketSpec_To_v1alpha2_LinodeObjectStorageBucketSpec(in *LinodeObjectStorageBucketSpec, out *infrastructurev1alpha2.LinodeObjectStorageBucketSpec, s conversion.Scope) error {
	// WARNING: in.Cluster requires manual conversion: does not exist in peer-type
	out.Region = in.Cluster
	out.CredentialsRef = in.CredentialsRef
	return nil
}
func Convert_v1alpha1_LinodeObjectStorageBucketStatus_To_v1alpha2_LinodeObjectStorageBucketStatus(in *LinodeObjectStorageBucketStatus, out *infrastructurev1alpha2.LinodeObjectStorageBucketStatus, s conversion.Scope) error {
	out.Ready = in.Ready
	out.FailureMessage = in.FailureMessage
	out.Conditions = in.Conditions
	out.Hostname = in.Hostname
	out.CreationTime = in.CreationTime
	// WARNING: in.LastKeyGeneration requires manual conversion: does not exist in peer-type
	// WARNING: in.KeySecretName requires manual conversion: does not exist in peer-type
	// WARNING: in.AccessKeyRefs requires manual conversion: does not exist in peer-type
	return nil
}

func Convert_v1alpha2_LinodeObjectStorageBucketSpec_To_v1alpha1_LinodeObjectStorageBucketSpec(in *infrastructurev1alpha2.LinodeObjectStorageBucketSpec, out *LinodeObjectStorageBucketSpec, s conversion.Scope) error {
	// WARNING: in.Region requires manual conversion: does not exist in peer-type
	out.Cluster = in.Region
	out.CredentialsRef = in.CredentialsRef
	out.KeyGeneration = ptr.To(0)
	out.SecretType = DefaultSecretTypeObjectStorageBucket
	return nil
}

func Convert_v1alpha2_LinodeObjectStorageBucket_To_v1alpha1_LinodeObjectStorageBucket(in *infrastructurev1alpha2.LinodeObjectStorageBucket, out *LinodeObjectStorageBucket, scope conversion.Scope) error {
	if in.Status.Hostname != nil && *in.Status.Hostname != "" {
		in.Spec.Region = strings.Split(*in.Status.Hostname, ".")[1]
	} else {
		in.Spec.Region += "-1"
	}
	return autoConvert_v1alpha2_LinodeObjectStorageBucket_To_v1alpha1_LinodeObjectStorageBucket(in, out, scope)
}
