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

	"github.com/linode/linodego"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

// FirewallScope defines the basic context for an actuator to operate upon.
type FirewallScope struct {
	client client.Client

	PatchHelper    *patch.Helper
	LinodeClient   *linodego.Client
	LinodeFirewall *infrav1alpha1.LinodeFirewall
}

// FirewallScopeParams defines the input parameters used to create a new Scope.
type FirewallScopeParams struct {
	Client         client.Client
	LinodeFirewall *infrav1alpha1.LinodeFirewall
}

func validateFirewallScopeParams(params FirewallScopeParams) error {
	if params.LinodeFirewall == nil {
		return errors.New("linodeFirewall is required when creating a FirewallScope")
	}

	return nil
}

// NewFirewallScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewFirewallScope(apiKey string, params FirewallScopeParams) (*FirewallScope, error) {
	if err := validateFirewallScopeParams(params); err != nil {
		return nil, err
	}

	linodeClient := createLinodeClient(apiKey)

	helper, err := patch.NewHelper(params.LinodeFirewall, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &FirewallScope{
		client:         params.Client,
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
	if controllerutil.AddFinalizer(s.LinodeFirewall, infrav1alpha1.GroupVersion.String()) {
		return s.Close(ctx)
	}

	return nil
}
