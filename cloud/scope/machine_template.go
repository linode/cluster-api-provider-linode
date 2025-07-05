/*
Copyright 2025 Akamai Technologies, Inc.

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

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
)

// MachineTemplateScope defines the basic context for an actuator to operate upon.
type MachineTemplateScope struct {
	PatchHelper           *patch.Helper
	LinodeMachineTemplate *infrav1alpha2.LinodeMachineTemplate
}

// MachineTemplateScopeParams defines the input parameters used to create a new MachineTemplateScope.
type MachineTemplateScopeParams struct {
	Client                clients.K8sClient
	LinodeMachineTemplate *infrav1alpha2.LinodeMachineTemplate
}

// validateMachineTemplateScope validates the parameters for creating a MachineTemplateScope.
func validateMachineTemplateScope(params MachineTemplateScopeParams) error {
	if params.LinodeMachineTemplate == nil {
		return errors.New("LinodeMachineTemplate is required when creating a MachineTemplateScope")
	}

	return nil
}

// PatchObject persists the machine template configuration and status.
func (s *MachineTemplateScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.LinodeMachineTemplate)
}

// Close closes the current scope persisting the machine template configuration and status.
func (s *MachineTemplateScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// NewMachineTemplateScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachineTemplateScope(ctx context.Context, params MachineTemplateScopeParams) (*MachineTemplateScope, error) {
	if err := validateMachineTemplateScope(params); err != nil {
		return nil, err
	}

	helper, err := patch.NewHelper(params.LinodeMachineTemplate, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &MachineTemplateScope{
		LinodeMachineTemplate: params.LinodeMachineTemplate,
		PatchHelper:           helper,
	}, nil
}
