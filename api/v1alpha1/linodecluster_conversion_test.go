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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestConvertLinodeClusterTo(t *testing.T) {
	t.Parallel()

	src := &LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: LinodeClusterSpec{
			Network: NetworkSpec{
				LoadBalancerType:     "test-type",
				LoadBalancerPort:     12345,
				NodeBalancerID:       ptr.To(1234),
				NodeBalancerConfigID: ptr.To(2345),
			},
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
			Region:               "test-region",
		},
	}
	expectedDst := &infrav1alpha2.LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: infrav1alpha2.LinodeClusterSpec{
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType:              "test-type",
				NodeBalancerID:                ptr.To(1234),
				ApiserverLoadBalancerPort:     12345,
				ApiserverNodeBalancerConfigID: ptr.To(2345),
				AdditionalPorts:               []infrav1alpha2.LinodeNBPortConfig{},
			},
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
			Region:               "test-region",
		},
	}
	srcList := &LinodeClusterList{
		Items: append([]LinodeCluster{}, *src),
	}
	expectedDstList := &infrav1alpha2.LinodeClusterList{
		Items: append([]infrav1alpha2.LinodeCluster{}, *expectedDst),
	}
	dstList := &infrav1alpha2.LinodeClusterList{}
	dst := &infrav1alpha2.LinodeCluster{}

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

func TestConvertLinodeClusterFrom(t *testing.T) {
	t.Parallel()

	src := &infrav1alpha2.LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: infrav1alpha2.LinodeClusterSpec{
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType:              "test-type",
				NodeBalancerID:                ptr.To(1234),
				ApiserverLoadBalancerPort:     12345,
				ApiserverNodeBalancerConfigID: ptr.To(2345),
				AdditionalPorts: []infrav1alpha2.LinodeNBPortConfig{
					{
						Port:                 8132,
						NodeBalancerConfigID: ptr.To(1111),
					},
				},
			},
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
			Region:               "test-region",
		},
	}
	expectedDst := &LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: LinodeClusterSpec{
			Network: NetworkSpec{
				LoadBalancerType:     "test-type",
				LoadBalancerPort:     12345,
				NodeBalancerID:       ptr.To(1234),
				NodeBalancerConfigID: ptr.To(2345),
			},
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "1.2.3.4"},
			Region:               "test-region",
		},
	}

	srcList := &infrav1alpha2.LinodeClusterList{
		Items: append([]infrav1alpha2.LinodeCluster{}, *src),
	}
	expectedDstList := &LinodeClusterList{
		Items: append([]LinodeCluster{}, *expectedDst),
	}
	if err := utilconversion.MarshalData(src, expectedDst); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}
	dstList := &LinodeClusterList{}
	dst := &LinodeCluster{}

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
