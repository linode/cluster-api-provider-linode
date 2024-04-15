package scope

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
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
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
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
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
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
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
				},
			},
			true,
		},
		{
			"Invalid MachineScopeParams - no Cluster in MachineScopeParams",
			args{
				params: MachineScopeParams{
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				},
			},
			true,
		},
		{
			"Invalid MachineScopeParams - no Machine in MachineScopeParams",
			args{
				params: MachineScopeParams{
					Cluster:       &clusterv1.Cluster{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
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
	NewTestSuite(mock.MockK8sClient{}).Run(t, Paths(
		Mock("scheme 1", func(ctx MockContext) {
			ctx.K8sClient.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
				s := runtime.NewScheme()
				infrav1alpha1.AddToScheme(s)
				return s
			})
		}),
		Either(
			Mock("scheme 2", func(ctx MockContext) {
				ctx.K8sClient.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
			}),
			Result("has finalizer", func(ctx MockContext) {
				mScope, err := NewMachineScope(ctx, "token", MachineScopeParams{
					Client:        ctx.K8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers: []string{infrav1alpha1.GroupVersion.String()},
						},
					},
				})
				require.NoError(t, err)
				assert.NoError(t, mScope.AddFinalizer(ctx))
				require.Len(t, mScope.LinodeMachine.Finalizers, 1)
				assert.Equal(t, mScope.LinodeMachine.Finalizers[0], infrav1alpha1.GroupVersion.String())
			}),
		),
		Either(
			Case(
				Mock("able to patch", func(ctx MockContext) {
					ctx.K8sClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(nil)
				}),
				Result("finalizer added", func(ctx MockContext) {
					mScope, err := NewMachineScope(ctx, "token", MachineScopeParams{
						Client:        ctx.K8sClient,
						Cluster:       &clusterv1.Cluster{},
						Machine:       &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha1.LinodeCluster{},
						LinodeMachine: &infrav1alpha1.LinodeMachine{},
					})
					require.NoError(t, err)
					assert.NoError(t, mScope.AddFinalizer(ctx))
					require.Len(t, mScope.LinodeMachine.Finalizers, 1)
					assert.Equal(t, mScope.LinodeMachine.Finalizers[0], infrav1alpha1.GroupVersion.String())
				}),
			),
			Case(
				Mock("unable to patch", func(ctx MockContext) {
					ctx.K8sClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(errors.New("fail"))
				}),
				Result("error", func(ctx MockContext) {
					mScope, err := NewMachineScope(ctx, "token", MachineScopeParams{
						Client:        ctx.K8sClient,
						Cluster:       &clusterv1.Cluster{},
						Machine:       &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha1.LinodeCluster{},
						LinodeMachine: &infrav1alpha1.LinodeMachine{},
					})
					require.NoError(t, err)

					assert.Error(t, mScope.AddFinalizer(ctx))
				}),
			),
		),
	))
}

func TestNewMachineScope(t *testing.T) {
	NewTestSuite(mock.MockK8sClient{}).Run(t, Paths(
		Either(
			Result("invalid params", func(ctx MockContext) {
				mScope, err := NewMachineScope(ctx, "token", MachineScopeParams{})
				require.ErrorContains(t, err, "is required")
				assert.Nil(t, mScope)
			}),
			Result("no token", func(ctx MockContext) {
				mScope, err := NewMachineScope(ctx, "", MachineScopeParams{
					Client:        ctx.K8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				})
				require.ErrorContains(t, err, "failed to create linode client")
				assert.Nil(t, mScope)
			}),
			Case(
				Mock("no secret", func(ctx MockContext) {
					ctx.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "example"))
				}),
				Result("error", func(ctx MockContext) {
					mScope, err := NewMachineScope(ctx, "", MachineScopeParams{
						Client:        ctx.K8sClient,
						Cluster:       &clusterv1.Cluster{},
						Machine:       &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha1.LinodeCluster{},
						LinodeMachine: &infrav1alpha1.LinodeMachine{
							Spec: infrav1alpha1.LinodeMachineSpec{
								CredentialsRef: &corev1.SecretReference{
									Name:      "example",
									Namespace: "test",
								},
							},
						},
					})
					require.ErrorContains(t, err, "credentials from secret ref")
					assert.Nil(t, mScope)
				}),
			),
		),
		Either(
			Mock("valid scheme", func(ctx MockContext) {
				ctx.K8sClient.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
			}),
			Case(
				Mock("invalid scheme", func(ctx MockContext) {
					ctx.K8sClient.EXPECT().Scheme().Return(runtime.NewScheme())
				}),
				Result("cannot init patch helper", func(ctx MockContext) {
					mScope, err := NewMachineScope(ctx, "token", MachineScopeParams{
						Client:        ctx.K8sClient,
						Cluster:       &clusterv1.Cluster{},
						Machine:       &clusterv1.Machine{},
						LinodeCluster: &infrav1alpha1.LinodeCluster{},
						LinodeMachine: &infrav1alpha1.LinodeMachine{},
					})
					require.ErrorContains(t, err, "failed to init patch helper")
					assert.Nil(t, mScope)
				}),
			),
		),
		Either(
			Mock("credentials in secret", func(ctx MockContext) {
				ctx.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						*obj = corev1.Secret{
							Data: map[string][]byte{
								"apiToken": []byte("token"),
							},
						}
						return nil
					})
			}),
			Result("default credentials", func(ctx MockContext) {
				mScope, err := NewMachineScope(ctx, "token", MachineScopeParams{
					Client:        ctx.K8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				})
				require.NoError(t, err)
				assert.NotNil(t, mScope)
			}),
		),
		Either(
			Result("credentials from LinodeMachine credentialsRef", func(ctx MockContext) {
				mScope, err := NewMachineScope(ctx, "", MachineScopeParams{
					Client:        ctx.K8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{
						Spec: infrav1alpha1.LinodeMachineSpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
				})
				require.NoError(t, err)
				assert.NotNil(t, mScope)
			}),
			Result("credentials from LinodeCluster credentialsRef", func(ctx MockContext) {
				mScope, err := NewMachineScope(ctx, "token", MachineScopeParams{
					Client:  ctx.K8sClient,
					Cluster: &clusterv1.Cluster{},
					Machine: &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{
						Spec: infrav1alpha1.LinodeClusterSpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				})
				require.NoError(t, err)
				assert.NotNil(t, mScope)
			}),
		),
	))
}

func TestMachineScopeGetBootstrapData(t *testing.T) {
	NewTestSuite(mock.MockK8sClient{}).Run(t, Paths(
		Mock("able to get secret", func(ctx MockContext) {
			ctx.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
					secret := corev1.Secret{Data: map[string][]byte{"value": []byte("test-data")}}
					*obj = secret
					return nil
				})
		}),
		Result("success", func(ctx MockContext) {
			mScope := MachineScope{
				client: ctx.K8sClient,
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("test-data"),
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{},
			}

			data, err := mScope.GetBootstrapData(ctx)
			require.NoError(t, err)
			assert.Equal(t, data, []byte("test-data"))
		}),
		Either(
			Mock("unable to get secret", func(ctx MockContext) {
				ctx.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
					Return(apierrors.NewNotFound(schema.GroupResource{}, "test-data"))
			}),
			Mock("secret is missing data", func(ctx MockContext) {
				ctx.K8sClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						*obj = corev1.Secret{}
						return nil
					})
			}),
			Result("secret ref missing", func(ctx MockContext) {
				mScope := MachineScope{
					client:        ctx.K8sClient,
					Machine:       &clusterv1.Machine{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				}

				data, err := mScope.GetBootstrapData(ctx)
				require.ErrorContains(t, err, "bootstrap data secret is nil")
				assert.Len(t, data, 0)
			}),
		),
		Result("error", func(ctx MockContext) {
			mScope := MachineScope{
				client: ctx.K8sClient,
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("test-data"),
						},
					},
				},
				LinodeMachine: &infrav1alpha1.LinodeMachine{},
			}

			data, err := mScope.GetBootstrapData(ctx)
			require.Error(t, err)
			assert.Len(t, data, 0)
		}),
	))
}
