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

package v1alpha1

import (
	"context"
	"slices"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeObjectStorageBucket(t *testing.T) {
	t.Parallel()

	var (
		bucket = LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodeObjectStorageBucketSpec{
				Cluster: "example-1",
			},
		}
		region            = linodego.Region{ID: "test"}
		capabilities      = []string{LinodeObjectStorageCapability}
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
					assert.NoError(t, bucket.validateLinodeObjectStorageBucket(ctx, mck.LinodeClient))
				}),
			),
		),
		OneOf(
			Path(
				Call("invalid cluster format", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bucket := bucket
					bucket.Spec.Cluster = "invalid"
					assert.Error(t, bucket.validateLinodeObjectStorageBucket(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("region not supported", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities_zero)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					assert.Error(t, bucket.validateLinodeObjectStorageBucket(ctx, mck.LinodeClient))
				}),
			),
		),
	)
}
