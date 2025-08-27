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

func TestValidateLinodeVPC(t *testing.T) {
	t.Parallel()

	var (
		vpc = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "example",
			},
		}
		region                        = linodego.Region{ID: "test"}
		capabilities                  = []string{linodego.CapabilityVPCs}
		capabilities_zero             = []string{}
		regionNotFoundError           = "spec.region: Not found: \"example\""
		vpcCapabilityError            = "spec.region: Invalid value: \"example\": no capability: VPCs"
		InvalidSubnetLabelError       = "spec.Subnets[0].Label: Invalid value: \"$\": can only contain ASCII letters, numbers, and hyphens (-)"
		ErrorSubnetRangeInvalidPrefix = "spec.Subnets[0].IPv4: Invalid value: \"10.0.0.0/32\": allowed prefix lengths: 1-29"
		ErrorSubnetRangeNotPrivate    = "spec.Subnets[0].IPv4: Invalid value: \"9.9.9.0/24\": range must belong to a private address space as defined in RFC1918"
		ErrorSubnetRange              = "spec.Subnets[0].IPv4: Invalid value: \"IPv4 CIDR\": must be IPv4 range in CIDR canonical form"
		ErrorSubnetRangeNotIPv4       = "spec.Subnets[0].IPv4: Invalid value: \"10.9.9.9/8\": must be IPv4 range in CIDR canonical form"
		validator                     = &linodeVPCValidator{}
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
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					require.Empty(t, errs)
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "foo", IPv4: "10.0.0.0/24"}, {Label: "bar", IPv4: "10.0.1.0/24"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("invalid region", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, regionNotFoundError)
					}
				}),
			),
			Path(
				Call("region not supported", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities_zero)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, vpcCapabilityError)
					}
				})),
		),
		OneOf(
			Path(
				Call("no subnet label", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{IPv4: "10.0.0.0/8"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						require.Error(t, err)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "$", IPv4: "10.0.0.0/8"}}

					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, InvalidSubnetLabelError)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "--", IPv4: "10.0.0.0/8"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						require.Error(t, err)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "test", IPv4: "IPv4 CIDR"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, ErrorSubnetRange)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "test", IPv4: "10.9.9.9/8"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, ErrorSubnetRangeNotIPv4)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "test", IPv4: "10.0.0.0/32"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, ErrorSubnetRangeInvalidPrefix)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "test", IPv4: "9.9.9.0/24"}}

					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, ErrorSubnetRangeNotPrivate)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "test", IPv4: "192.168.128.0/24"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						require.Error(t, err)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "test", IPv4: "10.255.255.1/24"}, {Label: "test", IPv4: "10.255.255.0/24"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						require.Error(t, err)
					}
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
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "foo", IPv4: "10.0.0.0/8"}, {Label: "bar", IPv4: "10.0.0.0/24"}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						require.Error(t, err)
					}
				}),
			),
			Path(
				Call("subnet ipv6 range is incorrect", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{{Label: "foo", IPv4: "10.0.0.0/8", IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{{Range: ptr.To("")}}}}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						require.Error(t, err)
					}
				}),
			),
		),
	)
}

func TestValidateVPCIPv6Ranges(t *testing.T) {
	t.Parallel()

	var (
		vpc = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "example",
			},
		}
		region                     = linodego.Region{ID: "test"}
		capabilities               = []string{linodego.CapabilityVPCs}
		ErrorIPv6RangeInvalid      = "spec.IPv6Range[0].Range: Invalid value: \"48\": IPv6 range must be either 'auto', valid IPv6 prefix or start with /. Example: auto, /52, 2001:db8::/52"
		ErrorIPv6RangeInvalidChars = "spec.IPv6Range[0].Range: Invalid value: \"/a48\": IPv6 range doesn't contain a valid number after /"
		ErrorIPv6RangeOutOfRange   = "spec.IPv6Range[0].Range: Invalid value: \"/130\": IPv6 range must be between /0 and /128"
		validator                  = &linodeVPCValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid ipv6 ranges in vpc", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.IPv6Range = []infrav1alpha2.VPCCreateOptionsIPv6{
						{Range: ptr.To("/48")},
						{Range: ptr.To("/52")},
						{Range: ptr.To("auto")},
						{Range: ptr.To("2001:db8::/52")},
					}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					require.Empty(t, errs)
				}),
			),
			Path(
				Call("valid ipv6 ranges in subnets", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{Label: "foo", IPv4: "10.0.0.0/24", IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{{Range: ptr.To("/52")}}},
						{Label: "bar", IPv4: "10.0.1.0/24", IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{{Range: ptr.To("/64")}}},
						{Label: "buzz", IPv4: "10.0.2.0/24", IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{{Range: ptr.To("auto")}}},
						{Label: "bazz", IPv4: "10.0.3.0/24", IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{{Range: ptr.To("2001:db8::/56")}}},
					}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("ipv6 range missing /", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.IPv6Range = []infrav1alpha2.VPCCreateOptionsIPv6{
						{Range: ptr.To("48")},
					}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, ErrorIPv6RangeInvalid)
					}
				}),
			),
			Path(
				Call("ipv6 range containing chars", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.IPv6Range = []infrav1alpha2.VPCCreateOptionsIPv6{
						{Range: ptr.To("/a48")},
					}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, ErrorIPv6RangeInvalidChars)
					}
				}),
			),
			Path(
				Call("ipv6 range out of bounds", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					vpc := vpc
					vpc.Spec.IPv6Range = []infrav1alpha2.VPCCreateOptionsIPv6{
						{Range: ptr.To("/130")},
					}
					errs := validator.validateLinodeVPCSpec(ctx, mck.LinodeClient, vpc.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, ErrorIPv6RangeOutOfRange)
					}
				}),
			),
		),
	)
}

func TestValidateCreateLinodeVPC(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		vpc = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "example",
			},
		}
		vpcLongName = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      longName,
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "example",
			},
		}
		validator         = &linodeVPCValidator{}
		credentialsRefVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				CredentialsRef: &corev1.SecretReference{
					Name: "vpc-credentials",
				},
				Region: "us-ord",
			},
		}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("invalid request", func(ctx context.Context, mck Mock) {

				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &vpc)
					assert.ErrorContains(t, err, "\"example\" is invalid: spec.region: Not found:")
				}),
			),
			Path(
				Call("name too long", func(ctx context.Context, mck Mock) {

				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &vpcLongName)
					assert.ErrorContains(t, err, labelLengthDetail)
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
									Name:      "vpc-credentials",
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
					str, err := getCredentialDataFromRef(ctx, mockK8sClient, *credentialsRefVPC.Spec.CredentialsRef, vpc.GetNamespace())
					require.NoError(t, err)
					assert.Equal(t, []byte("token"), str)
				}),
			),
		),
	)
}

func TestValidateLinodeVPCUpdate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		oldVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "example",
			},
		}
		newVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "example",
			},
		}

		validator = &linodeVPCValidator{Client: mockK8sClient}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("update", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateUpdate(ctx, &oldVPC, &newVPC)
					assert.NoError(t, err)
				}),
			),
		),
	)
}

func TestValidateLinodeVPCDelete(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		vpc = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "example",
			},
		}

		validator = &linodeVPCValidator{Client: mockK8sClient}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("delete", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateDelete(ctx, &vpc)
					assert.NoError(t, err)
				}),
			),
		),
	)
}
