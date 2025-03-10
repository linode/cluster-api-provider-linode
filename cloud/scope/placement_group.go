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

package scope

import (
	"context"
	"errors"
	"fmt"


	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

// PlacementGroupScope defines the basic context for an actuator to operate upon.
type PlacementGroupScope struct {
	Client K8sClient

	PatchHelper          *patch.Helper
	LinodeClient         LinodeClient
	LinodePlacementGroup *infrav1alpha2.LinodePlacementGroup
	Cluster              *clusterv1.Cluster
}

// PlacementGroupScopeParams defines the input parameters used to create a new Scope.
type PlacementGroupScopeParams struct {
	Client               K8sClient
	LinodePlacementGroup *infrav1alpha2.LinodePlacementGroup
	Cluster              *clusterv1.Cluster
}

func validatePlacementGroupScope(params PlacementGroupScopeParams) error {
	if params.LinodePlacementGroup == nil {
		return errors.New("linodePlacementGroup is required when creating a PlacementGroupScope")
	}

	return nil
}

// PatchObject persists the placement group configuration and status.
func (s *PlacementGroupScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.LinodePlacementGroup)
}

// Close closes the current scope persisting the placement group configuration and status.
func (s *PlacementGroupScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *PlacementGroupScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.LinodePlacementGroup, infrav1alpha2.PlacementGroupFinalizer) {
		return s.Close(ctx)
	}

	return nil
}

func (s *PlacementGroupScope) AddCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodePlacementGroup.Spec.CredentialsRef == nil {
		return nil
	}

	return addCredentialsFinalizer(ctx, s.Client,
		*s.LinodePlacementGroup.Spec.CredentialsRef, s.LinodePlacementGroup.GetNamespace(),
		toFinalizer(s.LinodePlacementGroup))
}

func (s *PlacementGroupScope) RemoveCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodePlacementGroup.Spec.CredentialsRef == nil {
		return nil
	}

	return removeCredentialsFinalizer(ctx, s.Client,
		*s.LinodePlacementGroup.Spec.CredentialsRef, s.LinodePlacementGroup.GetNamespace(),
		toFinalizer(s.LinodePlacementGroup))
}

// NewPlacementGroupScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
//
//nolint:dupl // This is pretty much the same as VPC, maybe a candidate to use generics later.
func NewPlacementGroupScope(ctx context.Context, linodeClientConfig ClientConfig, params PlacementGroupScopeParams) (*PlacementGroupScope, error) {
	if err := validatePlacementGroupScope(params); err != nil {
		return nil, err
	}
	linodeClient, err := CreateLinodeClient(
		linodeClientConfig,
		WithRetryCount(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	helper, err := patch.NewHelper(params.LinodePlacementGroup, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &PlacementGroupScope{
		Client:               params.Client,
		LinodeClient:         linodeClient,
		LinodePlacementGroup: params.LinodePlacementGroup,
		PatchHelper:          helper,
		Cluster:              params.Cluster,
	}, nil
}

func (s *PlacementGroupScope) SetCredentialRefTokenForLinodeClients(ctx context.Context) error {
	if s.LinodePlacementGroup.Spec.CredentialsRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, s.Client, *s.LinodePlacementGroup.Spec.CredentialsRef, s.LinodePlacementGroup.GetNamespace(), "apiToken")
		if err != nil {
			return fmt.Errorf("credentials from secret ref: %w", err)
		}
		s.LinodeClient = s.LinodeClient.SetToken(string(apiToken))
		return nil
	}
	return nil
}
