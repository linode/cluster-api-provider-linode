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
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeVPC(t *testing.T) {
	t.Parallel()

	var (
		vpc = LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodeVPCSpec{
				Region: "example",
			},
		}
		region            = linodego.Region{ID: "test"}
		capabilities      = []string{LinodeVPCCapability}
		capabilities_zero = []string{}
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
					assert.NoError(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("valid with subnets", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "foo", IPv4: "10.0.0.0/24"}, {Label: "bar", IPv4: "10.0.1.0/24"}}
					assert.NoError(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
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
			assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
		}),
		OneOf(
			Path(
				Call("no subnet label", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{IPv4: "10.0.0.0/8"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("invalid subnet label", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "$", IPv4: "10.0.0.0/8"}}

					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("invalid subnet label", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "--", IPv4: "10.0.0.0/8"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),

			Path(
				Call("subnet range not IPv4 CIDR", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "test", IPv4: "IPv4 CIDR"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("subnet range not CIDR canonical form", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "test", IPv4: "10.9.9.9/8"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("subnet range invalid prefix length", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "test", IPv4: "10.0.0.0/32"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("subnet range not private", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "test", IPv4: "9.9.9.0/24"}}

					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("subnet range overlaps reserved range(s)", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "test", IPv4: "192.168.128.0/24"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("subnet labels not unique", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "test", IPv4: "10.255.255.1/24"}, {Label: "test", IPv4: "10.255.255.0/24"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("subnet ranges overlaps", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []VPCSubnetCreateOptions{{Label: "foo", IPv4: "10.0.0.0/8"}, {Label: "bar", IPv4: "10.0.0.0/24"}}
					assert.Error(t, vpc.validateLinodeVPC(ctx, mck.LinodeClient))
				}),
			),
		),
	)
}
