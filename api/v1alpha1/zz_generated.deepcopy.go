//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/linode/linodego"
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InstanceConfigInterfaceCreateOptions) DeepCopyInto(out *InstanceConfigInterfaceCreateOptions) {
	*out = *in
	if in.SubnetID != nil {
		in, out := &in.SubnetID, &out.SubnetID
		*out = new(int)
		**out = **in
	}
	if in.IPv4 != nil {
		in, out := &in.IPv4, &out.IPv4
		*out = new(VPCIPv4)
		**out = **in
	}
	if in.IPRanges != nil {
		in, out := &in.IPRanges, &out.IPRanges
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InstanceConfigInterfaceCreateOptions.
func (in *InstanceConfigInterfaceCreateOptions) DeepCopy() *InstanceConfigInterfaceCreateOptions {
	if in == nil {
		return nil
	}
	out := new(InstanceConfigInterfaceCreateOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InstanceMetadataOptions) DeepCopyInto(out *InstanceMetadataOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InstanceMetadataOptions.
func (in *InstanceMetadataOptions) DeepCopy() *InstanceMetadataOptions {
	if in == nil {
		return nil
	}
	out := new(InstanceMetadataOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeCluster) DeepCopyInto(out *LinodeCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeCluster.
func (in *LinodeCluster) DeepCopy() *LinodeCluster {
	if in == nil {
		return nil
	}
	out := new(LinodeCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeClusterList) DeepCopyInto(out *LinodeClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinodeCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeClusterList.
func (in *LinodeClusterList) DeepCopy() *LinodeClusterList {
	if in == nil {
		return nil
	}
	out := new(LinodeClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeClusterSpec) DeepCopyInto(out *LinodeClusterSpec) {
	*out = *in
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	in.Network.DeepCopyInto(&out.Network)
	if in.VPCRef != nil {
		in, out := &in.VPCRef, &out.VPCRef
		*out = new(v1.ObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeClusterSpec.
func (in *LinodeClusterSpec) DeepCopy() *LinodeClusterSpec {
	if in == nil {
		return nil
	}
	out := new(LinodeClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeClusterStatus) DeepCopyInto(out *LinodeClusterStatus) {
	*out = *in
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.ClusterStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeClusterStatus.
func (in *LinodeClusterStatus) DeepCopy() *LinodeClusterStatus {
	if in == nil {
		return nil
	}
	out := new(LinodeClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeClusterTemplate) DeepCopyInto(out *LinodeClusterTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeClusterTemplate.
func (in *LinodeClusterTemplate) DeepCopy() *LinodeClusterTemplate {
	if in == nil {
		return nil
	}
	out := new(LinodeClusterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeClusterTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeClusterTemplateList) DeepCopyInto(out *LinodeClusterTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinodeClusterTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeClusterTemplateList.
func (in *LinodeClusterTemplateList) DeepCopy() *LinodeClusterTemplateList {
	if in == nil {
		return nil
	}
	out := new(LinodeClusterTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeClusterTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeClusterTemplateResource) DeepCopyInto(out *LinodeClusterTemplateResource) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeClusterTemplateResource.
func (in *LinodeClusterTemplateResource) DeepCopy() *LinodeClusterTemplateResource {
	if in == nil {
		return nil
	}
	out := new(LinodeClusterTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeClusterTemplateSpec) DeepCopyInto(out *LinodeClusterTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeClusterTemplateSpec.
func (in *LinodeClusterTemplateSpec) DeepCopy() *LinodeClusterTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(LinodeClusterTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachine) DeepCopyInto(out *LinodeMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachine.
func (in *LinodeMachine) DeepCopy() *LinodeMachine {
	if in == nil {
		return nil
	}
	out := new(LinodeMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachineList) DeepCopyInto(out *LinodeMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinodeMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachineList.
func (in *LinodeMachineList) DeepCopy() *LinodeMachineList {
	if in == nil {
		return nil
	}
	out := new(LinodeMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachineSpec) DeepCopyInto(out *LinodeMachineSpec) {
	*out = *in
	if in.ProviderID != nil {
		in, out := &in.ProviderID, &out.ProviderID
		*out = new(string)
		**out = **in
	}
	if in.InstanceID != nil {
		in, out := &in.InstanceID, &out.InstanceID
		*out = new(int)
		**out = **in
	}
	if in.AuthorizedKeys != nil {
		in, out := &in.AuthorizedKeys, &out.AuthorizedKeys
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AuthorizedUsers != nil {
		in, out := &in.AuthorizedUsers, &out.AuthorizedUsers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.StackScriptData != nil {
		in, out := &in.StackScriptData, &out.StackScriptData
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Interfaces != nil {
		in, out := &in.Interfaces, &out.Interfaces
		*out = make([]InstanceConfigInterfaceCreateOptions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Tags != nil {
		in, out := &in.Tags, &out.Tags
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(InstanceMetadataOptions)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachineSpec.
func (in *LinodeMachineSpec) DeepCopy() *LinodeMachineSpec {
	if in == nil {
		return nil
	}
	out := new(LinodeMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachineStatus) DeepCopyInto(out *LinodeMachineStatus) {
	*out = *in
	if in.Addresses != nil {
		in, out := &in.Addresses, &out.Addresses
		*out = make([]v1beta1.MachineAddress, len(*in))
		copy(*out, *in)
	}
	if in.InstanceState != nil {
		in, out := &in.InstanceState, &out.InstanceState
		*out = new(linodego.InstanceStatus)
		**out = **in
	}
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.MachineStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachineStatus.
func (in *LinodeMachineStatus) DeepCopy() *LinodeMachineStatus {
	if in == nil {
		return nil
	}
	out := new(LinodeMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachineTemplate) DeepCopyInto(out *LinodeMachineTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachineTemplate.
func (in *LinodeMachineTemplate) DeepCopy() *LinodeMachineTemplate {
	if in == nil {
		return nil
	}
	out := new(LinodeMachineTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeMachineTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachineTemplateList) DeepCopyInto(out *LinodeMachineTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinodeMachineTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachineTemplateList.
func (in *LinodeMachineTemplateList) DeepCopy() *LinodeMachineTemplateList {
	if in == nil {
		return nil
	}
	out := new(LinodeMachineTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeMachineTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachineTemplateResource) DeepCopyInto(out *LinodeMachineTemplateResource) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachineTemplateResource.
func (in *LinodeMachineTemplateResource) DeepCopy() *LinodeMachineTemplateResource {
	if in == nil {
		return nil
	}
	out := new(LinodeMachineTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeMachineTemplateSpec) DeepCopyInto(out *LinodeMachineTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeMachineTemplateSpec.
func (in *LinodeMachineTemplateSpec) DeepCopy() *LinodeMachineTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(LinodeMachineTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeObjectStorageBucket) DeepCopyInto(out *LinodeObjectStorageBucket) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeObjectStorageBucket.
func (in *LinodeObjectStorageBucket) DeepCopy() *LinodeObjectStorageBucket {
	if in == nil {
		return nil
	}
	out := new(LinodeObjectStorageBucket)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeObjectStorageBucket) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeObjectStorageBucketList) DeepCopyInto(out *LinodeObjectStorageBucketList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinodeObjectStorageBucket, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeObjectStorageBucketList.
func (in *LinodeObjectStorageBucketList) DeepCopy() *LinodeObjectStorageBucketList {
	if in == nil {
		return nil
	}
	out := new(LinodeObjectStorageBucketList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeObjectStorageBucketList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeObjectStorageBucketSpec) DeepCopyInto(out *LinodeObjectStorageBucketSpec) {
	*out = *in
	if in.ApiKeySecretRef != nil {
		in, out := &in.ApiKeySecretRef, &out.ApiKeySecretRef
		*out = new(v1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.KeyGeneration != nil {
		in, out := &in.KeyGeneration, &out.KeyGeneration
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeObjectStorageBucketSpec.
func (in *LinodeObjectStorageBucketSpec) DeepCopy() *LinodeObjectStorageBucketSpec {
	if in == nil {
		return nil
	}
	out := new(LinodeObjectStorageBucketSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeObjectStorageBucketStatus) DeepCopyInto(out *LinodeObjectStorageBucketStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeObjectStorageBucketStatus.
func (in *LinodeObjectStorageBucketStatus) DeepCopy() *LinodeObjectStorageBucketStatus {
	if in == nil {
		return nil
	}
	out := new(LinodeObjectStorageBucketStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeVPC) DeepCopyInto(out *LinodeVPC) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeVPC.
func (in *LinodeVPC) DeepCopy() *LinodeVPC {
	if in == nil {
		return nil
	}
	out := new(LinodeVPC)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeVPC) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeVPCList) DeepCopyInto(out *LinodeVPCList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinodeVPC, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeVPCList.
func (in *LinodeVPCList) DeepCopy() *LinodeVPCList {
	if in == nil {
		return nil
	}
	out := new(LinodeVPCList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinodeVPCList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeVPCSpec) DeepCopyInto(out *LinodeVPCSpec) {
	*out = *in
	if in.VPCID != nil {
		in, out := &in.VPCID, &out.VPCID
		*out = new(int)
		**out = **in
	}
	if in.Subnets != nil {
		in, out := &in.Subnets, &out.Subnets
		*out = make([]VPCSubnetCreateOptions, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeVPCSpec.
func (in *LinodeVPCSpec) DeepCopy() *LinodeVPCSpec {
	if in == nil {
		return nil
	}
	out := new(LinodeVPCSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinodeVPCStatus) DeepCopyInto(out *LinodeVPCStatus) {
	*out = *in
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(VPCStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinodeVPCStatus.
func (in *LinodeVPCStatus) DeepCopy() *LinodeVPCStatus {
	if in == nil {
		return nil
	}
	out := new(LinodeVPCStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkSpec) DeepCopyInto(out *NetworkSpec) {
	*out = *in
	if in.NodeBalancerConfigID != nil {
		in, out := &in.NodeBalancerConfigID, &out.NodeBalancerConfigID
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkSpec.
func (in *NetworkSpec) DeepCopy() *NetworkSpec {
	if in == nil {
		return nil
	}
	out := new(NetworkSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VPCIPv4) DeepCopyInto(out *VPCIPv4) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VPCIPv4.
func (in *VPCIPv4) DeepCopy() *VPCIPv4 {
	if in == nil {
		return nil
	}
	out := new(VPCIPv4)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VPCSubnetCreateOptions) DeepCopyInto(out *VPCSubnetCreateOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VPCSubnetCreateOptions.
func (in *VPCSubnetCreateOptions) DeepCopy() *VPCSubnetCreateOptions {
	if in == nil {
		return nil
	}
	out := new(VPCSubnetCreateOptions)
	in.DeepCopyInto(out)
	return out
}
