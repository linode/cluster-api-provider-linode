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
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/paused"
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
	Recorder           record.EventRecorder
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
			// If we're deleting and cluster isn't found, that's okay
			if !linodeplacementgroup.DeletionTimestamp.IsZero() && apierrors.IsNotFound(err) {
				log.Info("Cluster not found but LinodePlacementGroup is being deleted, continuing with deletion")
			} else {
				log.Error(err, "failed to fetch cluster from metadata")
				return ctrl.Result{}, err
			}
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
		isPaused, _, err := paused.EnsurePausedCondition(ctx, pgScope.Client, pgScope.Cluster, pgScope.LinodePlacementGroup)
		if err != nil {
			return ctrl.Result{}, err
		}
		if isPaused {
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

			conditions.Set(pgScope.LinodePlacementGroup, metav1.Condition{
				Type:    string(clusterv1.ReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  string(failureReason),
				Message: err.Error(),
			})

			r.Recorder.Event(pgScope.LinodePlacementGroup, corev1.EventTypeWarning, string(failureReason), err.Error())
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
	if err != nil && !reconciler.HasStaleCondition(pgScope.LinodePlacementGroup, string(clusterv1.ReadyCondition), reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)) {
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
		conditions.Set(pgScope.LinodePlacementGroup, metav1.Condition{
			Type:    string(clusterv1.ReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  string(infrav1alpha2.CreatePlacementGroupError),
			Message: err.Error(),
		})
		r.Recorder.Event(pgScope.LinodePlacementGroup, corev1.EventTypeWarning, string(infrav1alpha2.CreatePlacementGroupError), err.Error())

		return err
	}

	if err := r.reconcilePlacementGroup(ctx, pgScope, logger); err != nil {
		logger.Error(err, "Failed to create Placement Group")
		conditions.Set(pgScope.LinodePlacementGroup, metav1.Condition{
			Type:    string(clusterv1.ReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  string(infrav1alpha2.CreatePlacementGroupError),
			Message: err.Error(),
		})
		r.Recorder.Event(pgScope.LinodePlacementGroup, corev1.EventTypeWarning, string(infrav1alpha2.CreatePlacementGroupError), err.Error())

		return err
	}
	pgScope.LinodePlacementGroup.Status.Ready = true

	if pgScope.LinodePlacementGroup.Spec.PGID != nil {
		r.Recorder.Event(pgScope.LinodePlacementGroup, corev1.EventTypeNormal, "Created", fmt.Sprintf("Created Placement Group %d", *pgScope.LinodePlacementGroup.Spec.PGID))
	}

	return nil
}

//nolint:nestif,gocognit // As simple as possible.
func (r *LinodePlacementGroupReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, pgScope *scope.PlacementGroupScope) (ctrl.Result, error) {
	logger.Info("deleting Placement Group")

	if pgScope.LinodePlacementGroup.Spec.PGID != nil {
		pg, err := pgScope.LinodeClient.GetPlacementGroup(ctx, *pgScope.LinodePlacementGroup.Spec.PGID)
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to fetch Placement Group")

			if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
				logger.Info("re-queuing Placement Group deletion")

				return ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}, nil
			}

			return ctrl.Result{}, err
		}

		if pg != nil {
			if len(pg.Members) > 0 {
				logger.Info("Placement Group still has node(s) attached, unassigning them")
				members := make([]int, 0, len(pg.Members))
				for _, member := range pg.Members {
					members = append(members, member.LinodeID)
				}

				_, err := pgScope.LinodeClient.UnassignPlacementGroupLinodes(ctx, pg.ID, linodego.PlacementGroupUnAssignOptions{
					Linodes: members,
				})

				if err != nil {
					return ctrl.Result{}, fmt.Errorf("unassigning linodes from pg %d: %w", pg.ID, err)
				}
			}

			err = pgScope.LinodeClient.DeletePlacementGroup(ctx, *pgScope.LinodePlacementGroup.Spec.PGID)
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
				logger.Error(err, "Failed to delete Placement Group")

				if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
					logger.Info("re-queuing Placement Group deletion")

					return ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}, nil
				}

				return ctrl.Result{}, err
			}
		}
	} else {
		logger.Info("Placement Group ID is missing, nothing to do")
	}

	conditions.Set(pgScope.LinodePlacementGroup, metav1.Condition{
		Type:    string(clusterv1.ReadyCondition),
		Status:  metav1.ConditionFalse,
		Reason:  string(clusterv1.DeletedReason),
		Message: "Placement Group deleted",
	})

	r.Recorder.Event(pgScope.LinodePlacementGroup, corev1.EventTypeNormal, clusterv1.DeletedReason, "Placement Group has cleaned up")

	pgScope.LinodePlacementGroup.Spec.PGID = nil

	if err := pgScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to update credentials secret")

		if pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("re-queuing Placement Group deletion")

			return ctrl.Result{RequeueAfter: reconciler.DefaultPGControllerReconcilerDelay}, nil
		}

		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(pgScope.LinodePlacementGroup, infrav1alpha2.PlacementGroupFinalizer)
	// TODO: remove this check and removal later
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
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureReady(mgr.GetScheme(), mgr.GetLogger())),
		).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodePlacementGroupReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
