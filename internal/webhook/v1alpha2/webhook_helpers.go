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

package v1alpha2

import (
	"context"
	"fmt"
	"slices"

	"github.com/linode/linodego/v2"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/linode/cluster-api-provider-linode/clients"
)

const (
	// minLabelLength is the minimum length for a Linode resource label
	minLabelLength = 3
	// maxLabelLength is the maximum length for a Linode resource label
	maxLabelLength    = 32
	labelLengthDetail = "must be between 3 and 32 characters"
)

func validateLabelLength(label string, path *field.Path) *field.Error {
	if len(label) < minLabelLength || len(label) > maxLabelLength {
		return field.Invalid(path, label, labelLengthDetail)
	}

	return nil
}

func validateRegion(ctx context.Context, linodegoclient clients.LinodeClient, id string, path *field.Path, capabilities ...linodego.RegionCapability) *field.Error {
	region, err := linodegoclient.GetRegion(ctx, id)
	if err != nil {
		return field.NotFound(path, id)
	}

	for _, capability := range capabilities {
		if !slices.Contains(region.Capabilities, string(capability)) {
			return field.Invalid(path, id, fmt.Sprintf("no capability: %s", capability))
		}
	}

	return nil
}

func validateLinodeType(ctx context.Context, linodegoclient clients.LinodeClient, id string, path *field.Path) (*linodego.LinodeType, *field.Error) {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	plan, err := linodegoclient.GetType(ctx, id)
	if err != nil {
		return nil, field.NotFound(path, id)
	}

	return plan, nil
}
