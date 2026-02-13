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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	wrappedruntimeclient "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimeclient"
	wrappedruntimereconciler "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimereconciler"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// LinodePlacementGroupReconciler reconciles a LinodePlacementGroup object
type LinodePlacementGroupReconciler struct {
	client.Client
	Recorder           events.EventRecorder
	LinodeClientConfig scope.ClientConfig
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups/finalizers,verbs=update

// +kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the Placement Group closer to the desired state.
//

func (r *LinodePlacementGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx).WithName("LinodePlacementGroupReconciler").WithValues("name", req.String())

	linodeplacementgroup := &infrav1alpha2.LinodePlacementGroup{}
	if err := r.TracedClient().Get(ctx, req.NamespacedName, linodeplacementgroup); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch LinodePlacementGroup")
		}

		return ctrl.Result{}, err
	}
	var cluster *clusterv1.Cluster
	var err error
	if _, ok := linodeplacementgroup.Labels[clusterv1.ClusterNameLabel]; ok {
		cluster, err = kutil.GetClusterFromMetadata(ctx, r.TracedClient(), linodeplacementgroup.ObjectMeta)
		if err != nil {
			if client.IgnoreNotFound(err) != nil {
				log.Error(err, "failed to fetch cluster from metadata")
				return ctrl.Result{}, err
			}
			log.Info("Cluster not found but LinodePlacementGroup is being deleted, continuing with deletion")
		}

		// Set ownerRef to LinodeCluster
		// It will handle the case where the cluster is not found
		if err := util.SetOwnerReferenceToLinodeCluster(ctx, r.TracedClient(), cluster, linodeplacementgroup, r.Scheme()); err != nil {
			log.Error(err, "Failed to set owner reference to LinodeCluster")
			return ctrl.Result{}, err
		}
	}

	pgScope, err := scope.NewPlacementGroupScope(
		ctx,
		r.LinodeClientConfig,
		scope.PlacementGroupScopeParams{
			Client:               r.TracedClient(),
			LinodePlacementGroup: linodeplacementgroup,
			Cluster:              cluster,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create Placement Group scope")

		return ctrl.Result{}, fmt.Errorf("failed to create Placement Group scope: %w", err)
	}

	// Only check pause if not deleting or if cluster still exists
	if linodeplacementgroup.DeletionTimestamp.IsZero() || cluster != nil {
		if pgScope.LinodePlacementGroup.IsPaused() {
			log.Info("linodeplacementgroup or linked cluster is paused, skipping reconciliation")
			return ctrl.Result{}, nil
		}
	}

	return r.reconcile(ctx, log, pgScope)
}

func (r *LinodePlacementGroupReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	pgScope *scope.PlacementGroupScope,
) (res ctrl.Result, err error) {
	res = ctrl.Result{}

	pgScope.LinodePlacementGroup.Status.Ready = false
	pgScope.LinodePlacementGroup.Status.FailureReason = nil
	pgScope.LinodePlacementGroup.Status.FailureMessage = util.Pointer("")

	failureReason := infrav1alpha2.LinodePlacementGroupStatusError("UnknownError")
	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if err != nil {
			pgScope.LinodePlacementGroup.Status.FailureReason = util.Pointer(failureReason)
			pgScope.LinodePlacementGroup.Status.FailureMessage = util.Pointer(err.Error())

			pgScope.LinodePlacementGroup.SetCondition(metav1.Condition{
				Type:    clusterv1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  string(failureReason),
				Message: err.Error(),
			})

			r.Recorder.Eventf(
				pgScope.LinodePlacementGroup,
				nil,
				corev1.EventTypeWarning,
				string(failureReason),
				"Reconcile",
				err.Error(),
			)
		}

		// Always close the scope when exiting this function so we can persist any LinodePlacement Group changes.
		// This ignores any resource not found errors when reconciling deletions.
		if patchErr := pgScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch LinodePlacementGroup")

			err = errors.Join(err, patchErr)
		}
	}()

	// Override the controller credentials with ones from the Placement Groups's Secret reference (if supplied).
	if err := pgScope.SetCredentialRefTokenForLinodeClients(ctx); err != nil {
		logger.Error(err, "failed to update linode client token from Credential Ref")
		return res, err
	}

	// Delete
	if !pgScope.LinodePlacementGroup.DeletionTimestamp.IsZero() {
		failureReason = infrav1alpha2.DeletePlacementGroupError

		res, err = r.reconcileDelete(ctx, logger, pgScope)

		return
	}

	// Add the finalizer if not already there
	err = pgScope.AddFinalizer(ctx)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")

		return
	}

	// Update
	if pgScope.LinodePlacementGroup.Spec.PGID != nil {
		logger = logger.WithValues("pgID", *pgScope.LinodePlacementGroup.Spec.PGID)

		logger.Info("updating placement group")

		// Update is essentially a no-op as everything is immutable, just set it to ready and move on
		pgScope.LinodePlacementGroup.Status.Ready = true

		return
	}

	// Create
	failureReason = infrav1alpha2.CreatePlacementGroupError

	err = r.reconcileCreate(ctx, logger, pgScope)
	if err != nil && !reconciler.HasStaleCondition(pgScope.LinodePlacementGroup.GetCondition(string(clusterv1.ReadyCondition)),
		reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)) {
		logger.Info("re-queuing Placement Group creation")

		res = ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}
		err = nil
	}

	return
}

//nolint:dupl // same as VPC - future generics candidate.
func (r *LinodePlacementGroupReconciler) reconcileCreate(ctx context.Context, logger logr.Logger, pgScope *scope.PlacementGroupScope) error {
	logger.Info("creating placement group")

	if err := pgScope.AddCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to update credentials secret")
		pgScope.LinodePlacementGroup.SetCondition(metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  string(infrav1alpha2.CreatePlacementGroupError),
			Message: err.Error(),
		})

		return err
	}

	if err := r.reconcilePlacementGroup(ctx, pgScope, logger); err != nil {
		logger.Error(err, "Failed to create Placement Group")
		pgScope.LinodePlacementGroup.SetCondition(metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  string(infrav1alpha2.CreatePlacementGroupError),
			Message: err.Error(),
		})
		r.Recorder.Eventf(
			pgScope.LinodePlacementGroup,
			nil,
			corev1.EventTypeWarning,
			string(infrav1alpha2.CreatePlacementGroupError),
			"CreatePlacementGroup",
			err.Error(),
		)

		return err
	}
	pgScope.LinodePlacementGroup.Status.Ready = true

	return nil
}

//nolint:nestif,gocognit // As simple as possible.
func (r *LinodePlacementGroupReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, pgScope *scope.PlacementGroupScope) (ctrl.Result, error) {
	logger.Info("deleting Placement Group")

	if pgScope.LinodePlacementGroup.Spec.PGID != nil {
		pgID := *pgScope.LinodePlacementGroup.Spec.PGID
		logger = logger.WithValues("pgID", pgID)

		pg, err := pgScope.LinodeClient.GetPlacementGroup(ctx, pgID)
		if err != nil {
			// Handle 404 Not Found - treat as deleted
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) == nil {
				logger.Info("Placement Group not found via API, assuming already deleted")
				// Skip to finalizer removal outside this block
			} else {
				logger.Error(err, "Failed to fetch Placement Group from API")
				if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
					logger.Info("Re-queuing Placement Group deletion due to fetch error")
					return ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}, nil
				}
				return ctrl.Result{}, fmt.Errorf("failed to fetch placement group %d after timeout: %w", pgID, err)
			}
		}

		if pg != nil {
			if len(pg.Members) > 0 {
				logger.Info("Placement Group still has node(s) attached", "count", len(pg.Members))
				waitTimeout := reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerWaitForHasNodesTimeout)
				if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(waitTimeout).After(time.Now()) {
					logger.Info("Placement Group has node(s) attached, re-queuing deletion to wait for detachment", "requeueAfter", reconciler.DefaultPGControllerReconcilerDelay)
					return ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}, nil
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
				// Handle 404 Not Found during delete - treat as deleted
				if util.IgnoreLinodeAPIError(err, http.StatusNotFound) == nil {
					logger.Info("Placement Group already deleted (API 404 on delete call)")
					// Skip to finalizer removal outside this block
				} else {
					logger.Error(err, "Failed to delete Placement Group via API")
					if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
						// Need this requeue incase for some reason pg is not empty even though all the nodes are deleted.
						// This should give enough time for PG to get updated on the backend and we can delete it next time.
						logger.Info("Re-queuing Placement Group deletion due to API delete error")
						return ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}, nil
					}
					return ctrl.Result{}, fmt.Errorf("failed to delete placement group %d after timeout: %w", pgID, err)
				}
			} else {
				logger.Info("Placement Group deleted successfully via API")
			}
		}
	} else {
		logger.Info("Placement Group ID is missing, nothing to do")
	}

	pgScope.LinodePlacementGroup.SetCondition(metav1.Condition{
		Type:    clusterv1.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  clusterv1.DeletionCompletedReason,
		Message: "Placement Group deleted",
	})

	pgScope.LinodePlacementGroup.Spec.PGID = nil

	if err := pgScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to remove credentials secret finalizer")
		if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("Re-queuing Placement Group deletion due to credential finalizer removal error")
			return ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to remove credential finalizer after timeout: %w", err)
	}

	controllerutil.RemoveFinalizer(pgScope.LinodePlacementGroup, infrav1alpha2.PlacementGroupFinalizer)

	// Restore original main finalizer removal logic
	if controllerutil.ContainsFinalizer(pgScope.LinodePlacementGroup, infrav1alpha2.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(pgScope.LinodePlacementGroup, infrav1alpha2.GroupVersion.String())
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl // this is same as Placement Group, worth making generic later.
func (r *LinodePlacementGroupReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	linodePlacementGroupMapper, err := kutil.ClusterToTypedObjectsMapper(
		r.TracedClient(),
		&infrav1alpha2.LinodePlacementGroupList{},
		mgr.GetScheme(),
	)
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodePlacementGroups: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodePlacementGroup{}).
		WithOptions(options).
		WithEventFilter(predicate.And(
			predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), mgr.GetLogger(), r.WatchFilterValue),
			predicate.GenerationChangedPredicate{},
			predicate.Funcs{UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject, okOld := e.ObjectOld.(*infrav1alpha2.LinodePlacementGroup)
				newObject, okNew := e.ObjectNew.(*infrav1alpha2.LinodePlacementGroup)
				if okOld && okNew && oldObject.Spec.PGID == nil && newObject.Spec.PGID != nil {
					// We just created the PG, don't enqueue and update
					return false
				}
				return true
			}},
		)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodePlacementGroupMapper),
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureProvisioned(mgr.GetScheme(), mgr.GetLogger())),
		).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodePlacementGroupReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
