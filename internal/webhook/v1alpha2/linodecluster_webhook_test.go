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

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeCluster(t *testing.T) {
	t.Parallel()

	var (
		cluster = infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "example",
				Network: infrav1alpha2.NetworkSpec{
					LoadBalancerType: "NodeBalancer",
					AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
						{
							Port:                 8132,
							NodeBalancerConfigID: ptr.To(1234),
						},
					},
				},
			},
		}
		validator              = &linodeClusterValidator{}
		expectedErrorSubString = "spec.region: Not found: \"example\""
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
				assert.ErrorContains(t, err, expectedErrorSubString)
			}
		}),
	)
}

func TestValidateCreate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		cluster = infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "example",
				Network: infrav1alpha2.NetworkSpec{
					LoadBalancerType: "NodeBalancer",
				},
			},
		}
		expectedErrorSubString = "\"example\" is invalid: spec.region: Not found:"
		credentialsRefCluster  = infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				CredentialsRef: &corev1.SecretReference{
					Name: "cluster-credentials",
				},
				Region: "us-ord",
				Network: infrav1alpha2.NetworkSpec{
					LoadBalancerType: "NodeBalancer",
				},
			},
		}
		validator = &linodeClusterValidator{Client: mockK8sClient}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("invalid request", func(ctx context.Context, mck Mock) {

				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &cluster)
					assert.ErrorContains(t, err, expectedErrorSubString)
				}),
			),
		),
		OneOf(
			Path(
				Call("verfied linodeClient", func(ctx context.Context, mck Mock) {
					mockK8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
							cred := corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cluster-credentials",
									Namespace: "example",
								},
								Data: map[string][]byte{
									"apiToken": []byte("token"),
								},
							}
							*obj = cred

							return nil
						}).AnyTimes()
				}),
				Result("valid", func(ctx context.Context, mck Mock) {
					str, err := getCredentialDataFromRef(ctx, mockK8sClient, *credentialsRefCluster.Spec.CredentialsRef, cluster.GetNamespace())
					require.NoError(t, err)
					assert.Equal(t, []byte("token"), str)
				}),
			),
		),
	)
}

func TestValidateDNSLinodeCluster(t *testing.T) {
	t.Parallel()

	var (
		validCluster = infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-ord",
				Network: infrav1alpha2.NetworkSpec{
					LoadBalancerType:    "dns",
					DNSRootDomain:       "test.net",
					DNSUniqueIdentifier: "abc123",
				},
			},
		}
		inValidCluster = infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-ord",
				Network: infrav1alpha2.NetworkSpec{
					LoadBalancerType: "dns",
				},
			},
		}
		validator = &linodeClusterValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid dns", func(ctx context.Context, mck Mock) {
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

func TestValidateVlanAndVPC(t *testing.T) {
	t.Parallel()

	var (
		validCluster = infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-ord",
				Network: infrav1alpha2.NetworkSpec{
					UseVlan: true,
				},
			},
		}
		inValidCluster = infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-ord",
				Network: infrav1alpha2.NetworkSpec{
					UseVlan: true,
				},
				VPCRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeVPC",
				},
			},
		}
		validator = &linodeClusterValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid vlan", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, validCluster.Spec)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(Call("vlan and VPC set", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, inValidCluster.Spec)
			for _, err := range errs {
				require.Contains(t, err.Error(), "Cannot use VLANs and VPCs together")
			}
		}),
	)
}

func TestValidateVPCIDAndVPCRef(t *testing.T) {
	t.Parallel()

	var (
		invalidCluster = &infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-ord",
				VPCID:  ptr.To(12345),
				VPCRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeVPC",
				},
			},
		}
		validClusterWithVPCID = &infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-ord",
				VPCID:  ptr.To(12345),
			},
		}
		validClusterWithVPCRef = &infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-ord",
				VPCRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeVPC",
				},
			},
		}
		validator = &linodeClusterValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid with VPCID", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Any()).Return(&linodego.VPC{
						ID: 12345,
						Subnets: []linodego.VPCSubnet{
							{
								ID:    1001,
								Label: "subnet-1",
							},
						},
					}, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, validClusterWithVPCID.Spec)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("valid with VPCRef", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, validClusterWithVPCRef.Spec)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("both VPCID and VPCRef set", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeClusterSpec(ctx, mck.LinodeClient, invalidCluster.Spec)
					require.NotEmpty(t, errs)
					require.Contains(t, errs[0].Error(), "Cannot specify both VPCID and VPCRef")
				}),
			),
		),
	)
}
