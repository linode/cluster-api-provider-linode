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

	"github.com/linode/linodego/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeMachineTemplateCreate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		// Template with no LinodeInterfaces — the account settings check is skipped entirely.
		templateWithoutInterfaces = infrav1alpha2.LinodeMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineTemplateSpec{
				Template: infrav1alpha2.LinodeMachineTemplateResource{
					Spec: infrav1alpha2.LinodeMachineSpec{
						Region: "us-ord",
						Type:   "g6-standard-1",
					},
				},
			},
		}
		// Template with LinodeInterfaces and a credRef whose secret is not found.
		// setupClientWithCredentials returns skipAPIValidation=true so the account
		// settings check is also skipped.
		templateWithInterfacesAndMissingCred = infrav1alpha2.LinodeMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineTemplateSpec{
				Template: infrav1alpha2.LinodeMachineTemplateResource{
					Spec: infrav1alpha2.LinodeMachineSpec{
						Region:           "us-ord",
						Type:             "g6-standard-1",
						LinodeInterfaces: []infrav1alpha2.LinodeInterfaceCreateOptions{{}},
						CredentialsRef: &corev1.SecretReference{
							Name:      "template-credentials",
							Namespace: "example",
						},
					},
				},
			},
		}
		// Template with LinodeInterfaces and a resolved credRef — API validation runs using the mock linode client.
		templateWithoutCred = infrav1alpha2.LinodeMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineTemplateSpec{
				Template: infrav1alpha2.LinodeMachineTemplateResource{
					Spec: infrav1alpha2.LinodeMachineSpec{
						Region:           "us-ord",
						Type:             "g6-standard-1",
						LinodeInterfaces: []infrav1alpha2.LinodeInterfaceCreateOptions{{}},
						CredentialsRef: &corev1.SecretReference{
							Name:      "example-creds",
							Namespace: "example",
						},
					},
				},
			},
		}
		// Template with legacy Interfaces and a resolved credRef — API validation runs using the mock linode client.
		templateWithLegacyInterfacesAndNoCred = infrav1alpha2.LinodeMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineTemplateSpec{
				Template: infrav1alpha2.LinodeMachineTemplateResource{
					Spec: infrav1alpha2.LinodeMachineSpec{
						Region: "us-ord",
						Type:   "g6-standard-1",
						Interfaces: []infrav1alpha2.InstanceConfigInterfaceCreateOptions{
							{Purpose: "public"},
						},
						CredentialsRef: &corev1.SecretReference{
							Name:      "example-creds",
							Namespace: "example",
						},
					},
				},
			},
		}
		validator = &linodeMachineTemplateValidator{Client: mockK8sClient}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			// No interface fields → account settings check is skipped entirely.
			Path(
				Call("no interfaces set", func(ctx context.Context, mck Mock) {}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &templateWithoutInterfaces)
					require.NoError(t, err)
				}),
			),
			// Credentials secret not found → skipAPIValidation=true → account settings check skipped.
			Path(
				Call("credentials secret not found", func(ctx context.Context, mck Mock) {
					mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(apierrors.NewNotFound(corev1.Resource("secrets"), "template-credentials")).AnyTimes()
				}),
				Result("success — API validation skipped", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &templateWithInterfacesAndMissingCred)
					require.NoError(t, err)
				}),
			),
			// Linode Interfaces set, account has LegacyConfigOnly → forbidden.
			Path(
				Call("LinodeInterfaces + LegacyConfigOnly", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(&linodego.AccountSettings{
						InterfacesForNewLinodes: linodego.LegacyConfigOnly,
					}, nil)
				}),
				Result("forbidden error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineTemplateSpec(ctx, mck.LinodeClient, templateWithoutCred.Spec)
					require.NotEmpty(t, errs)
					assert.Contains(t, errs[0].Detail, "legacy_config_only")
				}),
			),
			// Legacy interfaces set, account has LinodeOnly → forbidden.
			Path(
				Call("legacy interfaces + LinodeOnly", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(&linodego.AccountSettings{
						InterfacesForNewLinodes: linodego.LinodeOnly,
					}, nil)
				}),
				Result("forbidden error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineTemplateSpec(ctx, mck.LinodeClient, templateWithLegacyInterfacesAndNoCred.Spec)
					require.NotEmpty(t, errs)
					assert.Contains(t, errs[0].Detail, "linode_only")
				}),
			),
			// Linode Interfaces set, account allows both → permitted.
			Path(
				Call("LinodeInterfaces + LegacyConfigDefaultButLinodeAllowed", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetAccountSettings(gomock.Any()).Return(&linodego.AccountSettings{
						InterfacesForNewLinodes: linodego.LegacyConfigDefaultButLinodeAllowed,
					}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodeMachineTemplateSpec(ctx, mck.LinodeClient, templateWithoutCred.Spec)
					require.Empty(t, errs)
				}),
			),
		),
	)
}

func TestValidateLinodeMachineTemplateUpdate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		oldTemplate = infrav1alpha2.LinodeMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
		}
		newTemplate = infrav1alpha2.LinodeMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeMachineTemplateSpec{
				Template: infrav1alpha2.LinodeMachineTemplateResource{
					Spec: infrav1alpha2.LinodeMachineSpec{
						Region:           "us-ord",
						LinodeInterfaces: []infrav1alpha2.LinodeInterfaceCreateOptions{{}},
					},
				},
			},
		}
		validator = &linodeMachineTemplateValidator{Client: mockK8sClient}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("update", func(ctx context.Context, mck Mock) {}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateUpdate(ctx, &oldTemplate, &newTemplate)
					assert.NoError(t, err)
				}),
			),
		),
	)
}

func TestValidateLinodeMachineTemplateDelete(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		template = infrav1alpha2.LinodeMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
		}
		validator = &linodeMachineTemplateValidator{Client: mockK8sClient}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("delete", func(ctx context.Context, mck Mock) {}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateDelete(ctx, &template)
					assert.NoError(t, err)
				}),
			),
		),
	)
}
