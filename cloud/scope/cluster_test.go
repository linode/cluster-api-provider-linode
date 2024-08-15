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

// import (
// 	"context"
// 	"fmt"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"go.uber.org/mock/gomock"
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/apimachinery/pkg/types"
// 	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
// 	"sigs.k8s.io/controller-runtime/pkg/client"

// 	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
// 	"github.com/linode/cluster-api-provider-linode/mock"
// )

// func TestValidateClusterScopeParams(t *testing.T) {
// 	t.Parallel()
// 	type args struct {
// 		params ClusterScopeParams
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr bool
// 	}{
// 		{
// 			"Valid ClusterScopeParams",
// 			args{
// 				params: ClusterScopeParams{
// 					Cluster:       &clusterv1.Cluster{},
// 					LinodeCluster: &infrav1alpha2.LinodeCluster{},
// 				},
// 			},
// 			false,
// 		},
// 		{
// 			"Invalid ClusterScopeParams - empty ClusterScopeParams",
// 			args{
// 				params: ClusterScopeParams{},
// 			},
// 			true,
// 		},
// 		{
// 			"Invalid ClusterScopeParams - no LinodeCluster in ClusterScopeParams",
// 			args{
// 				params: ClusterScopeParams{
// 					Cluster: &clusterv1.Cluster{},
// 				},
// 			},
// 			true,
// 		},

// 		{
// 			"Invalid ClusterScopeParams - no Cluster in ClusterScopeParams",
// 			args{
// 				params: ClusterScopeParams{
// 					LinodeCluster: &infrav1alpha2.LinodeCluster{},
// 				},
// 			},
// 			true,
// 		},
// 	}
// 	for _, tt := range tests {
// 		testcase := tt
// 		t.Run(testcase.name, func(t *testing.T) {
// 			t.Parallel()
// 			if err := validateClusterScopeParams(testcase.args.params); (err != nil) != testcase.wantErr {
// 				t.Errorf("validateClusterScopeParams() error = %v, wantErr %v", err, testcase.wantErr)
// 			}
// 		})
// 	}
// }

// func TestClusterScopeMethods(t *testing.T) {
// 	t.Parallel()
// 	type fields struct {
// 		Cluster       *clusterv1.Cluster
// 		LinodeCluster *infrav1alpha2.LinodeCluster
// 	}

// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		expects func(mock *mock.MockK8sClient)
// 	}{
// 		{
// 			name: "Success - finalizer should be added to the Linode Cluster object",
// 			fields: fields{
// 				Cluster: &clusterv1.Cluster{},
// 				LinodeCluster: &infrav1alpha2.LinodeCluster{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name: "test-cluster",
// 					},
// 				},
// 			},
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				}).Times(2)
// 				mock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
// 			},
// 		},
// 		{
// 			name: "AddFinalizer error - finalizer should not be added to the Linode Cluster object. Function returns nil since it was already present",
// 			fields: fields{
// 				Cluster: &clusterv1.Cluster{},
// 				LinodeCluster: &infrav1alpha2.LinodeCluster{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:       "test-cluster",
// 						Finalizers: []string{infrav1alpha2.ClusterFinalizer},
// 					},
// 				},
// 			},
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				}).Times(1)
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		testcase := tt
// 		t.Run(testcase.name, func(t *testing.T) {
// 			t.Parallel()

// 			ctrl := gomock.NewController(t)
// 			defer ctrl.Finish()

// 			mockK8sClient := mock.NewMockK8sClient(ctrl)

// 			testcase.expects(mockK8sClient)

// 			cScope, err := NewClusterScope(
// 				context.Background(),
// 				ClientConfig{Token: "test-key"},
// 				ClusterScopeParams{
// 					Cluster:       testcase.fields.Cluster,
// 					LinodeCluster: testcase.fields.LinodeCluster,
// 					Client:        mockK8sClient,
// 				})
// 			if err != nil {
// 				t.Errorf("NewClusterScope() error = %v", err)
// 			}

// 			if err := cScope.AddFinalizer(context.Background()); err != nil {
// 				t.Errorf("ClusterScope.AddFinalizer() error = %v", err)
// 			}

// 			if cScope.LinodeCluster.Finalizers[0] != infrav1alpha2.ClusterFinalizer {
// 				t.Errorf("Finalizer was not added")
// 			}
// 		})
// 	}
// }

// func TestNewClusterScope(t *testing.T) {
// 	t.Parallel()
// 	type args struct {
// 		apiKey string
// 		params ClusterScopeParams
// 	}
// 	tests := []struct {
// 		name          string
// 		args          args
// 		expectedError error
// 		expects       func(mock *mock.MockK8sClient)
// 	}{
// 		{
// 			name: "Success - Pass in valid args and get a valid ClusterScope",
// 			args: args{
// 				apiKey: "test-key",
// 				params: ClusterScopeParams{
// 					Cluster:       &clusterv1.Cluster{},
// 					LinodeCluster: &infrav1alpha2.LinodeCluster{},
// 				},
// 			},
// 			expectedError: nil,
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				})
// 			},
// 		},
// 		{
// 			name: "Success - Validate getCredentialDataFromRef() returns some apiKey data and we create a valid ClusterScope",
// 			args: args{
// 				apiKey: "test-key",
// 				params: ClusterScopeParams{
// 					Client:  nil,
// 					Cluster: &clusterv1.Cluster{},
// 					LinodeCluster: &infrav1alpha2.LinodeCluster{
// 						Spec: infrav1alpha2.LinodeClusterSpec{
// 							CredentialsRef: &corev1.SecretReference{
// 								Name:      "example",
// 								Namespace: "test",
// 							},
// 						},
// 					},
// 				},
// 			},
// 			expectedError: nil,
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				})
// 				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
// 					cred := corev1.Secret{
// 						Data: map[string][]byte{
// 							"apiToken": []byte("example"),
// 						},
// 					}
// 					*obj = cred
// 					return nil
// 				})
// 			},
// 		},
// 		{
// 			name: "Error - ValidateClusterScopeParams triggers error because ClusterScopeParams is empty",
// 			args: args{
// 				apiKey: "test-key",
// 				params: ClusterScopeParams{},
// 			},
// 			expectedError: fmt.Errorf("cluster is required when creating a ClusterScope"),
// 			expects:       func(mock *mock.MockK8sClient) {},
// 		},
// 		{
// 			name: "Error - patchHelper returns error. Checking error handle for when new patchHelper is invoked",
// 			args: args{
// 				apiKey: "test-key",
// 				params: ClusterScopeParams{
// 					Cluster:       &clusterv1.Cluster{},
// 					LinodeCluster: &infrav1alpha2.LinodeCluster{},
// 				},
// 			},
// 			expectedError: fmt.Errorf("failed to init patch helper:"),
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().Return(runtime.NewScheme())
// 			},
// 		},
// 		{
// 			name: "Error - Using getCredentialDataFromRef(), func returns an error. Unable to create a valid ClusterScope",
// 			args: args{
// 				apiKey: "test-key",
// 				params: ClusterScopeParams{
// 					Client:  nil,
// 					Cluster: &clusterv1.Cluster{},
// 					LinodeCluster: &infrav1alpha2.LinodeCluster{
// 						Spec: infrav1alpha2.LinodeClusterSpec{
// 							CredentialsRef: &corev1.SecretReference{
// 								Name:      "example",
// 								Namespace: "test",
// 							},
// 						},
// 					},
// 				},
// 			},
// 			expectedError: fmt.Errorf("credentials from secret ref: get credentials secret test/example: failed to get secret"),
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to get secret"))
// 			},
// 		},
// 		{
// 			name: "Error - createLinodeCluster throws an error for passing empty apiKey. Unable to create a valid ClusterScope",
// 			args: args{
// 				apiKey: "",
// 				params: ClusterScopeParams{
// 					Cluster:       &clusterv1.Cluster{},
// 					LinodeCluster: &infrav1alpha2.LinodeCluster{},
// 				},
// 			},
// 			expectedError: fmt.Errorf("failed to create linode client: token cannot be empty"),
// 			expects:       func(mock *mock.MockK8sClient) {},
// 		},
// 	}

// 	for _, tt := range tests {
// 		testcase := tt
// 		t.Run(testcase.name, func(t *testing.T) {
// 			t.Parallel()

// 			ctrl := gomock.NewController(t)
// 			defer ctrl.Finish()

// 			mockK8sClient := mock.NewMockK8sClient(ctrl)

// 			testcase.expects(mockK8sClient)

// 			testcase.args.params.Client = mockK8sClient

// 			got, err := NewClusterScope(context.Background(), ClientConfig{Token: testcase.args.apiKey}, testcase.args.params)

// 			if testcase.expectedError != nil {
// 				assert.ErrorContains(t, err, testcase.expectedError.Error())
// 			} else {
// 				assert.NotEmpty(t, got)
// 			}
// 		})
// 	}
// }

// func TestClusterAddCredentialsRefFinalizer(t *testing.T) {
// 	t.Parallel()
// 	type fields struct {
// 		Cluster       *clusterv1.Cluster
// 		LinodeCluster *infrav1alpha2.LinodeCluster
// 	}

// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		expects func(mock *mock.MockK8sClient)
// 	}{
// 		{
// 			name: "Success - finalizer should be added to the Linode Cluster credentials Secret",
// 			fields: fields{
// 				Cluster: &clusterv1.Cluster{},
// 				LinodeCluster: &infrav1alpha2.LinodeCluster{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name: "test-cluster",
// 					},
// 					Spec: infrav1alpha2.LinodeClusterSpec{
// 						CredentialsRef: &corev1.SecretReference{
// 							Name:      "example",
// 							Namespace: "test",
// 						},
// 					},
// 				},
// 			},
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				})
// 				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
// 					cred := corev1.Secret{
// 						ObjectMeta: metav1.ObjectMeta{
// 							Name:      "example",
// 							Namespace: "test",
// 						},
// 						Data: map[string][]byte{
// 							"apiToken": []byte("example"),
// 						},
// 					}
// 					*obj = cred

// 					return nil
// 				}).Times(2)
// 				mock.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
// 			},
// 		},
// 		{
// 			name: "No-op - no Linode Cluster credentials Secret",
// 			fields: fields{
// 				Cluster: &clusterv1.Cluster{},
// 				LinodeCluster: &infrav1alpha2.LinodeCluster{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name: "test-cluster",
// 					},
// 				},
// 			},
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				})
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		testcase := tt
// 		t.Run(testcase.name, func(t *testing.T) {
// 			t.Parallel()

// 			ctrl := gomock.NewController(t)
// 			defer ctrl.Finish()

// 			mockK8sClient := mock.NewMockK8sClient(ctrl)

// 			testcase.expects(mockK8sClient)

// 			cScope, err := NewClusterScope(
// 				context.Background(),
// 				ClientConfig{Token: "test-key"},
// 				ClusterScopeParams{
// 					Cluster:       testcase.fields.Cluster,
// 					LinodeCluster: testcase.fields.LinodeCluster,
// 					Client:        mockK8sClient,
// 				})
// 			if err != nil {
// 				t.Errorf("NewClusterScope() error = %v", err)
// 			}

// 			if err := cScope.AddCredentialsRefFinalizer(context.Background()); err != nil {
// 				t.Errorf("ClusterScope.AddCredentialsRefFinalizer() error = %v", err)
// 			}
// 		})
// 	}
// }

// func TestRemoveCredentialsRefFinalizer(t *testing.T) {
// 	t.Parallel()
// 	type fields struct {
// 		Cluster       *clusterv1.Cluster
// 		LinodeCluster *infrav1alpha2.LinodeCluster
// 	}

// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		expects func(mock *mock.MockK8sClient)
// 	}{
// 		{
// 			name: "Success - finalizer should be removed from the Linode Cluster credentials Secret",
// 			fields: fields{
// 				Cluster: &clusterv1.Cluster{},
// 				LinodeCluster: &infrav1alpha2.LinodeCluster{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name: "test-cluster",
// 					},
// 					Spec: infrav1alpha2.LinodeClusterSpec{
// 						CredentialsRef: &corev1.SecretReference{
// 							Name:      "example",
// 							Namespace: "test",
// 						},
// 					},
// 				},
// 			},
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				})
// 				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
// 					cred := corev1.Secret{
// 						ObjectMeta: metav1.ObjectMeta{
// 							Name:      "example",
// 							Namespace: "test",
// 						},
// 						Data: map[string][]byte{
// 							"apiToken": []byte("example"),
// 						},
// 					}
// 					*obj = cred

// 					return nil
// 				}).Times(2)
// 				mock.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
// 			},
// 		},
// 		{
// 			name: "No-op - no Linode Cluster credentials Secret",
// 			fields: fields{
// 				Cluster: &clusterv1.Cluster{},
// 				LinodeCluster: &infrav1alpha2.LinodeCluster{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name: "test-cluster",
// 					},
// 				},
// 			},
// 			expects: func(mock *mock.MockK8sClient) {
// 				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
// 					s := runtime.NewScheme()
// 					infrav1alpha2.AddToScheme(s)
// 					return s
// 				})
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		testcase := tt
// 		t.Run(testcase.name, func(t *testing.T) {
// 			t.Parallel()

// 			ctrl := gomock.NewController(t)
// 			defer ctrl.Finish()

// 			mockK8sClient := mock.NewMockK8sClient(ctrl)

// 			testcase.expects(mockK8sClient)

// 			cScope, err := NewClusterScope(
// 				context.Background(),
// 				ClientConfig{Token: "test-key"},
// 				ClusterScopeParams{
// 					Cluster:       testcase.fields.Cluster,
// 					LinodeCluster: testcase.fields.LinodeCluster,
// 					Client:        mockK8sClient,
// 				})
// 			if err != nil {
// 				t.Errorf("NewClusterScope() error = %v", err)
// 			}

// 			if err := cScope.RemoveCredentialsRefFinalizer(context.Background()); err != nil {
// 				t.Errorf("ClusterScope.RemoveCredentialsRefFinalizer() error = %v", err)
// 			}
// 		})
// 	}
// }
