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
	"errors"
	"slices"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodePlacementGroup(t *testing.T) {
	t.Parallel()

	var (
		pg = LinodePlacementGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodePlacementGroupSpec{
				Region: "us-sea",
			},
		}
		region            = linodego.Region{ID: "test"}
		capabilities      = []string{LinodePlacementGroupCapability}
		capabilities_zero = []string{}

		validator = &linodePlacementGroupValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodePlacementGroupSpec(ctx, mck.LinodeClient, pg.Spec, pg.ObjectMeta.Name)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(Call("invalid region", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
			})),
			Path(Call("region not supported", func(ctx context.Context, mck Mock) {
				region := region
				region.Capabilities = slices.Clone(capabilities_zero)
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			errs := validator.validateLinodePlacementGroupSpec(ctx, mck.LinodeClient, pg.Spec, pg.ObjectMeta.Name)
			for _, err := range errs {
				require.Error(t, err)
			}
		}),
		OneOf(
			Path(
				Call("invalid placementgroup label", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					pg := pg
					pg.Name = "a20_b!4"
					errs := validator.validateLinodePlacementGroupSpec(ctx, mck.LinodeClient, pg.Spec, pg.ObjectMeta.Name)
					for _, err := range errs {
						require.Error(t, err)
					}
				}),
			),
		),
	)
}
