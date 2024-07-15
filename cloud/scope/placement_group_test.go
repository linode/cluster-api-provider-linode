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

package scope

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestValidatePlacementGroupScopeParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr bool
		params  PlacementGroupScopeParams
	}{
		{
			name:    "Valid PlacementGroupScopeParams",
			wantErr: false,
			params: PlacementGroupScopeParams{
				LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{},
			},
		},
		{
			name:    "Invalid PlacementGroupScopeParams",
			wantErr: true,
			params:  PlacementGroupScopeParams{},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if err := validatePlacementGroupScope(testcase.params); (err != nil) != testcase.wantErr {
				t.Errorf("PlacementGroupScopeParams() error = %v, wantErr %v", err, testcase.wantErr)
			}
		})
	}
}

func TestNewPlacementGroupScope(t *testing.T) {
	t.Parallel()
	type args struct {
		apiKey string
		params PlacementGroupScopeParams
	}
	tests := []struct {
		name          string
		args          args
		want          *PlacementGroupScope
		expectedError error
		expects       func(m *mock.MockK8sClient)
	}{
		{
			name: "Success - Pass in valid args and get a valid PlacementGroupScope",
			args: args{
				apiKey: "test-key",
				params: PlacementGroupScopeParams{
					LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{},
				},
			},
			expectedError: nil,
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
			},
		},
		{
			name: "Success - Validate getCredentialDataFromRef() returns some apiKey data and we create a valid PlacementGroupScope",
			args: args{
				apiKey: "test-key",
				params: PlacementGroupScopeParams{
					LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
						Spec: infrav1alpha1.LinodePlacementGroupSpec{
							CredentialsRef: &corev1.SecretReference{
								Namespace: "test-namespace",
								Name:      "test-name",
							},
						},
					},
				},
			},
			expectedError: nil,
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"apiToken": []byte("example-api-token"),
						},
					}
					*obj = cred
					return nil
				})
			},
		},
		{
			name: "Error - Pass in invalid args and get an error",
			args: args{
				apiKey: "test-key",
				params: PlacementGroupScopeParams{},
			},
			expects:       func(mock *mock.MockK8sClient) {},
			expectedError: fmt.Errorf("linodePlacementGroup is required when creating a PlacementGroupScope"),
		},
		{
			name: "Error - Pass in valid args but get an error when getting the credentials secret",
			args: args{
				apiKey: "test-key",
				params: PlacementGroupScopeParams{
					LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
						Spec: infrav1alpha1.LinodePlacementGroupSpec{
							CredentialsRef: &corev1.SecretReference{
								Namespace: "test-namespace",
								Name:      "test-name",
							},
						},
					},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("test error"))
			},
			expectedError: fmt.Errorf("credentials from secret ref: get credentials secret test-namespace/test-name: test error"),
		},
		{
			name: "Error - Pass in valid args but get an error when creating a new linode client",
			args: args{
				apiKey: "",
				params: PlacementGroupScopeParams{
					LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{},
				},
			},
			expects:       func(mock *mock.MockK8sClient) {},
			expectedError: fmt.Errorf("failed to create linode client: missing Linode API key"),
		},
		{
			name: "Error - Pass in valid args but get an error when creating a new patch helper",
			args: args{
				apiKey: "test-key",
				params: PlacementGroupScopeParams{
					LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{},
				},
			},
			expectedError: fmt.Errorf("failed to init patch helper:"),
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().Return(runtime.NewScheme())
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)

			testcase.expects(mockK8sClient)

			testcase.args.params.Client = mockK8sClient

			got, err := NewPlacementGroupScope(context.Background(), testcase.args.apiKey, testcase.args.params)

			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestPlacementGroupScopeMethods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		LinodePlacementGroup *infrav1alpha1.LinodePlacementGroup
		expects              func(mock *mock.MockK8sClient)
	}{
		{
			name: "Success - finalizer should be added to the Linode Placement Group object",
			LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pg",
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				}).Times(2)
				mock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "AddFinalizer error - finalizer should not be added to the Linode VPC object. Function returns nil since it was already present",
			LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pg",
					Finalizers: []string{infrav1alpha1.PlacementGroupFinalizer},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				}).Times(1)
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)

			testcase.expects(mockK8sClient)

			pgScope, err := NewPlacementGroupScope(
				context.Background(),
				"test-key",
				PlacementGroupScopeParams{
					Client:               mockK8sClient,
					LinodePlacementGroup: testcase.LinodePlacementGroup,
				},
			)
			if err != nil {
				t.Errorf("NewPlacementGroupScope() error = %v", err)
			}

			if err := pgScope.AddFinalizer(context.Background()); err != nil {
				t.Errorf("NewPlacementGroupScope.AddFinalizer() error = %v", err)
			}

			if pgScope.LinodePlacementGroup.Finalizers[0] != infrav1alpha1.PlacementGroupFinalizer {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}

func TestPlacementGroupAddCredentialsRefFinalizer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		LinodePlacementGroup *infrav1alpha1.LinodePlacementGroup
		expects              func(mock *mock.MockK8sClient)
	}{
		{
			name: "Success - finalizer should be added to the Linode Placement Group credentials Secret",
			LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pg",
				},
				Spec: infrav1alpha1.LinodePlacementGroupSpec{
					CredentialsRef: &corev1.SecretReference{
						Name:      "example",
						Namespace: "test",
					},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example",
							Namespace: "test",
						},
						Data: map[string][]byte{
							"apiToken": []byte("example"),
						},
					}
					*obj = cred

					return nil
				}).Times(2)
				mock.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "No-op - no Linode Cluster credentials Secret",
			LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pg",
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)

			testcase.expects(mockK8sClient)

			pgScope, err := NewPlacementGroupScope(
				context.Background(),
				"test-key",
				PlacementGroupScopeParams{
					Client:               mockK8sClient,
					LinodePlacementGroup: testcase.LinodePlacementGroup,
				},
			)
			if err != nil {
				t.Errorf("NewPlacementGroupScope() error = %v", err)
			}

			if err := pgScope.AddCredentialsRefFinalizer(context.Background()); err != nil {
				t.Errorf("NewPlacementGroupScope.AddCredentialsRefFinalizer() error = %v", err)
			}
		})
	}
}

func TestPlacementGroupRemoveCredentialsRefFinalizer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		LinodePlacementGroup *infrav1alpha1.LinodePlacementGroup
		expects              func(mock *mock.MockK8sClient)
	}{
		{
			name: "Success - finalizer should be added to the Linode Placement Group credentials Secret",
			LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pg",
				},
				Spec: infrav1alpha1.LinodePlacementGroupSpec{
					CredentialsRef: &corev1.SecretReference{
						Name:      "example",
						Namespace: "test",
					},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example",
							Namespace: "test",
						},
						Data: map[string][]byte{
							"apiToken": []byte("example"),
						},
					}
					*obj = cred

					return nil
				}).Times(2)
				mock.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "No-op - no Linode Placement Group credentials Secret",
			LinodePlacementGroup: &infrav1alpha1.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pg",
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)

			testcase.expects(mockK8sClient)

			pgScope, err := NewPlacementGroupScope(
				context.Background(),
				"test-key",
				PlacementGroupScopeParams{
					Client:               mockK8sClient,
					LinodePlacementGroup: testcase.LinodePlacementGroup,
				},
			)
			if err != nil {
				t.Errorf("NewPlacementGroupScope() error = %v", err)
			}

			if err := pgScope.RemoveCredentialsRefFinalizer(context.Background()); err != nil {
				t.Errorf("PlacementGroupScope.RemoveCredentialsRefFinalizer() error = %v", err)
			}
		})
	}
}
