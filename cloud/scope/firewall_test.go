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

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestValidateFirewallScopeParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr bool
		params  FirewallScopeParams
	}{
		{
			name:    "Valid FirewallScopeParams",
			wantErr: false,
			params: FirewallScopeParams{
				LinodeFirewall: &infrav1alpha2.LinodeFirewall{},
			},
		},
		{
			name:    "Invalid FirewallScopeParams",
			wantErr: true,
			params:  FirewallScopeParams{},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if err := validateFirewallScopeParams(testcase.params); (err != nil) != testcase.wantErr {
				t.Errorf("FirewallScopeParams() error = %v, wantErr %v", err, testcase.wantErr)
			}
		})
	}
}

func TestNewFirewallScope(t *testing.T) {
	t.Parallel()
	type args struct {
		apiKey string
		params FirewallScopeParams
	}
	tests := []struct {
		name          string
		args          args
		want          *FirewallScopeParams
		expectedError error
		expects       func(m *mock.MockK8sClient)
	}{
		{
			name: "Success - Pass in valid args and get a valid FirewallScope",
			args: args{
				apiKey: "test-key",
				params: FirewallScopeParams{
					LinodeFirewall: &infrav1alpha2.LinodeFirewall{},
				},
			},
			expectedError: nil,
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				})
			},
		},
		{
			name: "Success - Validate getCredentialDataFromRef() returns some apiKey data and we create a valid FirewallScope",
			args: args{
				apiKey: "test-key",
				params: FirewallScopeParams{
					LinodeFirewall: &infrav1alpha2.LinodeFirewall{
						Spec: infrav1alpha2.LinodeFirewallSpec{
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
					infrav1alpha2.AddToScheme(s)
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
				params: FirewallScopeParams{},
			},
			expects:       func(mock *mock.MockK8sClient) {},
			expectedError: fmt.Errorf("linodeFirewall is required when creating a FirewallScope"),
		},
		{
			name: "Error - Pass in valid args but get an error when getting the credentials secret",
			args: args{
				apiKey: "test-key",
				params: FirewallScopeParams{
					LinodeFirewall: &infrav1alpha2.LinodeFirewall{
						Spec: infrav1alpha2.LinodeFirewallSpec{
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
				params: FirewallScopeParams{
					LinodeFirewall: &infrav1alpha2.LinodeFirewall{},
				},
			},
			expects:       func(mock *mock.MockK8sClient) {},
			expectedError: fmt.Errorf("failed to create linode client: token cannot be empty"),
		},
		{
			name: "Error - Pass in valid args but get an error when creating a new patch helper",
			args: args{
				apiKey: "test-key",
				params: FirewallScopeParams{
					LinodeFirewall: &infrav1alpha2.LinodeFirewall{},
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

			got, err := NewFirewallScope(context.Background(), ClientConfig{Token: testcase.args.apiKey}, testcase.args.params)

			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestFirewallScopeMethods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		LinodeFirewall *infrav1alpha2.LinodeFirewall
		expects        func(mock *mock.MockK8sClient)
	}{
		{
			name: "Success - finalizer should be added to the Linode Firewall object",
			LinodeFirewall: &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-fw",
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).Times(2)
				mock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "AddFinalizer error - finalizer should not be added to the Linode Firewall object. Function returns nil since it was already present",
			LinodeFirewall: &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-fw",
					Finalizers: []string{infrav1alpha2.FirewallFinalizer},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
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

			fwScope, err := NewFirewallScope(
				context.Background(),
				ClientConfig{Token: "test-key"},
				FirewallScopeParams{
					Client:         mockK8sClient,
					LinodeFirewall: testcase.LinodeFirewall,
				},
			)
			if err != nil {
				t.Errorf("NewFirewallScope() error = %v", err)
			}

			if err := fwScope.AddFinalizer(context.Background()); err != nil {
				t.Errorf("NewFirewallScope.AddFinalizer() error = %v", err)
			}

			if fwScope.LinodeFirewall.Finalizers[0] != infrav1alpha2.FirewallFinalizer {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}

func TestFirewallAddCredentialsRefFinalizer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		LinodeFirewall *infrav1alpha2.LinodeFirewall
		expects        func(mock *mock.MockK8sClient)
	}{
		{
			name: "Success - finalizer should be added to the Linode Firewall credentials Secret",
			LinodeFirewall: &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-fw",
				},
				Spec: infrav1alpha2.LinodeFirewallSpec{
					CredentialsRef: &corev1.SecretReference{
						Name:      "example",
						Namespace: "test",
					},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
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
			LinodeFirewall: &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-fw",
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
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

			pgScope, err := NewFirewallScope(
				context.Background(),
				ClientConfig{Token: "test-key"},
				FirewallScopeParams{
					Client:         mockK8sClient,
					LinodeFirewall: testcase.LinodeFirewall,
				},
			)
			if err != nil {
				t.Errorf("NewFirewallScope() error = %v", err)
			}

			if err := pgScope.AddCredentialsRefFinalizer(context.Background()); err != nil {
				t.Errorf("NewFirewallScope.AddCredentialsRefFinalizer() error = %v", err)
			}
		})
	}
}

func TestFirewallRemoveCredentialsRefFinalizer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		LinodeFirewall *infrav1alpha2.LinodeFirewall
		expects        func(mock *mock.MockK8sClient)
	}{
		{
			name: "Success - finalizer should be added to the Linode Firewall credentials Secret",
			LinodeFirewall: &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-fw",
				},
				Spec: infrav1alpha2.LinodeFirewallSpec{
					CredentialsRef: &corev1.SecretReference{
						Name:      "example",
						Namespace: "test",
					},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
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
			name: "No-op - no Linode Firewall credentials Secret",
			LinodeFirewall: &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-fw",
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
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

			pgScope, err := NewFirewallScope(
				context.Background(),
				ClientConfig{Token: "test-key"},
				FirewallScopeParams{
					Client:         mockK8sClient,
					LinodeFirewall: testcase.LinodeFirewall,
				},
			)
			if err != nil {
				t.Errorf("NewFirewallScope() error = %v", err)
			}

			if err := pgScope.RemoveCredentialsRefFinalizer(context.Background()); err != nil {
				t.Errorf("FirewallScope.RemoveCredentialsRefFinalizer() error = %v", err)
			}
		})
	}
}
