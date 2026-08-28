package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func TestMachineTemplatePropagatesMutableFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, infrav1alpha2.AddToScheme(scheme))

	template := &infrav1alpha2.LinodeMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "template", Namespace: "default"},
		Spec: infrav1alpha2.LinodeMachineTemplateSpec{
			Template: infrav1alpha2.LinodeMachineTemplateResource{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Region:           "us-ord",
					Type:             "g6-standard-1",
					Tags:             []string{"new-tag"},
					FirewallID:       20,
					PlacementGroupID: 30,
				},
			},
		},
		Status: infrav1alpha2.LinodeMachineTemplateStatus{
			Tags:             []string{"new-tag"},
			FirewallID:       20,
			PlacementGroupID: 30,
		},
	}
	machine := &infrav1alpha2.LinodeMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine",
			Namespace: "default",
			Annotations: map[string]string{
				clusterv1.TemplateClonedFromNameAnnotation: template.Name,
			},
		},
		Spec: infrav1alpha2.LinodeMachineSpec{Region: "us-ord", Type: "g6-standard-1"},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(template).WithObjects(template, machine).Build()

	lmtScope, err := scope.NewMachineTemplateScope(ctx, scope.MachineTemplateScopeParams{
		Client:                k8sClient,
		LinodeMachineTemplate: template,
	})
	require.NoError(t, err)

	reconciler := &LinodeMachineTemplateReconciler{Client: k8sClient, Logger: logr.Discard()}
	_, err = reconciler.reconcile(ctx, lmtScope)
	require.NoError(t, err)

	updatedMachine := &infrav1alpha2.LinodeMachine{}
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(machine), updatedMachine))
	require.Equal(t, []string{"new-tag"}, updatedMachine.Spec.Tags)
	require.Equal(t, 20, updatedMachine.Spec.FirewallID)
	require.Equal(t, 30, updatedMachine.Spec.PlacementGroupID)

	updatedTemplate := &infrav1alpha2.LinodeMachineTemplate{}
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(template), updatedTemplate))
	require.Equal(t, []string{"new-tag"}, updatedTemplate.Status.Tags)
	require.Equal(t, 20, updatedTemplate.Status.FirewallID)
	require.Equal(t, 30, updatedTemplate.Status.PlacementGroupID)
}
