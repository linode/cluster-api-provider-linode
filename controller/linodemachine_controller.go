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
	cerrs "sigs.k8s.io/cluster-api/errors"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// default etcd Disk size in MB
const defaultEtcdDiskSize = 10240

var skippedMachinePhases = map[string]bool{
	string(clusterv1.MachinePhasePending): true,
	string(clusterv1.MachinePhaseFailed):  true,
	string(clusterv1.MachinePhaseUnknown): true,
}

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
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	Scheme           *runtime.Scheme
	ReconcileTimeout time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachines/finalizers,verbs=update

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

	linodeMachine := &infrav1alpha1.LinodeMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, linodeMachine); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch LinodeMachine")
		}

		return ctrl.Result{}, err
	}

	machine, err := r.getOwnerMachine(ctx, *linodeMachine, log)
	if err != nil || machine == nil {
		return ctrl.Result{}, err
	}
	log = log.WithValues("LinodeMachine", machine.Name)

	cluster, err := r.getClusterFromMetadata(ctx, *machine, log)
	if err != nil || cluster == nil {
		return ctrl.Result{}, err
	}

	machineScope, err := scope.NewMachineScope(
		ctx,
		r.LinodeApiKey,
		scope.MachineScopeParams{
			Client:        r.Client,
			Cluster:       cluster,
			Machine:       machine,
			LinodeCluster: &infrav1alpha1.LinodeCluster{},
			LinodeMachine: linodeMachine,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create machine scope")

		return ctrl.Result{}, fmt.Errorf("failed to create machine scope: %w", err)
	}

	return r.reconcile(ctx, log, machineScope)
}

func (r *LinodeMachineReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (res ctrl.Result, err error) {
	res = ctrl.Result{}

	machineScope.LinodeMachine.Status.Ready = false
	machineScope.LinodeMachine.Status.FailureReason = nil
	machineScope.LinodeMachine.Status.FailureMessage = util.Pointer("")

	failureReason := cerrs.MachineStatusError("UnknownError")
	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if err != nil {
			machineScope.LinodeMachine.Status.FailureReason = util.Pointer(failureReason)
			machineScope.LinodeMachine.Status.FailureMessage = util.Pointer(err.Error())

			conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, err.Error())

			r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist any LinodeMachine changes.
		if patchErr := machineScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(patchErr, apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch LinodeMachine")

			err = errors.Join(err, patchErr)
		}
	}()

	// Add the finalizer if not already there
	err = machineScope.AddFinalizer(ctx)
	if err != nil {
		logger.Error(err, "Failed to add finalizer")

		return
	}

	// Delete
	if !machineScope.LinodeMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = cerrs.DeleteMachineError

		err = r.reconcileDelete(ctx, logger, machineScope)

		return
	}

	linodeClusterKey := client.ObjectKey{
		Namespace: machineScope.LinodeMachine.Namespace,
		Name:      machineScope.Cluster.Spec.InfrastructureRef.Name,
	}

	if err = r.Client.Get(ctx, linodeClusterKey, machineScope.LinodeCluster); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			logger.Error(err, "Failed to fetch Linode cluster")
		}

		return
	}

	// Set the newest retrieved instance state once after all operations are done
	var linodeInstance *linodego.Instance
	defer func() {
		machineScope.LinodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)
		if linodeInstance != nil {
			machineScope.LinodeMachine.Status.InstanceState = &linodeInstance.Status
		}
	}()

	// Update
	if machineScope.LinodeMachine.Status.PreflightState == infrav1alpha1.MachinePreflightReady {
		failureReason = cerrs.UpdateMachineError

		logger = logger.WithValues("ID", *machineScope.LinodeMachine.Spec.InstanceID)

		res, linodeInstance, err = r.reconcileUpdate(ctx, logger, machineScope)

		return
	}

	// Create
	failureReason = cerrs.CreateMachineError
	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap data secret is not yet available")
		res = ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForBootstrapDelay}

		return
	}
	res, linodeInstance, err = r.reconcileCreate(ctx, logger, machineScope)

	return
}

func (r *LinodeMachineReconciler) reconcileCreate(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (ctrl.Result, *linodego.Instance, error) {
	logger.Info("creating machine")

	tags := []string{machineScope.LinodeCluster.Name}

	listFilter := util.Filter{
		ID:    machineScope.LinodeMachine.Spec.InstanceID,
		Label: machineScope.LinodeMachine.Name,
		Tags:  tags,
	}
	filter, err := listFilter.String()
	if err != nil {
		return ctrl.Result{}, nil, err
	}
	linodeInstances, err := machineScope.LinodeClient.ListInstances(ctx, linodego.NewListOptions(1, filter))
	if err != nil {
		logger.Error(err, "Failed to list Linode machine instances")

		// TODO: What transient errors returned from the API should we requeue on?
		return ctrl.Result{}, nil, err
	}

	if kutil.IsControlPlaneMachine(machineScope.Machine) {
		return r.reconcileCreateControlNode(ctx, logger, machineScope, linodeInstances, tags)
	}

	return r.reconcileCreateWorkerNode(ctx, logger, machineScope, linodeInstances, tags)
}

func (r *LinodeMachineReconciler) reconcileCreateWorkerNode(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstances []linodego.Instance,
	tags []string,
) (ctrl.Result, *linodego.Instance, error) {
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

			return ctrl.Result{}, nil, err
		}

		linodeInstance, err = machineScope.LinodeClient.CreateInstance(ctx, *createOpts)
		// TODO: Investigate why there is an extra nil check on linodeInstance if err is already not nil
		if err != nil || linodeInstance == nil {
			logger.Error(err, "Failed to create Linode machine instance")

			// TODO: What transient errors returned from the API should we requeue on?
			return ctrl.Result{}, nil, err
		}
		machineScope.LinodeMachine.Spec.InstanceID = &linodeInstance.ID
		machineScope.LinodeMachine.Status.PreflightState = infrav1alpha1.MachinePreflightCreated

	default:
		err := errors.New("multiple instances")
		logger.Error(err, "multiple instances found", "tags", tags)

		return ctrl.Result{}, nil, err
	}

	if machineScope.LinodeMachine.Status.PreflightState < infrav1alpha1.MachinePreflightReady {
		if linodeInstance.Status == linodego.InstanceRunning {
			logger.Info("Linode instance already running")
		} else if err := machineScope.LinodeClient.BootInstance(ctx, linodeInstance.ID, 0); err != nil {
			logger.Error(err, "Failed to boot instance")

			// TODO: What transient errors returned from the API should we requeue on?
			return ctrl.Result{}, linodeInstance, err
		}
		machineScope.LinodeMachine.Status.PreflightState = infrav1alpha1.MachinePreflightReady
	}

	machineScope.LinodeMachine.Status.Ready = true
	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))
	machineScope.LinodeMachine.Status.Addresses = buildInstanceAddrs(linodeInstance)

	return ctrl.Result{}, linodeInstance, nil
}

func (r *LinodeMachineReconciler) reconcileCreateControlNode(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstances []linodego.Instance,
	tags []string,
) (ctrl.Result, *linodego.Instance, error) {
	// get the bootstrap data for the Linode instance and set it for create config
	createOpts, err := r.newCreateConfig(ctx, machineScope, tags, logger)
	if err != nil {
		logger.Error(err, "Failed to create Linode machine InstanceCreateOptions")

		return ctrl.Result{}, nil, err
	}

	var linodeInstance *linodego.Instance

	switch len(linodeInstances) {
	case 1:
		logger.Info("Linode instance already exists")

		linodeInstance = &linodeInstances[0]

	case 0:
		// Omit image and interfaces when creating the instance to configure disks and config profile manually
		image := createOpts.Image
		createOpts.Image = ""
		interfaces := createOpts.Interfaces
		createOpts.Interfaces = nil

		linodeInstance, err = machineScope.LinodeClient.CreateInstance(ctx, *createOpts)
		if err != nil || linodeInstance == nil {
			logger.Error(err, "Failed to create Linode machine instance")

			// TODO: What transient errors returned from the API should we requeue on?
			return ctrl.Result{}, nil, err
		}
		machineScope.LinodeMachine.Spec.InstanceID = &linodeInstance.ID
		machineScope.LinodeMachine.Status.PreflightState = infrav1alpha1.MachinePreflightCreated
		createOpts.Image = image
		createOpts.Interfaces = interfaces

	default:
		err := errors.New("multiple instances")
		logger.Error(err, "multiple instances found", "tags", tags)

		return ctrl.Result{}, linodeInstance, err
	}

	var instanceConfig *linodego.InstanceConfig

	if machineScope.LinodeMachine.Status.PreflightState < infrav1alpha1.MachinePreflightConfigured {
		instanceConfig, err = r.configureControlPlane(ctx, logger, machineScope, linodeInstance.ID, *createOpts)
		if err != nil {
			logger.Error(err, "Failed to configure instance profile")

			// TODO: What transient errors returned from the API should we requeue on?
			return ctrl.Result{}, linodeInstance, err
		}
		machineScope.LinodeMachine.Status.PreflightState = infrav1alpha1.MachinePreflightConfigured
	}

	if machineScope.LinodeMachine.Status.PreflightState < infrav1alpha1.MachinePreflightDisksReady {
		if instanceConfig == nil {
			configs, err := machineScope.LinodeClient.ListInstanceConfigs(ctx, linodeInstance.ID, &linodego.ListOptions{})
			if err != nil || len(configs) == 0 {
				logger.Error(err, "Failed to list instance configs")

				// TODO: What transient errors returned from the API should we requeue on?
				return ctrl.Result{}, linodeInstance, err
			}
			instanceConfig = &configs[0]
		}

		ok, err := r.checkControlPlaneDiskStatuses(ctx, machineScope, linodeInstance.ID, instanceConfig)
		if err != nil {
			logger.Error(err, "Failed to check instance disks statuses")

			// TODO: What terminal errors returned from the API should we NOT requeue on?
			// For now, always requeue since we expect this to initially error
			// and don't return an error so we don't trigger controller-manager's incremental backoff
			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForControlPlaneDisksDelay}, nil, nil
		}
		if !ok {
			logger.Info("Waiting for control plane disks to become ready")
			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForControlPlaneDisksDelay}, nil, nil
		}
		logger.Info("Control plane disks are ready")
		machineScope.LinodeMachine.Status.PreflightState = infrav1alpha1.MachinePreflightDisksReady
	}

	if machineScope.LinodeMachine.Status.PreflightState < infrav1alpha1.MachinePreflightBooted {
		if linodeInstance.Status == linodego.InstanceRunning {
			logger.Info("Linode instance already running")
		} else if err := machineScope.LinodeClient.BootInstance(ctx, linodeInstance.ID, 0); err != nil {
			logger.Error(err, "Failed to boot instance")

			// TODO: What transient errors returned from the API should we requeue on?
			return ctrl.Result{}, linodeInstance, err
		}
		machineScope.LinodeMachine.Status.PreflightState = infrav1alpha1.MachinePreflightBooted
	}

	machineScope.LinodeMachine.Status.Ready = true
	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))
	machineScope.LinodeMachine.Status.Addresses = buildInstanceAddrs(linodeInstance)

	if machineScope.LinodeMachine.Status.PreflightState < infrav1alpha1.MachinePreflightReady {
		if err := services.AddNodeToNB(ctx, logger, machineScope); err != nil {
			logger.Error(err, "Failed to add instance to Node Balancer backend")

			// TODO: What transient errors returned from the API should we requeue on?
			return ctrl.Result{}, linodeInstance, err
		}
		machineScope.LinodeMachine.Status.PreflightState = infrav1alpha1.MachinePreflightReady
	}

	return ctrl.Result{}, linodeInstance, nil
}

func (r *LinodeMachineReconciler) configureControlPlane(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
	createOpts linodego.InstanceCreateOptions,
) (*linodego.InstanceConfig, error) {
	instanceType, err := machineScope.LinodeClient.GetType(ctx, createOpts.Type)
	if err != nil {
		logger.Error(err, "Failed to retrieve type for instance")

		return nil, err
	}

	// create the root disk
	rootDisk, err := machineScope.LinodeClient.CreateInstanceDisk(
		ctx,
		linodeInstanceID,
		linodego.InstanceDiskCreateOptions{
			Label:           "root",
			Size:            instanceType.Disk - defaultEtcdDiskSize,
			Image:           createOpts.Image,
			RootPass:        createOpts.RootPass,
			Filesystem:      string(linodego.FilesystemExt4),
			AuthorizedKeys:  createOpts.AuthorizedKeys,
			AuthorizedUsers: createOpts.AuthorizedUsers,
			StackscriptID:   createOpts.StackScriptID,
			StackscriptData: createOpts.StackScriptData,
		},
	)
	if err != nil {
		logger.Error(err, "Failed to create root disk")

		return nil, err
	}

	// create the etcd disk
	etcdDisk, err := machineScope.LinodeClient.CreateInstanceDisk(
		ctx,
		linodeInstanceID,
		linodego.InstanceDiskCreateOptions{
			Label:      "etcd-data",
			Size:       defaultEtcdDiskSize,
			Filesystem: string(linodego.FilesystemExt4),
		},
	)
	if err != nil {
		logger.Error(err, "Failed to create etcd disk")

		return nil, err
	}

	instanceConfig, err := machineScope.LinodeClient.CreateInstanceConfig(
		ctx,
		linodeInstanceID,
		linodego.InstanceConfigCreateOptions{
			Label: fmt.Sprintf("%s disk profile", createOpts.Image),
			Devices: linodego.InstanceConfigDeviceMap{
				SDA: &linodego.InstanceConfigDevice{DiskID: rootDisk.ID},
				SDB: &linodego.InstanceConfigDevice{DiskID: etcdDisk.ID},
			},
			Helpers: &linodego.InstanceConfigHelpers{
				UpdateDBDisabled:  true,
				Distro:            true,
				ModulesDep:        true,
				Network:           true,
				DevTmpFsAutomount: true,
			},
			Interfaces: createOpts.Interfaces,
			Kernel:     "linode/grub2",
		},
	)
	if err != nil {
		logger.Error(err, "Failed to create config profile for instance")

		return nil, err
	}

	return instanceConfig, nil
}

func (r *LinodeMachineReconciler) checkControlPlaneDiskStatuses(
	ctx context.Context,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
	instanceConfig *linodego.InstanceConfig,
) (bool, error) {
	rootDisk, err := machineScope.LinodeClient.GetInstanceDisk(ctx, linodeInstanceID, instanceConfig.Devices.SDA.DiskID)
	if err != nil {
		return false, err
	}

	etcdDisk, err := machineScope.LinodeClient.GetInstanceDisk(ctx, linodeInstanceID, instanceConfig.Devices.SDB.DiskID)
	if err != nil {
		return false, err
	}

	return rootDisk.Status == linodego.DiskReady && etcdDisk.Status == linodego.DiskReady, nil
}

func (r *LinodeMachineReconciler) reconcileUpdate(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (res reconcile.Result, linodeInstance *linodego.Instance, err error) {
	logger.Info("updating machine")

	res = ctrl.Result{}

	if machineScope.LinodeMachine.Spec.InstanceID == nil {
		return res, nil, errors.New("missing instance ID")
	}

	if linodeInstance, err = machineScope.LinodeClient.GetInstance(ctx, *machineScope.LinodeMachine.Spec.InstanceID); err != nil {
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to get Linode machine instance")
		} else {
			logger.Info("Instance not found, let's create a new one")

			// Create new machine
			machineScope.LinodeMachine.Spec.ProviderID = nil
			machineScope.LinodeMachine.Spec.InstanceID = nil

			conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, "missing", clusterv1.ConditionSeverityWarning, "instance not found")
		}

		return res, nil, err
	}

	if _, ok := requeueInstanceStatuses[linodeInstance.Status]; ok {
		if linodeInstance.Updated.Add(reconciler.DefaultMachineControllerWaitForRunningTimeout).After(time.Now()) {
			logger.Info("Instance has one operaton running, re-queuing reconciliation", "status", linodeInstance.Status)

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, linodeInstance, nil
		}

		logger.Info("Instance has one operaton long running, skipping reconciliation", "status", linodeInstance.Status)

		conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "skipped due to long running operation")

		return res, linodeInstance, nil
	} else if linodeInstance.Status != linodego.InstanceRunning {
		logger.Info("Instance has incompatible status, skipping reconciliation", "status", linodeInstance.Status)

		conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "incompatible status")

		return res, linodeInstance, nil
	}

	machineScope.LinodeMachine.Status.Ready = true

	conditions.MarkTrue(machineScope.LinodeMachine, clusterv1.ReadyCondition)

	r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeNormal, string(clusterv1.ReadyCondition), "instance is running")

	return res, linodeInstance, nil
}

func (r *LinodeMachineReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) error {
	logger.Info("deleting machine")

	if machineScope.LinodeMachine.Spec.InstanceID == nil {
		logger.Info("Machine ID is missing, nothing to do")
		controllerutil.RemoveFinalizer(machineScope.LinodeMachine, infrav1alpha1.GroupVersion.String())

		return nil
	}

	if err := services.DeleteNodeFromNB(ctx, logger, machineScope); err != nil {
		logger.Error(err, "Failed to remove node from Node Balancer backend")

		return err
	}

	if err := machineScope.LinodeClient.DeleteInstance(ctx, *machineScope.LinodeMachine.Spec.InstanceID); err != nil {
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to delete Linode machine instance")

			return err
		}
	}

	conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "instance deleted")

	r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeNormal, clusterv1.DeletedReason, "instance has cleaned up")

	machineScope.LinodeMachine.Spec.ProviderID = nil
	machineScope.LinodeMachine.Spec.InstanceID = nil
	controllerutil.RemoveFinalizer(machineScope.LinodeMachine, infrav1alpha1.GroupVersion.String())

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeMachine{}).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(kutil.MachineToInfrastructureMapFunc(infrav1alpha1.GroupVersion.WithKind("LinodeMachine"))),
		).
		Watches(
			&infrav1alpha1.LinodeCluster{},
			handler.EnqueueRequestsFromMapFunc(r.linodeClusterToLinodeMachines(mgr.GetLogger())),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue)).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return controller.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(r.requeueLinodeMachinesForUnpausedCluster(mgr.GetLogger())),
		predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger()),
	)
}
