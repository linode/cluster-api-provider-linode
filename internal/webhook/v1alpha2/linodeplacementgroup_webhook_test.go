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
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"
	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodePlacementGroup(t *testing.T) {
	t.Parallel()

	var (
		pg = infrav1alpha2.LinodePlacementGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodePlacementGroupSpec{
				Region: "example",
			},
		}
		region                      = linodego.Region{ID: "test"}
		capabilities                = []string{LinodePlacementGroupCapability}
		capabilities_zero           = []string{}
		invalidRegionError          = "spec.region: Not found: \"example\""
		invalidRegionNoPGCapability = "spec.region: Invalid value: \"example\": no capability: Placement Group"
		invalidPGLabelError         = "metadata.name: Invalid value: \"a20_b!4\": can only contain ASCII letters, numbers, hyphens (-), underscores (_) and periods (.), must start and end with a alphanumeric character"
		validator                   = LinodePlacementGroupCustomValidator{}
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
					errs := validator.validateLinodePlacementGroupSpec(ctx, mck.LinodeClient, pg.Spec, pg.ObjectMeta.Name)
					require.Empty(t, errs)
				}),
			),
		),
		OneOf(
			Path(Call("invalid region", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(nil, errors.New("invalid region")).AnyTimes()
			}),
				Result("error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodePlacementGroupSpec(ctx, mck.LinodeClient, pg.Spec, pg.ObjectMeta.Name)
					for _, err := range errs {
						assert.ErrorContains(t, err, invalidRegionError)
					}
				})),
			Path(Call("region not supported", func(ctx context.Context, mck Mock) {
				region := region
				region.Capabilities = slices.Clone(capabilities_zero)
				mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
			}),
				Result("error", func(ctx context.Context, mck Mock) {
					errs := validator.validateLinodePlacementGroupSpec(ctx, mck.LinodeClient, pg.Spec, pg.ObjectMeta.Name)
					for _, err := range errs {
						assert.ErrorContains(t, err, invalidRegionNoPGCapability)
					}
				})),
		),

		OneOf(
			Path(
				Call("invalid placementgroup label", func(ctx context.Context, mck Mock) {
					region := region
					region.Capabilities = slices.Clone(capabilities)
					mck.LinodeClient.EXPECT().GetRegion(gomock.Any(), gomock.Any()).Return(&region, nil).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					pg := pg
					pg.Name = "a20_b!4"
					errs := validator.validateLinodePlacementGroupSpec(ctx, mck.LinodeClient, pg.Spec, pg.ObjectMeta.Name)
					for _, err := range errs {
						assert.ErrorContains(t, err, invalidPGLabelError)
					}
				}),
			),
		),
	)
}

func TestValidateCreateLinodePlacementGroup(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)

	var (
		pg = infrav1alpha2.LinodePlacementGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodePlacementGroupSpec{
				Region: "example",
			},
		}

		credentialsRefPG = infrav1alpha2.LinodePlacementGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodePlacementGroupSpec{
				CredentialsRef: &corev1.SecretReference{
					Name: "pg-credentials",
				},
				Region: "us-ord",
			},
		}
		expectedErrorSubString = "\"example\" is invalid: spec.region: Not found:"
		validator              = LinodePlacementGroupCustomValidator{Client: mockK8sClient}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("invalid request", func(ctx context.Context, mck Mock) {
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &pg)
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
									Name:      "pg-credentials",
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
					str, err := getCredentialDataFromRef(ctx, mockK8sClient, *credentialsRefPG.Spec.CredentialsRef, credentialsRefPG.GetNamespace())
					require.NoError(t, err)
					assert.Equal(t, []byte("token"), str)
				}),
			),
		),
	)
}
