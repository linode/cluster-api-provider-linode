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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func Test_validateVPCScopeParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr bool
		params VPCScopeParams
	}{
		{
			name: "Valid VPCScopeParams",
			wantErr: false,
			params: VPCScopeParams{
				LinodeVPC: &infrav1alpha1.LinodeVPC{},
			},
		},
		{
			name: "Invalid VPCScopeParams",
			wantErr: true,
			params: VPCScopeParams{},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if err := validateVPCScopeParams(testcase.params); (err != nil) != testcase.wantErr {
				t.Errorf("validateVPCScopeParams() error = %v, wantErr %v", err, testcase.wantErr)
			}
		})
	}
}

func TestNewVPCScope(t *testing.T) {
	t.Parallel()
	type args struct {
		apiKey string
		params VPCScopeParams
	}
	tests := []struct {
		name          string
		args          args
		want          *VPCScope
		expectedError error
		getFunc       func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
		patchFunc     func(obj client.Object, crClient client.Client) (*patch.Helper, error)
	}{
		{
			name: "Success - Pass in valid args and get a valid VPCScope",
			args: args{
				apiKey: "test-key",
				params: VPCScopeParams{
					LinodeVPC: &infrav1alpha1.LinodeVPC{},
				},
			},
			expectedError: nil,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Success - Validate getCredentialDataFromRef() returns some apiKey data and we create a valid ClusterScope",
			args: args{
				apiKey: "test-key",
				params: VPCScopeParams{
					LinodeVPC: &infrav1alpha1.LinodeVPC{
						Spec: infrav1alpha1.LinodeVPCSpec{
							CredentialsRef: &corev1.SecretReference{
								Namespace: "test-namespace",
								Name:      "test-name",
							},
						},
					},
				},
			},
			expectedError: nil,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				cred := corev1.Secret{
					Data: map[string][]byte{
						"apiToken": []byte("example-api-token"),
					},
				}
				*obj = cred
				return nil
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error - Pass in invalid args and get an error",
			args: args{
				apiKey: "test-key",
				params: VPCScopeParams{},
			},
			expectedError: fmt.Errorf("linodeVPC is required when creating a VPCScope"),
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error - Pass in valid args but get an error when getting the credentials secret",
			args: args{
				apiKey: "test-key",
				params: VPCScopeParams{
					LinodeVPC: &infrav1alpha1.LinodeVPC{
						Spec: infrav1alpha1.LinodeVPCSpec{
							CredentialsRef: &corev1.SecretReference{
								Namespace: "test-namespace",
								Name:      "test-name",
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("credentials from secret ref: get credentials secret test-namespace/test-name: test error"),
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return fmt.Errorf("test error")
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error - Pass in valid args but get an error when creating a new linode client",
			args: args{
				apiKey: "",
				params: VPCScopeParams{
					LinodeVPC: &infrav1alpha1.LinodeVPC{},
				},
			},
			expectedError: fmt.Errorf("failed to create linode client: missing Linode API key"),
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error - Pass in valid args but get an error when creating a new patch helper",
			args: args{
				apiKey: "test-key",
				params: VPCScopeParams{
					LinodeVPC: &infrav1alpha1.LinodeVPC{},
				},
			},
			expectedError: fmt.Errorf("failed to init patch helper: test error"),
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return nil, fmt.Errorf("test error")
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockk8sClient(ctrl)

			if testcase.args.params.LinodeVPC != nil && testcase.args.params.LinodeVPC.Spec.CredentialsRef != nil {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testcase.getFunc)
			}

			testcase.args.params.Client = mockK8sClient

			got, err := NewVPCScope(context.Background(), testcase.args.apiKey, testcase.args.params, testcase.patchFunc)

			if testcase.expectedError != nil {
				assert.EqualError(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestVPCScopeMethods(t *testing.T) {
	t.Parallel()
	type fields struct {
		LinodeVPC *infrav1alpha1.LinodeVPC
	}
	tests := []struct {
		name     string
		LinodeVPC *infrav1alpha1.LinodeVPC
		wantErr  bool
		patchErr error
	}{
		{
			name: "Success - finalizer should be added to the Linode VPC object",
			LinodeVPC: &infrav1alpha1.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vpc",
				},
			},
			wantErr:  false,
			patchErr: nil,
		},
		{
			name: "AddFinalizer error - finalizer should not be added to the Linode VPC object. Function returns nil since it was already present",
			LinodeVPC: &infrav1alpha1.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-vpc",
					Finalizers: []string{infrav1alpha1.GroupVersion.String()},
				},
			},
			wantErr:  false,
			patchErr: nil,
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPatchHelper := mock.NewMockPatchHelper(ctrl)
			mockK8sClient := mock.NewMockk8sClient(ctrl)

			lClient, err := createLinodeClient("test-key")
			if err != nil {
				t.Errorf("failed to create linode client: %v", err)
			}

			vScope := &VPCScope{
				client:       mockK8sClient,
				PatchHelper:  mockPatchHelper,
				LinodeClient: lClient,
				LinodeVPC:    testcase.LinodeVPC,
			}

			if vScope.LinodeVPC.Finalizers == nil {
				mockPatchHelper.EXPECT().Patch(gomock.Any(), gomock.Any()).Return(testcase.patchErr)
			}

			if err := vScope.AddFinalizer(context.Background()); err != nil {
				t.Errorf("ClusterScope.AddFinalizer() error = %v", err)
			}

			if vScope.LinodeVPC.Finalizers[0] != infrav1alpha1.GroupVersion.String() {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}
