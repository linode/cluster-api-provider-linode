package scope

import (
	"context"
	"reflect"
	"testing"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
		Cluster       *clusterv1.Cluster
		Machine       *clusterv1.Machine
		LinodeClient  *linodego.Client
		LinodeCluster *infrav1alpha1.LinodeCluster
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
				Cluster:       &clusterv1.Cluster{},
				Machine:       &clusterv1.Machine{},
				LinodeClient:  &linodego.Client{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
				LinodeMachine: &infrav1alpha1.LinodeMachine{},
			},
			false,
		},
		{
			"AddFinalizer error - finalizer should not be added to the Linode Machine object. Function returns nil since it was already present",
			fields{
				Cluster:       &clusterv1.Cluster{},
				Machine:       &clusterv1.Machine{},
				LinodeClient:  &linodego.Client{},
				LinodeCluster: &infrav1alpha1.LinodeCluster{},
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
				Cluster:       testcase.fields.Cluster,
				Machine:       testcase.fields.Machine,
				LinodeClient:  testcase.fields.LinodeClient,
				LinodeCluster: testcase.fields.LinodeCluster,
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
		ctx    context.Context
		apiKey string
		params MachineScopeParams
	}
	tests := []struct {
		name    string
		args    args
		want    *MachineScope
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewMachineScope(testcase.args.ctx, testcase.args.apiKey, testcase.args.params)
			if (err != nil) != testcase.wantErr {
				t.Errorf("NewMachineScope() error = %v, wantErr %v", err, testcase.wantErr)
				return
			}
			if !reflect.DeepEqual(got, testcase.want) {
				t.Errorf("NewMachineScope() = %v, want %v", got, testcase.want)
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
