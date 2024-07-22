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

// ConvertTo converts this LinodeMachine to the Hub version (v1alpha2).
func (src *LinodeMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*infrastructurev1alpha2.LinodeMachine)
	if !ok {
		return errors.New("failed to convert LinodeMachine version from v1alpha1 to v1alpha2")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.InstanceID = src.Spec.InstanceID
	dst.Spec.Region = src.Spec.Region
	dst.Spec.Type = src.Spec.Type
	dst.Spec.Group = src.Spec.Group
	dst.Spec.RootPass = src.Spec.RootPass
	dst.Spec.AuthorizedKeys = src.Spec.AuthorizedKeys
	dst.Spec.AuthorizedUsers = src.Spec.AuthorizedUsers
	dst.Spec.BackupID = src.Spec.BackupID
	dst.Spec.Image = src.Spec.Image
	if src.Spec.Interfaces != nil {
		dst.Spec.Interfaces = []infrastructurev1alpha2.InstanceConfigInterfaceCreateOptions{}
		for _, configInterface := range src.Spec.Interfaces {
			dst.Spec.Interfaces = append(dst.Spec.Interfaces, infrastructurev1alpha2.InstanceConfigInterfaceCreateOptions{
				IPAMAddress: configInterface.IPAMAddress,
				Label:       configInterface.Label,
				Purpose:     configInterface.Purpose,
				Primary:     configInterface.Primary,
				SubnetID:    configInterface.SubnetID,
				IPv4:        (*infrastructurev1alpha2.VPCIPv4)(configInterface.IPv4),
				IPRanges:    configInterface.IPRanges,
			})
		}
	}
	dst.Spec.BackupsEnabled = src.Spec.BackupsEnabled
	dst.Spec.PrivateIP = src.Spec.PrivateIP
	dst.Spec.ProviderID = src.Spec.ProviderID
	dst.Spec.Tags = src.Spec.Tags
	dst.Spec.FirewallID = src.Spec.FirewallID
	dst.Spec.OSDisk = (*infrastructurev1alpha2.InstanceDisk)(src.Spec.OSDisk)
	if src.Spec.DataDisks != nil {
		dst.Spec.DataDisks = map[string]*infrastructurev1alpha2.InstanceDisk{}
		for diskName, dataDisk := range src.Spec.DataDisks {
			dst.Spec.DataDisks[diskName] = (*infrastructurev1alpha2.InstanceDisk)(dataDisk)
		}
	}
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = src.Status.FailureReason
	dst.Status.Addresses = src.Status.Addresses
	dst.Status.InstanceState = src.Status.InstanceState

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *LinodeMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*infrastructurev1alpha2.LinodeMachine)
	if !ok {
		return errors.New("failed to convert LinodeMachine version from v1alpha2 to v1alpha1")
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.InstanceID = src.Spec.InstanceID
	dst.Spec.Region = src.Spec.Region
	dst.Spec.Type = src.Spec.Type
	dst.Spec.Group = src.Spec.Group
	dst.Spec.RootPass = src.Spec.RootPass
	dst.Spec.AuthorizedKeys = src.Spec.AuthorizedKeys
	dst.Spec.AuthorizedUsers = src.Spec.AuthorizedUsers
	dst.Spec.BackupID = src.Spec.BackupID
	dst.Spec.Image = src.Spec.Image
	if src.Spec.Interfaces != nil {
		dst.Spec.Interfaces = []InstanceConfigInterfaceCreateOptions{}
		for _, configInterface := range src.Spec.Interfaces {
			dst.Spec.Interfaces = append(dst.Spec.Interfaces, InstanceConfigInterfaceCreateOptions{
				IPAMAddress: configInterface.IPAMAddress,
				Label:       configInterface.Label,
				Purpose:     configInterface.Purpose,
				Primary:     configInterface.Primary,
				SubnetID:    configInterface.SubnetID,
				IPv4:        (*VPCIPv4)(configInterface.IPv4),
				IPRanges:    configInterface.IPRanges,
			})
		}
	}
	dst.Spec.BackupsEnabled = src.Spec.BackupsEnabled
	dst.Spec.ProviderID = src.Spec.ProviderID
	dst.Spec.PrivateIP = src.Spec.PrivateIP
	dst.Spec.Tags = src.Spec.Tags
	dst.Spec.FirewallID = src.Spec.FirewallID
	dst.Spec.OSDisk = (*InstanceDisk)(src.Spec.OSDisk)
	if src.Spec.DataDisks != nil {
		dst.Spec.DataDisks = map[string]*InstanceDisk{}
		for diskName, dataDisk := range src.Spec.DataDisks {
			dst.Spec.DataDisks[diskName] = (*InstanceDisk)(dataDisk)
		}
	}
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = src.Status.FailureReason
	dst.Status.Addresses = src.Status.Addresses
	dst.Status.InstanceState = src.Status.InstanceState
	return nil
}
