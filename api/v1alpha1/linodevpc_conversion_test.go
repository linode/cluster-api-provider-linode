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
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestConvertVPCTo(t *testing.T) {
	t.Parallel()

	src := &LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vpc",
		},
		Spec: LinodeVPCSpec{
			VPCID:       ptr.To(1234),
			Description: "test vpc",
			Region:      "us-ord",
			Subnets: []VPCSubnetCreateOptions{{
				Label: "subnet1",
				IPv4:  "10.0.0.0/24",
			}},
			CredentialsRef: nil,
		},
	}
	expectedDst := &infrav1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vpc",
		},
		Spec: infrav1alpha2.LinodeVPCSpec{
			VPCID:       ptr.To(1234),
			Description: "test vpc",
			Region:      "us-ord",
			Subnets: []infrav1alpha2.VPCSubnetCreateOptions{{
				Label: "subnet1",
				IPv4:  "10.0.0.0/24",
			}},
			CredentialsRef: nil,
		},
	}
	srcList := &LinodeVPCList{
		Items: append([]LinodeVPC{}, *src),
	}
	expectedDstList := &infrav1alpha2.LinodeVPCList{
		Items: append([]infrav1alpha2.LinodeVPC{}, *expectedDst),
	}
	dstList := &infrav1alpha2.LinodeVPCList{}
	dst := &infrav1alpha2.LinodeVPC{}

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("convert v1alpha1 to v1alpha2", func(ctx context.Context, mck Mock) {
					err := src.ConvertTo(dst)
					if err != nil {
						t.Fatalf("ConvertTo failed: %v", err)
					}
				}),
				Result("conversion succeeded", func(ctx context.Context, mck Mock) {
					if diff := cmp.Diff(expectedDst, dst); diff != "" {
						t.Errorf("ConvertTo() mismatch (-expected +got):\n%s", diff)
					}
				}),
			),
			Path(
				Call("convert v1alpha1 list to v1alpha2 list", func(ctx context.Context, mck Mock) {
					err := srcList.ConvertTo(dstList)
					if err != nil {
						t.Fatalf("ConvertTo failed: %v", err)
					}
				}),
				Result("conversion succeeded", func(ctx context.Context, mck Mock) {
					if diff := cmp.Diff(expectedDstList, dstList); diff != "" {
						t.Errorf("ConvertTo() mismatch (-expected +got):\n%s", diff)
					}
				}),
			),
		),
	)
}

func TestConvertVPCFrom(t *testing.T) {
	t.Parallel()

	src := &infrav1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vpc",
		},
		Spec: infrav1alpha2.LinodeVPCSpec{
			VPCID:       ptr.To(1234),
			Description: "test vpc",
			Region:      "us-ord",
			Subnets: []infrav1alpha2.VPCSubnetCreateOptions{{
				Label: "subnet1",
				IPv4:  "10.0.0.0/24",
			}},
			CredentialsRef: nil,
		},
	}
	expectedDst := &LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vpc",
		},
		Spec: LinodeVPCSpec{
			VPCID:       ptr.To(1234),
			Description: "test vpc",
			Region:      "us-ord",
			Subnets: []VPCSubnetCreateOptions{{
				Label: "subnet1",
				IPv4:  "10.0.0.0/24",
			}},
			CredentialsRef: nil,
		},
	}

	srcList := &infrav1alpha2.LinodeVPCList{
		Items: append([]infrav1alpha2.LinodeVPC{}, *src),
	}
	expectedDstList := &LinodeVPCList{
		Items: append([]LinodeVPC{}, *expectedDst),
	}
	if err := utilconversion.MarshalData(src, expectedDst); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}
	dstList := &LinodeVPCList{}
	dst := &LinodeVPC{}

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("convert v1alpha2 to v1alpha1", func(ctx context.Context, mck Mock) {
					err := dst.ConvertFrom(src)
					if err != nil {
						t.Fatalf("ConvertFrom failed: %v", err)
					}
				}),
				Result("conversion succeeded", func(ctx context.Context, mck Mock) {
					if diff := cmp.Diff(expectedDst, dst); diff != "" {
						t.Errorf("ConvertFrom() mismatch (-expected +got):\n%s", diff)
					}
				}),
			),
			Path(
				Call("convert v1alpha2 list to v1alpha1 list", func(ctx context.Context, mck Mock) {
					err := dstList.ConvertFrom(srcList)
					if err != nil {
						t.Fatalf("ConvertFrom failed: %v", err)
					}
				}),
				Result("conversion succeeded", func(ctx context.Context, mck Mock) {
					if diff := cmp.Diff(expectedDstList, dstList); diff != "" {
						t.Errorf("ConvertFrom() mismatch (-expected +got):\n%s", diff)
					}
				}),
			),
		),
	)
}
