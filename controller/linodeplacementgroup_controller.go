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
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
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
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	Scheme           *runtime.Scheme
	ReconcileTimeout time.Duration
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

	log := ctrl.LoggerFrom(ctx).WithName("LinodePlacementGroupReconciler").WithValues("name", req.NamespacedName.String())

	linodeplacementgroup := &infrav1alpha2.LinodePlacementGroup{}
	if err := r.TracedClient().Get(ctx, req.NamespacedName, linodeplacementgroup); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch LinodePlacementGroup")
		}

		return ctrl.Result{}, err
	}

	pgScope, err := scope.NewPlacementGroupScope(
		ctx,
		r.LinodeApiKey,
		scope.PlacementGroupScopeParams{
			Client:               r.TracedClient(),
			LinodePlacementGroup: linodeplacementgroup,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create Placement Group scope")

		return ctrl.Result{}, fmt.Errorf("failed to create Placement Group scope: %w", err)
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

			conditions.MarkFalse(pgScope.LinodePlacementGroup, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, err.Error())

			r.Recorder.Event(pgScope.LinodePlacementGroup, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist any LinodePlacement Group changes.
		// This ignores any resource not found errors when reconciling deletions.
		if patchErr := pgScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch LinodePlacementGroup")

			err = errors.Join(err, patchErr)
		}
	}()

	// Delete
	if !pgScope.LinodePlacementGroup.ObjectMeta.DeletionTimestamp.IsZero() {
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
	if err != nil && !reconciler.HasConditionSeverity(pgScope.LinodePlacementGroup, clusterv1.ReadyCondition, clusterv1.ConditionSeverityError) {
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

		reconciler.RecordDecayingCondition(pgScope.LinodePlacementGroup, clusterv1.ReadyCondition, string(infrav1alpha2.CreatePlacementGroupError), err.Error(), reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout))

		r.Recorder.Event(pgScope.LinodePlacementGroup, corev1.EventTypeWarning, string(infrav1alpha2.CreatePlacementGroupError), err.Error())

		return err
	}

	if err := r.reconcilePlacementGroup(ctx, pgScope, logger); err != nil {
		logger.Error(err, "Failed to create Placement Group")

		reconciler.RecordDecayingCondition(pgScope.LinodePlacementGroup, clusterv1.ReadyCondition, string(infrav1alpha2.CreatePlacementGroupError), err.Error(), reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultPGControllerReconcileTimeout))

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

	conditions.MarkFalse(pgScope.LinodePlacementGroup, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "Placement Group deleted")

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
		WithEventFilter(
			predicate.And(
				// Filter for objects with a specific WatchLabel.
				predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
				// Do not reconcile the Delete events generated by the
				// controller itself.
				predicate.Funcs{
					DeleteFunc: func(e event.DeleteEvent) bool { return false },
				},
			)).Watches(
		&clusterv1.Cluster{},
		handler.EnqueueRequestsFromMapFunc(linodePlacementGroupMapper),
		builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
	).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodePlacementGroupReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
