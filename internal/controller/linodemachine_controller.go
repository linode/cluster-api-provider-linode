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
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/paused"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
	ConditionPreflightBootstrapDataSecretReady  = "PreflightBootstrapDataSecretReady"
	ConditionPreflightLinodeFirewallReady       = "PreflightLinodeFirewallReady"
	ConditionPreflightMetadataSupportConfigured = "PreflightMetadataSupportConfigured"
	ConditionPreflightCreated                   = "PreflightCreated"
	ConditionPreflightRootDiskResizing          = "PreflightRootDiskResizing"
	ConditionPreflightRootDiskResized           = "PreflightRootDiskResized"
	ConditionPreflightAdditionalDisksCreated    = "PreflightAdditionalDisksCreated"
	ConditionPreflightConfigured                = "PreflightConfigured"
	ConditionPreflightBootTriggered             = "PreflightBootTriggered"
	ConditionPreflightReady                     = "PreflightReady"

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
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
	// Feature flags
	GzipCompressionEnabled bool
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

func (r *LinodeMachineReconciler) pauseReferencedFirewall(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, isPaused, conditionChanged bool) error {
	if machineScope.LinodeMachine.Spec.FirewallRef == nil {
		logger.Info("Paused reconciliation is skipped due to missing Firewall ref")
		return nil
	}
	linodeFW := infrav1alpha2.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: machineScope.LinodeMachine.Spec.FirewallRef.Namespace,
			Name:      machineScope.LinodeMachine.Spec.FirewallRef.Name,
		},
	}

	if err := machineScope.Client.Get(ctx, client.ObjectKeyFromObject(&linodeFW), &linodeFW); err != nil {
		return err
	}

	annotations := linodeFW.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if isPaused {
		logger.Info("CAPI cluster is paused, pausing Firewall too")
		// if we're paused, we should slap the pause annotation on our children
		// get the firewall & add the annotation
		annotations[clusterv1.PausedAnnotation] = pauseAnnotationValue
	} else if conditionChanged {
		// we are not paused here, but were previously paused
		logger.Info("CAPI cluster is no longer paused, removing pause annotation from Firewall")
		delete(annotations, clusterv1.PausedAnnotation)
	}
	linodeFW.SetAnnotations(annotations)
	fwPatchHelper, err := patch.NewHelper(&linodeFW, machineScope.Client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper for firewalls: %w", err)
	}
	if err := fwPatchHelper.Patch(ctx, &linodeFW); err != nil {
		return fmt.Errorf("failed to patch firewall: %w", err)
	}
	return nil
}

func (r *LinodeMachineReconciler) pauseReferencedPlacementGroup(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, isPaused, conditionChanged bool) error {
	if machineScope.LinodeMachine.Spec.PlacementGroupRef == nil {
		logger.Info("Paused reconciliation is skipped due to missing placement group ref")
		return nil
	}
	linodePG := infrav1alpha2.LinodePlacementGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: machineScope.LinodeMachine.Spec.PlacementGroupRef.Namespace,
			Name:      machineScope.LinodeMachine.Spec.PlacementGroupRef.Name,
		},
	}

	if err := machineScope.Client.Get(ctx, client.ObjectKeyFromObject(&linodePG), &linodePG); err != nil {
		return err
	}

	annotations := linodePG.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if isPaused {
		logger.Info("CAPI cluster is paused, pausing Placement Group too")
		// if we're paused, we should slap the pause annotation on our children
		// get the firewall & add the annotation
		annotations[clusterv1.PausedAnnotation] = pauseAnnotationValue
	} else if conditionChanged {
		// we are not paused here, but were previously paused
		logger.Info("CAPI cluster is no longer paused, removing pause annotation from Placement Group ")
		delete(annotations, clusterv1.PausedAnnotation)
	}

	linodePG.SetAnnotations(annotations)
	fwPatchHelper, err := patch.NewHelper(&linodePG, machineScope.Client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper for firewalls: %w", err)
	}
	if err := fwPatchHelper.Patch(ctx, &linodePG); err != nil {
		return fmt.Errorf("failed to patch firewall: %w", err)
	}
	return nil
}

func (r *LinodeMachineReconciler) reconcilePause(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) error {
	// Pausing a machine Pauses the firewall referred by the machine
	isPaused, conditionChanged, err := paused.EnsurePausedCondition(ctx, machineScope.Client, machineScope.Cluster, machineScope.LinodeMachine)

	if err == nil && !isPaused && !conditionChanged {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.pauseReferencedFirewall(ctx, logger, machineScope, isPaused, conditionChanged); err != nil {
		return fmt.Errorf("failed to pause referenced firewall: %w", err)
	}
	if err := r.pauseReferencedPlacementGroup(ctx, logger, machineScope, isPaused, conditionChanged); err != nil {
		return fmt.Errorf("failed to pause referenced placement group: %w", err)
	}
	return nil
}

func (r *LinodeMachineReconciler) reconcile(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (res ctrl.Result, err error) {
	failureReason := util.UnknownError
	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if err != nil {
			// Only set failure reason if the error is not retryable.
			if linodego.ErrHasStatus(err, http.StatusBadRequest) {
				machineScope.LinodeMachine.Status.FailureReason = util.Pointer(failureReason)
				machineScope.LinodeMachine.Status.FailureMessage = util.Pointer(err.Error())
				conditions.Set(machineScope.LinodeMachine, metav1.Condition{
					Type:    string(clusterv1.ReadyCondition),
					Status:  metav1.ConditionFalse,
					Reason:  failureReason,
					Message: err.Error(),
				})
			}

			// Record the event regardless of whether the error is retryable or not for visibility.
			r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeWarning, failureReason, err.Error())
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

	// Pause
	if err := r.reconcilePause(ctx, logger, machineScope); err != nil {
		return ctrl.Result{}, err
	}

	// Delete
	if !machineScope.LinodeMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		failureReason = util.DeleteError
		return r.reconcileDelete(ctx, logger, machineScope)
	}

	// Make sure bootstrap data is available and populated.
	if !reconciler.ConditionTrue(machineScope.LinodeMachine, string(ConditionPreflightBootstrapDataSecretReady)) && machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap data secret is not yet available")
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:   ConditionPreflightBootstrapDataSecretReady,
			Status: metav1.ConditionFalse,
			Reason: WaitingForBootstrapDataReason,
		})
		return ctrl.Result{}, nil
	}

	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightBootstrapDataSecretReady,
		Status: metav1.ConditionTrue,
		Reason: "BootstrapDataSecretReady", // We have to set the reason to not fail object patching
	})

	// Update
	if machineScope.LinodeMachine.Status.InstanceState != nil {
		failureReason = util.UpdateError
		return r.reconcileUpdate(ctx, logger, machineScope)
	}

	// Create
	failureReason = util.CreateError
	return r.reconcileCreate(ctx, logger, machineScope)
}

//nolint:cyclop,gocognit // can't make it simpler with existing API
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

	if machineScope.LinodeMachine.Spec.FirewallRef != nil {
		if !reconciler.ConditionTrue(machineScope.LinodeMachine, string(ConditionPreflightLinodeFirewallReady)) && machineScope.LinodeMachine.Spec.ProviderID == nil {
			res, err := r.reconcilePreflightLinodeFirewallCheck(ctx, logger, machineScope)
			if err != nil || !res.IsZero() {
				conditions.Set(machineScope.LinodeMachine, metav1.Condition{
					Type:   ConditionPreflightLinodeFirewallReady,
					Status: metav1.ConditionFalse,
					Reason: "LinodeFirewallNotYetAvailable", // We have to set the reason to not fail object patching
				})
				return res, err
			}
		}
	}

	// Should we check if the VPC ref in LinodeCluster is ready? Or is it enough to check if the VPC exists?
	if vpcRef := getVPCRefFromScope(machineScope); vpcRef != nil {
		if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightLinodeVPCReady) && machineScope.LinodeMachine.Spec.ProviderID == nil {
			res, err := r.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
			if err != nil || !res.IsZero() {
				return res, err
			}
		}
	}

	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightMetadataSupportConfigured) && machineScope.LinodeMachine.Spec.ProviderID == nil {
		res, err := r.reconcilePreflightMetadataSupportConfigure(ctx, logger, machineScope)
		if err != nil || !res.IsZero() {
			return res, err
		}
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

func (r *LinodeMachineReconciler) reconcilePreflightVPC(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, vpcRef *corev1.ObjectReference) (ctrl.Result, error) {
	name := vpcRef.Name
	namespace := vpcRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeMachine.Namespace
	}
	linodeVPC := infrav1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if err := machineScope.Client.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC); err != nil {
		logger.Error(err, "Failed to fetch LinodeVPC")
		if reconciler.HasStaleCondition(machineScope.LinodeMachine,
			ConditionPreflightLinodeVPCReady,
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultClusterControllerReconcileTimeout)) {
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightLinodeVPCReady,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightLinodeVPCReady,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: err.Error(),
		})
		return ctrl.Result{RequeueAfter: reconciler.DefaultClusterControllerReconcileDelay}, nil
	} else if !linodeVPC.Status.Ready {
		logger.Info("LinodeVPC is not yet available")
		return ctrl.Result{RequeueAfter: reconciler.DefaultClusterControllerReconcileDelay}, nil
	}
	r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeNormal, string(clusterv1.ReadyCondition), "LinodeVPC is now available")
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightLinodeVPCReady,
		Status: metav1.ConditionTrue,
		Reason: "LinodeVPCReady", // We have to set the reason to not fail object patching
	})
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightLinodeFirewallCheck(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	name := machineScope.LinodeMachine.Spec.FirewallRef.Name
	namespace := machineScope.LinodeMachine.Spec.FirewallRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeMachine.Namespace
	}
	linodeFirewall := infrav1alpha2.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if err := machineScope.Client.Get(ctx, client.ObjectKeyFromObject(&linodeFirewall), &linodeFirewall); err != nil {
		logger.Error(err, "Failed to find linode Firewall")
		if reconciler.HasStaleCondition(machineScope.LinodeMachine,
			ConditionPreflightLinodeFirewallReady,
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightLinodeFirewallReady,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightLinodeFirewallReady,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: err.Error(),
		})
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerRetryDelay}, nil
	} else if !linodeFirewall.Status.Ready {
		logger.Info("Linode firewall not yet ready")
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerRetryDelay}, nil
	}
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightLinodeFirewallReady,
		Status: metav1.ConditionTrue,
		Reason: "LinodeFirewallReady", // We have to set the reason to not fail object patching
	})
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightMetadataSupportConfigure(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	region, err := machineScope.LinodeClient.GetRegion(ctx, machineScope.LinodeMachine.Spec.Region)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to fetch region %s", machineScope.LinodeMachine.Spec.Region))
		return retryIfTransient(err, logger)
	}
	regionMetadataSupport := slices.Contains(region.Capabilities, linodego.CapabilityMetadata)
	imageName := reconciler.DefaultMachineControllerLinodeImage
	if machineScope.LinodeMachine.Spec.Image != "" {
		imageName = machineScope.LinodeMachine.Spec.Image
	}
	image, err := machineScope.LinodeClient.GetImage(ctx, imageName)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to fetch image %s", imageName))
		return retryIfTransient(err, logger)
	}
	imageMetadataSupport := slices.Contains(image.Capabilities, "cloud-init")
	machineScope.LinodeMachine.Status.CloudinitMetadataSupport = true
	if !imageMetadataSupport || !regionMetadataSupport {
		logger.Info("cloud-init metadata support not available", "imageMetadataSupport", imageMetadataSupport, "regionMetadataSupport", regionMetadataSupport)
		machineScope.LinodeMachine.Status.CloudinitMetadataSupport = false
	}
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightMetadataSupportConfigured,
		Status: metav1.ConditionTrue,
		Reason: "LinodeMetadataSupportConfigured", // We have to set the reason to not fail object patching
	})
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightCreate(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	// get the bootstrap data for the Linode instance and set it for create config
	createOpts, err := newCreateConfig(ctx, machineScope, r.GzipCompressionEnabled, logger)
	if err != nil {
		logger.Error(err, "Failed to create Linode machine InstanceCreateOptions")
		return retryIfTransient(err, logger)
	}

	linodeInstance, retryAfter, err := createInstance(ctx, logger, machineScope, createOpts)
	if errors.Is(err, util.ErrRateLimit) {
		return ctrl.Result{RequeueAfter: retryAfter}, nil
	}

	if err != nil {
		logger.Error(err, "Failed to create Linode machine instance")
		if reconciler.HasStaleCondition(machineScope.LinodeMachine,
			ConditionPreflightCreated,
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightCreated,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightCreated,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: err.Error(),
		})
		return retryIfTransient(err, logger)
	}

	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightCreated,
		Status: metav1.ConditionTrue,
		Reason: "LinodeMachinePreflightCreated", // We have to set the reason to not fail object patching
	})
	// Set the provider ID since the instance is successfully created
	machineScope.LinodeMachine.Spec.ProviderID = util.Pointer(fmt.Sprintf("linode://%d", linodeInstance.ID))
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightConfigure(ctx context.Context, instanceID int, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	if err := configureDisks(ctx, logger, machineScope, instanceID); err != nil {
		if reconciler.HasStaleCondition(machineScope.LinodeMachine,
			ConditionPreflightConfigured,
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightConfigured,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightConfigured,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: err.Error(),
		})
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
	}
	if machineScope.LinodeMachine.Spec.Configuration != nil && machineScope.LinodeMachine.Spec.Configuration.Kernel != "" {
		instanceConfig, err := getDefaultInstanceConfig(ctx, machineScope, instanceID)
		if err != nil {
			logger.Error(err, "Failed to get default instance configuration")
			return retryIfTransient(err, logger)
		}

		if _, err := machineScope.LinodeClient.UpdateInstanceConfig(ctx, instanceID, instanceConfig.ID, linodego.InstanceConfigUpdateOptions{Kernel: machineScope.LinodeMachine.Spec.Configuration.Kernel}); err != nil {
			logger.Error(err, "Failed to update default instance configuration")
			return retryIfTransient(err, logger)
		}
	}
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightConfigured,
		Status: metav1.ConditionTrue,
		Reason: "LinodeMachinePreflightConfigured", // We have to set the reason to not fail object patching
	})
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightBoot(ctx context.Context, instanceID int, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	if err := machineScope.LinodeClient.BootInstance(ctx, instanceID, 0); err != nil && !strings.HasSuffix(err.Error(), "already booted.") {
		logger.Error(err, "Failed to boot instance")
		if reconciler.HasStaleCondition(machineScope.LinodeMachine,
			ConditionPreflightBootTriggered,
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightBootTriggered,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightBootTriggered,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: err.Error(),
		})
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
	}
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightBootTriggered,
		Status: metav1.ConditionTrue,
		Reason: "LinodeMachinePreflightBootTriggered", // We have to set the reason to not fail object patching
	})
	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcilePreflightReady(ctx context.Context, instanceID int, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	addrs, err := buildInstanceAddrs(ctx, machineScope, instanceID)
	if err != nil {
		logger.Error(err, "Failed to get instance ip addresses")
		if reconciler.HasStaleCondition(machineScope.LinodeMachine,
			ConditionPreflightReady,
			reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultMachineControllerWaitForPreflightTimeout)) {
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightReady,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightReady,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: err.Error(),
		})
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerWaitForRunningDelay}, nil
	}
	machineScope.LinodeMachine.Status.Addresses = addrs
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightReady,
		Status: metav1.ConditionTrue,
		Reason: "LinodeMachinePreflightReady", // We have to set the reason to not fail object patching
	})
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
		return retryIfTransient(err, logger)
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
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    string(clusterv1.ReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  string(linodeInstance.Status),
				Message: "skipped due to long running operation",
			})
		}
	} else if linodeInstance.Status != linodego.InstanceRunning {
		logger.Info("Instance has incompatible status, skipping reconciliation", "status", linodeInstance.Status)
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    string(clusterv1.ReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  string(linodeInstance.Status),
			Message: "incompatible status",
		})
	} else {
		machineScope.LinodeMachine.Status.Ready = true
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:   string(clusterv1.ReadyCondition),
			Status: metav1.ConditionTrue,
			Reason: "LinodeMachineReady", // We have to set the reason to not fail object patching
		})
	}

	// Clean up after instance creation.
	if linodeInstance.Status == linodego.InstanceRunning && machineScope.Machine.Status.Phase == "Running" {
		if err := deleteBootstrapData(ctx, machineScope); err != nil {
			logger.Error(err, "Fail to bootstrap data")
		}
	}

	return ctrl.Result{}, nil
}

func (r *LinodeMachineReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (ctrl.Result, error) {
	logger.Info("deleting machine")

	if err := deleteBootstrapData(ctx, machineScope); err != nil {
		logger.Error(err, "Fail to bootstrap data")
	}

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

	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:    string(clusterv1.ReadyCondition),
		Status:  metav1.ConditionFalse,
		Reason:  string(clusterv1.DeletedReason),
		Message: "instance deleted",
	})

	r.Recorder.Event(machineScope.LinodeMachine, corev1.EventTypeNormal, clusterv1.DeletedReason, "instance has cleaned up")

	machineScope.LinodeMachine.Spec.ProviderID = nil
	machineScope.LinodeMachine.Status.InstanceState = nil

	if err := machineScope.RemoveCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "Failed to update credentials secret")
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(machineScope.LinodeMachine, infrav1alpha2.MachineFinalizer)

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
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureReady(mgr.GetScheme(), mgr.GetLogger())),
		).
		// we care about reconciling on metadata updates for LinodeMachines because the OwnerRef for the Machine is needed
		WithEventFilter(predicate.And(
			predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), mgr.GetLogger(), r.WatchFilterValue),
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
