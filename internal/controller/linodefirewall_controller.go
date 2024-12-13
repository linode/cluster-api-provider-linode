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

// LinodeFirewallReconciler reconciles a LinodeFirewall object
type LinodeFirewallReconciler struct {
	client.Client
	Recorder           record.EventRecorder
	LinodeClientConfig scope.ClientConfig
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=addresssets,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=firewallrules,verbs=get;list;watch;update;patch

func (r *LinodeFirewallReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx).WithName("LinodeFirewallReconciler").WithValues("name", req.NamespacedName.String())
	linodeFirewall := &infrav1alpha2.LinodeFirewall{}
	if err := r.TracedClient().Get(ctx, req.NamespacedName, linodeFirewall); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "failed to fetch firewall")
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Create the firewall scope.
	fwScope, err := scope.NewFirewallScope(
		ctx,
		r.LinodeClientConfig,
		scope.FirewallScopeParams{
			Client:         r.TracedClient(),
			LinodeFirewall: linodeFirewall,
		})
	if err != nil {
		log.Error(err, "failed to create firewall scope")

		return ctrl.Result{}, fmt.Errorf("failed to create cluster scope: %w", err)
	}

	return r.reconcile(ctx, log, fwScope)
}

func (r *LinodeFirewallReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	fwScope *scope.FirewallScope,
) (res ctrl.Result, err error) {
	res = ctrl.Result{}

	fwScope.LinodeFirewall.Status.Ready = false
	fwScope.LinodeFirewall.Status.FailureReason = nil
	fwScope.LinodeFirewall.Status.FailureMessage = util.Pointer("")

	failureReason := infrav1alpha2.FirewallStatusError("UnknownError")
	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if err != nil {
			fwScope.LinodeFirewall.Status.FailureReason = util.Pointer(failureReason)
			fwScope.LinodeFirewall.Status.FailureMessage = util.Pointer(err.Error())
			conditions.MarkFalse(fwScope.LinodeFirewall, clusterv1.ReadyCondition, string(failureReason), "", "%s", err.Error())
			r.Recorder.Event(fwScope.LinodeFirewall, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist any LinodeFirewall changes.
		// This ignores any resource not found errors when reconciling deletions.
		if patchErr := fwScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch Firewall")
			err = errors.Join(err, patchErr)
		}
	}()

	// Override the controller credentials with ones from the Firewall's Secret reference (if supplied).
	if err := fwScope.SetCredentialRefTokenForLinodeClients(ctx); err != nil {
		logger.Error(err, "failed to update linode client token from Credential Ref")
		return ctrl.Result{}, err
	}

	// Delete
	if !fwScope.LinodeFirewall.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = infrav1alpha2.DeleteFirewallError

		return r.reconcileDelete(ctx, logger, fwScope)
	}

	// Add the finalizer if not already there
	err = fwScope.AddFinalizer(ctx)
	if err != nil {
		logger.Error(err, "failed to add finalizer")

		return ctrl.Result{}, nil
	}

	action := "update"
	if fwScope.LinodeFirewall.Spec.FirewallID != nil {
		failureReason = infrav1alpha2.UpdateFirewallError
		logger = logger.WithValues("fwID", *fwScope.LinodeFirewall.Spec.FirewallID)
	} else {
		action = "create"
		failureReason = infrav1alpha2.CreateFirewallError
		if err = fwScope.AddCredentialsRefFinalizer(ctx); err != nil {
			logger.Error(err, "failed to update credentials secret")
			conditions.MarkFalse(fwScope.LinodeFirewall, clusterv1.ReadyCondition, string(failureReason), "", "%s", err.Error())
			r.Recorder.Event(fwScope.LinodeFirewall, corev1.EventTypeWarning, string(failureReason), err.Error())

			return ctrl.Result{}, nil
		}
	}
	if err = reconcileFirewall(ctx, r.Client, fwScope, logger); err != nil {
		logger.Error(err, fmt.Sprintf("failed to %s Firewall", action))
		conditions.MarkFalse(fwScope.LinodeFirewall, clusterv1.ReadyCondition, string(failureReason), "", "%s", err.Error())
		r.Recorder.Event(fwScope.LinodeFirewall, corev1.EventTypeWarning, string(failureReason), err.Error())

		switch {
		case errors.Is(err, errTooManyIPs):
			// Cannot reconcile firewall with too many ips, wait for an update to the spec
			return ctrl.Result{}, nil
		case util.IsRetryableError(err) && !reconciler.HasStaleCondition(fwScope.LinodeFirewall, clusterv1.ReadyCondition,
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultFWControllerReconcileTimeout)):
			logger.Info(fmt.Sprintf("re-queuing Firewall %s", action))

			return ctrl.Result{RequeueAfter: reconciler.DefaultFWControllerReconcilerDelay}, nil
		}

		return ctrl.Result{}, err
	}

	if action == "create" && fwScope.LinodeFirewall.Spec.FirewallID != nil {
		r.Recorder.Event(fwScope.LinodeFirewall, corev1.EventTypeNormal, "Created", fmt.Sprintf("Created Firewall %d", *fwScope.LinodeFirewall.Spec.FirewallID))
	} else {
		r.Recorder.Event(fwScope.LinodeFirewall, corev1.EventTypeNormal, "Updated", fmt.Sprintf("Updated Firewall %d", *fwScope.LinodeFirewall.Spec.FirewallID))
	}
	fwScope.LinodeFirewall.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *LinodeFirewallReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	fwScope *scope.FirewallScope,
) (ctrl.Result, error) {
	logger.Info("deleting firewall")

	if fwScope.LinodeFirewall.Spec.FirewallID == nil {
		logger.Info("firewall ID is missing, nothing to do")
		controllerutil.RemoveFinalizer(fwScope.LinodeFirewall, infrav1alpha2.FirewallFinalizer)

		return ctrl.Result{}, nil
	}

	if err := fwScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "failed to remove credentials finalizer")
		return ctrl.Result{}, err
	}

	err := fwScope.LinodeClient.DeleteFirewall(ctx, *fwScope.LinodeFirewall.Spec.FirewallID)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		logger.Error(err, "failed to delete Firewall")

		if fwScope.LinodeFirewall.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultFWControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("re-queuing Firewall deletion")

			return ctrl.Result{RequeueAfter: reconciler.DefaultFWControllerReconcilerDelay}, nil
		}

		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(fwScope.LinodeFirewall, infrav1alpha2.FirewallFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeFirewallReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	linodeFirewallMapper, err := kutil.ClusterToTypedObjectsMapper(
		r.TracedClient(),
		&infrav1alpha2.LinodeFirewallList{},
		mgr.GetScheme(),
	)
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeFirewalls: %w", err)
	}
	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodeFirewall{}).
		WithOptions(options).
		WithEventFilter(
			predicate.And(
				predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
				predicate.GenerationChangedPredicate{},
				predicate.Funcs{UpdateFunc: func(e event.UpdateEvent) bool {
					oldObject, okOld := e.ObjectOld.(*infrav1alpha2.LinodeFirewall)
					newObject, okNew := e.ObjectNew.(*infrav1alpha2.LinodeFirewall)
					if okOld && okNew && oldObject.Spec.FirewallID == nil && newObject.Spec.FirewallID != nil {
						// We just updated the fwID, don't enqueue request
						return false
					}
					return true
				}},
			)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeFirewallMapper),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
		).
		Watches(
			&infrav1alpha2.AddressSet{},
			handler.EnqueueRequestsFromMapFunc(findObjectsForObject(mgr.GetLogger(), r.TracedClient())),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&infrav1alpha2.FirewallRule{},
			handler.EnqueueRequestsFromMapFunc(findObjectsForObject(mgr.GetLogger(), r.TracedClient())),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeFirewallReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimereconciler.DefaultDecorator())
}
