/*
Copyright 2024.

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeFirewallCreate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		lfw = infrav1alpha2.LinodeFirewall{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeFirewallSpec{},
		}
		lfwLongName = infrav1alpha2.LinodeFirewall{
			ObjectMeta: metav1.ObjectMeta{
				Name:      longName,
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeFirewallSpec{},
		}
		validator = &LinodeFirewallCustomValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("name too long", func(ctx context.Context, mck Mock) {

				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &lfwLongName)
					assert.ErrorContains(t, err, labelLengthDetail)
				}),
			),
		),
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &lfw)
					require.NoError(t, err)
				}),
			),
		),
	)
}

func TestValidateLinodeFirewallUpdate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		oldFW = infrav1alpha2.LinodeFirewall{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeFirewallSpec{},
		}
		newFW = infrav1alpha2.LinodeFirewall{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeFirewallSpec{},
		}

		validator = &LinodeFirewallCustomValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("update", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateUpdate(ctx, &oldFW, &newFW)
					assert.NoError(t, err)
				}),
			),
		),
	)
}

func TestValidateLinodeFirewallDelete(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		lfw = infrav1alpha2.LinodeFirewall{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeFirewallSpec{},
		}

		validator = &LinodeFirewallCustomValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("delete", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateDelete(ctx, &lfw)
					assert.NoError(t, err)
				}),
			),
		),
	)
}
