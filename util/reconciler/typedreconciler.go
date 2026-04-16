package reconciler

import (
	"context"

	o11yreconciler "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimereconciler"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func AsReconcilerWithTracing[object client.Object](k8sClient client.Client, rec reconcile.ObjectReconciler[object]) reconcile.Reconciler {
	capiReconciler := &capiReconcilerAdapter[object]{
		objReconciler: rec,
		k8sClient:     k8sClient,
	}

	return o11yreconciler.NewRuntimeReconcilerWithTracing(
		reconcile.AsReconciler(
			k8sClient, capiReconciler),
		o11yreconciler.DefaultDecorator())
}

type capiReconcilerAdapter[object client.Object] struct {
	objReconciler reconcile.ObjectReconciler[object]
	k8sClient     client.Client
}

func (a *capiReconcilerAdapter[object]) Reconcile(ctx context.Context, o object) (reconcile.Result, error) {
	// Skip normal reconciliation when clusterctl marks the object for deletion during a move.
	// Reconciling here could recreate or mutate infrastructure while ownership is being handed off.
	if annotations := o.GetAnnotations(); annotations != nil {
		if _, exists := annotations[clusterctlv1.DeleteForMoveAnnotation]; exists {
			return reconcile.Result{}, nil
		}
	}
	return a.objReconciler.Reconcile(ctx, o)
}
