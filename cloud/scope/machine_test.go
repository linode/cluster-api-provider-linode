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

func TestMachineScopeAddFinalizer(t *testing.T) {
	t.Parallel()
	type fields struct {
		LinodeMachine *infrav1alpha1.LinodeMachine
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"Success - finalizer should be added to the Linode Machine object",
			fields{
				LinodeMachine: &infrav1alpha1.LinodeMachine{},
			},
			false,
		},
		{
			"AddFinalizer error - finalizer should not be added to the Linode Machine object. Function returns nil since it was already present",
			fields{
				LinodeMachine: &infrav1alpha1.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{infrav1alpha1.GroupVersion.String()},
					},
				},
			},
			false,
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

			mScope := &MachineScope{
				client:        mockK8sClient,
				PatchHelper:   mockPatchHelper,
				Cluster:       &clusterv1.Cluster{},
				Machine:       &clusterv1.Machine{},
				LinodeClient:  &linodego.Client{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
				LinodeMachine: testcase.fields.LinodeMachine,
			}

			if mScope.LinodeMachine.Finalizers == nil {
				mockPatchHelper.EXPECT().Patch(gomock.Any(), gomock.Any()).Return(nil)
			}

			if err := mScope.AddFinalizer(context.Background()); (err != nil) != testcase.wantErr {
				t.Errorf("MachineScope.AddFinalizer() error = %v, wantErr %v", err, testcase.wantErr)
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
		patchFunc   func(obj client.Object, crClient client.Client) (*patch.Helper, error)
		getFunc     func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
	}{
		// TODO: Add test cases.
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
			want:        &MachineScope{},
			expectedErr: nil,
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
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
			want:        &MachineScope{},
			expectedErr: nil,
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				cred := corev1.Secret{
					Data: map[string][]byte{
						"apiToken": []byte("example"),
					},
				}
				*obj = cred

				return nil
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
			want:        &MachineScope{},
			expectedErr: nil,
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				cred := corev1.Secret{
					Data: map[string][]byte{
						"apiToken": []byte("example"),
					},
				}
				*obj = cred

				return nil
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
			want:        &MachineScope{},
			expectedErr: errors.New("credentials from cluster secret ref: get credentials secret test/example: Creds not found"),
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return errors.New("Creds not found")
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
			want:        nil,
			expectedErr: errors.New("custer is required when creating a MachineScope"),
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
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
			want:        &MachineScope{},
			expectedErr: errors.New("failed to init patch helper: failed to create patch helper"),
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return nil, errors.New("failed to create patch helper")
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
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

			linodeMachine := testcase.args.params.LinodeMachine
			linodeCluster := testcase.args.params.LinodeCluster

			if (linodeMachine != nil && linodeMachine.Spec.CredentialsRef != nil) ||
				(linodeCluster != nil && linodeCluster.Spec.CredentialsRef != nil) {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testcase.getFunc).Times(1)
			}

			testcase.args.params.Client = mockK8sClient

			got, err := NewMachineScope(context.Background(), testcase.args.apiKey, testcase.args.params, testcase.patchFunc)

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
		getFunc     func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
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
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				cred := corev1.Secret{
					Data: map[string][]byte{
						"value": []byte("test-data"),
					},
				}
				*obj = cred
				return nil
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
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
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
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return errors.New("test-error")
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
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				cred := corev1.Secret{
					Data: map[string][]byte{},
				}
				*obj = cred
				return nil
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

			if testcase.fields.Machine.Spec.Bootstrap.DataSecretName != nil {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testcase.getFunc).Times(1)
			}

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
