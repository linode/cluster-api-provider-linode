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
	"math"
	"strconv"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeMachine(t *testing.T) {
	t.Parallel()

	var (
		machine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region: "example",
				Type:   "example",
			},
		}
		disk                                        = infrav1alpha2.InstanceDisk{Size: resource.MustParse("1G")}
		disk_zero                                   = infrav1alpha2.InstanceDisk{Size: *resource.NewQuantity(0, resource.BinarySI)}
		plan                                        = linodego.LinodeType{Disk: 2 * int(disk.Size.ScaledValue(resource.Mega))}
		plan_zero                                   = linodego.LinodeType{Disk: 0}
		plan_max                                    = linodego.LinodeType{Disk: math.MaxInt}
		expectedErrorSubStringOSDisk                = "Invalid value: \"1G\": sum disk sizes exceeds plan storage: 2G"
		expectedErrorSubStringOSDiskDataDiskInvalid = "spec.dataDisks.sda: Forbidden: allowed device paths: [sdb sdc sdd sde sdf sdg sdh]"
		expectedErrorSubStringOSDiskOSDiskInvalid   = "spec.osDisk: Invalid value: \"0\": invalid size"
		validator                                   = &linodeMachineValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_max, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, machine.Spec, SkipAPIValidation)
					require.Empty(t, errs)
				}),
				Call("valid with disks", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_max, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					machine := machine
					machine.Spec.OSDisk = disk.DeepCopy()
					machine.Spec.DataDisks = map[string]*infrav1alpha2.InstanceDisk{"sdb": disk.DeepCopy()}
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, machine.Spec, SkipAPIValidation)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(Call("invalid region", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
				mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			})),
			Path(Call("invalid type", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid type")).AnyTimes()
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, machine.Spec, SkipAPIValidation)
			for _, err := range errs {
				require.Error(t, err)
			}
		}),
		OneOf(
			Path(
				Call("exceed plan storage", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_zero, nil).AnyTimes()
				}),
				Result("os disk too large", func(ctx context.Context, mck Mock) {
					machine := machine
					machine.Spec.OSDisk = disk.DeepCopy()
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, machine.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, strconv.Itoa(plan_zero.Disk))
					}
				}),
			),
			Path(
				Call("exceed plan storage", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan, nil).AnyTimes()
				}),
				Result("data disk too large", func(ctx context.Context, mck Mock) {
					machine := machine
					machine.Spec.OSDisk = disk.DeepCopy()
					machine.Spec.DataDisks = map[string]*infrav1alpha2.InstanceDisk{"sdb": disk.DeepCopy(), "sdc": disk.DeepCopy()}
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, machine.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, expectedErrorSubStringOSDisk)
					}
				}),
			),
			Path(
				Call("data disk invalid path", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_max, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					machine := machine
					machine.Spec.DataDisks = map[string]*infrav1alpha2.InstanceDisk{"sda": disk.DeepCopy()}
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, machine.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, expectedErrorSubStringOSDiskDataDiskInvalid)
					}
				}),
			),
			Path(
				Call("invalid disk size", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_max, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					machine := machine
					machine.Spec.OSDisk = disk_zero.DeepCopy()
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, machine.Spec, SkipAPIValidation)
					for _, err := range errs {
						assert.ErrorContains(t, err, expectedErrorSubStringOSDiskOSDiskInvalid)
					}
				}),
			),
		),
	)
}

func TestValidateCreateLinodeMachine(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		machine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region: "example",
				Type:   "example",
			},
		}
		credentialsRefMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				CredentialsRef: &corev1.SecretReference{
					Name: "machine-credentials",
				},
				Region: "example",
				Type:   "example",
			},
		}
		expectedErrorSubString = "\"example\" is invalid: [spec.region: Not found: \"example\", spec.type: Not found: \"example\"]"
		validator              = &linodeMachineValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("invalid request", func(ctx context.Context, mck Mock) {
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &machine)
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
									Name:      "machine-credentials",
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
					str, err := getCredentialDataFromRef(ctx, mockK8sClient, *credentialsRefMachine.Spec.CredentialsRef, credentialsRefMachine.GetNamespace())
					require.NoError(t, err)
					assert.Equal(t, []byte("token"), str)
				}),
			),
		),
	)
}

func TestValidateVPCIDAndVPCRefOnMachine(t *testing.T) {
	t.Parallel()

	var (
		invalidMachine = &infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region: "us-ord",
				Type:   "g6-standard-1",
				VPCID:  ptr.To(12345),
				VPCRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeVPC",
				},
			},
		}
		validMachineWithVPCID = &infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region: "us-ord",
				Type:   "g6-standard-1",
				VPCID:  ptr.To(12345),
			},
		}
		validMachineWithVPCRef = &infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region: "us-ord",
				Type:   "g6-standard-1",
				VPCRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeVPC",
				},
			},
		}
		validator = &linodeMachineValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid with VPCID", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&linodego.LinodeType{
						ID:    "g6-standard-1",
						Disk:  50 * 1024, // 50GB
						Label: "Linode 2GB",
					}, nil).AnyTimes()
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
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, validMachineWithVPCID.Spec, SkipAPIValidation)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("valid with VPCRef", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&linodego.LinodeType{
						ID:    "g6-standard-1",
						Disk:  50 * 1024, // 50GB
						Label: "Linode 2GB",
					}, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, validMachineWithVPCRef.Spec, SkipAPIValidation)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("both VPCID and VPCRef set", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&linodego.LinodeType{
						ID:    "g6-standard-1",
						Disk:  50 * 1024, // 50GB
						Label: "Linode 2GB",
					}, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, invalidMachine.Spec, SkipAPIValidation)
					require.NotEmpty(t, errs)
					require.Contains(t, errs[0].Error(), "Cannot specify both VPCID and VPCRef")
				}),
			),
		),
	)
}

func TestValidateFirewallIDAndFirewallRef(t *testing.T) {
	t.Parallel()

	var (
		invalidMachine = &infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region:     "us-ord",
				Type:       "g6-standard-1",
				FirewallID: 5678,
				FirewallRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeFirewall",
				},
			},
		}
		validMachineWithFirewallID = &infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region:     "us-ord",
				Type:       "g6-standard-1",
				FirewallID: 5678,
			},
		}
		validMachineWithFirewallRef = &infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region: "us-ord",
				Type:   "g6-standard-1",
				FirewallRef: &corev1.ObjectReference{
					Namespace: "example",
					Name:      "example",
					Kind:      "LinodeFirewall",
				},
			},
		}
		validator = &linodeMachineValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid with FirewallID", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&linodego.LinodeType{
						ID:    "g6-standard-1",
						Disk:  50 * 1024, // 50GB
						Label: "Linode 2GB",
					}, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, validMachineWithFirewallID.Spec)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("valid with FirewallRef", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&linodego.LinodeType{
						ID:    "g6-standard-1",
						Disk:  50 * 1024, // 50GB
						Label: "Linode 2GB",
					}, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, validMachineWithFirewallRef.Spec)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(
				Call("both FirewallID and FirewallRef set", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&linodego.LinodeType{
						ID:    "g6-standard-1",
						Disk:  50 * 1024, // 50GB
						Label: "Linode 2GB",
					}, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineSpec(ctx, mck.LinodeClient, invalidMachine.Spec)
					require.NotEmpty(t, errs)
					require.Contains(t, errs[0].Error(), "Cannot specify both FirewallID and FirewallRef")
				}),
			),
		),
	)
}
