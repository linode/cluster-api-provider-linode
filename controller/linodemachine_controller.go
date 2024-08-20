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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	wrappedruntimeclient "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimeclient"
	wrappedruntimereconciler "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimereconciler"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

const (
	linodeBusyCode        = 400
	defaultDiskFilesystem = string(linodego.FilesystemExt4)

	// conditions for preflight instance creation
	ConditionPreflightCreated                clusterv1.ConditionType = "PreflightCreated"
	ConditionPreflightRootDiskResizing       clusterv1.ConditionType = "PreflightRootDiskResizing"
	ConditionPreflightRootDiskResized        clusterv1.ConditionType = "PreflightRootDiskResized"
	ConditionPreflightAdditionalDisksCreated clusterv1.ConditionType = "PreflightAdditionalDisksCreated"
	ConditionPreflightConfigured             clusterv1.ConditionType = "PreflightConfigured"
	ConditionPreflightBootTriggered          clusterv1.ConditionType = "PreflightBootTriggered"
	ConditionPreflightNetworking             clusterv1.ConditionType = "PreflightNetworking"
	ConditionPreflightLoadBalancing          clusterv1.ConditionType = "PreflightLoadbalancing"
	ConditionPreflightReady                  clusterv1.ConditionType = "PreflightReady"

	// WaitingForBootstrapDataReason used when machine is waiting for bootstrap data to be ready before proceeding.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
)

// statuses to keep requeueing on while an instance is booting
var requeueInstanceStatuses = map[linodego.InstanceStatus]bool{
	linodego.InstanceOffline:      true,
	linodego.InstanceBooting:      true,
	linodego.InstanceRebooting:    true,
	linodego.InstanceProvisioning: true,
	linodego.InstanceMigrating:    true,
	linodego.InstanceRebuilding:   true,
	linodego.InstanceCloning:      true,
	linodego.InstanceRestoring:    true,
	linodego.InstanceResizing:     true,
}

// LinodeMachineReconciler reconciles a LinodeMachine object
type LinodeMachineReconciler struct {
	client.Client
	Recorder           record.EventRecorder
	LinodeClientConfig scope.ClientConfig
	DnsClientConfig    scope.ClientConfig
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters/finalizers,verbs=update

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;watch;list
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;watch;list
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *LinodeMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx).WithName("LinodeMachineReconciler").WithValues("name", req.NamespacedName.String())

	linodeMachine := &infrav1alpha2.LinodeMachine{}
	if err := r.TracedClient().Get(ctx, req.NamespacedName, linodeMachine); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch LinodeMachine")
		}

		return ctrl.Result{}, err
	}

	// Fetch owner machine
	machine, err := kutil.GetOwnerMachine(ctx, r.TracedClient(), linodeMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("LinodeMachine", machine.Name)

	// Fetch the cluster
	cluster, err := kutil.GetClusterFromMetadata(ctx, r.TracedClient(), machine.ObjectMeta)
	if err != nil {
		log.Info("Failed to fetch cluster by label")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	machineScope, err := scope.NewMachineScope(
		ctx,
		r.LinodeClientConfig,
		r.DnsClientConfig,
		scope.MachineScopeParams{
			Client:        r.TracedClient(),
			Cluster:       cluster,
			Machine:       machine,
			LinodeCluster: &infrav1alpha2.LinodeCluster{},
			LinodeMachine: linodeMachine,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create machine scope")

		return ctrl.Result{}, fmt.Errorf("failed to create machine scope: %w", err)
	}

	return r.reconcile(ctx, log, machineScope)
}

func (r *LinodeMachineReconciler) reconcile(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (res ctrl.Result, err error) {
	failureReason := cerrs.MachineStatusError("UnknownError")
	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if err != nil {
			machineScope.LinodeMachine.Status.FailureReason = util.Pointer(failureReason)
			machineScope.LinodeMachine.Status.FailureMessage = util.Pointer(err.Error())
			conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, err.Error())
			r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist any LinodeMachine and LinodeCluster changes.
		// This ignores any resource not found errors when reconciling deletions.
		if patchErr := machineScope.CloseAll(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch LinodeMachine and LinodeCluster")
			err = errors.Join(err, patchErr)
		}
	}()

	// Add the finalizer if not already there
	if err = machineScope.AddFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Delete
	if !machineScope.LinodeMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = cerrs.DeleteMachineError
		return r.reconcileDelete(ctx, logger, machineScope)
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap data secret is not yet available")
		conditions.MarkFalse(machineScope.LinodeMachine, ConditionPreflightCreated, WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	// Update
	if machineScope.LinodeMachine.Spec.ProviderID != nil {
		failureReason = cerrs.UpdateMachineError
		return r.reconcileUpdate(ctx, logger, machineScope)
	}

	// Create
	failureReason = cerrs.CreateMachineError
	return r.reconcileCreate(ctx, logger, machineScope)
}

func (r *LinodeMachineReconciler) reconcileCreate(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (ctrl.Result, error) {
	logger.Info("creating machine")

	if err := machineScope.AddCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to update credentials secret")
		return ctrl.Result{}, err
	}

	tags := []string{machineScope.LinodeCluster.Name}
	var instanceID *int
	if machineScope.LinodeMachine.Spec.ProviderID != nil {
		instID, err := util.GetInstanceID(machineScope.LinodeMachine.Spec.ProviderID)
		if err != nil {
			logger.Error(err, "Failed to parse instance ID from provider ID")
			return ctrl.Result{}, err
		}
		instanceID = util.Pointer(instID)
	}

	listFilter := util.Filter{
		ID:    instanceID,
		Label: machineScope.LinodeMachine.Name,
		Tags:  tags,
	}
	filter, err := listFilter.String()
	if err != nil {
		return ctrl.Result{}, err
	}
	linodeInstances, err := machineScope.LinodeClient.ListInstances(ctx, linodego.NewListOptions(1, filter))
	if err != nil {
		logger.Error(err, "Failed to list Linode machine instances")

		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
	}

	var linodeInstance *linodego.Instance
	switch len(linodeInstances) {
	case 1:
		logger.Info("Linode instance already exists")
		linodeInstance = &linodeInstances[0]
	case 0:
		// get the bootstrap data for the Linode instance and set it for create config
		createOpts, err := r.newCreateConfig(ctx, machineScope, tags, logger)
		if err != nil {
			logger.Error(err, "Failed to create Linode machine InstanceCreateOptions")
			return retryIfTransient(err)
		}
		linodeInstance, err = machineScope.LinodeClient.CreateInstance(ctx, *createOpts)
		if err != nil {
			if util.IsRetryableError(err) {
				logger.Error(err, "Failed to create Linode instance due to API error, requeing")
				if linodego.ErrHasStatus(err, http.StatusTooManyRequests) {
					return ctrl.Result{RequeueAfter: reconciler.DefaultLinodeTooManyRequestsErrorRetryDelay}, nil
				}
				return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerRetryDelay}, nil
			}
			logger.Error(err, "Failed to create Linode machine instance")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightCreated, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}
	default:
		err = errors.New("multiple instances")
		logger.Error(err, "multiple instances found", "tags", tags)

		return ctrl.Result{}, err
	}

	conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightCreated)

	return r.reconcileInstanceCreate(ctx, logger, machineScope, linodeInstance)
}

//nolint:cyclop,gocognit // It is ok for the moment but need larger refactor.
func (r *LinodeMachineReconciler) reconcileInstanceCreate(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstance *linodego.Instance,
) (ctrl.Result, error) {
	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightConfigured) {
		if err := r.configureDisks(ctx, logger, machineScope, linodeInstance.ID); err != nil {
			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightConfigured, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightConfigured)
	}

	if machineScope.LinodeMachine.Spec.Configuration != nil && machineScope.LinodeMachine.Spec.Configuration.Kernel != "" {
		instanceConfig, err := r.getDefaultInstanceConfig(ctx, machineScope, linodeInstance.ID)
		if err != nil {
			logger.Error(err, "Failed to get default instance configuration")
			return retryIfTransient(err)
		}

		if _, err := machineScope.LinodeClient.UpdateInstanceConfig(ctx, linodeInstance.ID, instanceConfig.ID, linodego.InstanceConfigUpdateOptions{Kernel: machineScope.LinodeMachine.Spec.Configuration.Kernel}); err != nil {
			logger.Error(err, "Failed to update default instance configuration")
			return retryIfTransient(err)
		}
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered) {
		if err := machineScope.LinodeClient.BootInstance(ctx, linodeInstance.ID, 0); err != nil && !strings.HasSuffix(err.Error(), "already booted.") {
			logger.Error(err, "Failed to boot instance")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightBootTriggered, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered)
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightReady) {
		addrs, err := r.buildInstanceAddrs(ctx, machineScope, linodeInstance.ID)
		if err != nil {
			logger.Error(err, "Failed to get instance ip addresses")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightReady, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}
		machineScope.LinodeMachine.Status.Addresses = addrs

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightReady)
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightNetworking) {
		if err := r.addMachineToLB(ctx, machineScope); err != nil {
			logger.Error(err, "Failed to add machine to LB")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightNetworking, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}
		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightNetworking)
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightLoadBalancing) {
		// Add the finalizer if not already there
		if err := machineScope.AddLinodeClusterFinalizer(ctx); err != nil {
			logger.Error(err, "Failed to add linodecluster finalizer")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightLoadBalancing, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerRetryDelay}, nil
		}
		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightLoadBalancing)
	}

	// Set the instance state to signal preflight process is done
	machineScope.LinodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)
	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))

	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcileUpdate(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	logger.Info("updating machine")
	instanceID, err := util.GetInstanceID(machineScope.LinodeMachine.Spec.ProviderID)
	if err != nil {
		logger.Error(err, "Failed to parse instance ID from provider ID")
		return ctrl.Result{}, err
	}

	var linodeInstance *linodego.Instance
	if linodeInstance, err = machineScope.LinodeClient.GetInstance(ctx, instanceID); err != nil {
		return retryIfTransient(err)
	}
	if _, ok := requeueInstanceStatuses[linodeInstance.Status]; ok {
		if linodeInstance.Updated.Add(reconciler.DefaultMachineControllerWaitForRunningTimeout).After(time.Now()) {
			logger.Info("Instance has one operation running, re-queuing reconciliation", "status", linodeInstance.Status)
			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}
		logger.Info("Instance has one operation long running, skipping reconciliation", "status", linodeInstance.Status)
		conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "skipped due to long running operation")
		return ctrl.Result{}, nil
	} else if linodeInstance.Status != linodego.InstanceRunning {
		logger.Info("Instance has incompatible status, skipping reconciliation", "status", linodeInstance.Status)
		conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "incompatible status")
		return ctrl.Result{}, nil
	}
	machineScope.LinodeMachine.Status.Ready = true
	conditions.MarkTrue(machineScope.LinodeMachine, clusterv1.ReadyCondition)
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (ctrl.Result, error) {
	logger.Info("deleting machine")

	if machineScope.LinodeMachine.Spec.ProviderID == nil {
		logger.Info("Machine ID is missing, nothing to do")

		if err := machineScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
			logger.Error(err, "Failed to update credentials secret")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(machineScope.LinodeMachine, infrav1alpha2.MachineFinalizer)

		return ctrl.Result{}, nil
	}

	if err := r.removeMachineFromLB(ctx, logger, machineScope); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove machine from loadbalancer: %w", err)
	}

	// Add the finalizer if not already there
	if err := machineScope.RemoveLinodeClusterFinalizer(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("Failed to remove linodecluster finalizer %w", err)
	}

	instanceID, err := util.GetInstanceID(machineScope.LinodeMachine.Spec.ProviderID)
	if err != nil {
		logger.Error(err, "Failed to parse instance ID from provider ID")
		return ctrl.Result{}, err
	}

	if err := machineScope.LinodeClient.DeleteInstance(ctx, instanceID); err != nil {
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to delete Linode instance")

			if machineScope.LinodeMachine.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerRetryDelay)).After(time.Now()) {
				logger.Info("re-queuing Linode instance deletion")

				return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerRetryDelay}, nil
			}

			return ctrl.Result{}, err
		}
	}

	conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "instance deleted")

	r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeNormal, clusterv1.DeletedReason, "instance has cleaned up")

	machineScope.LinodeMachine.Spec.ProviderID = nil
	machineScope.LinodeMachine.Status.InstanceState = nil

	if err := machineScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to update credentials secret")
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(machineScope.LinodeMachine, infrav1alpha2.MachineFinalizer)
	// TODO: remove this check and removal later
	if controllerutil.ContainsFinalizer(machineScope.LinodeMachine, infrav1alpha1.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(machineScope.LinodeMachine, infrav1alpha1.GroupVersion.String())
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeMachineReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	linodeMachineMapper, err := kutil.ClusterToTypedObjectsMapper(
		r.TracedClient(),
		&infrav1alpha2.LinodeMachineList{},
		mgr.GetScheme(),
	)
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeMachines: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodeMachine{}).
		WithOptions(options).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(kutil.MachineToInfrastructureMapFunc(infrav1alpha2.GroupVersion.WithKind("LinodeMachine"))),
		).
		Watches(
			&infrav1alpha2.LinodeCluster{},
			handler.EnqueueRequestsFromMapFunc(r.linodeClusterToLinodeMachines(mgr.GetLogger())),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeMachineMapper),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
		).
		// we care about reconciling on metadata updates for LinodeMachines because the OwnerRef for the Machine is needed
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue)).
		Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeMachineReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
