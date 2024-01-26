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
	"bytes"
	"context"
	"encoding/gob"
	"errors"

	"github.com/go-logr/logr"
	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/linodego"
)

func (r *LinodeVPCReconciler) reconcileVPC(ctx context.Context, vpcScope *scope.VPCScope, logger logr.Logger) error {
	createConfig := linodeVPCSpecToVPCCreateConfig(vpcScope.LinodeVPC.Spec)
	if createConfig == nil {
		err := errors.New("failed to convert VPC spec to create VPC config")

		logger.Error(err, "Panic! Struct of LinodeVPCSpec is different than VPCCreateOptions")

		return err
	}

	if createConfig.Label == "" {
		createConfig.Label = util.RenderObjectLabel(vpcScope.LinodeVPC.UID)
	}

	if vpcs, err := vpcScope.LinodeClient.ListVPCs(ctx, linodego.NewListOptions(1, util.CreateLinodeAPIFilter(createConfig.Label, nil))); err != nil {
		logger.Error(err, "Failed to list VPCs")

		return err
	} else if len(vpcs) != 0 {
		// Labels are unique
		vpcScope.LinodeVPC.Spec.VPCID = &vpcs[0].ID

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

	return nil
}

func linodeVPCSpecToVPCCreateConfig(vpcSpec infrav1.LinodeVPCSpec) *linodego.VPCCreateOptions {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(vpcSpec)
	if err != nil {
		return nil
	}

	var createConfig linodego.VPCCreateOptions
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&createConfig)
	if err != nil {
		return nil
	}

	return &createConfig
}
