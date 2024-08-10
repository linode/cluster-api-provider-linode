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
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	cerrs "sigs.k8s.io/cluster-api/errors"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	wrappedruntimeclient "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimeclient"
	wrappedruntimereconciler "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimereconciler"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

const (
	ConditionLoadBalancingInitiated clusterv1.ConditionType = "ConditionLoadBalancingInitiated"
	ConditionLoadBalancingComplete  clusterv1.ConditionType = "ConditionLoadBalancingComplete"
)

// LinodeClusterReconciler reconciles a LinodeCluster object
type LinodeClusterReconciler struct {
	client.Client
	Recorder           record.EventRecorder
	LinodeClientConfig scope.ClientConfig
	DnsClientConfig    scope.ClientConfig
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *LinodeClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	logger := ctrl.LoggerFrom(ctx).WithName("LinodeClusterReconciler").WithValues("name", req.NamespacedName.String())
	linodeCluster := &infrav1alpha2.LinodeCluster{}
	if err := r.TracedClient().Get(ctx, req.NamespacedName, linodeCluster); err != nil {
		logger.Info("Failed to fetch Linode cluster", "error", err.Error())

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cluster, err := kutil.GetOwnerCluster(ctx, r.TracedClient(), linodeCluster.ObjectMeta)
	if err != nil {
		logger.Info("Failed to get owner cluster", "error", err.Error())

		return ctrl.Result{}, client.IgnoreNotFound(err)
	} else if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef, skipping reconciliation")

		return ctrl.Result{}, nil
	}

	// Create the cluster scope.
	clusterScope, err := scope.NewClusterScope(
		ctx,
		r.LinodeClientConfig,
		r.DnsClientConfig,
		scope.ClusterScopeParams{
			Client:            r.TracedClient(),
			Cluster:           cluster,
			LinodeCluster:     linodeCluster,
			LinodeMachineList: infrav1alpha2.LinodeMachineList{},
		},
	)

	if err != nil {
		logger.Info("Failed to create cluster scope", "error", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to create cluster scope: %w", err)
	}

	if clusterScope == nil {
		return ctrl.Result{}, nil
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

	labels := map[string]string{clusterv1.ClusterNameLabel: clusterScope.LinodeCluster.Name}
	if err := r.TracedClient().List(ctx, &clusterScope.LinodeMachines, client.InNamespace(clusterScope.LinodeCluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return res, err
	}

	controlPlaneObjKey := client.ObjectKey{
		Namespace: clusterScope.LinodeCluster.Namespace,
		Name:      clusterScope.LinodeCluster.Name + "-control-plane",
	}
	var controlPlane controlplanev1.KubeadmControlPlane
	if err := r.Get(ctx, controlPlaneObjKey, &controlPlane); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, fmt.Errorf("get controlplaneobj %q: %w", controlPlaneObjKey, err)
		}
	}
	if controlPlane.Spec.Replicas == nil {
		return ctrl.Result{RequeueAfter: reconciler.DefaultClusterControllerReconcileDelay}, nil
	}

	// Always close the scope when exiting this function so we can persist any LinodeCluster changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := clusterScope.Close(ctx); utilerrors.FilterOut(util.UnwrapError(err), apierrors.IsNotFound) != nil && reterr == nil {
			logger.Error(err, "failed to patch LinodeCluster")
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !clusterScope.LinodeCluster.DeletionTimestamp.IsZero() {
		if err := r.reconcileDelete(ctx, logger, clusterScope); err != nil {
			if !reconciler.HasConditionSeverity(clusterScope.LinodeCluster, clusterv1.ReadyCondition, clusterv1.ConditionSeverityError) {
				logger.Info("re-queuing cluster/nb deletion")
				return ctrl.Result{RequeueAfter: reconciler.DefaultClusterControllerReconcileDelay}, nil
			}
			return res, err
		}
		return res, nil
	}

	if err := clusterScope.AddFinalizer(ctx); err != nil {
		logger.Error(err, "failed to update cluster finalizer")
		return res, err
	}

	// Create
	if clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint.Host == "" {
		if err := r.reconcileCreate(ctx, logger, clusterScope); err != nil {
			if !reconciler.HasConditionSeverity(clusterScope.LinodeCluster, clusterv1.ReadyCondition, clusterv1.ConditionSeverityError) {
				logger.Info("re-queuing cluster/load-balancer creation")
				return ctrl.Result{RequeueAfter: reconciler.DefaultClusterControllerReconcileDelay}, nil
			}
			return res, err
		}
		r.Recorder.Event(clusterScope.LinodeCluster, corev1.EventTypeNormal, string(clusterv1.ReadyCondition), "Load balancer is ready")
	}

	clusterScope.LinodeCluster.Status.Ready = true
	conditions.MarkTrue(clusterScope.LinodeCluster, clusterv1.ReadyCondition)

	for _, eachMachine := range clusterScope.LinodeMachines.Items {
		if len(eachMachine.Status.Addresses) == 0 {
			return ctrl.Result{RequeueAfter: reconciler.DefaultClusterControllerReconcileDelay}, nil
		}
	}

	if reconciler.ConditionTrue(clusterScope.LinodeCluster, ConditionLoadBalancingComplete) {
		return res, nil
	}
	if err := r.addMachineToLB(ctx, clusterScope); err != nil {
		return ctrl.Result{RequeueAfter: reconciler.DefaultClusterControllerReconcileDelay}, nil
	}
	if len(clusterScope.LinodeMachines.Items) < int(*controlPlane.Spec.Replicas) {
		conditions.MarkTrue(clusterScope.LinodeCluster, ConditionLoadBalancingInitiated)
	} else {
		conditions.MarkTrue(clusterScope.LinodeCluster, ConditionLoadBalancingComplete)
	}

	return res, nil
}

func setFailureReason(clusterScope *scope.ClusterScope, failureReason cerrs.ClusterStatusError, err error, lcr *LinodeClusterReconciler) {
	clusterScope.LinodeCluster.Status.FailureReason = util.Pointer(failureReason)
	clusterScope.LinodeCluster.Status.FailureMessage = util.Pointer(err.Error())

	reconciler.RecordDecayingCondition(clusterScope.LinodeCluster, clusterv1.ReadyCondition, string(failureReason), err.Error(), reconciler.DefaultTimeout(lcr.ReconcileTimeout, reconciler.DefaultClusterControllerReconcileTimeout))

	lcr.Recorder.Event(clusterScope.LinodeCluster, corev1.EventTypeWarning, string(failureReason), err.Error())
}

func (r *LinodeClusterReconciler) reconcileCreate(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	if err := clusterScope.AddCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "failed to update credentials finalizer")
		setFailureReason(clusterScope, cerrs.CreateClusterError, err, r)
		return err
	}

	// handle creation for the loadbalancer for the control plane
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "dns" {
		r.handleDNS(clusterScope)
	} else if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "NodeBalancer" || clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "" {
		if err := r.handleNBCreate(ctx, logger, clusterScope); err != nil {
			return err
		}
	}

	return nil
}

func (r *LinodeClusterReconciler) handleNBCreate(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	linodeNB, err := services.EnsureNodeBalancer(ctx, clusterScope, logger)
	if err != nil {
		logger.Error(err, "failed to ensure nodebalancer")
		setFailureReason(clusterScope, cerrs.CreateClusterError, err, r)
		return err
	}
	if linodeNB == nil {
		err = fmt.Errorf("nodeBalancer created was nil")
		setFailureReason(clusterScope, cerrs.CreateClusterError, err, r)
		return err
	}
	clusterScope.LinodeCluster.Spec.Network.NodeBalancerID = &linodeNB.ID

	// create the configs for the nodeabalancer if not already specified
	configs, err := services.EnsureNodeBalancerConfigs(ctx, clusterScope, logger)
	if err != nil {
		logger.Error(err, "failed to ensure nodebalancer configs")
		setFailureReason(clusterScope, cerrs.CreateClusterError, err, r)
		return err
	}

	clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID = util.Pointer(configs[0].ID)
	additionalPorts := make([]infrav1alpha2.LinodeNBPortConfig, 0)
	for _, config := range configs[1:] {
		portConfig := infrav1alpha2.LinodeNBPortConfig{
			Port:                 config.Port,
			NodeBalancerConfigID: &config.ID,
		}
		additionalPorts = append(additionalPorts, portConfig)
	}
	clusterScope.LinodeCluster.Spec.Network.AdditionalPorts = additionalPorts

	clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: *linodeNB.IPv4,
		Port: int32(configs[0].Port),
	}

	return nil
}

func (r *LinodeClusterReconciler) handleDNS(clusterScope *scope.ClusterScope) {
	clusterSpec := clusterScope.LinodeCluster.Spec
	clusterMetadata := clusterScope.LinodeCluster.ObjectMeta
	uniqueID := ""
	if clusterSpec.Network.DNSUniqueIdentifier != "" {
		uniqueID = "-" + clusterSpec.Network.DNSUniqueIdentifier
	}
	subDomain := clusterMetadata.Name + uniqueID

	if clusterScope.LinodeCluster.Spec.Network.DNSSubDomainOverride != "" {
		subDomain = clusterScope.LinodeCluster.Spec.Network.DNSSubDomainOverride
	}
	dnsHost := subDomain + "." + clusterSpec.Network.DNSRootDomain
	apiLBPort := services.DefaultApiserverLBPort
	if clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiLBPort = clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}
	clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: dnsHost,
		Port: int32(apiLBPort),
	}
}

func (r *LinodeClusterReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	logger.Info("deleting cluster")
	if clusterScope.LinodeCluster.Spec.Network.NodeBalancerID == nil && !(reconciler.ConditionTrue(clusterScope.LinodeCluster, ConditionLoadBalancingInitiated) || reconciler.ConditionTrue(clusterScope.LinodeCluster, ConditionLoadBalancingComplete)) {
		logger.Info("NodeBalancer ID is missing, nothing to do")

		if err := clusterScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
			logger.Error(err, "failed to remove credentials finalizer")
			setFailureReason(clusterScope, cerrs.DeleteClusterError, err, r)
			return err
		}
		controllerutil.RemoveFinalizer(clusterScope.LinodeCluster, infrav1alpha2.ClusterFinalizer)
		r.Recorder.Event(clusterScope.LinodeCluster, corev1.EventTypeWarning, "NodeBalancerIDMissing", "NodeBalancer ID is missing, nothing to do")

		return nil
	}

	if err := r.removeMachineFromLB(ctx, logger, clusterScope); err != nil {
		return fmt.Errorf("remove machine from loadbalancer: %w", err)
	}
	conditions.MarkFalse(clusterScope.LinodeCluster, ConditionLoadBalancingInitiated, "clear loadbalancer", clusterv1.ConditionSeverityWarning, "")
	conditions.MarkFalse(clusterScope.LinodeCluster, ConditionLoadBalancingComplete, "clear loadbalancer entries", clusterv1.ConditionSeverityWarning, "")

	err := clusterScope.LinodeClient.DeleteNodeBalancer(ctx, *clusterScope.LinodeCluster.Spec.Network.NodeBalancerID)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		logger.Error(err, "failed to delete NodeBalancer")
		setFailureReason(clusterScope, cerrs.DeleteClusterError, err, r)
		return err
	}

	conditions.MarkFalse(clusterScope.LinodeCluster, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "Load balancer deleted")
	r.Recorder.Event(clusterScope.LinodeCluster, corev1.EventTypeNormal, clusterv1.DeletedReason, "Load balancer deleted")

	clusterScope.LinodeCluster.Spec.Network.NodeBalancerID = nil
	clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID = nil
	clusterScope.LinodeCluster.Spec.Network.AdditionalPorts = []infrav1alpha2.LinodeNBPortConfig{}

	if err := clusterScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "failed to remove credentials finalizer")
		setFailureReason(clusterScope, cerrs.DeleteClusterError, err, r)
		return err
	}
	controllerutil.RemoveFinalizer(clusterScope.LinodeCluster, infrav1alpha2.ClusterFinalizer)
	// TODO: remove these checks and removals later
	if controllerutil.ContainsFinalizer(clusterScope.LinodeCluster, infrav1alpha1.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(clusterScope.LinodeCluster, infrav1alpha1.GroupVersion.String())
	}
	if controllerutil.ContainsFinalizer(clusterScope.LinodeCluster, infrav1alpha2.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(clusterScope.LinodeCluster, infrav1alpha2.GroupVersion.String())
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeClusterReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	linodeMachineMapper, err := kutil.ClusterToTypedObjectsMapper(
		r.TracedClient(),
		&infrav1alpha2.LinodeMachineList{},
		mgr.GetScheme(),
	)
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeCluster: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodeCluster{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(
				kutil.ClusterToInfrastructureMapFunc(context.TODO(), infrav1alpha2.GroupVersion.WithKind("LinodeCluster"), mgr.GetClient(), &infrav1alpha2.LinodeCluster{}),
			),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
		).
		Watches(
			&infrav1alpha2.LinodeMachine{},
			handler.EnqueueRequestsFromMapFunc(linodeMachineMapper),
		).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeClusterReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimereconciler.DefaultDecorator())
}

func (r *LinodeClusterReconciler) addMachineToLB(ctx context.Context, clusterScope *scope.ClusterScope) error {
	logger := logr.FromContextOrDiscard(ctx)
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType != "dns" {
		if err := services.AddNodesToNB(ctx, logger, clusterScope); err != nil {
			return err
		}
	} else {
		if err := services.EnsureDNSEntries(ctx, clusterScope, "create"); err != nil {
			return err
		}
	}

	return nil
}

func (r *LinodeClusterReconciler) removeMachineFromLB(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "NodeBalancer" {
		if err := services.DeleteNodesFromNB(ctx, logger, clusterScope); err != nil {
			logger.Error(err, "Failed to remove node from Node Balancer backend")
			return err
		}
	} else if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "dns" {
		if err := services.EnsureDNSEntries(ctx, clusterScope, "delete"); err != nil {
			logger.Error(err, "Failed to remove IP from DNS")
			return err
		}
	}
	return nil
}
