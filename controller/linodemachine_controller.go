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
//
//nolint:gocyclo,cyclop // As simple as possible.
func (r *LinodeMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx).WithName("LinodeMachineReconciler").WithValues("name", req.NamespacedName.String())

	linodeMachine := &infrav1alpha1.LinodeMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, linodeMachine); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch Linode machine")
		}

		return ctrl.Result{}, err
	}

	machine, err := kutil.GetOwnerMachine(ctx, r.Client, linodeMachine.ObjectMeta)
	switch {
	case err != nil:
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch owner machine")
		}

		return ctrl.Result{}, err
	case machine == nil:
		log.Info("Machine Controller has not yet set OwnerRef, skipping reconciliation")

		return ctrl.Result{}, nil
	case skippedMachinePhases[machine.Status.Phase]:

		return ctrl.Result{}, nil
	default:
		match := false
		for i := range linodeMachine.OwnerReferences {
			if match = linodeMachine.OwnerReferences[i].UID == machine.UID; match {
				break
			}
		}

		if !match {
			log.Info("Failed to find the referenced owner machine, skipping reconciliation", "references", linodeMachine.OwnerReferences, "machine", machine.ObjectMeta)

			return ctrl.Result{}, nil
		}
	}

	log = log.WithValues("Linode machine: ", machine.Name)

	cluster, err := kutil.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	switch {
	case err != nil:
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch cluster by label")
		}

		return ctrl.Result{}, err
	case cluster == nil:
		err = errors.New("missing cluster")

		log.Error(err, "Missing cluster")

		return ctrl.Result{}, err
	case cluster.Spec.InfrastructureRef == nil:
		err = errors.New("missing infrastructure reference")

		log.Error(err, "Missing infrastructure reference")

		return ctrl.Result{}, err
	}

	linodeCluster := &infrav1alpha1.LinodeCluster{}

	machineScope, err := scope.NewMachineScope(
		ctx,
		r.LinodeApiKey,
		scope.MachineScopeParams{
			Client:        r.Client,
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

	clusterScope, err := scope.NewClusterScope(
		ctx,
		r.LinodeApiKey,
		scope.ClusterScopeParams{
			Client:        r.Client,
			Cluster:       cluster,
			LinodeCluster: linodeCluster,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create cluster scope")

		return ctrl.Result{}, fmt.Errorf("failed to create cluster scope: %w", err)
	}

	return r.reconcile(ctx, log, machineScope, clusterScope)
}

func (r *LinodeMachineReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	clusterScope *scope.ClusterScope,
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

	var linodeInstance *linodego.Instance
	defer func() {
		machineScope.LinodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)
		if linodeInstance != nil {
			machineScope.LinodeMachine.Status.InstanceState = &linodeInstance.Status
		}
	}()

	// Update
	if machineScope.LinodeMachine.Spec.InstanceID != nil {
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
	linodeInstance, err = r.reconcileCreate(ctx, logger, machineScope, clusterScope)

	return
}

func (r *LinodeMachineReconciler) reconcileCreate(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	clusterScope *scope.ClusterScope,
) (*linodego.Instance, error) {
	logger.Info("creating machine")

	tags := []string{machineScope.LinodeCluster.Name}

	listFilter := util.Filter{
		ID:    machineScope.LinodeMachine.Spec.InstanceID,
		Label: machineScope.LinodeMachine.Name,
		Tags:  tags,
	}
	linodeInstances, err := machineScope.LinodeClient.ListInstances(ctx, linodego.NewListOptions(1, listFilter.String()))
	if err != nil {
		logger.Error(err, "Failed to list Linode machine instances")

		return nil, err
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
			logger.Error(err, "Failed to create Linode machine create config")

			return nil, err
		}

		if machineScope.LinodeCluster.Spec.VPCRef != nil {
			iface, err := r.getVPCInterfaceConfig(ctx, machineScope, createOpts.Interfaces, logger)
			if err != nil {
				logger.Error(err, "Failed to get VPC interface config")

				return nil, err
			}

			createOpts.Interfaces = append(createOpts.Interfaces, *iface)
		}

		linodeInstance, err = machineScope.LinodeClient.CreateInstance(ctx, *createOpts)
		if err != nil {
			logger.Error(err, "Failed to create Linode machine instance")

			return nil, err
		}

		if err = r.configureDisksControlPlane(ctx, logger, machineScope, linodeInstance.ID); err != nil {
			logger.Error(err, "Failed to configure instance disks")

			return nil, err
		}

		if err = machineScope.LinodeClient.BootInstance(ctx, linodeInstance.ID, 0); err != nil {
			logger.Error(err, "Failed to boot instance")

			return nil, err
		}

	default:
		err = errors.New("multiple instances")

		logger.Error(err, "Panic! Multiple instances found. This might be a concurrency issue in the controller!!!", "tags", tags)

		return nil, err
	}

	if linodeInstance == nil {
		err = errors.New("missing instance")

		logger.Error(err, "Panic! Failed to create isntance")

		return nil, err
	}

	machineScope.LinodeMachine.Status.Ready = true
	machineScope.LinodeMachine.Spec.InstanceID = &linodeInstance.ID
	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))

	machineScope.LinodeMachine.Status.Addresses = []clusterv1.MachineAddress{}
	for _, addr := range linodeInstance.IPv4 {
		addrType := clusterv1.MachineExternalIP
		if addr.IsPrivate() {
			addrType = clusterv1.MachineInternalIP
		}
		machineScope.LinodeMachine.Status.Addresses = append(machineScope.LinodeMachine.Status.Addresses, clusterv1.MachineAddress{
			Type:    addrType,
			Address: addr.String(),
		})
	}

	if err = services.AddNodeToNB(ctx, logger, machineScope, clusterScope); err != nil {
		logger.Error(err, "Failed to add instance to Node Balancer backend")

		return linodeInstance, err
	}

	return linodeInstance, nil
}

func (r *LinodeMachineReconciler) configureDisksControlPlane(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	linodeInstanceID int,
) error {
	if !kutil.IsControlPlaneMachine(machineScope.Machine) {
		return nil
	}
	// get the default instance config
	configs, err := machineScope.LinodeClient.ListInstanceConfigs(ctx, linodeInstanceID, &linodego.ListOptions{})
	if err != nil || len(configs) == 0 {
		logger.Error(err, "Failed to list instance configs")

		return err
	}
	instanceConfig := &configs[0]

	// carve out space for the etcd disk
	rootDiskID := instanceConfig.Devices.SDA.DiskID
	rootDisk, err := machineScope.LinodeClient.GetInstanceDisk(ctx, linodeInstanceID, rootDiskID)
	if err != nil {
		logger.Error(err, "Failed to get root disk for instance")

		return err
	}
	diskSize := rootDisk.Size - defaultEtcdDiskSize
	if err = machineScope.LinodeClient.ResizeInstanceDisk(ctx, linodeInstanceID, rootDiskID, diskSize); err != nil {
		logger.Error(err, "Failed to resize root disk")

		return err
	}

	// create the etcd disk
	_, err = machineScope.LinodeClient.CreateInstanceDisk(
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

		return err
	}

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
		err = util.IgnoreLinodeAPIError(err, http.StatusNotFound)
		if err != nil {
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
