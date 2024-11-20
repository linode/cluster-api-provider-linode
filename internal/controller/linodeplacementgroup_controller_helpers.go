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

func (r *LinodePlacementGroupReconciler) reconcilePlacementGroup(ctx context.Context, pgScope *scope.PlacementGroupScope, logger logr.Logger) error {
	createConfig := linodePlacementGroupSpecToPGCreateConfig(pgScope.LinodePlacementGroup.Spec)
	if createConfig == nil {
		err := errors.New("failed to convert Placement Group spec to create PG config")

		logger.Error(err, "Panic! Struct of LinodePlacementGroup is different than PlacementGroupCreateOptions")

		return err
	}

	createConfig.Label = pgScope.LinodePlacementGroup.GetName()
	listFilter := util.Filter{
		ID:    pgScope.LinodePlacementGroup.Spec.PGID,
		Label: createConfig.Label,
		Tags:  nil,
	}
	filter, err := listFilter.String()
	if err != nil {
		return err
	}
	if pgs, err := pgScope.LinodeClient.ListPlacementGroups(ctx, linodego.NewListOptions(1, filter)); err != nil {
		logger.Error(err, "Failed to list Placement Groups")
		return err
	} else if len(pgs) != 0 {
		pgScope.LinodePlacementGroup.Spec.PGID = &pgs[0].ID
		return nil
	}

	pg, err := pgScope.LinodeClient.CreatePlacementGroup(ctx, *createConfig)
	if err != nil {
		logger.Error(err, "Failed to create placement group")

		return err
	} else if pg == nil {
		err = errors.New("missing Placement Group")

		logger.Error(err, "Panic! Failed to create Placement Group")

		return err
	}

	pgScope.LinodePlacementGroup.Spec.PGID = &pg.ID

	return nil
}

func linodePlacementGroupSpecToPGCreateConfig(pgSpec infrav1alpha2.LinodePlacementGroupSpec) *linodego.PlacementGroupCreateOptions {
	return &linodego.PlacementGroupCreateOptions{
		Region:               pgSpec.Region,
		PlacementGroupType:   linodego.PlacementGroupType(pgSpec.PlacementGroupType),
		PlacementGroupPolicy: linodego.PlacementGroupPolicy(pgSpec.PlacementGroupPolicy),
	}
}
