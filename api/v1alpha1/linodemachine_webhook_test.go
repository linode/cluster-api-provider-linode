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
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeMachine(t *testing.T) {
	t.Parallel()

	var (
		machine = LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: LinodeMachineSpec{
				Region: "example",
				Type:   "example",
			},
		}
		disk      = InstanceDisk{Size: resource.MustParse("1G")}
		disk_zero = InstanceDisk{Size: *resource.NewQuantity(0, resource.BinarySI)}
		plan      = linodego.LinodeType{Disk: 2 * int(disk.Size.ScaledValue(resource.Mega))}
		plan_zero = linodego.LinodeType{Disk: 0}
		plan_max  = linodego.LinodeType{Disk: math.MaxInt}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_max, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					assert.NoError(t, machine.validateLinodeMachine(ctx, mck.LinodeClient))
				}),
				Call("valid with disks", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_max, nil).AnyTimes()
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					machine := machine
					machine.Spec.OSDisk = disk.DeepCopy()
					machine.Spec.DataDisks = map[string]*InstanceDisk{"sdb": disk.DeepCopy()}
					assert.NoError(t, machine.validateLinodeMachine(ctx, mck.LinodeClient))
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
			assert.Error(t, machine.validateLinodeMachine(ctx, mck.LinodeClient))
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
					assert.ErrorContains(t, machine.validateLinodeMachine(ctx, mck.LinodeClient), strconv.Itoa(plan_zero.Disk))
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
					machine.Spec.DataDisks = map[string]*InstanceDisk{"sdb": disk.DeepCopy(), "sdc": disk.DeepCopy()}
					assert.Error(t, machine.validateLinodeMachine(ctx, mck.LinodeClient))
				}),
			),
			Path(
				Call("data disk invalid path", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
					mck.LinodeClient.EXPECT().GetType(gomock.Any(), gomock.Any()).Return(&plan_max, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					machine := machine
					machine.Spec.DataDisks = map[string]*InstanceDisk{"sda": disk.DeepCopy()}
					assert.Error(t, machine.validateLinodeMachine(ctx, mck.LinodeClient))
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
					assert.Error(t, machine.validateLinodeMachine(ctx, mck.LinodeClient))
				}),
			),
		),
	)
}
