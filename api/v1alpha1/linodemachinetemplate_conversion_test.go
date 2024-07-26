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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestLinodeMachineTemplateConvertTo(t *testing.T) {
	t.Parallel()

	src := &LinodeMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-machine"},
		Spec: LinodeMachineTemplateSpec{
			Template: LinodeMachineTemplateResource{
				Spec: LinodeMachineSpec{
					ProviderID:      ptr.To("linode://1234"),
					InstanceID:      ptr.To(1234),
					Region:          "us-mia",
					Type:            "g6-standard-2",
					Group:           "",
					RootPass:        "abc123",
					AuthorizedKeys:  []string{"authorizedKey1"},
					AuthorizedUsers: []string{"authorizedUser1"},
					BackupID:        1234,
					Image:           "linode/ubuntu24.04",
					Interfaces:      []InstanceConfigInterfaceCreateOptions{{Primary: true}},
					BackupsEnabled:  false,
					PrivateIP:       ptr.To(true),
					Tags:            []string{"test instance"},
					FirewallID:      123,
					OSDisk: ptr.To(InstanceDisk{
						DiskID:     0,
						Size:       *resource.NewQuantity(12, resource.DecimalSI),
						Label:      "main disk",
						Filesystem: "",
					}),
					DataDisks: map[string]*InstanceDisk{"sdb": {
						DiskID:     0,
						Size:       *resource.NewQuantity(145, resource.DecimalSI),
						Label:      "etcd disk",
						Filesystem: "",
					},
						"sdc": {
							DiskID:     0,
							Size:       *resource.NewQuantity(543, resource.DecimalSI),
							Label:      "another disk",
							Filesystem: "",
						}},
					CredentialsRef: &corev1.SecretReference{
						Namespace: "default",
						Name:      "cred-secret",
					},
				},
			},
		},
	}
	expectedDst := &infrav1alpha2.LinodeMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-machine"},
		Spec: infrav1alpha2.LinodeMachineTemplateSpec{
			Template: infrav1alpha2.LinodeMachineTemplateResource{
				Spec: infrav1alpha2.LinodeMachineSpec{
					ProviderID:      ptr.To("linode://1234"),
					InstanceID:      ptr.To(1234),
					Region:          "us-mia",
					Type:            "g6-standard-2",
					Group:           "",
					RootPass:        "abc123",
					AuthorizedKeys:  []string{"authorizedKey1"},
					AuthorizedUsers: []string{"authorizedUser1"},
					BackupID:        1234,
					Image:           "linode/ubuntu24.04",
					Interfaces:      []infrav1alpha2.InstanceConfigInterfaceCreateOptions{{Primary: true}},
					BackupsEnabled:  false,
					PrivateIP:       ptr.To(true),
					Tags:            []string{"test instance"},
					FirewallID:      123,
					OSDisk: ptr.To(infrav1alpha2.InstanceDisk{
						DiskID:     0,
						Size:       *resource.NewQuantity(12, resource.DecimalSI),
						Label:      "main disk",
						Filesystem: "",
					}),
					DataDisks: map[string]*infrav1alpha2.InstanceDisk{"sdb": {
						DiskID:     0,
						Size:       *resource.NewQuantity(145, resource.DecimalSI),
						Label:      "etcd disk",
						Filesystem: "",
					},
						"sdc": {
							DiskID:     0,
							Size:       *resource.NewQuantity(543, resource.DecimalSI),
							Label:      "another disk",
							Filesystem: "",
						}},
					CredentialsRef: &corev1.SecretReference{
						Namespace: "default",
						Name:      "cred-secret",
					},
				},
			},
		},
	}
	dst := &infrav1alpha2.LinodeMachineTemplate{}

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

func TestLinodeMachineTemplateConvertFrom(t *testing.T) {
	t.Parallel()

	src := &infrav1alpha2.LinodeMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-machine"},
		Spec: infrav1alpha2.LinodeMachineTemplateSpec{
			Template: infrav1alpha2.LinodeMachineTemplateResource{
				Spec: infrav1alpha2.LinodeMachineSpec{
					ProviderID:      ptr.To("linode://1234"),
					InstanceID:      ptr.To(1234),
					Region:          "us-mia",
					Type:            "g6-standard-2",
					Group:           "",
					RootPass:        "abc123",
					AuthorizedKeys:  []string{"authorizedKey1"},
					AuthorizedUsers: []string{"authorizedUser1"},
					BackupID:        1234,
					Image:           "linode/ubuntu24.04",
					Interfaces:      []infrav1alpha2.InstanceConfigInterfaceCreateOptions{{Primary: true}},
					BackupsEnabled:  false,
					PrivateIP:       ptr.To(true),
					Tags:            []string{"test instance"},
					FirewallID:      123,
					OSDisk: ptr.To(infrav1alpha2.InstanceDisk{
						DiskID:     0,
						Size:       *resource.NewQuantity(12, resource.DecimalSI),
						Label:      "main disk",
						Filesystem: "",
					}),
					DataDisks: map[string]*infrav1alpha2.InstanceDisk{"sdb": {
						DiskID:     0,
						Size:       *resource.NewQuantity(145, resource.DecimalSI),
						Label:      "etcd disk",
						Filesystem: "",
					},
						"sdc": {
							DiskID:     0,
							Size:       *resource.NewQuantity(543, resource.DecimalSI),
							Label:      "another disk",
							Filesystem: "",
						}},
					CredentialsRef: &corev1.SecretReference{
						Namespace: "default",
						Name:      "cred-secret",
					},
				},
			},
		},
	}
	expectedDst := &LinodeMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-machine"},
		Spec: LinodeMachineTemplateSpec{
			Template: LinodeMachineTemplateResource{
				Spec: LinodeMachineSpec{
					ProviderID:      ptr.To("linode://1234"),
					InstanceID:      ptr.To(1234),
					Region:          "us-mia",
					Type:            "g6-standard-2",
					Group:           "",
					RootPass:        "abc123",
					AuthorizedKeys:  []string{"authorizedKey1"},
					AuthorizedUsers: []string{"authorizedUser1"},
					BackupID:        1234,
					Image:           "linode/ubuntu24.04",
					Interfaces:      []InstanceConfigInterfaceCreateOptions{{Primary: true}},
					BackupsEnabled:  false,
					PrivateIP:       ptr.To(true),
					Tags:            []string{"test instance"},
					FirewallID:      123,
					OSDisk: ptr.To(InstanceDisk{
						DiskID:     0,
						Size:       *resource.NewQuantity(12, resource.DecimalSI),
						Label:      "main disk",
						Filesystem: "",
					}),
					DataDisks: map[string]*InstanceDisk{"sdb": {
						DiskID:     0,
						Size:       *resource.NewQuantity(145, resource.DecimalSI),
						Label:      "etcd disk",
						Filesystem: "",
					},
						"sdc": {
							DiskID:     0,
							Size:       *resource.NewQuantity(543, resource.DecimalSI),
							Label:      "another disk",
							Filesystem: "",
						}},
					CredentialsRef: &corev1.SecretReference{
						Namespace: "default",
						Name:      "cred-secret",
					},
				},
			},
		},
	}
	if err := utilconversion.MarshalData(src, expectedDst); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}
	dst := &LinodeMachineTemplate{}

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
