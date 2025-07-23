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

package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

var (
	// ErrVPCNotFound is a sentinel error to indicate that a VPC was not found.
	ErrVPCNotFound = errors.New("VPC not found")
)

func reconcileVPC(ctx context.Context, vpcScope *scope.VPCScope, logger logr.Logger) error {
	createConfig := linodeVPCSpecToVPCCreateConfig(vpcScope.LinodeVPC.Spec)
	if createConfig == nil {
		err := errors.New("failed to convert VPC spec to create VPC config")
		logger.Error(err, "Panic! Struct of LinodeVPCSpec is different than VPCCreateOptions")
		return err
	}

	createConfig.Label = vpcScope.LinodeVPC.Name
	listFilter := util.Filter{
		ID:   vpcScope.LinodeVPC.Spec.VPCID,
		Tags: nil,
	}
	filter, err := listFilter.String()
	if err != nil {
		return err
	}

	vpcs, err := vpcScope.LinodeClient.ListVPCs(ctx, linodego.NewListOptions(1, filter))
	if err != nil {
		logger.Error(err, "Failed to list VPCs")
		return err
	}

	if len(vpcs) != 0 {
		return reconcileExistingVPC(ctx, vpcScope, &vpcs[0])
	}

	vpc, err := vpcScope.LinodeClient.CreateVPC(ctx, *createConfig)
	if err != nil {
		logger.Error(err, "Failed to create VPC")
		return err
	}

	setVPCFields(&vpcScope.LinodeVPC.Spec, vpc)
	updateVPCSpecSubnets(vpcScope, vpc)

	return nil
}

func reconcileExistingVPC(ctx context.Context, vpcScope *scope.VPCScope, vpc *linodego.VPC) error {
	setVPCFields(&vpcScope.LinodeVPC.Spec, vpc)

	// Build a map of VPC subnets by both label and ID. We check for
	// the subnet ID but fallback to the label because the ID is not guaranteed
	// to be set until we've processed the subnet at least once.
	type SubnetConfig struct {
		ID    int
		Label string
		IPv6  []linodego.VPCIPv6Range
	}
	subnetsByLabel := make(map[string]SubnetConfig, len(vpc.Subnets))
	subnetsById := make(map[int]SubnetConfig, len(vpc.Subnets))
	for _, subnet := range vpc.Subnets {
		config := SubnetConfig{subnet.ID, subnet.Label, subnet.IPv6}
		subnetsByLabel[subnet.Label], subnetsById[subnet.ID] = config, config
	}

	// adopt or create subnets
	for idx, subnet := range vpcScope.LinodeVPC.Spec.Subnets {
		if subnet.SubnetID != 0 {
			if config, ok := subnetsById[subnet.SubnetID]; ok {
				vpcScope.LinodeVPC.Spec.Subnets[idx].Label = config.Label
				vpcScope.LinodeVPC.Spec.Subnets[idx].IPv6 = config.IPv6
			}
		} else if config, ok := subnetsByLabel[subnet.Label]; ok {
			// Handle subnets that exist in the Linode API but have not had their
			// ID set on the LinodeVPC yet.
			vpcScope.LinodeVPC.Spec.Subnets[idx].SubnetID = config.ID
			vpcScope.LinodeVPC.Spec.Subnets[idx].IPv6 = config.IPv6
		} else {
			// Handle subnets that we need to create in the Linode API.
			ipv6 := []linodego.VPCSubnetCreateOptionsIPv6{}
			for _, ipv6Range := range subnet.IPv6Range {
				ipv6 = append(ipv6, linodego.VPCSubnetCreateOptionsIPv6{
					Range: ipv6Range.Range,
				})
			}
			createSubnetConfig := linodego.VPCSubnetCreateOptions{
				Label: subnet.Label,
				IPv4:  subnet.IPv4,
				IPv6:  ipv6,
			}

			newSubnet, err := vpcScope.LinodeClient.CreateVPCSubnet(ctx, createSubnetConfig, *vpcScope.LinodeVPC.Spec.VPCID)
			if err != nil {
				return err
			}
			setSubnetFields(&vpcScope.LinodeVPC.Spec.Subnets[idx], newSubnet)
		}
	}

	return nil
}

// updateVPCSpecSubnets updates Subnets in linodeVPC spec and adds linode specific ID to them
func updateVPCSpecSubnets(vpcScope *scope.VPCScope, vpc *linodego.VPC) {
	for idx, specSubnet := range vpcScope.LinodeVPC.Spec.Subnets {
		for _, vpcSubnet := range vpc.Subnets {
			if specSubnet.Label == vpcSubnet.Label {
				setSubnetFields(&vpcScope.LinodeVPC.Spec.Subnets[idx], &vpcSubnet)
				break
			}
		}
	}
}

// setVPCFields sets the VPCID and IPv6 in the LinodeVPCSpec from the Linode VPC.
func setVPCFields(vpc *infrav1alpha2.LinodeVPCSpec, linodeVPC *linodego.VPC) {
	vpc.VPCID = &linodeVPC.ID
	// Clear existing IPv6 ranges and set new ones
	vpc.IPv6 = nil
	for _, ipv6 := range linodeVPC.IPv6 {
		vpc.IPv6 = append(vpc.IPv6, linodego.VPCIPv6Range{Range: ipv6.Range})
	}
}

// setSubnetFields sets the SubnetID and IPv6 in the VPCSubnetCreateOptions from the Linode VPCSubnet.
func setSubnetFields(subnet *infrav1alpha2.VPCSubnetCreateOptions, vpcSubnet *linodego.VPCSubnet) {
	subnet.SubnetID = vpcSubnet.ID
	// Clear existing IPv6 ranges and set new ones
	subnet.IPv6 = nil
	for _, ipv6 := range vpcSubnet.IPv6 {
		subnet.IPv6 = append(subnet.IPv6, linodego.VPCIPv6Range{Range: ipv6.Range})
	}
}

func linodeVPCSpecToVPCCreateConfig(vpcSpec infrav1alpha2.LinodeVPCSpec) *linodego.VPCCreateOptions {
	vpcIPv6 := make([]linodego.VPCCreateOptionsIPv6, len(vpcSpec.IPv6Range))
	for idx, ipv6 := range vpcSpec.IPv6Range {
		vpcIPv6[idx] = linodego.VPCCreateOptionsIPv6{
			Range: ipv6.Range,
		}
	}

	subnets := make([]linodego.VPCSubnetCreateOptions, len(vpcSpec.Subnets))
	for idx, subnet := range vpcSpec.Subnets {
		ipv6 := []linodego.VPCSubnetCreateOptionsIPv6{}
		for _, ipv6Range := range subnet.IPv6Range {
			ipv6 = append(ipv6, linodego.VPCSubnetCreateOptionsIPv6{
				Range: ipv6Range.Range,
			})
		}
		subnets[idx] = linodego.VPCSubnetCreateOptions{
			Label: subnet.Label,
			IPv4:  subnet.IPv4,
			IPv6:  ipv6,
		}
	}

	return &linodego.VPCCreateOptions{
		Description: vpcSpec.Description,
		Region:      vpcSpec.Region,
		Subnets:     subnets,
		IPv6:        vpcIPv6,
	}
}

// getVPC fetches a VPC and handles not-found errors.
// It returns the VPC if found.
// It returns nil, ErrVPCNotFound if the VPC is not found.
// It returns nil and an error for other API errors.
func getVPC(ctx context.Context, vpcScope *scope.VPCScope) (*linodego.VPC, error) {
	if vpcScope.LinodeVPC.Spec.VPCID == nil {
		return nil, ErrVPCNotFound
	}

	vpc, err := vpcScope.LinodeClient.GetVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID)
	if err != nil {
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) == nil {
			return nil, ErrVPCNotFound
		}
		return nil, err
	}
	return vpc, nil
}
