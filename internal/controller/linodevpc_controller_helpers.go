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

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
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
	if vpcs, err := vpcScope.LinodeClient.ListVPCs(ctx, linodego.NewListOptions(1, filter)); err != nil {
		logger.Error(err, "Failed to list VPCs")

		return err
	} else if len(vpcs) != 0 {
		// Labels are unique
		vpcScope.LinodeVPC.Spec.VPCID = &vpcs[0].ID
		updateVPCSpecSubnets(vpcScope, &vpcs[0])

		return nil
	}

	vpc, err := vpcScope.LinodeClient.CreateVPC(ctx, *createConfig)
	if err != nil {
		logger.Error(err, "Failed to create VPC")

		return err
	} else if vpc == nil {
		err = errors.New("missing VPC")

		logger.Error(err, "Panic! Failed to create VPC")

		return err
	}

	vpcScope.LinodeVPC.Spec.VPCID = &vpc.ID
	updateVPCSpecSubnets(vpcScope, vpc)

	return nil
}

// updateVPCSpecSubnets updates Subnets in linodeVPC spec and adds linode specific ID to them
func updateVPCSpecSubnets(vpcScope *scope.VPCScope, vpc *linodego.VPC) {
	for i, specSubnet := range vpcScope.LinodeVPC.Spec.Subnets {
		for _, vpcSubnet := range vpc.Subnets {
			if specSubnet.Label == vpcSubnet.Label {
				vpcScope.LinodeVPC.Spec.Subnets[i].SubnetID = vpcSubnet.ID
				break
			}
		}
	}
}

func linodeVPCSpecToVPCCreateConfig(vpcSpec infrav1alpha2.LinodeVPCSpec) *linodego.VPCCreateOptions {
	subnets := make([]linodego.VPCSubnetCreateOptions, len(vpcSpec.Subnets))
	for idx, subnet := range vpcSpec.Subnets {
		subnets[idx] = linodego.VPCSubnetCreateOptions{
			Label: subnet.Label,
			IPv4:  subnet.IPv4,
		}
	}
	return &linodego.VPCCreateOptions{
		Description: vpcSpec.Description,
		Region:      vpcSpec.Region,
		Subnets:     subnets,
	}
}
