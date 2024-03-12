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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestValidateClusterScopeParams(t *testing.T) {
	t.Parallel()
	type args struct {
		params ClusterScopeParams
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Valid ClusterScopeParams",
			args{
				params: ClusterScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
				},
			},
			false,
		},
		{
			"Invalid ClusterScopeParams - empty ClusterScopeParams",
			args{
				params: ClusterScopeParams{},
			},
			true,
		},
		{
			"Invalid ClusterScopeParams - no LinodeCluster in ClusterScopeParams",
			args{
				params: ClusterScopeParams{
					Cluster: &clusterv1.Cluster{},
				},
			},
			true,
		},

		{
			"Invalid ClusterScopeParams - no Cluster in ClusterScopeParams",
			args{
				params: ClusterScopeParams{
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if err := validateClusterScopeParams(testcase.args.params); (err != nil) != testcase.wantErr {
				t.Errorf("validateClusterScopeParams() error = %v, wantErr %v", err, testcase.wantErr)
			}
		})
	}
}

func TestClusterScopeMethods(t *testing.T) {
	t.Parallel()
	type fields struct {
		Cluster       *clusterv1.Cluster
		LinodeCluster *infrav1alpha1.LinodeCluster
	}

	tests := []struct {
		name     string
		fields   fields
		patchErr error
	}{
		{
			name: "Success - finalizer should be added to the Linode Cluster object",
			fields: fields{
				Cluster: &clusterv1.Cluster{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			patchErr: nil,
		},
		{
			name: "AddFinalizer error - finalizer should not be added to the Linode Cluster object. Function returns nil since it was already present",
			fields: fields{
				Cluster: &clusterv1.Cluster{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-cluster",
						Finalizers: []string{infrav1alpha1.GroupVersion.String()},
					},
				},
			},
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
				t.Errorf("createLinodeClient() error = %v", err)
			}

			cScope := &ClusterScope{
				client:        mockK8sClient,
				PatchHelper:   mockPatchHelper,
				LinodeClient:  lClient,
				Cluster:       testcase.fields.Cluster,
				LinodeCluster: testcase.fields.LinodeCluster,
			}

			if cScope.LinodeCluster.Finalizers == nil {
				mockPatchHelper.EXPECT().Patch(gomock.Any(), gomock.Any()).Return(testcase.patchErr)
			}

			if err := cScope.AddFinalizer(context.Background()); err != nil {
				t.Errorf("ClusterScope.AddFinalizer() error = %v", err)
			}

			if cScope.LinodeCluster.Finalizers[0] != infrav1alpha1.GroupVersion.String() {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}

func TestNewClusterScope(t *testing.T) {
	t.Parallel()
	type args struct {
		apiKey string
		params ClusterScopeParams
	}
	tests := []struct {
		name          string
		args          args
		expectedError error
		getFunc       func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
		patchFunc     func(obj client.Object, crClient client.Client) (*patch.Helper, error)
	}{
		{
			name: "Success - Pass in valid args and get a valid ClusterScope",
			args: args{
				apiKey: "test-key",
				params: ClusterScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
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
				params: ClusterScopeParams{
					Client:  nil,
					Cluster: &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{
						Spec: infrav1alpha1.LinodeClusterSpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: nil,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				cred := corev1.Secret{
					Data: map[string][]byte{
						"apiToken": []byte("example"),
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
			name: "Error - ValidateClusterScopeParams triggers error because ClusterScopeParams is empty",
			args: args{
				apiKey: "test-key",
				params: ClusterScopeParams{},
			},
			expectedError: fmt.Errorf("cluster is required when creating a ClusterScope"),
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error - patchHelper returns error. Checking error handle for when new patchHelper is invoked",
			args: args{
				apiKey: "test-key",
				params: ClusterScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
				},
			},
			expectedError: fmt.Errorf("failed to init patch helper: obj is nil"),
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return nil, fmt.Errorf("obj is nil")
			},
		},
		{
			name: "Error - Using getCredentialDataFromRef(), func returns an error. Unable to create a valid ClusterScope",
			args: args{
				apiKey: "test-key",
				params: ClusterScopeParams{
					Client:  nil,
					Cluster: &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{
						Spec: infrav1alpha1.LinodeClusterSpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("credentials from secret ref: get credentials secret test/example: failed to get secret"),
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return fmt.Errorf("failed to get secret")
			},
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error - createLinodeCluster throws an error for passing empty apiKey. Unable to create a valid ClusterScope",
			args: args{
				apiKey: "",
				params: ClusterScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
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
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockk8sClient(ctrl)

			if testcase.args.params.LinodeCluster != nil && testcase.args.params.LinodeCluster.Spec.CredentialsRef != nil {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testcase.getFunc)
			}

			testcase.args.params.Client = mockK8sClient

			got, err := NewClusterScope(context.Background(), testcase.args.apiKey, testcase.args.params, testcase.patchFunc)

			if testcase.expectedError != nil {
				assert.EqualError(t, err, testcase.expectedError.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}
