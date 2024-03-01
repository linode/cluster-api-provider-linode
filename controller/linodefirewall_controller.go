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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// LinodeFirewallReconciler reconciles a LinodeFirewall object
type LinodeFirewallReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	ReconcileTimeout time.Duration
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls/finalizers,verbs=update

func (r *LinodeFirewallReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	logger := ctrl.LoggerFrom(ctx).WithName("LinodeFirewallReconciler").WithValues("name", req.NamespacedName.String())
	linodeFirewall := &infrav1alpha1.LinodeFirewall{}
	if err := r.Client.Get(ctx, req.NamespacedName, linodeFirewall); err != nil {
		logger.Info("Failed to fetch Linode firewall", "error", err.Error())

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	linodeCluster := &infrav1alpha1.LinodeCluster{}

	// Create the firewall scope.
	firewallScope, err := scope.NewFirewallScope(
		r.LinodeApiKey,
		scope.FirewallScopeParams{
			Client:         r.Client,
			LinodeFirewall: linodeFirewall,
			LinodeCluster:  linodeCluster,
		})
	if err != nil {
		logger.Info("Failed to create firewall scope", "error", err.Error())

		return ctrl.Result{}, fmt.Errorf("failed to create cluster scope: %w", err)
	}

	return r.reconcile(ctx, firewallScope, logger)
}

func (r *LinodeFirewallReconciler) reconcile(
	ctx context.Context,
	firewallScope *scope.FirewallScope,
	logger logr.Logger,
) (res ctrl.Result, reterr error) {
	res = ctrl.Result{}

	firewallScope.LinodeFirewall.Status.Ready = false
	firewallScope.LinodeFirewall.Status.FailureReason = nil
	firewallScope.LinodeFirewall.Status.FailureMessage = util.Pointer("")

	// Always close the scope when exiting this function so we can persist any LinodeCluster changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := firewallScope.Close(ctx); utilerrors.FilterOut(err, apierrors.IsNotFound) != nil && reterr == nil {
			logger.Error(err, "failed to patch LinodeCluster")
			reterr = err
		}
	}()

	// Handle delete
	if !firewallScope.LinodeFirewall.DeletionTimestamp.IsZero() {
		return res, r.reconcileDelete(ctx, logger, firewallScope)
	}

	// Add finalizer if it's not already there
	if err := firewallScope.AddFinalizer(ctx); err != nil {
		return res, err
	}

	// Handle create
	if firewallScope.LinodeFirewall.Spec.FirewallID == nil {
		if err := r.reconcileCreate(ctx, logger, firewallScope); err != nil {
			return res, err
		}
		r.Recorder.Event(
			firewallScope.LinodeFirewall,
			corev1.EventTypeNormal,
			string(clusterv1.ReadyCondition),
			"Firewall is ready",
		)
	}

	// Handle updates
	if err := r.reconcileUpdate(ctx, logger, firewallScope); err != nil {
		return res, err
	}
	r.Recorder.Event(
		firewallScope.LinodeFirewall,
		corev1.EventTypeNormal,
		string(clusterv1.ReadyCondition),
		"Firewall is ready",
	)

	firewallScope.LinodeFirewall.Status.Ready = true
	conditions.MarkTrue(firewallScope.LinodeFirewall, clusterv1.ReadyCondition)

	return res, nil
}

func (r *LinodeFirewallReconciler) setFailureReason(
	firewallScope *scope.FirewallScope,
	failureReason infrav1alpha1.FirewallStatusError,
	err error,
) {
	firewallScope.LinodeFirewall.Status.FailureReason = util.Pointer(failureReason)
	firewallScope.LinodeFirewall.Status.FailureMessage = util.Pointer(err.Error())

	conditions.MarkFalse(
		firewallScope.LinodeFirewall,
		clusterv1.ReadyCondition,
		string(failureReason),
		clusterv1.ConditionSeverityError,
		"%s",
		err.Error(),
	)

	r.Recorder.Event(firewallScope.LinodeFirewall, corev1.EventTypeWarning, string(failureReason), err.Error())
}

func (r *LinodeFirewallReconciler) reconcileCreate(
	ctx context.Context,
	logger logr.Logger,
	firewallScope *scope.FirewallScope,
) error {
	linodeFW, err := services.HandleFirewall(ctx, firewallScope, logger)
	if err != nil || linodeFW == nil {
		r.setFailureReason(firewallScope, infrav1alpha1.CreateFirewallError, err)

		return err
	}
	firewallScope.LinodeFirewall.Spec.FirewallID = util.Pointer(linodeFW.ID)

	return nil
}

func (r *LinodeFirewallReconciler) reconcileUpdate(
	ctx context.Context,
	logger logr.Logger,
	firewallScope *scope.FirewallScope,
) error {
	linodeFW, err := services.HandleFirewall(ctx, firewallScope, logger)
	if err != nil || linodeFW == nil {
		r.setFailureReason(firewallScope, infrav1alpha1.UpdateFirewallError, err)

		return err
	}
	firewallScope.LinodeFirewall.Spec.FirewallID = util.Pointer(linodeFW.ID)

	return nil
}

func (r *LinodeFirewallReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	firewallScope *scope.FirewallScope,
) error {
	if firewallScope.LinodeFirewall.Spec.FirewallID == nil {
		logger.Info("Firewall ID is missing, nothing to do")
		controllerutil.RemoveFinalizer(firewallScope.LinodeFirewall, infrav1alpha1.GroupVersion.String())

		return nil
	}

	if err := firewallScope.LinodeClient.DeleteFirewall(ctx, *firewallScope.LinodeFirewall.Spec.FirewallID); err != nil {
		logger.Info("Failed to delete Linode NodeBalancer", "error", err.Error())

		// Not found is not an error
		apiErr := linodego.Error{}
		if errors.As(err, &apiErr) && apiErr.Code != http.StatusNotFound {
			r.setFailureReason(firewallScope, infrav1alpha1.DeleteFirewallError, err)

			return err
		}
	}

	conditions.MarkFalse(
		firewallScope.LinodeFirewall,
		clusterv1.ReadyCondition,
		clusterv1.DeletedReason,
		clusterv1.ConditionSeverityInfo,
		"Firewall deleted",
	)

	firewallScope.LinodeFirewall.Spec.FirewallID = nil
	controllerutil.RemoveFinalizer(firewallScope.LinodeFirewall, infrav1alpha1.GroupVersion.String())

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeFirewallReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeFirewall{}).
		Complete(r)
}
