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
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

var (
	errVPCHasNodesAttached        = errors.New("vpc still has node(s) attached")
	errVPCHasNodesAttachedTimeout = errors.New("vpc still has node(s) attached past the expected duration")
)

// LinodeVPCReconciler reconciles a LinodeVPC object
type LinodeVPCReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	Scheme           *runtime.Scheme
	ReconcileTimeout time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs/finalizers,verbs=update

// +kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the VPC closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LinodeVPC object against the actual VPC state, and then
// perform operations to make the VPC state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *LinodeVPCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx).WithName("LinodeVPCReconciler").WithValues("name", req.NamespacedName.String())

	linodeVPC := &infrav1alpha1.LinodeVPC{}
	if err := r.Client.Get(ctx, req.NamespacedName, linodeVPC); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch LinodeVPC")
		}

		return ctrl.Result{}, err
	}

	vpcScope, err := scope.NewVPCScope(
		ctx,
		r.LinodeApiKey,
		scope.VPCScopeParams{
			Client:    r.Client,
			LinodeVPC: linodeVPC,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create VPC scope")

		return ctrl.Result{}, fmt.Errorf("failed to create VPC scope: %w", err)
	}

	return r.reconcile(ctx, log, vpcScope)
}

func (r *LinodeVPCReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	vpcScope *scope.VPCScope,
) (res ctrl.Result, err error) {
	res = ctrl.Result{}

	vpcScope.LinodeVPC.Status.Ready = false
	vpcScope.LinodeVPC.Status.FailureReason = nil
	vpcScope.LinodeVPC.Status.FailureMessage = util.Pointer("")

	failureReason := infrav1alpha1.VPCStatusError("UnknownError")
	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if err != nil {
			vpcScope.LinodeVPC.Status.FailureReason = util.Pointer(failureReason)
			vpcScope.LinodeVPC.Status.FailureMessage = util.Pointer(err.Error())

			conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, err.Error())

			r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist
		// any LinodeVPC changes. This ignores any resource not found errors
		// when reconciling deletions.
		if patchErr := vpcScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch LinodeVPC")

			err = errors.Join(err, patchErr)
		}
	}()

	// Delete
	if !vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = infrav1alpha1.DeleteVPCError

		err = r.reconcileDelete(ctx, logger, vpcScope)
		if err != nil {
			switch {
			case errors.Is(err, errVPCHasNodesAttachedTimeout):
				logger.Info("VPC has node(s) attached for long, skipping reconciliation")

			case errors.Is(err, errVPCHasNodesAttached):
				logger.Info("VPC has node(s) attached, re-queuing reconciliation")

				res = ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerWaitForHasNodesDelay}
				err = nil

			default:
				if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
					logger.Info("re-queuing VPC deletion")

					res = ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}
					err = nil
				}
			}
		}

		return
	}

	// Add the finalizer if not already there
	err = vpcScope.AddFinalizer(ctx)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")

		return
	}

	// Update
	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		failureReason = infrav1alpha1.UpdateVPCError

		logger = logger.WithValues("vpcID", *vpcScope.LinodeVPC.Spec.VPCID)

		err = r.reconcileUpdate(ctx, logger, vpcScope)
		if err != nil && !reconciler.HasConditionSeverity(vpcScope.LinodeVPC, clusterv1.ReadyCondition, clusterv1.ConditionSeverityError) {
			logger.Info("re-queuing VPC update")

			res = ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}
			err = nil
		}

		return
	}

	// Create
	failureReason = infrav1alpha1.CreateVPCError

	err = r.reconcileCreate(ctx, logger, vpcScope)
	if err != nil && !reconciler.HasConditionSeverity(vpcScope.LinodeVPC, clusterv1.ReadyCondition, clusterv1.ConditionSeverityError) {
		logger.Info("re-queuing VPC creation")

		res = ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}
		err = nil
	}

	return
}

func (r *LinodeVPCReconciler) reconcileCreate(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	logger.Info("creating vpc")

	if err := r.reconcileVPC(ctx, vpcScope, logger); err != nil {
		logger.Error(err, "Failed to create VPC")

		reconciler.RecordDecayingCondition(vpcScope.LinodeVPC, clusterv1.ReadyCondition, string(infrav1alpha1.CreateVPCError), err.Error(), reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout))

		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(infrav1alpha1.CreateVPCError), err.Error())

		return err
	}
	vpcScope.LinodeVPC.Status.Ready = true

	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeNormal, "Created", fmt.Sprintf("Created VPC %d", *vpcScope.LinodeVPC.Spec.VPCID))
	}

	return nil
}

//nolint:unused // Update is not supported at the moment
func (r *LinodeVPCReconciler) reconcileUpdate(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	logger.Info("updating vpc")

	if err := r.reconcileVPC(ctx, vpcScope, logger); err != nil {
		logger.Error(err, "Failed to update VPC")

		reconciler.RecordDecayingCondition(vpcScope.LinodeVPC, clusterv1.ReadyCondition, string(infrav1alpha1.UpdateVPCError), err.Error(), reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout))

		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(infrav1alpha1.UpdateVPCError), err.Error())

		return err
	}
	vpcScope.LinodeVPC.Status.Ready = true

	return nil
}

//nolint:nestif // As simple as possible.
func (r *LinodeVPCReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	logger.Info("deleting VPC")

	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		vpc, err := vpcScope.LinodeClient.GetVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID)
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to fetch VPC")

			return err
		}

		if vpc != nil {
			for i := range vpc.Subnets {
				if len(vpc.Subnets[i].Linodes) == 0 {
					continue
				}

				if vpc.Updated.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerWaitForHasNodesTimeout)).After(time.Now()) {
					return errVPCHasNodesAttached
				}

				conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityError, "skipped due to node(s) attached")

				return errVPCHasNodesAttachedTimeout
			}

			err = vpcScope.LinodeClient.DeleteVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID)
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
				logger.Error(err, "Failed to delete VPC")

				return err
			}
		}
	} else {
		logger.Info("VPC ID is missing, nothing to do")
	}

	conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "VPC deleted")

	r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeNormal, clusterv1.DeletedReason, "VPC has cleaned up")

	vpcScope.LinodeVPC.Spec.VPCID = nil
	controllerutil.RemoveFinalizer(vpcScope.LinodeVPC, infrav1alpha1.GroupVersion.String())

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeVPCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeVPC{}).
		WithEventFilter(
			predicate.And(
				// Filter for objects with a specific WatchLabel.
				predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
				// Do not reconcile the Delete events generated by the
				// controller itself.
				predicate.Funcs{
					DeleteFunc: func(e event.DeleteEvent) bool { return false },
				},
			)).Build(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	linodeVPCMapper, err := kutil.ClusterToTypedObjectsMapper(r.Client, &infrav1alpha1.LinodeVPCList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeVPCs: %w", err)
	}

	return controller.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(linodeVPCMapper),
		predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger()),
	)
}
