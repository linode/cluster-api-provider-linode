package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/linodego/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestReconcilePlacementGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		desiredID               int
		currentID               int
		placementGroupRef       *infrav1alpha2.LinodePlacementGroup
		expectedUnassignGroupID int
		expectedAssignGroupID   int
		unassignErr             error
		assignErr               error
	}{
		{name: "no placement group"},
		{name: "already assigned", desiredID: 10, currentID: 10},
		{name: "assigns ungrouped instance", desiredID: 10, expectedAssignGroupID: 10},
		{name: "moves instance", desiredID: 10, currentID: 5, expectedUnassignGroupID: 5, expectedAssignGroupID: 10},
		{name: "removes instance", currentID: 5, expectedUnassignGroupID: 5},
		{
			name: "resolves placement group reference",
			placementGroupRef: &infrav1alpha2.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "managed", Namespace: "default"},
				Spec:       infrav1alpha2.LinodePlacementGroupSpec{PGID: new(10)},
				Status:     infrav1alpha2.LinodePlacementGroupStatus{Ready: true},
			},
			currentID: 10,
		},
		{
			name:      "direct ID takes precedence over placement group reference",
			desiredID: 10,
			placementGroupRef: &infrav1alpha2.LinodePlacementGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "managed", Namespace: "default"},
				Spec:       infrav1alpha2.LinodePlacementGroupSpec{PGID: new(20)},
				Status:     infrav1alpha2.LinodePlacementGroupStatus{Ready: true},
			},
			currentID: 10,
		},
		{name: "returns unassign error", desiredID: 10, currentID: 5, expectedUnassignGroupID: 5, unassignErr: errors.New("unassign failed")},
		{name: "returns assign error", desiredID: 10, expectedAssignGroupID: 10, assignErr: errors.New("assign failed")},
		{name: "returns assign error after unassigning original group", desiredID: 10, currentID: 5, expectedUnassignGroupID: 5, expectedAssignGroupID: 10, assignErr: errors.New("assign failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			linodeClient := mock.NewMockLinodeClient(ctrl)
			machineScope := &scope.MachineScope{
				LinodeClient: linodeClient,
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
					Spec: infrav1alpha2.LinodeMachineSpec{
						PlacementGroupID: tt.desiredID,
					},
				},
			}

			if tt.placementGroupRef != nil {
				scheme := runtime.NewScheme()
				require.NoError(t, infrav1alpha2.AddToScheme(scheme))
				machineScope.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.placementGroupRef).Build()
				machineScope.LinodeMachine.Spec.PlacementGroupRef = &corev1.ObjectReference{
					Name:      tt.placementGroupRef.Name,
					Namespace: tt.placementGroupRef.Namespace,
				}
			}

			instance := &linodego.Instance{ID: 100}
			if tt.currentID != 0 {
				instance.PlacementGroup = &linodego.InstancePlacementGroup{ID: tt.currentID}
			}

			var unassignCall *gomock.Call
			if tt.expectedUnassignGroupID != 0 {
				unassignCall = linodeClient.EXPECT().UnassignPlacementGroupLinodes(
					gomock.Any(),
					tt.expectedUnassignGroupID,
					linodego.PlacementGroupUnAssignOptions{Linodes: []int{100}},
				).Return(nil, tt.unassignErr)
			}
			if tt.expectedAssignGroupID != 0 && tt.unassignErr == nil {
				assignCall := linodeClient.EXPECT().AssignPlacementGroupLinodes(
					gomock.Any(),
					tt.expectedAssignGroupID,
					linodego.PlacementGroupAssignOptions{Linodes: []int{100}},
				).Return(nil, tt.assignErr)
				if unassignCall != nil {
					assignCall.After(unassignCall)
				}
			}

			err := (&LinodeMachineReconciler{}).reconcilePlacementGroup(context.Background(), logr.Discard(), machineScope, instance)
			if tt.unassignErr != nil || tt.assignErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigurePlacementGroupWithDirectID(t *testing.T) {
	t.Parallel()

	createConfig := &linodego.InstanceCreateOptions{}
	machineScope := &scope.MachineScope{
		LinodeMachine: &infrav1alpha2.LinodeMachine{
			Spec: infrav1alpha2.LinodeMachineSpec{PlacementGroupID: 10},
		},
	}

	require.NoError(t, configurePlacementGroup(context.Background(), machineScope, createConfig, logr.Discard()))
	require.NotNil(t, createConfig.PlacementGroup)
	require.Equal(t, 10, createConfig.PlacementGroup.ID)
}
