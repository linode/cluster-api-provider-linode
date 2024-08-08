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

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

// FirewallScope defines the basic context for an actuator to operate upon.
type FirewallScope struct {
	Client K8sClient

	PatchHelper    *patch.Helper
	LinodeClient   LinodeClient
	LinodeFirewall *infrav1alpha2.LinodeFirewall
}

// FirewallScopeParams defines the input parameters used to create a new Scope.
type FirewallScopeParams struct {
	Client         K8sClient
	LinodeFirewall *infrav1alpha2.LinodeFirewall
}

func validateFirewallScopeParams(params FirewallScopeParams) error {
	if params.LinodeFirewall == nil {
		return errors.New("linodeFirewall is required when creating a FirewallScope")
	}

	return nil
}

// NewFirewallScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
//
//nolint:dupl // this is the same as PlacementGroups - worth making into generics later.
func NewFirewallScope(ctx context.Context, linodeClientConfig ClientConfig, params FirewallScopeParams) (*FirewallScope, error) {
	if err := validateFirewallScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Firewall's Secret reference (if supplied).
	if params.LinodeFirewall.Spec.CredentialsRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, params.Client, *params.LinodeFirewall.Spec.CredentialsRef, params.LinodeFirewall.GetNamespace(), "apiToken")
		if err != nil {
			return nil, fmt.Errorf("credentials from secret ref: %w", err)
		}
		linodeClientConfig.Token = string(apiToken)
	}
	linodeClient, err := CreateLinodeClient(linodeClientConfig,
		WithRetryCount(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	helper, err := patch.NewHelper(params.LinodeFirewall, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &FirewallScope{
		Client:         params.Client,
		LinodeClient:   linodeClient,
		LinodeFirewall: params.LinodeFirewall,
		PatchHelper:    helper,
	}, nil
}

// PatchObject persists the machine configuration and status.
func (s *FirewallScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.LinodeFirewall)
}

// Close closes the current scope persisting the machine configuration and status.
func (s *FirewallScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *FirewallScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.LinodeFirewall, infrav1alpha2.GroupVersion.String()) {
		return s.Close(ctx)
	}

	return nil
}

func (s *FirewallScope) AddCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodeFirewall.Spec.CredentialsRef == nil {
		return nil
	}

	return addCredentialsFinalizer(ctx, s.Client,
		*s.LinodeFirewall.Spec.CredentialsRef, s.LinodeFirewall.GetNamespace(),
		toFinalizer(s.LinodeFirewall))
}

func (s *FirewallScope) RemoveCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodeFirewall.Spec.CredentialsRef == nil {
		return nil
	}

	return removeCredentialsFinalizer(ctx, s.Client,
		*s.LinodeFirewall.Spec.CredentialsRef, s.LinodeFirewall.GetNamespace(),
		toFinalizer(s.LinodeFirewall))
}
