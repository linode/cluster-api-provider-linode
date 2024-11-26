package scope

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateMachineScopeParams(t *testing.T) {
	t.Parallel()
	type args struct {
		params MachineScopeParams
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"Valid MachineScopeParams",
			args{
				params: MachineScopeParams{
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha2.LinodeCluster{},
					LinodeMachine: &infrav1alpha2.LinodeMachine{},
				},
			},
			false,
		},
		{
			"Invalid MachineScopeParams - empty MachineScopeParams",
			args{
				params: MachineScopeParams{},
			},
			true,
		},
		{
			"Invalid MachineScopeParams - no LinodeCluster in MachineScopeParams",
			args{
				params: MachineScopeParams{
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeMachine: &infrav1alpha2.LinodeMachine{},
				},
			},
			true,
		},
		{
			"Invalid MachineScopeParams - no LinodeMachine in MachineScopeParams",
			args{
				params: MachineScopeParams{
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha2.LinodeCluster{},
				},
			},
			true,
		},
		{
			"Invalid MachineScopeParams - no Cluster in MachineScopeParams",
			args{
				params: MachineScopeParams{
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha2.LinodeCluster{},
					LinodeMachine: &infrav1alpha2.LinodeMachine{},
				},
			},
			true,
		},
		{
			"Invalid MachineScopeParams - no Machine in MachineScopeParams",
			args{
				params: MachineScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha2.LinodeCluster{},
					LinodeMachine: &infrav1alpha2.LinodeMachine{},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if err := validateMachineScopeParams(testcase.args.params); (err != nil) != testcase.wantErr {
				t.Errorf("validateMachineScopeParams() error = %v, wantErr %v", err, testcase.wantErr)
			}
		})
	}
}

func TestMachineScopeAddFinalizer(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockK8sClient{}).Run(
		Call("scheme 1", func(ctx context.Context, mck Mock) {
			mck.K8sClient.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
				s := runtime.NewScheme()
				infrav1alpha2.AddToScheme(s)
				return s
			}).AnyTimes()
		}),
		OneOf(
			Path(Call("scheme 2", func(ctx context.Context, mck Mock) {
				mck.K8sClient.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
			})),
			Path(Result("has finalizer", func(ctx context.Context, mck Mock) {
				mScope, err := NewMachineScope(
					ctx,
					ClientConfig{Token: "apiToken"},
					MachineScopeParams{
						Client:        mck.K8sClient,
						Cluster:       &clusterv1.Cluster{},
						Machine:       &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha2.LinodeCluster{},
						LinodeMachine: &infrav1alpha2.LinodeMachine{
							ObjectMeta: metav1.ObjectMeta{
								Finalizers: []string{infrav1alpha2.MachineFinalizer},
							},
						},
					},
				)
				require.NoError(t, err)
				require.NoError(t, mScope.AddFinalizer(ctx))
				require.Len(t, mScope.LinodeMachine.Finalizers, 1)
				assert.Equal(t, infrav1alpha2.MachineFinalizer, mScope.LinodeMachine.Finalizers[0])
			})),
		),
		OneOf(
			Path(
				Call("able to patch", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(nil)
				}),
				Result("finalizer added", func(ctx context.Context, mck Mock) {
					mScope, err := NewMachineScope(
						ctx,
						ClientConfig{Token: "apiToken"},
						MachineScopeParams{
							Client:        mck.K8sClient,
							Cluster:       &clusterv1.Cluster{},
							Machine:       &clusterv1.Machine{},
							LinodeCluster: &infrav1alpha2.LinodeCluster{},
							LinodeMachine: &infrav1alpha2.LinodeMachine{},
						})
					require.NoError(t, err)
					require.NoError(t, mScope.AddFinalizer(ctx))
					require.Len(t, mScope.LinodeMachine.Finalizers, 1)
					assert.Equal(t, infrav1alpha2.MachineFinalizer, mScope.LinodeMachine.Finalizers[0])
				}),
			),
			Path(
				Call("unable to patch", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(errors.New("fail")).AnyTimes()
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					mScope, err := NewMachineScope(
						ctx,
						ClientConfig{Token: "apiToken"},
						MachineScopeParams{
							Client:        mck.K8sClient,
							Cluster:       &clusterv1.Cluster{},
							Machine:       &clusterv1.Machine{},
							LinodeCluster: &infrav1alpha2.LinodeCluster{},
							LinodeMachine: &infrav1alpha2.LinodeMachine{},
						})
					require.NoError(t, err)

					assert.Error(t, mScope.AddFinalizer(ctx))
				}),
			),
		),
	)
}

func TestNewMachineScope(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockK8sClient{}).Run(
		OneOf(
			Path(
				Result("invalid params", func(ctx context.Context, mck Mock) {
					mScope, err := NewMachineScope(
						ctx,
						ClientConfig{Token: "apiToken"},
						MachineScopeParams{},
					)
					require.ErrorContains(t, err, "is required")
					assert.Nil(t, mScope)
				})),
			Path(
				Result("no token", func(ctx context.Context, mck Mock) {
					mScope, err := NewMachineScope(ctx, ClientConfig{Token: ""}, MachineScopeParams{
						Client:        mck.K8sClient,
						Cluster:       &clusterv1.Cluster{},
						Machine:       &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha2.LinodeCluster{},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					require.ErrorContains(t, err, "failed to create linode client")
					assert.Nil(t, mScope)
				})),
		),
		OneOf(
			Path(
				Call("valid scheme", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
						s := runtime.NewScheme()
						infrav1alpha2.AddToScheme(s)
						return s
					}).AnyTimes()
				})),
			Path(
				Call("invalid scheme", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Scheme().Return(runtime.NewScheme()).AnyTimes()
				}),
				Result("cannot init patch helper", func(ctx context.Context, mck Mock) {
					mScope, err := NewMachineScope(ctx, ClientConfig{Token: "apiToken"}, MachineScopeParams{
						Client:        mck.K8sClient,
						Cluster:       &clusterv1.Cluster{},
						Machine:       &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha2.LinodeCluster{},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					require.ErrorContains(t, err, "failed to init patch helper")
					assert.Nil(t, mScope)
				}),
			),
		),
		OneOf(
			Path(Result("default credentials used", func(ctx context.Context, mck Mock) {
				mScope, err := NewMachineScope(ctx, ClientConfig{Token: "apiToken"}, MachineScopeParams{
					Client:        mck.K8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha2.LinodeCluster{},
					LinodeMachine: &infrav1alpha2.LinodeMachine{},
				})
				require.NoError(t, err)
				assert.NotNil(t, mScope)
			})),
			Path(
				Call("cluster object store used", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket_name": []byte("fake"),
							"s3_endpoint": []byte("fake"),
							"access_key":  []byte("fake"),
							"secret_key":  []byte("fake"),
						}}
						*obj = secret
						return nil
					})
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					mScope, err := NewMachineScope(ctx, ClientConfig{Token: "apiToken"}, MachineScopeParams{
						Client:  mck.K8sClient,
						Cluster: &clusterv1.Cluster{},
						Machine: &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"},
								},
							}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					require.NoError(t, err)
					assert.NotNil(t, mScope)
				})),
		),
	)
}

func TestMachineScopeGetBootstrapData(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockK8sClient{}).Run(
		Call("able to get secret", func(ctx context.Context, mck Mock) {
			mck.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
					secret := corev1.Secret{Data: map[string][]byte{"value": []byte("test-data")}}
					*obj = secret
					return nil
				})
		}),
		Result("success", func(ctx context.Context, mck Mock) {
			mScope := MachineScope{
				Client: mck.K8sClient,
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("test-data"),
						},
					},
				},
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
			}

			data, err := mScope.GetBootstrapData(ctx)
			require.NoError(t, err)
			assert.Equal(t, data, []byte("test-data"))
		}),
		OneOf(
			Path(Call("unable to get secret", func(ctx context.Context, mck Mock) {
				mck.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
					Return(apierrors.NewNotFound(schema.GroupResource{}, "test-data"))
			})),
			Path(Call("secret is missing data", func(ctx context.Context, mck Mock) {
				mck.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						*obj = corev1.Secret{}
						return nil
					})
			})),
			Path(Result("secret ref missing", func(ctx context.Context, mck Mock) {
				mScope := MachineScope{
					Client:        mck.K8sClient,
					Machine:       &clusterv1.Machine{},
					LinodeMachine: &infrav1alpha2.LinodeMachine{},
				}

				data, err := mScope.GetBootstrapData(ctx)
				require.ErrorContains(t, err, "bootstrap data secret is nil")
				assert.Empty(t, data)
			})),
		),
		Result("error", func(ctx context.Context, mck Mock) {
			mScope := MachineScope{
				Client: mck.K8sClient,
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("test-data"),
						},
					},
				},
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
			}

			data, err := mScope.GetBootstrapData(ctx)
			require.Error(t, err)
			assert.Empty(t, data)
		}),
	)
}

func TestMachineAddCredentialsRefFinalizer(t *testing.T) {
	t.Parallel()
	type fields struct {
		LinodeMachine *infrav1alpha2.LinodeMachine
	}
	tests := []struct {
		name    string
		fields  fields
		expects func(mock *mock.MockK8sClient)
	}{
		{
			"Success - finalizer should be added to the Linode Machine credentials Secret",
			fields{
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{
						CredentialsRef: &corev1.SecretReference{
							Name:      "example",
							Namespace: "test",
						},
					},
				},
			},
			func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
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
				}).AnyTimes()
				mock.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "No-op - no Linode Machine credentials Secret",
			fields: fields{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
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

			mScope, err := NewMachineScope(
				context.Background(),
				ClientConfig{Token: "apiToken"},
				MachineScopeParams{
					Client:        mockK8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha2.LinodeCluster{},
					LinodeMachine: testcase.fields.LinodeMachine,
				},
			)
			if err != nil {
				t.Errorf("NewMachineScope() error = %v", err)
			}

			if err := mScope.AddCredentialsRefFinalizer(context.Background()); err != nil {
				t.Errorf("MachineScope.AddCredentialsRefFinalizer() error = %v", err)
			}
		})
	}
}

func TestMachineRemoveCredentialsRefFinalizer(t *testing.T) {
	t.Parallel()
	type fields struct {
		LinodeMachine *infrav1alpha2.LinodeMachine
	}
	tests := []struct {
		name    string
		fields  fields
		expects func(mock *mock.MockK8sClient)
	}{
		{
			"Success - finalizer should be added to the Linode Machine credentials Secret",
			fields{
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{
						CredentialsRef: &corev1.SecretReference{
							Name:      "example",
							Namespace: "test",
						},
					},
				},
			},
			func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
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
				}).AnyTimes()
				mock.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "No-op - no Linode Machine credentials Secret",
			fields: fields{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
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

			mScope, err := NewMachineScope(
				context.Background(),
				ClientConfig{Token: "apiToken"},
				MachineScopeParams{
					Client:        mockK8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha2.LinodeCluster{},
					LinodeMachine: testcase.fields.LinodeMachine,
				},
			)
			if err != nil {
				t.Errorf("NewMachineScope() error = %v", err)
			}

			if err := mScope.RemoveCredentialsRefFinalizer(context.Background()); err != nil {
				t.Errorf("MachineScope.RemoveCredentialsRefFinalizer() error = %v", err)
			}
		})
	}
}

func TestMachineSetCredentialRefTokenForLinodeClients(t *testing.T) {
	t.Parallel()
	type fields struct {
		LinodeMachine *infrav1alpha2.LinodeMachine
		LinodeCluster *infrav1alpha2.LinodeCluster
	}
	tests := []struct {
		name          string
		fields        fields
		expects       func(mock *mock.MockK8sClient)
		expectedError error
	}{
		{
			name: "Success - Using LinodeMachine.Spec.CredentialsRef",
			fields: fields{
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{
						CredentialsRef: &corev1.SecretReference{
							Name:      "example",
							Namespace: "test",
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{},
			},
			expectedError: nil,
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
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
				}).AnyTimes()
			},
		},
		{
			name: "Error getting Linode Machine credentials Secret",
			fields: fields{
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{
						CredentialsRef: &corev1.SecretReference{
							Name:      "example",
							Namespace: "test",
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{},
			},
			expectedError: fmt.Errorf("credentials from secret ref: get credentials secret test/example: test error"),
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("test error"))
			},
		},
		{
			name: "Success - Using LinodeCluster.Spec.CredentialsRef",
			fields: fields{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						CredentialsRef: &corev1.SecretReference{
							Name:      "example",
							Namespace: "test",
						},
					},
				},
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
			},
			expectedError: nil,
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
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
				}).AnyTimes()
			},
		},
		{
			name: "Error getting Linode Cluster credentials Secret",
			fields: fields{
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						CredentialsRef: &corev1.SecretReference{
							Name:      "example",
							Namespace: "test",
						},
					},
				},
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
			},
			expectedError: fmt.Errorf("credentials from secret ref: get credentials secret test/example: test error"),
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).AnyTimes()
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("test error"))
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

			mScope, err := NewMachineScope(
				context.Background(),
				ClientConfig{Token: "apiToken"},
				MachineScopeParams{
					Client:        mockK8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: testcase.fields.LinodeCluster,
					LinodeMachine: testcase.fields.LinodeMachine,
				},
			)
			if err != nil {
				t.Errorf("NewMachineScope() error = %v", err)
			}

			if err := mScope.SetCredentialRefTokenForLinodeClients(context.Background()); err != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}
