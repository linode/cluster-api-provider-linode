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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	cerrs "sigs.k8s.io/cluster-api/errors"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// LinodeClusterReconciler reconciles a LinodeCluster object
type LinodeClusterReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	ReconcileTimeout time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters/finalizers,verbs=update

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *LinodeClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	logger := ctrl.LoggerFrom(ctx).WithName("LinodeClusterReconciler").WithValues("name", req.NamespacedName.String())
	linodeCluster := &infrav1alpha1.LinodeCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, linodeCluster); err != nil {
		logger.Info("Failed to fetch Linode cluster", "error", err.Error())

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cluster, err := kutil.GetOwnerCluster(ctx, r.Client, linodeCluster.ObjectMeta)
	if err != nil {
		logger.Info("Failed to get owner cluster", "error", err.Error())

		return ctrl.Result{}, client.IgnoreNotFound(err)
	} else if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef, skipping reconciliation")

		return ctrl.Result{}, nil
	}
	if annotations.IsPaused(cluster, linodeCluster) {
		logger.Info("LinodeCluster of linked Cluster is marked as paused. Won't reconcile")

		return ctrl.Result{}, nil
	}

	// Create the cluster scope.
	clusterScope, err := scope.NewClusterScope(
		ctx,
		r.LinodeApiKey,
		scope.ClusterScopeParams{
			Client:        r.Client,
			Cluster:       cluster,
			LinodeCluster: linodeCluster,
		})
	if err != nil {
		logger.Info("Failed to create cluster scope", "error", err.Error())

		return ctrl.Result{}, fmt.Errorf("failed to create cluster scope: %w", err)
	}

	return r.reconcile(ctx, clusterScope, logger)
}

func (r *LinodeClusterReconciler) reconcile(
	ctx context.Context,
	clusterScope *scope.ClusterScope,
	logger logr.Logger,
) (res ctrl.Result, reterr error) {
	res = ctrl.Result{}

	clusterScope.LinodeCluster.Status.Ready = false
	clusterScope.LinodeCluster.Status.FailureReason = nil
	clusterScope.LinodeCluster.Status.FailureMessage = util.Pointer("")

	// Always close the scope when exiting this function so we can persist any LinodeCluster changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := clusterScope.Close(ctx); utilerrors.FilterOut(err, apierrors.IsNotFound) != nil && reterr == nil {
			logger.Error(err, "failed to patch LinodeCluster")
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !clusterScope.LinodeCluster.DeletionTimestamp.IsZero() {
		return res, r.reconcileDelete(ctx, logger, clusterScope)
	}

	if err := clusterScope.AddFinalizer(ctx); err != nil {
		return res, err
	}

	// Create cluster
	if clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint.Host == "" {
		if err := r.reconcileCreate(ctx, logger, clusterScope); err != nil {
			return res, err
		}
		r.Recorder.Event(clusterScope.LinodeCluster, corev1.EventTypeNormal, string(clusterv1.ReadyCondition), "Cluster is ready")
	} else {
		// Update cluster
		if err := r.reconcileUpdate(ctx, logger, clusterScope); err != nil {
			return res, err
		}
	}

	clusterScope.LinodeCluster.Status.Ready = true
	conditions.MarkTrue(clusterScope.LinodeCluster, clusterv1.ReadyCondition)

	return res, nil
}

func (r *LinodeClusterReconciler) createControlPlaneFirewallSpec(
	linodeCluster *infrav1alpha1.LinodeCluster,
) *infrav1alpha1.LinodeFirewallSpec {
	// Per the Linode API:
	// Must contain only valid IPv4 addresses or networks (both must be in ip/mask format)
	apiServerIPV4 := append(
		[]string{fmt.Sprintf("%s/32", linodeCluster.Spec.ControlPlaneEndpoint.Host)},
		linodeCluster.Spec.ControlPlaneFirewall.AllowedIPV4Addresses...,
	)
	apiServerIPV6 := append(
		[]string{},
		linodeCluster.Spec.ControlPlaneFirewall.AllowedIPV6Addresses...,
	)
	lbPort := services.DefaultLBPort
	if linodeCluster.Spec.Network.LoadBalancerPort != 0 {
		lbPort = linodeCluster.Spec.Network.LoadBalancerPort
	}
	controlPlaneRules := []infrav1alpha1.FirewallRule{{
		Action: "ACCEPT",
		Label:  "api-server",
		Ports:  strconv.Itoa(lbPort),
		Addresses: &infrav1alpha1.NetworkAddresses{
			IPv4: util.Pointer(apiServerIPV4),
			IPv6: util.Pointer(apiServerIPV6),
		},
	}}
	if linodeCluster.Spec.ControlPlaneFirewall.AllowSSH {
		sshRule := infrav1alpha1.FirewallRule{
			Action: "ACCEPT",
			Label:  "ssh",
			Ports:  "22",
			Addresses: &infrav1alpha1.NetworkAddresses{
				IPv4: util.Pointer(linodeCluster.Spec.ControlPlaneFirewall.AllowedIPV4Addresses),
				IPv6: util.Pointer(linodeCluster.Spec.ControlPlaneFirewall.AllowedIPV6Addresses),
			},
		}
		controlPlaneRules = append(controlPlaneRules, sshRule)
	}

	return &infrav1alpha1.LinodeFirewallSpec{
		ClusterUID:    string(linodeCluster.UID),
		FirewallID:    linodeCluster.Spec.ControlPlaneFirewall.FirewallID,
		Enabled:       linodeCluster.Spec.ControlPlaneFirewall.Enabled,
		Label:         linodeCluster.Name,
		InboundPolicy: "DROP",
		InboundRules:  controlPlaneRules,
	}
}

func (r *LinodeClusterReconciler) setFailureReason(clusterScope *scope.ClusterScope, failureReason cerrs.ClusterStatusError, err error) {
	clusterScope.LinodeCluster.Status.FailureReason = util.Pointer(failureReason)
	clusterScope.LinodeCluster.Status.FailureMessage = util.Pointer(err.Error())

	conditions.MarkFalse(clusterScope.LinodeCluster, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, "%s", err.Error())

	r.Recorder.Event(clusterScope.LinodeCluster, corev1.EventTypeWarning, string(failureReason), err.Error())
}

func (r *LinodeClusterReconciler) reconcileCreate(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	// handle NodeBalancer
	linodeNB, err := services.CreateNodeBalancer(ctx, clusterScope, logger)
	if err != nil {
		r.setFailureReason(clusterScope, cerrs.CreateClusterError, err)

		return err
	}
	clusterScope.LinodeCluster.Spec.Network.NodeBalancerID = linodeNB.ID
	linodeNBConfig, err := services.CreateNodeBalancerConfig(ctx, clusterScope, logger)
	if err != nil {
		r.setFailureReason(clusterScope, cerrs.CreateClusterError, err)

		return err
	}
	clusterScope.LinodeCluster.Spec.Network.NodeBalancerConfigID = util.Pointer(linodeNBConfig.ID)

	// Set the control plane endpoint with the new Nodebalancer host and port
	clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: *linodeNB.IPv4,
		Port: int32(linodeNBConfig.Port),
	}

	// build out the control plane Firewall rules
	controlPlaneFW := &infrav1alpha1.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-api-server", clusterScope.LinodeCluster.Name),
			Namespace: clusterScope.LinodeCluster.Namespace,
		},
		Spec: *r.createControlPlaneFirewallSpec(clusterScope.LinodeCluster),
	}

	// Handle the Firewall
	if err := r.Client.Create(ctx, controlPlaneFW); err != nil {
		r.setFailureReason(clusterScope, cerrs.CreateClusterError, err)

		return err
	}
	clusterScope.LinodeCluster.Spec.ControlPlaneFirewallRef = &corev1.ObjectReference{
		Kind:      controlPlaneFW.Kind,
		Namespace: controlPlaneFW.Namespace,
		Name:      controlPlaneFW.Name,
	}
	// NOTE: if we add a reconciler later on don't call this as the reconciler will take care of it
	firewall, err := services.HandleFirewall(ctx, controlPlaneFW, clusterScope.LinodeClient, logger)
	if err != nil {
		r.setFailureReason(clusterScope, cerrs.CreateClusterError, err)

		return err
	}

	clusterScope.LinodeCluster.Spec.ControlPlaneFirewall.FirewallID = util.Pointer(firewall.ID)

	return nil
}

func (r *LinodeClusterReconciler) reconcileUpdate(
	ctx context.Context,
	logger logr.Logger,
	clusterScope *scope.ClusterScope,
) error {
	// Update the Firewall if necessary
	controlPlaneFW := &infrav1alpha1.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-api-server", clusterScope.LinodeCluster.Name),
			Namespace: clusterScope.LinodeCluster.Namespace,
		},
		Spec: *r.createControlPlaneFirewallSpec(clusterScope.LinodeCluster),
	}

	if err := r.Client.Update(ctx, controlPlaneFW); err != nil {
		r.setFailureReason(clusterScope, cerrs.UpdateClusterError, err)

		return err
	}
	if _, err := services.HandleFirewall(ctx, controlPlaneFW, clusterScope.LinodeClient, logger); err != nil {
		r.setFailureReason(clusterScope, cerrs.UpdateClusterError, err)

		return err
	}

	return nil
}

func (r *LinodeClusterReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	logger.Info("deleting cluster")
	if clusterScope.LinodeCluster.Spec.ControlPlaneFirewall.FirewallID != nil {
		if err := clusterScope.LinodeClient.DeleteFirewall(
			ctx,
			*clusterScope.LinodeCluster.Spec.ControlPlaneFirewall.FirewallID,
		); err != nil {
			logger.Info("Failed to delete control plane Firewall", "error", err.Error())

			// Not found is not an error
			apiErr := linodego.Error{}
			if errors.As(err, &apiErr) && apiErr.Code != http.StatusNotFound {
				r.setFailureReason(clusterScope, cerrs.DeleteClusterError, err)

				return err
			}
		}
	}

	if clusterScope.LinodeCluster.Spec.Network.NodeBalancerID == 0 {
		logger.Info("NodeBalancer ID is missing, nothing to do")
		controllerutil.RemoveFinalizer(clusterScope.LinodeCluster, infrav1alpha1.GroupVersion.String())

		return nil
	}

	if err := clusterScope.LinodeClient.DeleteNodeBalancer(ctx, clusterScope.LinodeCluster.Spec.Network.NodeBalancerID); err != nil {
		logger.Info("Failed to delete Linode NodeBalancer", "error", err.Error())

		// Not found is not an error
		apiErr := linodego.Error{}
		if errors.As(err, &apiErr) && apiErr.Code != http.StatusNotFound {
			r.setFailureReason(clusterScope, cerrs.DeleteClusterError, err)

			return err
		}
	}

	conditions.MarkFalse(clusterScope.LinodeCluster, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "Load balancer deleted")

	clusterScope.LinodeCluster.Spec.Network.NodeBalancerID = 0
	clusterScope.LinodeCluster.Spec.ControlPlaneFirewall.FirewallID = nil
	clusterScope.LinodeCluster.Spec.Network.NodeBalancerConfigID = nil
	controllerutil.RemoveFinalizer(clusterScope.LinodeCluster, infrav1alpha1.GroupVersion.String())

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue)).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	err = controller.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(kutil.ClusterToInfrastructureMapFunc(context.TODO(), infrav1alpha1.GroupVersion.WithKind("LinodeCluster"), mgr.GetClient(), &infrav1alpha1.LinodeCluster{})),
		predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger()),
	)
	if err != nil {
		return fmt.Errorf("failed adding a watch for ready clusters: %w", err)
	}

	return nil
}
