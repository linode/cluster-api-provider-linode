package scope

import (
	"context"
	"errors"
	"reflect"
	"testing"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		name    string
		args    args
		want    *MachineScope
		wantErr bool
		expectedErr error
		patchFunc func(obj client.Object, crClient client.Client) (*patch.Helper, error)
		getFunc   func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
	}{
		// TODO: Add test cases.
		{
			name: "Success - Pass in valid args and get a valid MachineScope",
			args : args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				},
			},
			want: &MachineScope{},
			wantErr: false,
			expectedErr: nil,
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
		},
		{
			name: "Sucess - Pass in credential ref through MachineScopeParams.LinodeMachine and get a valid MachineScope",
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
			want: &MachineScope{},
			wantErr: false,
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
			name: "Sucess - Pass in credential ref through MachineScopeParams.LinodeCluster and get a valid MachineScope",
			args: args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
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
			want: &MachineScope{},
			wantErr: false,
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
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
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
			want: &MachineScope{},
			wantErr: true,
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
			want: nil,
			wantErr: true,
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
			args : args{
				apiKey: "test-key",
				params: MachineScopeParams{
					Client:        nil,
					Cluster:       &clusterv1.Cluster{},
					Machine:       &clusterv1.Machine{},
					LinodeCluster: &infrav1alpha1.LinodeCluster{},
					LinodeMachine: &infrav1alpha1.LinodeMachine{},
				},
			},
			want: &MachineScope{},
			wantErr: true,
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
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockk8sClient(ctrl)
			mockPatchHelper := mock.NewMockPatchHelper(ctrl)

			m := &MachineScope{
				client:        mockK8sClient,
				PatchHelper:   mockPatchHelper,
				Cluster:       tt.fields.Cluster,
				Machine:       tt.fields.Machine,
				LinodeClient:  tt.fields.LinodeClient,
				LinodeCluster: tt.fields.LinodeCluster,
				LinodeMachine: tt.fields.LinodeMachine,
			}
			got, err := m.GetBootstrapData(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("MachineScope.GetBootstrapData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MachineScope.GetBootstrapData() = %v, want %v", got, tt.want)
			}
		})
	}
}
