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

// VPCScope defines the basic context for an actuator to operate upon.
type VPCScope struct {
	Client K8sClient

	PatchHelper  *patch.Helper
	LinodeClient LinodeClient
	LinodeVPC    *infrav1alpha2.LinodeVPC
	Cluster      *clusterv1.Cluster
}

// VPCScopeParams defines the input parameters used to create a new Scope.
type VPCScopeParams struct {
	Client    K8sClient
	LinodeVPC *infrav1alpha2.LinodeVPC
	Cluster   *clusterv1.Cluster
}

func validateVPCScopeParams(params VPCScopeParams) error {
	if params.LinodeVPC == nil {
		return errors.New("linodeVPC is required when creating a VPCScope")
	}

	return nil
}

// NewVPCScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
//
//nolint:dupl // this is the same as PlacementGroups - worth making into generics later.
func NewVPCScope(ctx context.Context, linodeClientConfig ClientConfig, params VPCScopeParams) (*VPCScope, error) {
	if err := validateVPCScopeParams(params); err != nil {
		return nil, err
	}
	linodeClient, err := CreateLinodeClient(linodeClientConfig,
		WithRetryCount(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	helper, err := patch.NewHelper(params.LinodeVPC, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &VPCScope{
		Client:       params.Client,
		LinodeClient: linodeClient,
		LinodeVPC:    params.LinodeVPC,
		PatchHelper:  helper,
		Cluster:      params.Cluster,
	}, nil
}

// PatchObject persists the machine configuration and status.
func (s *VPCScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.LinodeVPC)
}

// Close closes the current scope persisting the machine configuration and status.
func (s *VPCScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *VPCScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.LinodeVPC, infrav1alpha2.VPCFinalizer) {
		return s.Close(ctx)
	}

	return nil
}

func (s *VPCScope) AddCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodeVPC.Spec.CredentialsRef == nil {
		return nil
	}

	return addCredentialsFinalizer(ctx, s.Client,
		*s.LinodeVPC.Spec.CredentialsRef, s.LinodeVPC.GetNamespace(),
		toFinalizer(s.LinodeVPC))
}

func (s *VPCScope) RemoveCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodeVPC.Spec.CredentialsRef == nil {
		return nil
	}

	return removeCredentialsFinalizer(ctx, s.Client,
		*s.LinodeVPC.Spec.CredentialsRef, s.LinodeVPC.GetNamespace(),
		toFinalizer(s.LinodeVPC))
}

func (s *VPCScope) SetCredentialRefTokenForLinodeClients(ctx context.Context) error {
	if s.LinodeVPC.Spec.CredentialsRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, s.Client, *s.LinodeVPC.Spec.CredentialsRef, s.LinodeVPC.GetNamespace(), "apiToken")
		if err != nil {
			return fmt.Errorf("credentials from secret ref: %w", err)
		}
		s.LinodeClient = s.LinodeClient.SetToken(string(apiToken))
		return nil
	}
	return nil
}
