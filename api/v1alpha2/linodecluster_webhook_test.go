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
	corev1 "k8s.io/api/core/v1"
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
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					assert.NoError(t, cluster.validateLinodeCluster(ctx, mck.LinodeClient))
				}),
			),
		),
		OneOf(
			Path(Call("invalid region", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			assert.Error(t, cluster.validateLinodeCluster(ctx, mck.LinodeClient))
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
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("invalid region", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					//nolint:contextcheck // no context passed
					_, err := cluster.ValidateCreate()
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
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					assert.NoError(t, validCluster.validateLinodeCluster(ctx, mck.LinodeClient))
				}),
			),
		),
		OneOf(
			Path(Call("no root domain set", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			require.ErrorContains(t, inValidCluster.validateLinodeCluster(ctx, mck.LinodeClient), "dnsRootDomain")
		}),
	)
}

func TestValidateVlanAndVPC(t *testing.T) {
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
					UseVlan: true,
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
					UseVlan: true,
				},
				VPCRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeVPC",
				},
			},
		}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					assert.NoError(t, validCluster.validateLinodeCluster(ctx, mck.LinodeClient))
				}),
			),
		),
		OneOf(
			Path(Call("vlan and VPC set", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			require.ErrorContains(t, inValidCluster.validateLinodeCluster(ctx, mck.LinodeClient), "Cannot use VLANs and VPCs together")
		}),
	)
}
