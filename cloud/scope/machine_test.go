package scope

import (
	"context"
	"errors"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
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

func TestMachineScopeMethods(t *testing.T) {
	t.Parallel()
	type fields struct {
		LinodeMachine *infrav1alpha1.LinodeMachine
	}
	tests := []struct {
		name    string
		fields  fields
		expects func(mock *mock.Mockk8sClient)
	}{
		// TODO: Add test cases.
		{
			"Success - finalizer should be added to the Linode Machine object",
			fields{
				LinodeMachine: &infrav1alpha1.LinodeMachine{},
			},
			func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				}).AnyTimes()
				mock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			"AddFinalizer error - finalizer should not be added to the Linode Machine object. Function returns nil since it was already present",
			fields{
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-machine",
						Finalizers: []string{infrav1alpha1.GroupVersion.String()},
					},
				},
			},
			func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
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

			mockK8sClient := mock.NewMockk8sClient(ctrl)

			testcase.expects(mockK8sClient)

			mScope, err := NewMachineScope(
				context.Background(),
				"test-key",
				MachineScopeParams{
					Client:        mockK8sClient,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: testcase.fields.LinodeMachine,
				},
			)
			if err != nil {
				t.Errorf("NewMachineScope() error = %v", err)
			}

			if err := mScope.AddFinalizer(context.Background()); err != nil {
				t.Errorf("MachineScope.AddFinalizer() error = %v", err)
			}

			if mScope.LinodeMachine.Finalizers[0] != infrav1alpha1.GroupVersion.String() {
				t.Errorf("Not able to add finalizer")
			}
		})
	}
}

func TestNewMachineScope(t *testing.T) {
	t.Parallel()
	type args struct {
		apiKey string
		params MachineScopeParams
	}
	tests := []struct {
		name        string
		args        args
		want        *MachineScope
		expectedErr error
		expects     func(mock *mock.Mockk8sClient)
	}{
		{
			name: "Success - Pass in valid args and get a valid MachineScope",
			args: args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				},
			},
			expectedErr: nil,
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
			},
		},
		{
			name: "Success - Pass in credential ref through MachineScopeParams.LinodeMachine and get a valid MachineScope",
			args: args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:        nil,
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
				},
			},
			expectedErr: nil,
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"apiToken": []byte("example"),
						},
					}
					*obj = cred
					return nil
				})
			},
		},
		{
			name: "Success - Pass in credential ref through MachineScopeParams.LinodeCluster and get a valid MachineScope",
			args: args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:  nil,
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
				},
			},
			expectedErr: nil,
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"apiToken": []byte("example"),
						},
					}
					*obj = cred
					return nil
				})
			},
		},
		{
			name: "Error - Pass in credential ref through MachineScopeParams.LinodeCluster and getCredentialDataFromRef() returns error",
			args: args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:  nil,
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
				},
			},
			expectedErr: errors.New("credentials from cluster secret ref: get credentials secret test/example: Creds not found"),
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("Creds not found"))
			},
		},
		{
			name: "Error - Pass in invalid args and get an error. Set ClusterScopeParams.Cluster to nil",
			args: args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:        nil,
					Cluster:       nil,
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				},
			},
			expectedErr: errors.New("custer is required when creating a MachineScope"),
			expects:     func(mock *mock.Mockk8sClient) {},
		},
		{
			name: "Error - Pass in valid args but couldn't get patch helper",
			args: args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				},
			},
			expectedErr: errors.New("failed to init patch helper: no kind is registered for the type v1alpha1.LinodeMachine in scheme \"pkg/runtime/scheme.go:100\""),
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Scheme().Return(runtime.NewScheme())
			},
		},
		{
			name: "Error - createLinodeClient() returns error for passing empty apiKey",
			args: args{
				apiKey: "",
				params: MachineScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				},
			},
			expectedErr: errors.New("failed to create linode client: missing Linode API key"),
			expects:     func(mock *mock.Mockk8sClient) {},
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockk8sClient(ctrl)

			testcase.expects(mockK8sClient)

			testcase.args.params.Client = mockK8sClient

			got, err := NewMachineScope(context.Background(), testcase.args.apiKey, testcase.args.params)

			if testcase.expectedErr != nil {
				assert.EqualError(t, err, testcase.expectedErr.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestMachineScopeGetBootstrapData(t *testing.T) {
	t.Parallel()
	type fields struct {
		Cluster       *clusterv1.Cluster
		Machine       *clusterv1.Machine
		LinodeClient  *linodego.Client
		LinodeCluster *infrav1alpha1.LinodeCluster
		LinodeMachine *infrav1alpha1.LinodeMachine
	}
	tests := []struct {
		name        string
		fields      fields
		want        []byte
		expectedErr error
		expects     func(mock *mock.Mockk8sClient)
	}{
		// TODO: Add test cases.
		{
			name: "Success - Using a valid MachineScope. Get bootstrap data",
			fields: fields{
				Cluster: &clusterv1.Cluster{},
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("test-data"),
						},
					},
				},
				LinodeClient:  &linodego.Client{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-linode-machine",
						Namespace: "test-namespace",
					},
				},
			},
			want:        []byte("test-data"),
			expectedErr: nil,
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("test-data"),
						},
					}
					*obj = cred
					return nil
				})
			},
		},
		{
			name: "Error - Set MachineScope.Machine.Spec.Bootstrap.DataSecretName to nil. Returns an error",
			fields: fields{
				Cluster: &clusterv1.Cluster{},
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: nil,
						},
					},
				},
				LinodeClient:  &linodego.Client{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-linode-machine",
						Namespace: "test-namespace",
					},
				},
			},
			want:        nil,
			expectedErr: errors.New("bootstrap data secret is nil for LinodeMachine test-namespace/test-linode-machine"),
			expects:     func(mock *mock.Mockk8sClient) {},
		},
		{
			name: "Error - client.Get return an error while retrieving bootstrap data secret",
			fields: fields{
				Cluster: &clusterv1.Cluster{},
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("test-data"),
						},
					},
				},
				LinodeClient:  &linodego.Client{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-linode-machine",
						Namespace: "test-namespace",
					},
				},
			},
			want:        nil,
			expectedErr: errors.New("failed to retrieve bootstrap data secret for LinodeMachine test-namespace/test-linode-machine"),
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("test-error"))
			},
		},
		{
			name: "Error - client.Get return some data but it doesn't contain the bootstrap data secret and secret key 'value' is missing",
			fields: fields{
				Cluster: &clusterv1.Cluster{},
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("test-data"),
						},
					},
				},
				LinodeClient:  &linodego.Client{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-linode-machine",
						Namespace: "test-namespace",
					},
				},
			},
			want:        nil,
			expectedErr: errors.New("bootstrap data secret value key is missing for LinodeMachine test-namespace/test-linode-machine"),
			expects: func(mock *mock.Mockk8sClient) {
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
						cred := corev1.Secret{
							Data: map[string][]byte{},
						}
						*obj = cred
						return nil
					},
				)
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
			testcase.expects(mockK8sClient)

			mScope := &MachineScope{
				client:        mockK8sClient,
				PatchHelper:   &patch.Helper{}, // empty patch helper
				Cluster:       testcase.fields.Cluster,
				Machine:       testcase.fields.Machine,
				LinodeClient:  testcase.fields.LinodeClient,
				LinodeCluster: testcase.fields.LinodeCluster,
				LinodeMachine: testcase.fields.LinodeMachine,
			}

			got, err := mScope.GetBootstrapData(context.Background())

			if testcase.expectedErr != nil {
				assert.EqualError(t, err, testcase.expectedErr.Error())
			} else {
				assert.Equal(t, testcase.want, got)
			}
		})
	}
}
