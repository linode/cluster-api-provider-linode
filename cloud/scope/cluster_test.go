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

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Tests the validateClusterScopeParams function
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
			"Invalid ClusterScopeParams - empty params",
			args{
				params: ClusterScopeParams{},
			},
			true,
		},
		{
			"Invalid ClusterScopeParams - nil LinodeCluster",
			args{
				params: ClusterScopeParams{
					Cluster: &clusterv1.Cluster{},
				},
			},
			true,
		},

		{
			"Invalid ClusterScopeParams - nil Cluster",
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
		// TODO: Add test cases.
		{
			name: "AddFinalizer success - finalizer should be added",
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
			name: "AddFinalizer error - finalizer should not be added. Function returns nil",
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

			s := &ClusterScope{
				client:        mockK8sClient,
				PatchHelper:   mockPatchHelper,
				LinodeClient:  createLinodeClient("test-key"),
				Cluster:       testcase.fields.Cluster,
				LinodeCluster: testcase.fields.LinodeCluster,
			}

			// Set expected behaviour for PatchHelper
			if s.LinodeCluster.Finalizers == nil {
				mockPatchHelper.EXPECT().Patch(gomock.Any(), gomock.Any()).Return(testcase.patchErr)
			}

			if err := s.AddFinalizer(context.Background()); err != nil {
				t.Errorf("ClusterScope.AddFinalizer() error = %v", err)
			}

			if s.LinodeCluster.Finalizers[0] != infrav1alpha1.GroupVersion.String() {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}

func TestNewClusterScope(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx    context.Context
		apiKey string
		params ClusterScopeParams
		setPatchHelper bool
	}
	tests := []struct {
		name    string
		args    args
		want    *ClusterScope
		wantErr bool
		getFunc func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
		patchFunc func (obj client.Object, crClient client.Client) (*patch.Helper, error)

	}{
		// TODO: Add test cases.
		{
			name: "Success - Get cluster scope obj",
			args: args{
				ctx:    context.Background(),
				apiKey: "test-key",
				params: ClusterScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
				},
				setPatchHelper: true,
			},
			want: &ClusterScope{
				client:        nil,
				PatchHelper:   nil,
				LinodeClient:  createLinodeClient("test-key"),
				Cluster:       &clusterv1.Cluster{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
			},
			wantErr: false,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func (obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error validating params",
			args: args{
				ctx:    context.Background(),
				apiKey: "test-key",
				params: ClusterScopeParams{},
				setPatchHelper: false,
			},
			want: &ClusterScope{
				client:        nil,
				PatchHelper:   nil,
				LinodeClient:  createLinodeClient("test-key"),
				Cluster:       &clusterv1.Cluster{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
			},
			wantErr: true,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func (obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error with patch helper",
			args: args{
				ctx:    context.Background(),
				apiKey: "test-key",
				params: ClusterScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
				},
				setPatchHelper: true,
			},
			want: &ClusterScope{
				client:        nil,
				PatchHelper:   nil,
				LinodeClient:  createLinodeClient("test-key"),
				Cluster:       &clusterv1.Cluster{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
			},
			wantErr: true,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			patchFunc: func (obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return nil, fmt.Errorf("obj is nil")
			},
		},
		{
			name: "Success using getCredentialDataFromRef()",
			args: args{
				ctx:    context.Background(),
				apiKey: "test-key",
				params: ClusterScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{
						Spec: infrav1alpha1.LinodeClusterSpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
				},
				setPatchHelper: true,
			},
			want: &ClusterScope{
				client:        nil,
				PatchHelper:   nil,
				LinodeClient:  createLinodeClient("example"),
				Cluster:       &clusterv1.Cluster{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
			},
			wantErr: false,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				cred := corev1.Secret{
					Data: map[string][]byte{
						"apiToken": []byte("example"),
					},
				}
				*obj = cred

				return nil
			},
			patchFunc: func (obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
		{
			name: "Error using getCredentialDataFromRef()",
			args: args{
				ctx:    context.Background(),
				apiKey: "test-key",
				params: ClusterScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{
						Spec: infrav1alpha1.LinodeClusterSpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
				},
				setPatchHelper: true,
			},
			want: &ClusterScope{
				client:        nil,
				PatchHelper:   nil,
				LinodeClient:  createLinodeClient("example"),
				Cluster:       &clusterv1.Cluster{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
			},
			wantErr: true,
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return fmt.Errorf("failed to get secret")
			},
			patchFunc: func (obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
		},
	}
	
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockk8sClient(ctrl)

			if testcase.args.params.LinodeCluster != nil && testcase.args.params.LinodeCluster.Spec.CredentialsRef != nil {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testcase.getFunc)
			}

			// Monkey patch patchNewHelper
			patchNewHelper = testcase.patchFunc

			// Set patch helper in want obj for assertion later
			if testcase.args.setPatchHelper {
				ph, _ := patchNewHelper(testcase.args.params.LinodeCluster, mockK8sClient)
				testcase.want.PatchHelper = ph
			}

			// set client in want obj for assertion later
			testcase.args.params.Client = mockK8sClient


			got, err := NewClusterScope(testcase.args.ctx, testcase.args.apiKey, testcase.args.params)
			if (err != nil) != testcase.wantErr {
				t.Errorf("NewClusterScope() error = %v, wantErr %v", err, testcase.wantErr)
				return
			}
			if testcase.args.params.LinodeCluster != nil && got == nil && testcase.want.PatchHelper == nil {
				t.Errorf("Got no ClusterScope")
			}
			// if !reflect.DeepEqual(got, testcase.want) {
			// 	t.Errorf("NewClusterScope() = %v, want %v", got, testcase.want)
			// }
		})
	}
}
