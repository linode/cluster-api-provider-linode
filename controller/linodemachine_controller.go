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

const (
	// default etcd disk size in MB
	defaultEtcdDiskSize    = 10240
	defaultDiskWaitSeconds = 5

	// conditions for preflight instance creation
	ConditionPreflightCreated          clusterv1.ConditionType = "PreflightCreated"
	ConditionPreflightRootDiskCreating clusterv1.ConditionType = "PreflightRootDiskCreating"
	ConditionPreflightRootDiskCreated  clusterv1.ConditionType = "PreflightRootDiskCreated"
	ConditionPreflightEtcdDiskCreating clusterv1.ConditionType = "PreflightEtcdDiskCreating"
	ConditionPreflightEtcdDiskCreated  clusterv1.ConditionType = "PreflightEtcdDiskCreated"
	ConditionPreflightConfigured       clusterv1.ConditionType = "PreflightConfigured"
	ConditionPreflightBootTriggered    clusterv1.ConditionType = "PreflightBootTriggered"
	ConditionPreflightReady            clusterv1.ConditionType = "PreflightReady"
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

//nolint:cyclop // keep top-level preflight condition checks in the same function for readability
func (r *LinodeMachineReconciler) reconcileCreate(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (ctrl.Result, error) {
	logger.Info("creating machine")

	tags := []string{machineScope.LinodeCluster.Name}

	listFilter := util.Filter{
		ID:    machineScope.LinodeMachine.Spec.InstanceID,
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
	var createOpts *linodego.InstanceCreateOptions

	switch len(linodeInstances) {
	case 1:
		logger.Info("Linode instance already exists")

		linodeInstance = &linodeInstances[0]
	case 0:
		// get the bootstrap data for the Linode instance and set it for create config
		createOpts, err = r.newCreateConfig(ctx, machineScope, tags, logger)
		if err != nil {
			logger.Error(err, "Failed to create Linode machine InstanceCreateOptions")

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		// Omit image, interfaces, and stackscript configuration when first creating a control plane node
		if kutil.IsControlPlaneMachine(machineScope.Machine) {
			createOptsSubset := *createOpts
			createOptsSubset.Image = ""
			createOptsSubset.Interfaces = nil
			createOptsSubset.StackScriptID = 0
			createOptsSubset.StackScriptData = nil
			linodeInstance, err = machineScope.LinodeClient.CreateInstance(ctx, createOptsSubset)
		} else {
			linodeInstance, err = machineScope.LinodeClient.CreateInstance(ctx, *createOpts)
		}
		if err != nil {
			logger.Error(err, "Failed to create Linode machine instance")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightCreated, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightCreated)
		machineScope.LinodeMachine.Spec.InstanceID = &linodeInstance.ID

	default:
		err = errors.New("multiple instances")
		logger.Error(err, "multiple instances found", "tags", tags)

		return ctrl.Result{}, err
	}

	if !reconciler.OneOfConditionsTrue(machineScope.LinodeMachine, ConditionPreflightConfigured) {
		if createOpts == nil {
			createOpts, err = r.newCreateConfig(ctx, machineScope, tags, logger)
			if err != nil {
				logger.Error(err, "Failed to create Linode machine InstanceCreateOptions")

				if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
					ConditionPreflightConfigured, string(cerrs.CreateMachineError), err.Error(),
					reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
					return ctrl.Result{}, err
				}

				return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
			}
		}

		if err = r.configureControlPlane(ctx, logger, machineScope, linodeInstance.ID, *createOpts); err != nil {
			logger.Error(err, "Failed to configure control plane")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightConfigured, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightConfigured)
	}

	if !reconciler.OneOfConditionsTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered) {
		if err = machineScope.LinodeClient.BootInstance(ctx, linodeInstance.ID, 0); err != nil {
			logger.Error(err, "Failed to boot instance")

			if reconciler.RecordDecayingCondition(machineScope.LinodeMachine,
				ConditionPreflightBootTriggered, string(cerrs.CreateMachineError), err.Error(),
				reconciler.DefaultMachineControllerPreflightTimeout(r.ReconcileTimeout)) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
		}

		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightBootTriggered)
	}

	if !reconciler.OneOfConditionsTrue(machineScope.LinodeMachine, ConditionPreflightReady) {
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

	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))
	machineScope.LinodeMachine.Status.Addresses = buildInstanceAddrs(linodeInstance)

	// Set the instance state to signal preflight process is done
	machineScope.LinodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)

	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) configureControlPlane(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
	createOpts linodego.InstanceCreateOptions,
) error {
	if !kutil.IsControlPlaneMachine(machineScope.Machine) {
		return nil
	}

	rootDisk, err := r.createRootDisk(ctx, logger, machineScope, linodeInstanceID, createOpts)
	if err != nil {
		return err
	}

	// TODO: Do we have to wait?
	if !reconciler.OneOfConditionsTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskCreated) {
		if _, err := machineScope.LinodeClient.WaitForInstanceDiskStatus(ctx, linodeInstanceID, rootDisk.ID, linodego.DiskReady, defaultDiskWaitSeconds); err != nil {
			conditions.MarkFalse(machineScope.LinodeMachine, ConditionPreflightRootDiskCreated, string(cerrs.CreateMachineError), clusterv1.ConditionSeverityWarning, err.Error())

			return err
		}

		conditions.Delete(machineScope.LinodeMachine, ConditionPreflightRootDiskCreating)
		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskCreated)
	}

	etcdDisk, err := r.createEtcdDisk(ctx, logger, machineScope, linodeInstanceID)
	if err != nil {
		return err
	}

	// TODO: Do we have to wait?
	if !reconciler.OneOfConditionsTrue(machineScope.LinodeMachine, ConditionPreflightEtcdDiskCreated) {
		if _, err := machineScope.LinodeClient.WaitForInstanceDiskStatus(ctx, linodeInstanceID, etcdDisk.ID, linodego.DiskReady, defaultDiskWaitSeconds); err != nil {
			conditions.MarkFalse(machineScope.LinodeMachine, ConditionPreflightEtcdDiskCreated, string(cerrs.CreateMachineError), clusterv1.ConditionSeverityWarning, err.Error())

			return err
		}

		conditions.Delete(machineScope.LinodeMachine, ConditionPreflightEtcdDiskCreating)
		conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightEtcdDiskCreated)
	}

	_, err = machineScope.LinodeClient.CreateInstanceConfig(
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

	return err
}

func (r *LinodeMachineReconciler) createRootDisk(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
	createOpts linodego.InstanceCreateOptions,
) (*linodego.InstanceDisk, error) {
	if reconciler.OneOfConditionsTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskCreated, ConditionPreflightRootDiskCreating) {
		listFilter := util.Filter{
			Label: "root",
		}
		filter, err := listFilter.String()
		if err != nil {
			return nil, err
		}

		disks, err := machineScope.LinodeClient.ListInstanceDisks(ctx, linodeInstanceID, linodego.NewListOptions(1, filter))
		if err != nil {
			return nil, err
		}

		if len(disks) != 1 {
			return nil, fmt.Errorf("unable to find expected root disk for instance %d", linodeInstanceID)
		}

		return &disks[0], nil
	}

	instanceType, err := machineScope.LinodeClient.GetType(ctx, createOpts.Type)
	if err != nil {
		logger.Error(err, "Failed to retrieve type for instance")

		conditions.MarkFalse(machineScope.LinodeMachine, ConditionPreflightRootDiskCreating, string(cerrs.CreateMachineError), clusterv1.ConditionSeverityWarning, err.Error())

		return nil, err
	}

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

		conditions.MarkFalse(machineScope.LinodeMachine, ConditionPreflightRootDiskCreating, string(cerrs.CreateMachineError), clusterv1.ConditionSeverityWarning, err.Error())

		return nil, err
	}

	conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskCreating)

	return rootDisk, nil
}

func (r *LinodeMachineReconciler) createEtcdDisk(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
) (*linodego.InstanceDisk, error) {
	if reconciler.OneOfConditionsTrue(machineScope.LinodeMachine, ConditionPreflightEtcdDiskCreated, ConditionPreflightEtcdDiskCreating) {
		listFilter := util.Filter{
			Label: "etcd-data",
		}
		filter, err := listFilter.String()
		if err != nil {
			return nil, err
		}

		disks, err := machineScope.LinodeClient.ListInstanceDisks(ctx, linodeInstanceID, linodego.NewListOptions(1, filter))
		if err != nil {
			return nil, err
		}

		if len(disks) != 1 {
			return nil, fmt.Errorf("unable to find expected etcd disk for instance %d", linodeInstanceID)
		}

		return &disks[0], nil
	}

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

		conditions.MarkFalse(machineScope.LinodeMachine, ConditionPreflightEtcdDiskCreating, string(cerrs.CreateMachineError), clusterv1.ConditionSeverityWarning, err.Error())

		return nil, err
	}

	conditions.MarkTrue(machineScope.LinodeMachine, ConditionPreflightEtcdDiskCreating)

	return etcdDisk, nil
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
