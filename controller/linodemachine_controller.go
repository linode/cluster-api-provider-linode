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
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-resty/resty/v2"
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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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

type PostRequestCounter struct {
	reqRemaining int
	refreshTime  int
}

var (
	mu       sync.RWMutex
	tokenMap = make(map[string]*PostRequestCounter, 0)
)

// LinodeMachineReconciler reconciles a LinodeMachine object
type LinodeMachineReconciler struct {
	client.Client
	Recorder           record.EventRecorder
	LinodeClientConfig scope.ClientConfig
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
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch LinodeMachine")
		return ctrl.Result{}, err
	}

	machine, err := kutil.GetOwnerMachine(ctx, r.TracedClient(), linodeMachine.ObjectMeta)
	if err != nil || machine == nil {
		return ctrl.Result{}, err
	}
	log = log.WithValues("LinodeMachine", machine.Name)

	cluster, err := kutil.GetClusterFromMetadata(ctx, r.TracedClient(), machine.ObjectMeta)
	if err != nil || cluster == nil {
		return ctrl.Result{}, err
	}

	linodeClusterKey := client.ObjectKey{
		Namespace: linodeMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	linodeCluster := &infrav1alpha2.LinodeCluster{}
	if err := r.Client.Get(ctx, linodeClusterKey, linodeCluster); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, fmt.Errorf("get linodecluster %q: %w", linodeClusterKey, err)
		}
	}
	log = log.WithValues("LinodeCluster", linodeCluster.Name)

	machineScope, err := scope.NewMachineScope(
		ctx,
		r.LinodeClientConfig,
		scope.MachineScopeParams{
			Client:        r.TracedClient(),
			Cluster:       cluster,
			Machine:       machine,
			LinodeCluster: linodeCluster,
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

			conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, "%s", err.Error())

			r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist any LinodeMachine changes.
		// This ignores any resource not found errors when reconciling deletions.
		if patchErr := machineScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch LinodeMachine and LinodeCluster")
			err = errors.Join(err, patchErr)
		}
	}()

	// Add the finalizer if not already there
	if err = machineScope.AddFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Override the controller credentials with ones from the Machine's Secret reference (if supplied).
	// Credentials will be used in the following order:
	//   1. LinodeMachine
	//   2. Owner LinodeCluster
	//   3. Controller
	if machineScope.LinodeMachine.Spec.CredentialsRef != nil || machineScope.LinodeCluster.Spec.CredentialsRef != nil {
		if err := machineScope.SetCredentialRefTokenForLinodeClients(ctx); err != nil {
			logger.Error(err, "failed to update linode client token from Credential Ref")
			return ctrl.Result{}, err
		}
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
	if machineScope.LinodeMachine.Status.InstanceState != nil {
		failureReason = cerrs.UpdateMachineError
		return r.reconcileUpdate(ctx, logger, machineScope)
	}

	// Create
	failureReason = cerrs.CreateMachineError
	return r.reconcileCreate(ctx, logger, machineScope)
}

//nolint:cyclop // can't make it simpler with existing API
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

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightCreated) && machineScope.LinodeMachine.Spec.ProviderID == nil {
		res, err := r.reconcilePreflightCreate(ctx, logger, machineScope)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}

	instanceID, err := util.GetInstanceID(machineScope.LinodeMachine.Spec.ProviderID)
	if err != nil {
		logger.Error(err, "Failed to parse instance ID from provider ID")
		return ctrl.Result{}, err
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightConfigured) {
		res, err := r.reconcilePreflightConfigure(ctx, instanceID, logger, machineScope)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered) {
		res, err := r.reconcilePreflightBoot(ctx, instanceID, logger, machineScope)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightReady) {
		res, err := r.reconcilePreflightReady(ctx, instanceID, logger, machineScope)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}
	// Set the instance state to signal preflight process is done
	machineScope.LinodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightCreate(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	// get the bootstrap data for the Linode instance and set it for create config
	createOpts, err := newCreateConfig(ctx, machineScope, logger)
	if err != nil {
		logger.Error(err, "Failed to create Linode machine InstanceCreateOptions")
		return retryIfTransient(err)
	}

	mu.Lock()
	if isPOSTLimitReached(r.LinodeClientConfig.Token, logger) {
		mu.Unlock()
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	machineScope.LinodeClient.OnAfterResponse(r.apiResponseRatelimitCounter)
	linodeInstance, err := machineScope.LinodeClient.CreateInstance(ctx, *createOpts)
	mu.Unlock()

	if err != nil {
		logger.Error(err, "Failed to create Linode machine instance")
		if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
			ConditionPreflightCreated, string(cerrs.CreateMachineError), err.Error(),
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			return ctrl.Result{}, err
		}
		return retryIfTransient(err)
	}
	conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightCreated)
	// Set the provider ID since the instance is successfully created
	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightConfigure(ctx context.Context, instanceID int, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	if err := configureDisks(ctx, logger, machineScope, instanceID); err != nil {
		if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
			ConditionPreflightConfigured, string(cerrs.CreateMachineError), err.Error(),
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
	}
	if machineScope.LinodeMachine.Spec.Configuration != nil && machineScope.LinodeMachine.Spec.Configuration.Kernel != "" {
		instanceConfig, err := getDefaultInstanceConfig(ctx, machineScope, instanceID)
		if err != nil {
			logger.Error(err, "Failed to get default instance configuration")
			return retryIfTransient(err)
		}

		if _, err := machineScope.LinodeClient.UpdateInstanceConfig(ctx, instanceID, instanceConfig.ID, linodego.InstanceConfigUpdateOptions{Kernel: machineScope.LinodeMachine.Spec.Configuration.Kernel}); err != nil {
			logger.Error(err, "Failed to update default instance configuration")
			return retryIfTransient(err)
		}
	}
	conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightConfigured)
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightBoot(ctx context.Context, instanceID int, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	if err := machineScope.LinodeClient.BootInstance(ctx, instanceID, 0); err != nil && !strings.HasSuffix(err.Error(), "already booted.") {
		logger.Error(err, "Failed to boot instance")
		if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
			ConditionPreflightBootTriggered, string(cerrs.CreateMachineError), err.Error(),
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
	}
	conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered)
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightReady(ctx context.Context, instanceID int, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	addrs, err := buildInstanceAddrs(ctx, machineScope, instanceID)
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
	// update the status
	machineScope.LinodeMachine.Status.InstanceState = &linodeInstance.Status
	// decide to requeue
	if _, ok := requeueInstanceStatuses[linodeInstance.Status]; ok {
		if linodeInstance.Updated.Add(reconciler.DefaultMachineControllerWaitForRunningTimeout).After(time.Now()) {
			logger.Info("Instance not yet ready", "status", linodeInstance.Status)
			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		} else {
			logger.Info("Instance not ready in time, skipping reconciliation", "status", linodeInstance.Status)
			conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "skipped due to long running operation")
		}
	} else if linodeInstance.Status != linodego.InstanceRunning {
		logger.Info("Instance has incompatible status, skipping reconciliation", "status", linodeInstance.Status)
		conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "incompatible status")
	} else {
		machineScope.LinodeMachine.Status.Ready = true
		conditions.MarkTrue(machineScope.LinodeMachine, clusterv1.ReadyCondition)
	}
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
			handler.EnqueueRequestsFromMapFunc(linodeClusterToLinodeMachines(mgr.GetLogger(), r.TracedClient())),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeMachineMapper),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
		).
		// we care about reconciling on metadata updates for LinodeMachines because the OwnerRef for the Machine is needed
		WithEventFilter(predicate.And(
			predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
			predicate.Funcs{UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject, okOld := e.ObjectOld.(*infrav1alpha2.LinodeMachine)
				newObject, okNew := e.ObjectNew.(*infrav1alpha2.LinodeMachine)
				if okOld && okNew && oldObject.Spec.ProviderID == nil && newObject.Spec.ProviderID != nil {
					// We just created the instance, don't enqueue and update
					return false
				}
				return true
			}},
		)).
		Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeMachineReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}

func (r *LinodeMachineReconciler) apiResponseRatelimitCounter(resp *resty.Response) error {
	if resp.Request.Method != "POST" || !strings.HasSuffix(resp.Request.URL, "/linode/instances") {
		return nil
	}

	postReqCtr, exists := tokenMap[r.LinodeClientConfig.Token]
	if !exists {
		postReqCtr = &PostRequestCounter{
			reqRemaining: 10,
			refreshTime:  0,
		}
		tokenMap[r.LinodeClientConfig.Token] = postReqCtr
	}

	var err error
	postReqCtr.reqRemaining, err = strconv.Atoi(resp.Header().Get("X-Ratelimit-Remaining"))
	if err != nil {
		return err
	}

	postReqCtr.refreshTime, err = strconv.Atoi(resp.Header().Get("X-Ratelimit-Reset"))
	if err != nil {
		return err
	}
	return nil
}

func isPOSTLimitReached(token string, logger logr.Logger) bool {
	postReqCtr, exists := tokenMap[token]
	if !exists {
		postReqCtr = &PostRequestCounter{
			reqRemaining: 10,
			refreshTime:  0,
		}
		tokenMap[token] = postReqCtr
	}

	logger.Info(fmt.Sprintf("Requests Remaining: %v, Refresh Time: %v, currentTime: %v", postReqCtr.reqRemaining, postReqCtr.refreshTime, time.Now().Unix()))
	if postReqCtr.reqRemaining == 5 || postReqCtr.reqRemaining == 0 {
		actualRefreshTime := postReqCtr.refreshTime
		if postReqCtr.reqRemaining == 5 {
			actualRefreshTime = postReqCtr.refreshTime - 15
		}
		if time.Now().Unix() <= int64(actualRefreshTime) {
			logger.Info("Cannot make more requests as max requests have been made. Waiting and retrying ...")
			return true
		} else if postReqCtr.reqRemaining == 0 {
			postReqCtr.reqRemaining = 10
		}
	}
	return false
}
