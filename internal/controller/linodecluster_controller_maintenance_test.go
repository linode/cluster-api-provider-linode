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

package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
)

const clusterLabelKey = "cluster.x-k8s.io/cluster-name"

func testSchemeForMaintenance(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, clusterv1.AddToScheme(s))
	require.NoError(t, infrav1alpha2.AddToScheme(s))
	return s
}

func newLinodeMachineWithID(name, ns, clusterName string, instanceID int) infrav1alpha2.LinodeMachine {
	return infrav1alpha2.LinodeMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				clusterLabelKey: clusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       name,
					UID:        types.UID("uid-" + name),
				},
			},
		},
		Spec: infrav1alpha2.LinodeMachineSpec{
			InstanceID: util.Pointer(instanceID),
		},
	}
}

func maintenanceForID(id int) linodego.AccountMaintenance {
	return linodego.AccountMaintenance{
		Entity: &linodego.Entity{
			Type: "linode",
			ID:   id,
		},
		Status: "scheduled",
	}
}

func newCapiCluster(name, ns string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
	}
}

// newCapiMachine UID must match the OwnerReference UID set by newLinodeMachineWithID.
func newCapiMachine(name, ns string) *clusterv1.Machine {
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			UID:       types.UID("uid-" + name),
		},
	}
}

func TestCollectMaintenanceInfo(t *testing.T) {
	t.Parallel()

	const clusterName = "test-cluster"
	const ns = defaultNamespace

	scheme := testSchemeForMaintenance(t)

	tests := []struct {
		name            string
		instanceIDs     []int
		apiMaintenances []linodego.AccountMaintenance
		apiError        error
		expectedNames   []string
		expectError     bool
	}{
		{
			name:            "no maintenance scheduled",
			instanceIDs:     []int{101, 102},
			apiMaintenances: []linodego.AccountMaintenance{},
			expectedNames:   nil,
		},
		{
			name:            "one machine has maintenance",
			instanceIDs:     []int{101, 102},
			apiMaintenances: []linodego.AccountMaintenance{maintenanceForID(101)},
			expectedNames:   []string{"machine-0"},
		},
		{
			name:        "multiple machines have maintenance",
			instanceIDs: []int{101, 102, 103},
			apiMaintenances: []linodego.AccountMaintenance{
				maintenanceForID(101),
				maintenanceForID(103),
			},
			expectedNames: []string{"machine-0", "machine-2"},
		},
		{
			name:            "maintenance entity ID not matching any machine is ignored",
			instanceIDs:     []int{101},
			apiMaintenances: []linodego.AccountMaintenance{maintenanceForID(999)},
			expectedNames:   nil,
		},
		{
			name:        "non-linode entity type is ignored",
			instanceIDs: []int{101},
			apiMaintenances: []linodego.AccountMaintenance{
				{Entity: &linodego.Entity{Type: "volume", ID: 101}, Status: "scheduled"},
			},
			expectedNames: nil,
		},
		{
			name:        "nil entity is ignored",
			instanceIDs: []int{101},
			apiMaintenances: []linodego.AccountMaintenance{
				{Entity: nil, Status: "scheduled"},
			},
			expectedNames: nil,
		},
		{
			name:        "maintenance API call fails",
			instanceIDs: []int{101},
			apiError:    errors.New("linode API unavailable"),
			expectError: true,
		},
		{
			name:            "no LinodeMachines in cluster",
			instanceIDs:     []int{},
			apiMaintenances: []linodego.AccountMaintenance{maintenanceForID(101)},
			expectedNames:   nil,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			mockLinodeClient.EXPECT().
				ListMaintenances(gomock.Any(), gomock.Any()).
				Return(tc.apiMaintenances, tc.apiError)

			cluster := newCapiCluster(clusterName, ns)
			objs := []client.Object{cluster}
			for i, id := range tc.instanceIDs {
				lm := newLinodeMachineWithID(fmt.Sprintf("machine-%d", i), ns, clusterName, id)
				objs = append(objs, &lm)
			}
			fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

			clusterScope := &scope.ClusterScope{
				Cluster:       cluster,
				LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
				LinodeClient:  mockLinodeClient,
				Client:        fakeClient,
			}

			result, err := (&LinodeClusterReconciler{}).collectMaintenanceInfo(context.Background(), clusterScope, testr.New(t))

			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var resultNames []string
			for _, lm := range result {
				resultNames = append(resultNames, lm.Name)
			}
			assert.ElementsMatch(t, tc.expectedNames, resultNames)
		})
	}
}

func TestSetMaintenanceConditions(t *testing.T) {
	t.Parallel()

	const clusterName = "test-cluster"
	const ns = defaultNamespace

	scheme := testSchemeForMaintenance(t)

	t.Run("no maintenance — no conditions set", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return([]linodego.AccountMaintenance{}, nil)

		cluster := newCapiCluster(clusterName, ns)
		lm := newLinodeMachineWithID("machine-1", ns, clusterName, 101)
		fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, &lm).Build()
		cs := &scope.ClusterScope{
			Cluster:       cluster,
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        fakeClient,
		}
		require.NoError(t, (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t)))
	})

	t.Run("one machine in maintenance — MaintenanceScheduled condition set", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return([]linodego.AccountMaintenance{
			maintenanceForID(101),
		}, nil)

		cluster := newCapiCluster(clusterName, ns)
		lm := newLinodeMachineWithID("machine-1", ns, clusterName, 101)
		machine := newCapiMachine("machine-1", ns)
		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster, &lm, machine).
			WithStatusSubresource(machine).
			Build()
		cs := &scope.ClusterScope{
			Cluster:       cluster,
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        fakeClient,
		}
		require.NoError(t, (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t)))

		updated := &clusterv1.Machine{}
		require.NoError(t, fakeClient.Get(context.Background(), client.ObjectKey{Name: "machine-1", Namespace: ns}, updated))
		var found bool
		for _, c := range updated.Status.Conditions {
			if c.Type == ConditionMaintenanceScheduled {
				found = true
				assert.Equal(t, metav1.ConditionTrue, c.Status)
			}
		}
		assert.True(t, found, "expected MaintenanceScheduled condition to be set")
	})

	t.Run("two machines in maintenance — both get conditions set", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return([]linodego.AccountMaintenance{
			maintenanceForID(101),
			maintenanceForID(102),
		}, nil)

		cluster := newCapiCluster(clusterName, ns)
		lm1 := newLinodeMachineWithID("machine-1", ns, clusterName, 101)
		lm2 := newLinodeMachineWithID("machine-2", ns, clusterName, 102)
		m1, m2 := newCapiMachine("machine-1", ns), newCapiMachine("machine-2", ns)
		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster, &lm1, &lm2, m1, m2).
			WithStatusSubresource(m1, m2).
			Build()
		cs := &scope.ClusterScope{
			Cluster:       cluster,
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        fakeClient,
		}
		require.NoError(t, (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t)))

		for _, name := range []string{"machine-1", "machine-2"} {
			updated := &clusterv1.Machine{}
			require.NoError(t, fakeClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: ns}, updated))
			var found bool
			for _, c := range updated.Status.Conditions {
				if c.Type == ConditionMaintenanceScheduled {
					found = true
				}
			}
			assert.Truef(t, found, "expected MaintenanceScheduled condition on %s", name)
		}
	})

	t.Run("only machines matching maintenance are patched — other machines unaffected", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return([]linodego.AccountMaintenance{
			maintenanceForID(101),
		}, nil)

		cluster := newCapiCluster(clusterName, ns)
		lm1 := newLinodeMachineWithID("machine-1", ns, clusterName, 101)
		lm2 := newLinodeMachineWithID("machine-2", ns, clusterName, 102)
		m1, m2 := newCapiMachine("machine-1", ns), newCapiMachine("machine-2", ns)
		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster, &lm1, &lm2, m1, m2).
			WithStatusSubresource(m1, m2).
			Build()
		cs := &scope.ClusterScope{
			Cluster:       cluster,
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        fakeClient,
		}
		require.NoError(t, (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t)))

		m1Updated := &clusterv1.Machine{}
		require.NoError(t, fakeClient.Get(context.Background(), client.ObjectKey{Name: "machine-1", Namespace: ns}, m1Updated))
		var m1HasCondition bool
		for _, c := range m1Updated.Status.Conditions {
			if c.Type == ConditionMaintenanceScheduled {
				m1HasCondition = true
			}
		}
		assert.True(t, m1HasCondition, "machine-1 should have MaintenanceScheduled condition")

		m2Updated := &clusterv1.Machine{}
		require.NoError(t, fakeClient.Get(context.Background(), client.ObjectKey{Name: "machine-2", Namespace: ns}, m2Updated))
		for _, c := range m2Updated.Status.Conditions {
			assert.NotEqual(t, ConditionMaintenanceScheduled, c.Type, "machine-2 should not have MaintenanceScheduled condition")
		}
	})

	t.Run("LinodeMachine has no InstanceID — skipped without error", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return([]linodego.AccountMaintenance{
			maintenanceForID(101),
		}, nil)

		cluster := newCapiCluster(clusterName, ns)
		lmNoID := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-noid",
				Namespace: ns,
				Labels:    map[string]string{clusterLabelKey: clusterName},
			},
		}
		fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, &lmNoID).Build()
		cs := &scope.ClusterScope{
			Cluster:       cluster,
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        fakeClient,
		}
		require.NoError(t, (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t)))
	})

	t.Run("LinodeMachine has no owner reference — skipped without error", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return([]linodego.AccountMaintenance{
			maintenanceForID(101),
		}, nil)

		cluster := newCapiCluster(clusterName, ns)
		lmNoOwner := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-orphan",
				Namespace: ns,
				Labels:    map[string]string{clusterLabelKey: clusterName},
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				InstanceID: util.Pointer(101),
			},
		}
		fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, &lmNoOwner).Build()
		cs := &scope.ClusterScope{
			Cluster:       cluster,
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        fakeClient,
		}
		require.NoError(t, (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t)))
	})

	t.Run("GetOwnerMachine fails — error aggregated", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		mk := mock.NewMockK8sClient(mockCtrl)

		lm := newLinodeMachineWithID("machine-1", ns, clusterName, 101)
		lmList := infrav1alpha2.LinodeMachineList{Items: []infrav1alpha2.LinodeMachine{lm}}

		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return([]linodego.AccountMaintenance{
			maintenanceForID(101),
		}, nil)
		mk.EXPECT().
			List(gomock.Any(), gomock.AssignableToTypeOf(&infrav1alpha2.LinodeMachineList{}), gomock.Any()).
			DoAndReturn(func(_ context.Context, list *infrav1alpha2.LinodeMachineList, _ ...client.ListOption) error {
				*list = lmList
				return nil
			})
		mk.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&clusterv1.Machine{}), gomock.Any()).
			Return(errors.New("API server unavailable"))

		cs := &scope.ClusterScope{
			Cluster:       newCapiCluster(clusterName, ns),
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        mk,
		}
		err := (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get owner Machine")
	})

	t.Run("ListMaintenances API error — returned immediately", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		ml := mock.NewMockLinodeClient(mockCtrl)
		ml.EXPECT().ListMaintenances(gomock.Any(), gomock.Any()).Return(nil, errors.New("API down"))

		cluster := newCapiCluster(clusterName, ns)
		lm := newLinodeMachineWithID("machine-1", ns, clusterName, 101)
		fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, &lm).Build()
		cs := &scope.ClusterScope{
			Cluster:       cluster,
			LinodeCluster: &infrav1alpha2.LinodeCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns}},
			LinodeClient:  ml,
			Client:        fakeClient,
		}
		err := (&LinodeClusterReconciler{}).setMaintenanceConditions(context.Background(), cs, testr.New(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API down")
	})
}

