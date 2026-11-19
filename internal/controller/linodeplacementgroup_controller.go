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
	"net/http"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/cluster-api-provider-linode/util/reconciler/common"
)

// LinodePlacementGroupReconciler reconciles a LinodePlacementGroup object
type LinodePlacementGroupReconciler struct {
	Recorder           events.EventRecorder
	LinodeClientConfig scope.ClientConfig
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
}

var _ common.Reconciler[*scope.PlacementGroupScope, *infrav1alpha2.LinodePlacementGroup] = &LinodePlacementGroupReconciler{}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

func (r *LinodePlacementGroupReconciler) Name() string {
	return "LinodePlacementGroup"
}

func (r *LinodePlacementGroupReconciler) Object() *infrav1alpha2.LinodePlacementGroup {
	return &infrav1alpha2.LinodePlacementGroup{}
}

func (r *LinodePlacementGroupReconciler) RequeueAfter() time.Duration {
	return reconciler.DefaultPGControllerReconcilerDelay
}

func (r *LinodePlacementGroupReconciler) Timeout() time.Duration {
	return reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)
}

func (r *LinodePlacementGroupReconciler) AddWatches(bldr *builder.Builder, cl clients.K8sClient) {
	bldr.WithEventFilter(predicate.And(
		predicates.ResourceHasFilterLabel(cl.Scheme(), ctrl.Log, r.WatchFilterValue),
		predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		),
		predicate.Funcs{UpdateFunc: placementGroupUpdatePredicate},
	))
}

// instructs the controller to not enqueue an update event if the PlacementGroup was just created.
func placementGroupUpdatePredicate(e event.UpdateEvent) bool {
	oldObject, okOld := e.ObjectOld.(*infrav1alpha2.LinodePlacementGroup)
	newObject, okNew := e.ObjectNew.(*infrav1alpha2.LinodePlacementGroup)
	if okOld && okNew && oldObject.Spec.PGID == nil && newObject.Spec.PGID != nil {
		return false
	}
	return true
}

func (r *LinodePlacementGroupReconciler) NewScope(ctx context.Context, cl clients.K8sClient, obj *infrav1alpha2.LinodePlacementGroup, cluster *clusterv1.Cluster) (*scope.PlacementGroupScope, error) {
	return scope.NewPlacementGroupScope(ctx, r.LinodeClientConfig, scope.PlacementGroupScopeParams{
		Client:               cl,
		LinodePlacementGroup: obj,
		Cluster:              cluster,
	})
}

func (r *LinodePlacementGroupReconciler) Apply(ctx context.Context, pgScope *scope.PlacementGroupScope, logger logr.Logger) (ctrl.Result, error) {
	if pgScope.LinodePlacementGroup.Spec.PGID != nil {
		// Update is essentially a no-op as everything is immutable, just set it to ready and move on
		pgScope.LinodePlacementGroup.Status.Ready = true
		return ctrl.Result{}, nil
	}

	pgScope.ErrorType = infrav1alpha2.CreatePlacementGroupError

	err := r.reconcilePlacementGroup(ctx, pgScope, logger)
	if err != nil {
		logger.Error(err, "Failed to create/update PlacementGroup")
		return ctrl.Result{}, err
	}

	pgScope.LinodePlacementGroup.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *LinodePlacementGroupReconciler) Delete(ctx context.Context, pgScope *scope.PlacementGroupScope, logger logr.Logger) (ctrl.Result, error) {
	pgScope.ErrorType = infrav1alpha2.DeletePlacementGroupError

	res, err := r.deletePlacementGroup(ctx, logger, pgScope)
	if err != nil {
		return res, err
	}
	if res.RequeueAfter > 0 {
		return res, nil
	}

	// TODO: Why write a condition here if the K8s object is being deleted?
	pgScope.LinodePlacementGroup.SetCondition(metav1.Condition{
		Type:    clusterv1.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  clusterv1.DeletionCompletedReason,
		Message: "Placement Group deleted",
	})

	pgScope.LinodePlacementGroup.Spec.PGID = nil

	return ctrl.Result{}, nil
}

func (r *LinodePlacementGroupReconciler) deletePlacementGroup(ctx context.Context, logger logr.Logger, pgScope *scope.PlacementGroupScope) (ctrl.Result, error) {
	if pgScope.LinodePlacementGroup.Spec.PGID == nil {
		logger.Info("Placement Group ID is missing, nothing to do")
		return ctrl.Result{}, nil
	}

	pgID := *pgScope.LinodePlacementGroup.Spec.PGID
	logger = logger.WithValues("pgID", pgID)

	pg, err := pgScope.LinodeClient.GetPlacementGroup(ctx, pgID)
	if err != nil {
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) == nil {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch Placement Group from API")
		if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("Re-queuing Placement Group deletion due to fetch error")
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(reconciler.DefaultPGControllerReconcilerDelay)}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch placement group %d after timeout: %w", pgID, err)
	}

	if len(pg.Members) > 0 {
		logger.Info("Placement Group still has node(s) attached", "count", len(pg.Members))
		waitTimeout := reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerWaitForHasNodesTimeout)
		if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(waitTimeout).After(time.Now()) {
			logger.Info("Placement Group has node(s) attached, re-queuing deletion to wait for detachment")
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(reconciler.DefaultPGControllerReconcilerDelay)}, nil
		}

		// Wait timeout exceeded, fail the deletion
		logger.Error(nil, "Placement Group deletion timed out waiting for node(s) to detach", "timeout", waitTimeout)
		pgScope.LinodePlacementGroup.SetCondition(metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  clusterv1.NotDeletingReason,
			Message: fmt.Sprintf("skipped due to %d node(s) still attached after %s timeout", len(pg.Members), waitTimeout),
		})
		r.Recorder.Eventf(
			pgScope.LinodePlacementGroup,
			nil,
			corev1.EventTypeWarning,
			clusterv1.NotDeletingReason,
			"DeletePlacementGroup",
			"Will not delete Placement Group %d with %d node(s) attached after %s timeout",
			pg.ID,
			len(pg.Members),
			waitTimeout,
		)
		return ctrl.Result{}, errors.New("will not delete Placement Group with node(s) attached")
	}

	// PG exists and is empty, proceed with deletion
	logger.Info("Placement Group is empty, attempting API deletion")
	err = pgScope.LinodeClient.DeletePlacementGroup(ctx, pgID)
	if err != nil {
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) == nil {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to delete Placement Group via API")
		if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
			// Need this requeue incase for some reason pg is not empty even though all the nodes are deleted.
			// This should give enough time for PG to get updated on the backend and we can delete it next time.
			logger.Info("Re-queuing Placement Group deletion due to API delete error")
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(reconciler.DefaultPGControllerReconcilerDelay)}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to delete placement group %d after timeout: %w", pgID, err)
	}

	return ctrl.Result{}, nil
}

func (r *LinodePlacementGroupReconciler) HandleFailureCondition(pgScope *scope.PlacementGroupScope, err error) {
	r.Recorder.Eventf(pgScope.LinodePlacementGroup, nil, corev1.EventTypeWarning, string(pgScope.ErrorType), "Reconcile", err.Error())

	pgScope.LinodePlacementGroup.Status.FailureReason = &pgScope.ErrorType
	pgScope.LinodePlacementGroup.Status.FailureMessage = new(err.Error())
	pgScope.LinodePlacementGroup.SetCondition(metav1.Condition{
		Type:    clusterv1.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  string(pgScope.ErrorType),
		Message: err.Error(),
	})
}
