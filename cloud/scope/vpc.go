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

	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

// VPCScope defines the basic context for an actuator to operate upon.
type VPCScope struct {
	Client K8sClient

	PatchHelper  *patch.Helper
	LinodeClient LinodeVPCClient
	LinodeVPC    *infrav1alpha1.LinodeVPC
}

// VPCScopeParams defines the input parameters used to create a new Scope.
type VPCScopeParams struct {
	Client    K8sClient
	LinodeVPC *infrav1alpha1.LinodeVPC
}

func validateVPCScopeParams(params VPCScopeParams) error {
	if params.LinodeVPC == nil {
		return errors.New("linodeVPC is required when creating a VPCScope")
	}

	return nil
}

// NewVPCScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewVPCScope(ctx context.Context, apiKey string, params VPCScopeParams) (*VPCScope, error) {
	if err := validateVPCScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the VPC's Secret reference (if supplied).
	if params.LinodeVPC.Spec.CredentialsRef != nil {
		data, err := getCredentialDataFromRef(ctx, params.Client, *params.LinodeVPC.Spec.CredentialsRef, params.LinodeVPC.GetNamespace())
		if err != nil {
			return nil, fmt.Errorf("credentials from secret ref: %w", err)
		}
		apiKey = string(data)
	}
	linodeClient, err := CreateLinodeClient(apiKey)
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
	if controllerutil.AddFinalizer(s.LinodeVPC, infrav1alpha1.GroupVersion.String()) {
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
