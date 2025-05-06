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
	"k8s.io/apimachinery/pkg/runtime"
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

// LinodeVPCReconciler reconciles a LinodeVPC object
type LinodeVPCReconciler struct {
	client.Client
	Recorder           record.EventRecorder
	LinodeClientConfig scope.ClientConfig
	WatchFilterValue   string
	Scheme             *runtime.Scheme
	ReconcileTimeout   time.Duration
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
//

func (r *LinodeVPCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx).WithName("LinodeVPCReconciler").WithValues("name", req.NamespacedName.String())
	linodeVPC := &infrav1alpha2.LinodeVPC{}
	if err := r.TracedClient().Get(ctx, req.NamespacedName, linodeVPC); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch LinodeVPC")
		}
		return ctrl.Result{}, err
	}

	var cluster *clusterv1.Cluster
	var err error
	if _, ok := linodeVPC.ObjectMeta.Labels[clusterv1.ClusterNameLabel]; ok {
		cluster, err = kutil.GetClusterFromMetadata(ctx, r.TracedClient(), linodeVPC.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to fetch cluster from metadata")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		if err := util.SetOwnerReferenceToLinodeCluster(ctx, r.TracedClient(), cluster, linodeVPC, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference to LinodeCluster")
			return ctrl.Result{}, err
		}
	}

	vpcScope, err := scope.NewVPCScope(
		ctx,
		r.LinodeClientConfig,
		scope.VPCScopeParams{
			Client:    r.TracedClient(),
			LinodeVPC: linodeVPC,
			Cluster:   cluster,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create VPC scope")
		return ctrl.Result{}, fmt.Errorf("failed to create VPC scope: %w", err)
	}

	// Only check pause if not deleting or if cluster still exists
	if linodeVPC.DeletionTimestamp.IsZero() || cluster != nil {
		isPaused, _, err := paused.EnsurePausedCondition(ctx, vpcScope.Client, vpcScope.Cluster, vpcScope.LinodeVPC)
		if err != nil {
			return ctrl.Result{}, err
		}
		if isPaused {
			log.Info("linodeVPC or linked cluster is paused, skipping reconciliation")
			return ctrl.Result{}, nil
		}
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

	failureReason := infrav1alpha2.VPCStatusError("UnknownError")
	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if err != nil {
			vpcScope.LinodeVPC.Status.FailureReason = util.Pointer(failureReason)
			vpcScope.LinodeVPC.Status.FailureMessage = util.Pointer(err.Error())

			conditions.Set(vpcScope.LinodeVPC, metav1.Condition{
				Type:    string(clusterv1.ReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  string(failureReason),
				Message: err.Error(),
			})

			r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist any LinodeVPC changes.
		// This ignores any resource not found errors when reconciling deletions.
		if patchErr := vpcScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch LinodeVPC")

			err = errors.Join(err, patchErr)
		}
	}()

	// Override the controller credentials with ones from the VPC's Secret reference (if supplied).
	if err := vpcScope.SetCredentialRefTokenForLinodeClients(ctx); err != nil {
		logger.Error(err, "failed to update linode client token from Credential Ref")
		return res, err
	}

	// Delete
	if !vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = infrav1alpha2.DeleteVPCError

		res, err = r.reconcileDelete(ctx, logger, vpcScope)

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
		failureReason = infrav1alpha2.UpdateVPCError

		logger = logger.WithValues("vpcID", *vpcScope.LinodeVPC.Spec.VPCID)

		err = r.reconcileUpdate(ctx, logger, vpcScope)
		if err != nil && !reconciler.HasStaleCondition(vpcScope.LinodeVPC, string(clusterv1.ReadyCondition),
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)) {
			logger.Info("re-queuing VPC update")

			res = ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}
			err = nil
		}

		return
	}

	// Create
	failureReason = infrav1alpha2.CreateVPCError

	err = r.reconcileCreate(ctx, logger, vpcScope)
	if err != nil && !reconciler.HasStaleCondition(vpcScope.LinodeVPC, string(clusterv1.ReadyCondition),
		reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)) {
		logger.Info("re-queuing VPC creation")

		res = ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}
		err = nil
	}

	return
}

//nolint:dupl // same as Placement Group - future generics candidate.
func (r *LinodeVPCReconciler) reconcileCreate(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	logger.Info("creating vpc")

	if err := vpcScope.AddCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to update credentials secret")
		conditions.Set(vpcScope.LinodeVPC, metav1.Condition{
			Type:    string(clusterv1.ReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  string(infrav1alpha2.CreateVPCError),
			Message: err.Error(),
		})
		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(infrav1alpha2.CreateVPCError), err.Error())

		return err
	}

	if err := reconcileVPC(ctx, vpcScope, logger); err != nil {
		logger.Error(err, "Failed to create VPC")
		conditions.Set(vpcScope.LinodeVPC, metav1.Condition{
			Type:    string(clusterv1.ReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  string(infrav1alpha2.CreateVPCError),
			Message: err.Error(),
		})
		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(infrav1alpha2.CreateVPCError), err.Error())

		return err
	}
	vpcScope.LinodeVPC.Status.Ready = true

	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeNormal, "Created", fmt.Sprintf("Created VPC %d", *vpcScope.LinodeVPC.Spec.VPCID))
	}

	return nil
}

func (r *LinodeVPCReconciler) reconcileUpdate(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	logger.Info("updating vpc")

	if err := reconcileVPC(ctx, vpcScope, logger); err != nil {
		logger.Error(err, "Failed to update VPC")
		conditions.Set(vpcScope.LinodeVPC, metav1.Condition{
			Type:    string(clusterv1.ReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  string(infrav1alpha2.UpdateVPCError),
			Message: err.Error(),
		})
		r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeWarning, string(infrav1alpha2.UpdateVPCError), err.Error())

		return err
	}
	vpcScope.LinodeVPC.Status.Ready = true

	return nil
}

//nolint:nestif,gocognit // As simple as possible.
func (r *LinodeVPCReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) (ctrl.Result, error) {
	logger.Info("deleting VPC")

	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		vpc, err := vpcScope.LinodeClient.GetVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID)
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to fetch VPC")

			if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
				logger.Info("re-queuing VPC deletion")

				return ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}, nil
			}

			return ctrl.Result{}, err
		}

		if vpc != nil {
			for i := range vpc.Subnets {
				if len(vpc.Subnets[i].Linodes) == 0 {
					continue
				}

				logger.Info("VPC subnets still has node(s) attached")

				if vpc.Updated.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerWaitForHasNodesTimeout)).After(time.Now()) {
					logger.Info("VPC has node(s) attached, re-queuing VPC deletion")

					return ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}, nil
				}

				conditions.Set(vpcScope.LinodeVPC, metav1.Condition{
					Type:    string(clusterv1.ReadyCondition),
					Status:  metav1.ConditionFalse,
					Reason:  string(clusterv1.DeletionFailedReason),
					Message: "skipped due to node(s) attached",
				})

				return ctrl.Result{}, errors.New("will not delete VPC with node(s) attached")
			}

			err = vpcScope.LinodeClient.DeleteVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID)
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
				logger.Error(err, "Failed to delete VPC")

				if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
					logger.Info("re-queuing VPC deletion")

					return ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}, nil
				}

				return ctrl.Result{}, err
			}
		}
	} else {
		logger.Info("VPC ID is missing, nothing to do")
	}

	conditions.Set(vpcScope.LinodeVPC, metav1.Condition{
		Type:    string(clusterv1.ReadyCondition),
		Status:  metav1.ConditionFalse,
		Reason:  string(clusterv1.DeletedReason),
		Message: "VPC deleted",
	})

	r.Recorder.Event(vpcScope.LinodeVPC, corev1.EventTypeNormal, clusterv1.DeletedReason, "VPC has cleaned up")

	vpcScope.LinodeVPC.Spec.VPCID = nil

	if err := vpcScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to update credentials secret")

		if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("re-queuing VPC deletion")

			return ctrl.Result{RequeueAfter: reconciler.DefaultVPCControllerReconcileDelay}, nil
		}

		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(vpcScope.LinodeVPC, infrav1alpha2.VPCFinalizer)
	// TODO: remove this check and removal later
	if controllerutil.ContainsFinalizer(vpcScope.LinodeVPC, infrav1alpha2.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(vpcScope.LinodeVPC, infrav1alpha2.GroupVersion.String())
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl // this is same as Placement Group, worth making generic later.
func (r *LinodeVPCReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	linodeVPCMapper, err := kutil.ClusterToTypedObjectsMapper(
		r.TracedClient(),
		&infrav1alpha2.LinodeVPCList{},
		mgr.GetScheme(),
	)
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeVPCs: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodeVPC{}).
		WithOptions(options).
		WithEventFilter(predicate.And(
			predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), mgr.GetLogger(), r.WatchFilterValue),
			predicate.GenerationChangedPredicate{},
			predicate.Funcs{UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject, okOld := e.ObjectOld.(*infrav1alpha2.LinodeVPC)
				newObject, okNew := e.ObjectNew.(*infrav1alpha2.LinodeVPC)
				if okOld && okNew && oldObject.Spec.VPCID == nil && newObject.Spec.VPCID != nil {
					// We just created the VPC, don't enqueue and update
					return false
				}
				return true
			}},
		)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeVPCMapper),
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureReady(mgr.GetScheme(), mgr.GetLogger())),
		).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeVPCReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
