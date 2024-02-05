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
	b64 "encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

var skippedMachinePhases = map[string]bool{
	string(clusterv1.MachinePhasePending): true,
	string(clusterv1.MachinePhaseFailed):  true,
	string(clusterv1.MachinePhaseUnknown): true,
}

var skippedInstanceStatuses = map[linodego.InstanceStatus]bool{
	linodego.InstanceShuttingDown: true,
	linodego.InstanceDeleting:     true,
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

	linodeMachine := &infrav1.LinodeMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, linodeMachine); err != nil {
		log.Error(err, "Failed to fetch Linode machine")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	machine, err := kutil.GetOwnerMachine(ctx, r.Client, linodeMachine.ObjectMeta)
	switch {
	case err != nil:
		log.Error(err, "Failed to fetch owner machine")

		return ctrl.Result{}, client.IgnoreNotFound(err)
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
	if err != nil {
		log.Info("Failed to fetch cluster by label")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	} else if cluster == nil {
		log.Info("Failed to find cluster by label")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	linodeCluster := &infrav1.LinodeCluster{}
	linodeClusterKey := client.ObjectKey{
		Namespace: linodeMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err = r.Client.Get(ctx, linodeClusterKey, linodeCluster); err != nil {
		log.Error(err, "Failed to fetch Linode cluster")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	machineScope, err := scope.NewMachineScope(
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

	return r.reconcile(ctx, machineScope, log)
}

func (r *LinodeMachineReconciler) reconcile(
	ctx context.Context,
	machineScope *scope.MachineScope,
	logger logr.Logger,
) (res ctrl.Result, err error) {
	res = ctrl.Result{}

	machineScope.LinodeMachine.Status.Ready = false
	machineScope.LinodeMachine.Status.FailureReason = nil
	machineScope.LinodeMachine.Status.FailureMessage = util.Pointer("")

	failureReason := cerrs.MachineStatusError("UnknownError")
	defer func() {
		if err != nil {
			machineScope.LinodeMachine.Status.FailureReason = util.Pointer(failureReason)
			machineScope.LinodeMachine.Status.FailureMessage = util.Pointer(err.Error())

			conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(failureReason), clusterv1.ConditionSeverityError, err.Error())

			r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeWarning, string(failureReason), err.Error())
		}

		// Always close the scope when exiting this function so we can persist any LinodeMachine changes.
		if patchErr := machineScope.Close(ctx); patchErr != nil && client.IgnoreNotFound(patchErr) != nil {
			logger.Error(patchErr, "failed to patch LinodeMachine")

			err = errors.Join(err, patchErr)
		}
	}()

	// Add the finalizer if not already there
	if err := machineScope.AddFinalizer(ctx); err != nil {
		return res, err
	}

	// Delete
	if !machineScope.LinodeMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = cerrs.DeleteMachineError

		err = r.reconcileDelete(ctx, logger, machineScope)

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

		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForBootstrapDelay}, nil
	}
	linodeInstance, err = r.reconcileCreate(ctx, machineScope, logger)

	return
}

func (r *LinodeMachineReconciler) reconcileCreate(ctx context.Context, machineScope *scope.MachineScope, logger logr.Logger) (*linodego.Instance, error) {
	logger.Info("creating machine")

	tags := []string{string(machineScope.LinodeCluster.UID), string(machineScope.LinodeMachine.UID)}

	linodeInstances, err := machineScope.LinodeClient.ListInstances(ctx, linodego.NewListOptions(1, util.CreateLinodeAPIFilter("", tags)))
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
		createConfig := linodeMachineSpecToInstanceCreateConfig(machineScope.LinodeMachine.Spec)
		if createConfig == nil {
			err = errors.New("failed to convert machine spec to create instance config")

			logger.Error(err, "Panic! Struct of LinodeMachineSpec is different than InstanceCreateOptions")

			return nil, err
		}
		createConfig.Tags = tags
		createConfig.SwapSize = util.Pointer(0)

		// get the bootstrap data for the Linode instance and set it for create config
		bootstrapData, err := machineScope.GetBootstrapData(ctx)
		if err != nil {
			logger.Info("Failed to get bootstrap data", "error", err.Error())

			return nil, err
		}
		createConfig.Metadata = &linodego.InstanceMetadataOptions{
			UserData: b64.StdEncoding.EncodeToString(bootstrapData),
		}

		if createConfig.Label == "" {
			createConfig.Label = util.RenderObjectLabel(machineScope.LinodeMachine.UID)
		}

		if machineScope.LinodeCluster.Spec.VPCRef != nil {
			iface, err := r.getVPCInterfaceConfig(ctx, machineScope, createConfig.Interfaces, logger)
			if err != nil {
				logger.Error(err, "Failed to get VPC interface confiog")

				return nil, err
			}

			createConfig.Interfaces = append(createConfig.Interfaces, *iface)
		}

		linodeInstance, err = machineScope.LinodeClient.CreateInstance(ctx, *createConfig)
		if err != nil {
			logger.Error(err, "Failed to create Linode machine instance")

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
	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode:///%s/%d", linodeInstance.Region, linodeInstance.ID))

	machineScope.LinodeMachine.Status.Addresses = []clusterv1.MachineAddress{}
	for _, add := range linodeInstance.IPv4 {
		machineScope.LinodeMachine.Status.Addresses = append(machineScope.LinodeMachine.Status.Addresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineExternalIP,
			Address: add.String(),
		})
	}

	return linodeInstance, nil
}

func (r *LinodeMachineReconciler) reconcileUpdate(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (res reconcile.Result, linodeInstance *linodego.Instance, err error) {
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

			conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string("missing"), clusterv1.ConditionSeverityWarning, "instance not found")
		}

		return res, linodeInstance, err
	}

	if _, ok := requeueInstanceStatuses[linodeInstance.Status]; ok {
		if linodeInstance.Updated.Add(reconciler.DefaultMachineControllerWaitForRunningTimeout).After(time.Now()) {
			logger.Info("Instance has one operaton running, re-queuing reconciliation", "status", linodeInstance.Status)

			return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, linodeInstance, nil
		}

		logger.Info("Instance has one operaton long running, skipping reconciliation", "status", linodeInstance.Status)

		conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "skipped due to long running operation")

		return res, linodeInstance, nil
	} else if _, ok := skippedInstanceStatuses[linodeInstance.Status]; ok || linodeInstance.Status != linodego.InstanceRunning {
		logger.Info("Instance has incompatible status, skipping reconciliation", "status", linodeInstance.Status)

		conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, string(linodeInstance.Status), clusterv1.ConditionSeverityInfo, "incompatible status")

		return res, linodeInstance, nil
	}

	machineScope.LinodeMachine.Status.Ready = true

	conditions.MarkTrue(machineScope.LinodeMachine, clusterv1.ReadyCondition)

	r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeNormal, string(clusterv1.ReadyCondition), "instance is running")

	return res, linodeInstance, nil
}

func (r *LinodeMachineReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) error {
	logger.Info("deleting machine")

	if machineScope.LinodeMachine.Spec.InstanceID != nil {
		if err := machineScope.LinodeClient.DeleteInstance(ctx, *machineScope.LinodeMachine.Spec.InstanceID); err != nil {
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
				logger.Error(err, "Failed to delete Linode machine instance")

				return err
			}
		}
	} else {
		logger.Info("Machine ID is missing, nothing to do")
	}

	conditions.MarkFalse(machineScope.LinodeMachine, clusterv1.ReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "instance deleted")

	r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeNormal, clusterv1.DeletedReason, "instance has cleaned up")

	machineScope.LinodeMachine.Spec.ProviderID = nil
	machineScope.LinodeMachine.Spec.InstanceID = nil
	controllerutil.RemoveFinalizer(machineScope.LinodeMachine, infrav1.GroupVersion.String())

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LinodeMachine{}).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(kutil.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("LinodeMachine"))),
		).
		Watches(
			&infrav1.LinodeCluster{},
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
