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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeCluster(t *testing.T) {
	t.Parallel()

	var (
		cluster = LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodeClusterSpec{
				Region: "example",
				Network: NetworkSpec{
					LoadBalancerType: "NodeBalancer",
					AdditionalPorts: []LinodeNBPortConfig{
						{
							Port:                 8132,
							NodeBalancerConfigID: ptr.To(1234),
						},
					},
				},
			},
		}
		validator = &linodeClusterValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, cluster.Spec)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(Call("invalid region", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, cluster.Spec)
			for _, err := range errs {
				require.Error(t, err)
			}
		}),
	)
}

func TestValidateCreate(t *testing.T) {
	t.Parallel()

	var (
		cluster = LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodeClusterSpec{
				Region: "example",
				Network: NetworkSpec{
					LoadBalancerType: "NodeBalancer",
				},
			},
		}
		validator = &linodeClusterValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("invalid region", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &cluster)
					assert.Error(t, err)
				}),
			),
		),
	)
}

func TestValidateDNSLinodeCluster(t *testing.T) {
	t.Parallel()

	var (
		validCluster = LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodeClusterSpec{
				Region: "us-ord",
				Network: NetworkSpec{
					LoadBalancerType:    "dns",
					DNSRootDomain:       "test.net",
					DNSUniqueIdentifier: "abc123",
				},
			},
		}
		inValidCluster = LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodeClusterSpec{
				Region: "us-ord",
				Network: NetworkSpec{
					LoadBalancerType: "dns",
				},
			},
		}
		validator = &linodeClusterValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, validCluster.Spec)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(Call("no root domain set", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, inValidCluster.Spec)
			for _, err := range errs {
				require.Contains(t, err.Error(), "dnsRootDomain")
			}
		}),
	)
}
