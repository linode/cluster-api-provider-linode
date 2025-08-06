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

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeObjectStorageBucket(t *testing.T) {
	t.Parallel()

	var (
		objvalidator = LinodeObjectStorageBucketCustomValidator{}
		bucket       = infrav1alpha2.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
				Region: "example",
			},
		}
		region            = linodego.Region{ID: "mock-region"}
		capabilities      = []string{linodego.CapabilityObjectStorage}
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
					bucket := bucket
					bucket.Spec.Region = "iad"
					assert.NoError(t, objvalidator.validateLinodeObjectStorageBucket(ctx, &bucket, mck.LinodeClient))
				}),
			),
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					bucket := bucket
					bucket.Spec.Region = "us-iad"
					assert.NoError(t, objvalidator.validateLinodeObjectStorageBucket(ctx, &bucket, mck.LinodeClient))
				}),
			),
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					bucket := bucket
					bucket.Spec.Region = "us-iad-1"
					assert.NoError(t, objvalidator.validateLinodeObjectStorageBucket(ctx, &bucket, mck.LinodeClient))
				}),
			),
		),
		OneOf(
			Path(
				Call("invalid region format in spec", func(ctx context.Context, mck Mock) {
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bucket := bucket
					bucket.Spec.Region = "123invalid"
					assert.Error(t, objvalidator.validateLinodeObjectStorageBucket(ctx, &bucket, mck.LinodeClient))
				}),
			),
			Path(
				Call("invalid region format in spec", func(ctx context.Context, mck Mock) {
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bucket := bucket
					bucket.Spec.Region = "invalid-2-2"
					assert.Error(t, objvalidator.validateLinodeObjectStorageBucket(ctx, &bucket, mck.LinodeClient))
				}),
			),
			Path(
				Call("region ID not present", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bucket := bucket
					bucket.Spec.Region = "us-1"
					assert.Error(t, objvalidator.validateLinodeObjectStorageBucket(ctx, &bucket, mck.LinodeClient))
				}),
			),
			Path(
				Call("region does not support Object storage capabilities", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities_zero)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					assert.Error(t, objvalidator.validateLinodeObjectStorageBucket(ctx, &bucket, mck.LinodeClient))
				}),
			),
		),
	)
}
