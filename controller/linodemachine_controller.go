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
	"k8s.io/apimachinery/pkg/api/resource"
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

const (
	linodeBusyCode        = 400
	defaultDiskFilesystem = string(linodego.FilesystemExt4)

	// conditions for preflight instance creation
	ConditionPreflightCreated                clusterv1.ConditionType = "PreflightCreated"
	ConditionPreflightRootDiskCreated        clusterv1.ConditionType = "PreflightRootDiskCreated"
	ConditionPreflightAdditionalDisksCreated clusterv1.ConditionType = "PreflightAdditionalDisksCreated"
	ConditionPreflightConfigured             clusterv1.ConditionType = "PreflightConfigured"
	ConditionPreflightBootTriggered          clusterv1.ConditionType = "PreflightBootTriggered"
	ConditionPreflightReady                  clusterv1.ConditionType = "PreflightReady"
)

var skippedMachinePhases = map[string]bool{
	string(clusterv1.MachinePhasePending): true,
	string(clusterv1.MachinePhaseFailed):  true,
	string(clusterv1.MachinePhaseUnknown): true,
}

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

	// Update
	if machineScope.LinodeMachine.Status.InstanceState != nil {
		var linodeInstance *linodego.Instance
		defer func() {
			if linodeInstance != nil {
				machineScope.LinodeMachine.Status.InstanceState = &linodeInstance.Status
			}
		}()

		failureReason = cerrs.UpdateMachineError

		if machineScope.LinodeMachine.Spec.InstanceID != nil {
			logger = logger.WithValues("ID", *machineScope.LinodeMachine.Spec.InstanceID)
		}

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
	res, err = r.reconcileCreate(ctx, logger, machineScope)

	return
}

func (r *LinodeMachineReconciler) reconcileCreate(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (ctrl.Result, error) {
	logger.Info("create/init machine")

	var createOpts *linodego.InstanceCreateOptions
	var err error

	tags := []string{machineScope.LinodeCluster.Name}

	if !conditions.IsTrue(machineScope.LinodeMachine, ConditionPreflightCreated) {
		// get the bootstrap data for the Linode instance and set it for create config
		createOpts, err = r.newCreateConfig(ctx, machineScope, tags, logger)
		if err != nil {
			logger.Error(err, "Failed to create Linode machine InstanceCreateOptions")

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		linodeInstance, err := r.createLinodeInstance(ctx, machineScope, createOpts)
		if err != nil {
			logger.Error(err, "Failed to get/create Linode machine instance")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightCreated, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		machineScope.LinodeMachine.Spec.InstanceID = &linodeInstance.ID
		machineScope.LinodeMachine.Status.Addresses = buildInstanceAddrs(linodeInstance)
		machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))
		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightCreated)
	}

	if !conditions.IsTrue(machineScope.LinodeMachine, ConditionPreflightConfigured) {
		if err = r.configureDisks(ctx, logger, machineScope, *machineScope.LinodeMachine.Spec.InstanceID, createOpts, tags); err != nil {
			if !linodego.ErrHasStatus(err, linodeBusyCode) {
				logger.Error(err, "Failed to configure disks")
			}

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightConfigured, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightConfigured)
	}

	if !conditions.IsTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered) {
		if err = machineScope.LinodeClient.BootInstance(ctx, *machineScope.LinodeMachine.Spec.InstanceID, 0); err != nil {
			if !linodego.ErrHasStatus(err, linodeBusyCode) {
				logger.Error(err, "Failed to boot instance")
			}

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightBootTriggered, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered)
	}

	if !conditions.IsTrue(machineScope.LinodeMachine, ConditionPreflightReady) {
		if err = services.AddNodeToNB(ctx, logger, machineScope); err != nil {
			logger.Error(err, "Failed to add instance to Node Balancer backend")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightReady, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightReady)
	}

	// Set the instance state to signal preflight process is done
	machineScope.LinodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)

	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) createLinodeInstance(
	ctx context.Context,
	machineScope *scope.MachineScope,
	createOpts *linodego.InstanceCreateOptions,
) (inst *linodego.Instance, err error) {
	if machineScope.LinodeMachine.Spec.DataDisks == nil && machineScope.LinodeMachine.Spec.OSDisk == nil {
		inst, err = machineScope.LinodeClient.CreateInstance(ctx, *createOpts)
	} else {
		// If disks are customized, omit image, interfaces, and stackscript config during creation
		createOptsSubset := *createOpts
		createOptsSubset.Image = ""
		createOptsSubset.Interfaces = nil
		createOptsSubset.StackScriptID = 0
		createOptsSubset.StackScriptData = nil
		inst, err = machineScope.LinodeClient.CreateInstance(ctx, createOptsSubset)
	}

	return
}

func (r *LinodeMachineReconciler) configureDisks(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
	createOpts *linodego.InstanceCreateOptions,
	tags []string,
) error {
	if machineScope.LinodeMachine.Spec.DataDisks == nil && machineScope.LinodeMachine.Spec.OSDisk == nil {
		return nil
	}

	if createOpts == nil {
		cOpts, err := r.newCreateConfig(ctx, machineScope, tags, logger)
		if err != nil {
			return err
		}

		createOpts = cOpts
	}

	if machineScope.LinodeMachine.Spec.DataDisks != nil && !conditions.IsTrue(machineScope.LinodeMachine, ConditionPreflightAdditionalDisksCreated) {
		for deviceName, disk := range machineScope.LinodeMachine.Spec.DataDisks {
			if err := r.createDisk(ctx, logger, machineScope, linodeInstanceID, *createOpts, deviceName, disk); err != nil {
				conditions.MarkFalse(
					machineScope.LinodeMachine,
					ConditionPreflightAdditionalDisksCreated,
					string(cerrs.CreateMachineError),
					clusterv1.ConditionSeverityWarning,
					err.Error(),
				)

				return err
			}
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightAdditionalDisksCreated)
	}

	if !conditions.IsTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskCreated) {
		if err := r.createDisk(ctx, logger, machineScope, linodeInstanceID, *createOpts, "sda", machineScope.LinodeMachine.Spec.OSDisk); err != nil {
			conditions.MarkFalse(
				machineScope.LinodeMachine,
				ConditionPreflightRootDiskCreated,
				string(cerrs.CreateMachineError),
				clusterv1.ConditionSeverityWarning,
				err.Error(),
			)

			return err
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskCreated)
	}

	deviceMap := linodego.InstanceConfigDeviceMap{
		SDA: &linodego.InstanceConfigDevice{DiskID: machineScope.LinodeMachine.Spec.OSDisk.DiskID},
	}
	if err := createInstanceConfigDeviceMap(machineScope.LinodeMachine.Spec.DataDisks, &deviceMap); err != nil {
		return err
	}

	_, err := machineScope.LinodeClient.CreateInstanceConfig(
		ctx,
		linodeInstanceID,
		linodego.InstanceConfigCreateOptions{
			Label:   fmt.Sprintf("%s disk profile", createOpts.Image),
			Devices: deviceMap,
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

	return err
}

func (r *LinodeMachineReconciler) createDisk(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
	instCreateOpts linodego.InstanceCreateOptions,
	deviceName string,
	disk *infrav1alpha1.InstanceDisk,
) error {
	// If the root disk is not customized, use the defaults from the instance type
	if deviceName == "sda" && disk == nil {
		instType, err := machineScope.LinodeClient.GetType(ctx, instCreateOpts.Type)
		if err != nil {
			logger.Error(err, "Failed to retrieve type for instance")

			return err
		}

		disk = &infrav1alpha1.InstanceDisk{
			Size:  *resource.NewScaledQuantity(int64(instType.Disk), resource.Mega),
			Label: "root",
		}
		machineScope.LinodeMachine.Spec.OSDisk = disk
	}

	if disk.DiskID != 0 {
		return nil
	}

	label := disk.Label
	if label == "" {
		label = deviceName
	}

	diskFilesystem := defaultDiskFilesystem
	if disk.Filesystem != "" {
		diskFilesystem = disk.Filesystem
	}

	createOpts := linodego.InstanceDiskCreateOptions{
		Label:      label,
		Size:       int(disk.Size.ScaledValue(resource.Mega)),
		Filesystem: diskFilesystem,
	}
	if deviceName == "sda" {
		var additionalDiskSize int
		if machineScope.LinodeMachine.Spec.DataDisks != nil {
			for _, disk := range machineScope.LinodeMachine.Spec.DataDisks {
				additionalDiskSize += int(disk.Size.ScaledValue(resource.Mega))
			}
		}
		createOpts.Size = createOpts.Size - additionalDiskSize
		createOpts.Image = instCreateOpts.Image
		createOpts.RootPass = instCreateOpts.RootPass
		createOpts.AuthorizedKeys = instCreateOpts.AuthorizedKeys
		createOpts.AuthorizedUsers = instCreateOpts.AuthorizedUsers
		createOpts.StackscriptID = instCreateOpts.StackScriptID
		createOpts.StackscriptData = instCreateOpts.StackScriptData
	}

	linodeDisk, err := machineScope.LinodeClient.CreateInstanceDisk(ctx, linodeInstanceID, createOpts)
	if err != nil {
		if !linodego.ErrHasStatus(err, linodeBusyCode) {
			logger.Error(err, "Failed to create disk", "DiskLabel", label)
		}

		return err
	}

	disk.DiskID = linodeDisk.ID

	return nil
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
			machineScope.LinodeMachine.Status.InstanceState = nil

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
	machineScope.LinodeMachine.Status.InstanceState = nil
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

	linodeMachineMapper, err := kutil.ClusterToTypedObjectsMapper(r.Client, &infrav1alpha1.LinodeMachineList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeMachines: %w", err)
	}

	return controller.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(linodeMachineMapper),
		predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger()),
	)
}
