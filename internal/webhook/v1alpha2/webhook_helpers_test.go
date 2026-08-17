/*
Copyright 2024 Akamai Technologies, Inc.

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
	"errors"
	"testing"

	"github.com/linode/linodego/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateInterfaceAccountSettings(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec")

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			// Neither interface type set → no API call, no error.
			Path(
				Call("neither interface type set", func(ctx context.Context, mck Mock) {}),
				Result("no error", func(ctx context.Context, mck Mock) {
					ferr, err := validateInterfaceAccountSettings(ctx, mck.LinodeClient, false, false, path)
					require.NoError(t, err)
					assert.Nil(t, ferr)
				}),
			),
			// Linode Interfaces requested, account only allows legacy → forbidden.
			Path(
				Call("LinodeInterfaces + LegacyConfigOnly", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(&linodego.AccountSettings{
						InterfacesForNewLinodes: linodego.LegacyConfigOnly,
					}, nil)
				}),
				Result("forbidden", func(ctx context.Context, mck Mock) {
					ferr, err := validateInterfaceAccountSettings(ctx, mck.LinodeClient, false, true, path)
					require.NoError(t, err)
					require.NotNil(t, ferr)
					assert.Contains(t, ferr.Detail, "legacy_config_only")
				}),
			),
			// Legacy interfaces requested, account only allows Linode → forbidden.
			Path(
				Call("legacy interfaces + LinodeOnly", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(&linodego.AccountSettings{
						InterfacesForNewLinodes: linodego.LinodeOnly,
					}, nil)
				}),
				Result("forbidden", func(ctx context.Context, mck Mock) {
					ferr, err := validateInterfaceAccountSettings(ctx, mck.LinodeClient, true, false, path)
					require.NoError(t, err)
					require.NotNil(t, ferr)
					assert.Contains(t, ferr.Detail, "linode_only")
				}),
			),
			// Linode Interfaces requested, account allows both → permitted.
			Path(
				Call("LinodeInterfaces + LegacyConfigDefaultButLinodeAllowed", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(&linodego.AccountSettings{
						InterfacesForNewLinodes: linodego.LegacyConfigDefaultButLinodeAllowed,
					}, nil)
				}),
				Result("permitted", func(ctx context.Context, mck Mock) {
					ferr, err := validateInterfaceAccountSettings(ctx, mck.LinodeClient, false, true, path)
					require.NoError(t, err)
					assert.Nil(t, ferr)
				}),
			),
			// Legacy interfaces requested, account allows both → permitted.
			Path(
				Call("legacy interfaces + LinodeDefaultButLegacyConfigAllowed", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(&linodego.AccountSettings{
						InterfacesForNewLinodes: linodego.LinodeDefaultButLegacyConfigAllowed,
					}, nil)
				}),
				Result("permitted", func(ctx context.Context, mck Mock) {
					ferr, err := validateInterfaceAccountSettings(ctx, mck.LinodeClient, true, false, path)
					require.NoError(t, err)
					assert.Nil(t, ferr)
				}),
			),
			// GetAccountSettings returns an error.
			Path(
				Call("GetAccountSettings returns error", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(nil, errors.New("API error"))
				}),
				Result("wraps error", func(ctx context.Context, mck Mock) {
					ferr, err := validateInterfaceAccountSettings(ctx, mck.LinodeClient, true, false, path)
					require.Error(t, err)
					require.ErrorContains(t, err, "failed to get customer account settings")
					assert.Nil(t, ferr)
				}),
			),
		),
	)
}
