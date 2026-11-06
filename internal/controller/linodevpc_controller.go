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
	"github.com/linode/linodego/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/cluster-api-provider-linode/util/reconciler/common"
)

// LinodeVPCReconciler reconciles a LinodeVPC object
type LinodeVPCReconciler struct {
	Recorder           events.EventRecorder
	LinodeClientConfig scope.ClientConfig
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
}

var _ common.Reconciler[*scope.VPCScope, *infrav1alpha2.LinodeVPC] = &LinodeVPCReconciler{}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

func (r *LinodeVPCReconciler) Name() string {
	return "LinodeVPC"
}

func (r *LinodeVPCReconciler) Object() *infrav1alpha2.LinodeVPC {
	return &infrav1alpha2.LinodeVPC{}
}

func (r *LinodeVPCReconciler) RequeueAfter() time.Duration {
	return reconciler.DefaultVPCControllerReconcileDelay
}

func (r *LinodeVPCReconciler) Timeout() time.Duration {
	return reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)
}

func (r *LinodeVPCReconciler) AddWatches(bldr *builder.Builder, cl clients.K8sClient) {
	bldr.WithEventFilter(predicate.And(
		predicates.ResourceHasFilterLabel(cl.Scheme(), ctrl.Log, r.WatchFilterValue),
		predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		),
		predicate.Funcs{UpdateFunc: func(e event.UpdateEvent) bool {
			oldObject, okOld := e.ObjectOld.(*infrav1alpha2.LinodeVPC)
			newObject, okNew := e.ObjectNew.(*infrav1alpha2.LinodeVPC)
			if okOld && okNew && oldObject.Spec.VPCID == nil && newObject.Spec.VPCID != nil {
				// We just created the VPC, don't enqueue and update
				return false
			}
			return true
		}},
	))
}

func (r *LinodeVPCReconciler) NewScope(ctx context.Context, cl clients.K8sClient, obj *infrav1alpha2.LinodeVPC, cluster *clusterv1.Cluster) (*scope.VPCScope, error) {
	return scope.NewVPCScope(ctx, r.LinodeClientConfig, scope.VPCScopeParams{
		Client:    cl,
		LinodeVPC: obj,
		Cluster:   cluster,
	})
}

func (r *LinodeVPCReconciler) Apply(ctx context.Context, vpcScope *scope.VPCScope, logger logr.Logger) (ctrl.Result, error) {
	if vpcScope.LinodeVPC.Spec.VPCID != nil {
		logger.Info("updating vpc")
		vpcScope.ErrorType = infrav1alpha2.UpdateVPCError
	} else {
		logger.Info("creating vpc")
		vpcScope.ErrorType = infrav1alpha2.CreateVPCError
	}

	err := reconcileVPC(ctx, vpcScope, logger)
	if err != nil {
		logger.Error(err, "Failed to create/update VPC")
		return ctrl.Result{}, err
	}

	vpcScope.LinodeVPC.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *LinodeVPCReconciler) Delete(ctx context.Context, vpcScope *scope.VPCScope, logger logr.Logger) (ctrl.Result, error) {
	vpcScope.ErrorType = infrav1alpha2.DeleteVPCError

	if vpcScope.LinodeVPC.Spec.Retain {
		return r.handleRetainedVPC(ctx, logger, vpcScope)
	}

	if err := r.deleteVPCResources(ctx, logger, vpcScope); err != nil {
		if errors.Is(err, util.ErrReconcileAgain) {
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(reconciler.DefaultVPCControllerReconcileDelay)}, nil
		}
		return ctrl.Result{}, err
	}

	// TODO: Why write a condition here if the K8s object is being deleted?
	vpcScope.LinodeVPC.SetCondition(metav1.Condition{
		Type:    clusterv1.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  clusterv1.DeletionCompletedReason,
		Message: "VPC deleted",
	})

	vpcScope.LinodeVPC.Spec.VPCID = nil

	return ctrl.Result{}, nil
}

func (r *LinodeVPCReconciler) handleRetainedVPC(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) (ctrl.Result, error) {
	logger.Info("VPC has retain flag, skipping VPC deletion")

	if err := r.handleRetainedSubnets(ctx, logger, vpcScope); err != nil {
		if errors.Is(err, util.ErrReconcileAgain) {
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(reconciler.DefaultVPCControllerReconcileDelay)}, nil
		}
		return ctrl.Result{}, err
	}

	// TODO: Why write a condition here if the K8s object is being deleted?
	vpcScope.LinodeVPC.SetCondition(metav1.Condition{
		Type:    clusterv1.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  clusterv1.NotDeletingReason,
		Message: "VPC retained as requested, associated cloud resource was not deleted.",
	})

	return ctrl.Result{}, nil
}

func (r *LinodeVPCReconciler) handleRetainedSubnets(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	vpc, err := getVPC(ctx, vpcScope)
	if err != nil {
		if errors.Is(err, ErrVPCNotFound) {
			return nil
		}
		logger.Error(err, "Failed to fetch VPC for subnet deletion")
		if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("re-queuing VPC deletion due to fetch error for subnet deletion")
			return util.ErrReconcileAgain
		}
		return err
	}

	// index the subnets by ID for quick lookup
	apiSubnets := make(map[int]linodego.VPCSubnet)
	for _, s := range vpc.Subnets {
		apiSubnets[s.ID] = s
	}

	for _, subnet := range vpcScope.LinodeVPC.Spec.Subnets {
		if subnet.Retain {
			continue
		}

		if apiSubnet, ok := apiSubnets[subnet.SubnetID]; ok {
			if len(apiSubnet.Linodes) > 0 {
				logger.Info("subnet still has node(s) attached", "subnetID", subnet.SubnetID)
				if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerWaitForHasNodesTimeout)).After(time.Now()) {
					logger.Info("re-queuing VPC deletion due to attached nodes in subnet")
					return util.ErrReconcileAgain
				}

				vpcScope.LinodeVPC.SetCondition(metav1.Condition{
					Type:    clusterv1.ReadyCondition,
					Status:  metav1.ConditionFalse,
					Reason:  clusterv1.NotDeletingReason,
					Message: fmt.Sprintf("will not delete subnet %d with node(s) attached", subnet.SubnetID),
				})
				return fmt.Errorf("will not delete subnet %d with node(s) attached", subnet.SubnetID)
			}
		}

		logger.Info("deleting subnet", "subnetID", subnet.SubnetID)
		deleteErr := vpcScope.LinodeClient.DeleteVPCSubnet(ctx, *vpcScope.LinodeVPC.Spec.VPCID, subnet.SubnetID)
		if deleteErr != nil {
			if err := util.IgnoreLinodeAPIError(deleteErr, http.StatusNotFound); err != nil {
				logger.Error(err, "Failed to delete subnet")
				if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
					logger.Info("re-queuing VPC deletion due to subnet delete error")
					return util.ErrReconcileAgain
				}
				return err
			}
		}
	}

	return nil
}

func (r *LinodeVPCReconciler) deleteVPCResources(ctx context.Context, logger logr.Logger, vpcScope *scope.VPCScope) error {
	vpc, err := getVPC(ctx, vpcScope)
	if err != nil {
		if errors.Is(err, ErrVPCNotFound) {
			logger.Info("VPC not found, nothing to do")
			return nil
		}
		logger.Error(err, "Failed to fetch VPC")
		if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("re-queuing VPC deletion due to fetch error")
			return util.ErrReconcileAgain
		}
		return err
	}

	for i := range vpc.Subnets {
		if len(vpc.Subnets[i].Linodes) == 0 {
			continue
		}

		logger.Info("VPC subnets still has node(s) attached")
		if vpc.Updated.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerWaitForHasNodesTimeout)).After(time.Now()) {
			logger.Info("VPC has node(s) attached, re-queuing VPC deletion")
			return util.ErrReconcileAgain
		}

		vpcScope.LinodeVPC.SetCondition(metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  clusterv1.NotDeletingReason,
			Message: "skipped due to node(s) attached",
		})
		return errors.New("will not delete VPC with node(s) attached")
	}

	if err := vpcScope.LinodeClient.DeleteVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID); err != nil {
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) == nil {
			return nil
		}
		logger.Error(err, "Failed to delete VPC")
		if vpcScope.LinodeVPC.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultVPCControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("re-queuing VPC deletion due to delete error")
			return util.ErrReconcileAgain
		}
		return err
	}

	return nil
}

func (r *LinodeVPCReconciler) HandleFailureCondition(vpcScope *scope.VPCScope, err error) {
	r.Recorder.Eventf(vpcScope.LinodeVPC, nil, corev1.EventTypeWarning, string(vpcScope.ErrorType), "Reconcile", err.Error())

	vpcScope.LinodeVPC.Status.FailureReason = &vpcScope.ErrorType
	vpcScope.LinodeVPC.Status.FailureMessage = new(err.Error())
	vpcScope.LinodeVPC.SetCondition(metav1.Condition{
		Type:    clusterv1.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  string(vpcScope.ErrorType),
		Message: err.Error(),
	})
}
