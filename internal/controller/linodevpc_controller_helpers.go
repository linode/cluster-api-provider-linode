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
		ID:    vpcScope.LinodeVPC.Spec.VPCID,
		Label: createConfig.Label,
		Tags:  nil,
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

	vpcScope.LinodeVPC.Spec.VPCID = &vpc.ID
	updateVPCSpecSubnets(vpcScope, vpc)

	return nil
}

func reconcileExistingVPC(ctx context.Context, vpcScope *scope.VPCScope, vpc *linodego.VPC) error {
	// Labels are unique
	vpcScope.LinodeVPC.Spec.VPCID = &vpc.ID

	// build a map of existing subnets to easily check for existence
	existingSubnets := make(map[string]int, len(vpc.Subnets))
	for _, subnet := range vpc.Subnets {
		existingSubnets[subnet.Label] = subnet.ID
	}

	// adopt or create subnets
	for idx, subnet := range vpcScope.LinodeVPC.Spec.Subnets {
		if subnet.SubnetID != 0 {
			continue
		}
		if id, ok := existingSubnets[subnet.Label]; ok {
			vpcScope.LinodeVPC.Spec.Subnets[idx].SubnetID = id
		} else {
			createSubnetConfig := linodego.VPCSubnetCreateOptions{
				Label: subnet.Label,
				IPv4:  subnet.IPv4,
			}
			newSubnet, err := vpcScope.LinodeClient.CreateVPCSubnet(ctx, createSubnetConfig, *vpcScope.LinodeVPC.Spec.VPCID)
			if err != nil {
				return err
			}
			vpcScope.LinodeVPC.Spec.Subnets[idx].SubnetID = newSubnet.ID
		}
	}

	return nil
}

// updateVPCSpecSubnets updates Subnets in linodeVPC spec and adds linode specific ID to them
func updateVPCSpecSubnets(vpcScope *scope.VPCScope, vpc *linodego.VPC) {
	for idx, specSubnet := range vpcScope.LinodeVPC.Spec.Subnets {
		for _, vpcSubnet := range vpc.Subnets {
			if specSubnet.Label == vpcSubnet.Label {
				vpcScope.LinodeVPC.Spec.Subnets[idx].SubnetID = vpcSubnet.ID
				break
			}
		}
	}
}

func linodeVPCSpecToVPCCreateConfig(vpcSpec infrav1alpha2.LinodeVPCSpec) *linodego.VPCCreateOptions {
	subnets := make([]linodego.VPCSubnetCreateOptions, len(vpcSpec.Subnets))
	vpcIPv6 := []linodego.VPCCreateOptionsIPv6{
		{
			Range: &vpcSpec.IPv6Range,
		},
	}

	for idx, subnet := range vpcSpec.Subnets {
		subnets[idx] = linodego.VPCSubnetCreateOptions{
			Label: subnet.Label,
			IPv4:  subnet.IPv4,
			IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
				{
					Range: &subnet.IPv6Range,
				},
			},
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
