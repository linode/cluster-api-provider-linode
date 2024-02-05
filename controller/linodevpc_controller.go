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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
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

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

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

	linodeVPC := &infrav1.LinodeVPC{}
	if err := r.Client.Get(ctx, req.NamespacedName, linodeVPC); err != nil {
		log.Error(err, "Failed to fetch LinodeVPC")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	vpcScope, err := scope.NewVPCScope(
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

	return r.reconcile(ctx, vpcScope, log)
}

func (r *LinodeVPCReconciler) reconcile(
	ctx context.Context,
	vpcScope *scope.VPCScope,
	logger logr.Logger,
) (res ctrl.Result, err error) {
	res = ctrl.Result{}

	vpcScope.LinodeVPC.Status.Ready = false
	vpcScope.LinodeVPC.Status.FailureReason = nil
	vpcScope.LinodeVPC.Status.FailureMessage = util.Pointer("")

	failureReason := infrav1.VPCStatusError("UnknownError")
	defer func() {
		if err != nil {
			vpcScope.LinodeVPC.Status.FailureReason = util.Pointer(failureReason)
			vpcScope.LinodeVPC.Status.FailureMessage = util.Pointer(err.Error())

			conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, err.Error())

			r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		if patchErr := vpcScope.PatchHelper.Patch(ctx, vpcScope.LinodeVPC); patchErr != nil && utilerrors.FilterOut(patchErr) != nil {
			logger.Error(patchErr, "failed to patch LinodeVPC")

			err = errors.Join(err, patchErr)
		}
	}()

	// Delete
	if !vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = infrav1.DeleteVPCError

		res, err = r.reconcileDelete(ctx, logger, vpcScope)

		return
	}

	controllerutil.AddFinalizer(vpcScope.LinodeVPC, infrav1.GroupVersion.String())

	// Update
	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		failureReason = infrav1.UpdateVPCError

		logger = logger.WithValues("vpcID", *vpcScope.LinodeVPC.Spec.VPCID)

		err = r.reconcileUpdate(ctx, logger, vpcScope)

		return
	}

	// Create
	failureReason = infrav1.CreateVPCError

	err = r.reconcileCreate(ctx, vpcScope, logger)

	return
}

func (r *LinodeVPCReconciler) reconcileCreate(ctx context.Context, vpcScope *scope.VPCScope, logger logr.Logger) error {
	logger.Info("creating vpc")

	if err := r.reconcileVPC(ctx, vpcScope, logger); err != nil {
		logger.Error(err, "Failed to create VPC")

		conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, string(infrav1.CreateVPCError), clusterv1.ConditionSeverityError, err.Error())

		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(infrav1.CreateVPCError), err.Error())

		return err
	}
	vpcScope.LinodeVPC.Status.Ready = true

	return nil
}

func (r *LinodeVPCReconciler) reconcileUpdate(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	logger.Info("updating vpc")

	// Update is not supported at the moment
	if err := r.reconcileVPC(ctx, vpcScope, logger); err != nil {
		logger.Error(err, "Failed to update VPC")

		conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, string(infrav1.UpdateVPCError), clusterv1.ConditionSeverityError, err.Error())

		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(infrav1.UpdateVPCError), err.Error())

		return err
	}
	vpcScope.LinodeVPC.Status.Ready = true

	return nil
}

//nolint:nestif // As simple as possible.
func (r *LinodeVPCReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) (reconcile.Result, error) {
	logger.Info("deleting VPC")

	res := ctrl.Result{}

	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		vpc, err := vpcScope.LinodeClient.GetVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID)
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to fetch VPC")

			return res, err
		} else if vpc == nil {
			return res, errors.New("failed to fetch VPC")
		}

		if vpc != nil {
			for i := range vpc.Subnets {
				if len(vpc.Subnets[i].Linodes) == 0 {
					continue
				}

				if vpc.Updated.Add(reconciler.DefaultVPCControllerWaitForHasNodesTimeout).After(time.Now()) {
					logger.Info("VPC has node(s) attached, re-queuing reconciliation")

					return ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerWaitForHasNodesDelay}, nil
				}

				logger.Info("VPC has node(s) attached for long, skipping reconciliation")

				conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityInfo, "skipped due to node(s) attached")

				return res, nil
			}

			err = vpcScope.LinodeClient.DeleteVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID)
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
				logger.Error(err, "Failed to delete VPC")

				return res, err
			}
		}
	} else {
		logger.Info("VPC ID is missing, nothing to do")
	}

	conditions.MarkFalse(vpcScope.LinodeVPC, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "VPC deleted")

	r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeNormal, clusterv1.DeletedReason, "VPC has cleaned up")

	vpcScope.LinodeVPC.Spec.VPCID = nil
	controllerutil.RemoveFinalizer(vpcScope.LinodeVPC, infrav1.GroupVersion.String())

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeVPCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LinodeVPC{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue)).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}
