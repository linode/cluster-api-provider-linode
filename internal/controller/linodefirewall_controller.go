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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/cluster-api-provider-linode/util/reconciler/common"
)

// LinodeFirewallReconciler reconciles a LinodeFirewall object
type LinodeFirewallReconciler struct {
	Recorder           events.EventRecorder
	LinodeClientConfig scope.ClientConfig
	WatchFilterValue   string
	ReconcileTimeout   time.Duration
}

var _ common.Reconciler[*scope.FirewallScope, *infrav1alpha2.LinodeFirewall] = &LinodeFirewallReconciler{}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=addresssets,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=firewallrules,verbs=get;list;watch;update;patch

func (r *LinodeFirewallReconciler) Name() string {
	return "LinodeFirewall"
}

func (r *LinodeFirewallReconciler) Object() *infrav1alpha2.LinodeFirewall {
	return &infrav1alpha2.LinodeFirewall{}
}

func (r *LinodeFirewallReconciler) RequeueAfter() time.Duration {
	return reconciler.DefaultFWControllerReconcilerDelay
}

func (r *LinodeFirewallReconciler) Timeout() time.Duration {
	return reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultFWControllerReconcileTimeout)
}

func (r *LinodeFirewallReconciler) AddWatches(bldr *builder.Builder, cl clients.K8sClient) {
	bldr.WithEventFilter(predicate.And(
		predicates.ResourceHasFilterLabel(cl.Scheme(), ctrl.Log, r.WatchFilterValue),
		predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		),
		predicate.Funcs{UpdateFunc: firewallUpdatePredicate},
	)).
		Watches(
			&infrav1alpha2.AddressSet{},
			handler.EnqueueRequestsFromMapFunc(findObjectsForObject(ctrl.Log, cl)),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&infrav1alpha2.FirewallRule{},
			handler.EnqueueRequestsFromMapFunc(findObjectsForObject(ctrl.Log, cl)),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
}

// instructs the controller to not enqueue an update event if the Firewall was just created.
func firewallUpdatePredicate(e event.UpdateEvent) bool {
	oldObject, okOld := e.ObjectOld.(*infrav1alpha2.LinodeFirewall)
	newObject, okNew := e.ObjectNew.(*infrav1alpha2.LinodeFirewall)
	if okOld && okNew && oldObject.Spec.FirewallID == nil && newObject.Spec.FirewallID != nil {
		return false
	}
	return true
}

func (r *LinodeFirewallReconciler) NewScope(ctx context.Context, cl clients.K8sClient, obj *infrav1alpha2.LinodeFirewall, cluster *clusterv1.Cluster) (*scope.FirewallScope, error) {
	return scope.NewFirewallScope(ctx, r.LinodeClientConfig, scope.FirewallScopeParams{
		Client:         cl,
		LinodeFirewall: obj,
		Cluster:        cluster,
	})
}

func (r *LinodeFirewallReconciler) Apply(ctx context.Context, fwScope *scope.FirewallScope, logger logr.Logger) (ctrl.Result, error) {
	if fwScope.LinodeFirewall.Spec.FirewallID != nil {
		logger = logger.WithValues("fwID", *fwScope.LinodeFirewall.Spec.FirewallID)
		fwScope.ErrorType = infrav1alpha2.UpdateFirewallError
	} else {
		fwScope.ErrorType = infrav1alpha2.CreateFirewallError
	}

	err := reconcileFirewall(ctx, fwScope.Client, fwScope, logger)
	if err != nil {
		if errors.Is(err, errTooManyIPs) {
			// Cannot reconcile firewall with too many ips, wait for an update to the spec
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to create/update Firewall")
		return ctrl.Result{}, err
	}

	fwScope.LinodeFirewall.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *LinodeFirewallReconciler) Delete(ctx context.Context, fwScope *scope.FirewallScope, logger logr.Logger) (ctrl.Result, error) {
	fwScope.ErrorType = infrav1alpha2.DeleteFirewallError

	if fwScope.LinodeFirewall.Spec.FirewallID == nil {
		logger.Info("Firewall ID is missing, nothing to do")
		return ctrl.Result{}, nil
	}

	fwID := *fwScope.LinodeFirewall.Spec.FirewallID
	logger = logger.WithValues("fwID", fwID)

	err := fwScope.LinodeClient.DeleteFirewall(ctx, fwID)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		logger.Error(err, "Failed to delete Firewall via API")
		if fwScope.LinodeFirewall.ObjectMeta.DeletionTimestamp.Add(reconciler.DefaultTimeout(r.ReconcileTimeout, reconciler.DefaultFWControllerReconcileTimeout)).After(time.Now()) {
			logger.Info("Re-queuing Firewall deletion due to API delete error")
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(reconciler.DefaultFWControllerReconcilerDelay)}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to delete firewall %d after timeout: %w", fwID, err)
	}

	return ctrl.Result{}, nil
}

func (r *LinodeFirewallReconciler) HandleFailureCondition(fwScope *scope.FirewallScope, err error) {
	r.Recorder.Eventf(fwScope.LinodeFirewall, nil, corev1.EventTypeWarning, string(fwScope.ErrorType), "Reconcile", err.Error())

	fwScope.LinodeFirewall.Status.FailureReason = &fwScope.ErrorType
	fwScope.LinodeFirewall.Status.FailureMessage = new(err.Error())
	fwScope.LinodeFirewall.SetCondition(metav1.Condition{
		Type:    clusterv1.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  string(fwScope.ErrorType),
		Message: err.Error(),
	})
}
