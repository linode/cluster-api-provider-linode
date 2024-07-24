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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestLinodeObjectStorageBucketConvertTo(t *testing.T) {
	t.Parallel()

	src := &LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{Name: "test-bucket"},
		Spec: LinodeObjectStorageBucketSpec{
			Cluster: "us-mia-1",
			CredentialsRef: &corev1.SecretReference{
				Namespace: "default",
				Name:      "cred-secret",
			},
			KeyGeneration: ptr.To(1),
			SecretType:    "Opaque",
		},
		Status: LinodeObjectStorageBucketStatus{},
	}
	expectedDst := &infrav1alpha2.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{Name: "test-bucket"},
		Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
			Region: "us-mia",
			CredentialsRef: &corev1.SecretReference{
				Namespace: "default",
				Name:      "cred-secret",
			},
			KeyGeneration: ptr.To(1),
			SecretType:    "Opaque",
		},
		Status: infrav1alpha2.LinodeObjectStorageBucketStatus{},
	}
	dst := &infrav1alpha2.LinodeObjectStorageBucket{}

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
		),
	)
}

func TestLLinodeObjectStorageBucketFrom(t *testing.T) {
	t.Parallel()

	src := &infrav1alpha2.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{Name: "test-bucket"},
		Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
			Region: "us-mia",
			CredentialsRef: &corev1.SecretReference{
				Namespace: "default",
				Name:      "cred-secret",
			},
			KeyGeneration: ptr.To(1),
			SecretType:    "Opaque",
		},
		Status: infrav1alpha2.LinodeObjectStorageBucketStatus{},
	}
	expectedDst := &LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{Name: "test-bucket"},
		Spec: LinodeObjectStorageBucketSpec{
			Cluster: "us-mia-1",
			CredentialsRef: &corev1.SecretReference{
				Namespace: "default",
				Name:      "cred-secret",
			},
			KeyGeneration: ptr.To(1),
			SecretType:    "Opaque",
		},
		Status: LinodeObjectStorageBucketStatus{},
	}
	dst := &LinodeObjectStorageBucket{}

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
		),
	)
}
