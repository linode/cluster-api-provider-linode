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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/linodego"
)

// LinodeObjectStorageKeyReconciler reconciles a LinodeObjectStorageKey object
type LinodeObjectStorageKeyReconciler struct {
	client.Client
	Logger           logr.Logger
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	Scheme           *runtime.Scheme
	ReconcileTimeout time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekey,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekey/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekey/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LinodeObjectStorageKey object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *LinodeObjectStorageKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	logger := r.Logger.WithValues("name", req.NamespacedName.String())

	objectStorageKey := &infrav1alpha1.LinodeObjectStorageKey{}
	if err := r.Client.Get(ctx, req.NamespacedName, objectStorageKey); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			logger.Error(err, "Failed to fetch LinodeObjectStorageKey", "name", req.NamespacedName.String())
		}

		return ctrl.Result{}, err
	}

	bScope, err := scope.NewObjectStorageKeyScope(
		ctx,
		r.LinodeApiKey,
		scope.ObjectStorageKeyScopeParams{
			Client: r.Client,
			Key:    objectStorageKey,
			Logger: &logger,
		},
	)
	if err != nil {
		logger.Error(err, "Failed to create object storage key scope")

		return ctrl.Result{}, fmt.Errorf("failed to create object storage key scope: %w", err)
	}

	return r.reconcile(ctx, bScope)
}

func (r *LinodeObjectStorageKeyReconciler) reconcile(ctx context.Context, bScope *scope.ObjectStorageKeyScope) (res ctrl.Result, reterr error) {
	// Always close the scope when exiting this function so we can persist any LinodeObjectStorageKey changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := bScope.Close(ctx); utilerrors.FilterOut(err, apierrors.IsNotFound) != nil && reterr == nil {
			bScope.Logger.Error(err, "failed to patch LinodeObjectStorageKey")
			reterr = err
		}
	}()

	if !bScope.Key.DeletionTimestamp.IsZero() {
		return res, r.reconcileDelete(ctx, bScope)
	}

	if err := bScope.AddFinalizer(ctx); err != nil {
		return res, err
	}

	if err := r.reconcileApply(ctx, bScope); err != nil {
		return res, err
	}

	return res, nil
}

func (r *LinodeObjectStorageKeyReconciler) setFailure(bScope *scope.ObjectStorageKeyScope, err error) {
	bScope.Key.Status.FailureMessage = util.Pointer(err.Error())
	r.Recorder.Event(bScope.Key, corev1.EventTypeWarning, "Failed", err.Error())
	conditions.MarkFalse(bScope.Key, clusterv1.ReadyCondition, "Failed", clusterv1.ConditionSeverityError, "%s", err.Error())
}

func (r *LinodeObjectStorageKeyReconciler) reconcileApply(ctx context.Context, bScope *scope.ObjectStorageKeyScope) error {
	bScope.Logger.Info("Reconciling apply")
	key := &linodego.ObjectStorageKey{}

	switch {
	case bScope.ShouldInitKey(), bScope.ShouldRotateKey():
		newKey, err := services.RotateObjectStorageKey(ctx, bScope)
		if err != nil {
			bScope.Logger.Error(err, "Failed to provision new access key")
			r.setFailure(bScope, err)

			return err
		}
		bScope.Key.Status.AccessKeyRef = []int{newKey.ID}
		key = newKey

		r.Recorder.Event(bScope.Key, corev1.EventTypeNormal, "KeysAssigned", "Object storage keys assigned")

	case bScope.Key.Status.AccessKeyRef != nil:
		secretDeleted, err := bScope.ShouldRestoreKeySecret(ctx)
		if err != nil {
			bScope.Logger.Error(err, "Failed to ensure access key secret exists")
			r.setFailure(bScope, err)

			return err
		}

		if secretDeleted {
			sameKey, err := services.GetObjectStorageKey(ctx, bScope)
			if err != nil {
				bScope.Logger.Error(err, "Failed to restore access key for deleted secret")
				r.setFailure(bScope, err)

				return err
			}
			key = sameKey
		}

		r.Recorder.Event(bScope.Key, corev1.EventTypeNormal, "KeysRetrieved", "Object storage keys retrieved")
	}

	if key != nil {
		// TODO: generate key secret
	}

	r.Recorder.Event(bScope.Key, corev1.EventTypeNormal, "Synced", "Object storage key synced")

	bScope.Key.Status.Ready = true
	conditions.MarkTrue(bScope.Key, clusterv1.ReadyCondition)

	return nil
}

func (r *LinodeObjectStorageKeyReconciler) reconcileDelete(ctx context.Context, bScope *scope.ObjectStorageKeyScope) error {
	bScope.Logger.Info("Reconciling delete")

	if err := services.RevokeObjectStorageKey(ctx, bScope); err != nil {
		bScope.Logger.Error(err, "failed to revoke access keys; keys must be manually revoked")
		r.setFailure(bScope, err)

		return err
	}

	if !controllerutil.RemoveFinalizer(bScope.Key, infrav1alpha1.ObjectStorageBucketFinalizer) {
		err := errors.New("failed to remove finalizer from bucket; unable to delete")
		bScope.Logger.Error(err, "controllerutil.RemoveFinalizer")
		r.setFailure(bScope, err)

		return err
	}
	// TODO: remove this check and removal later
	if controllerutil.ContainsFinalizer(bScope.Key, infrav1alpha1.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(bScope.Key, infrav1alpha1.GroupVersion.String())
	}

	r.Recorder.Event(bScope.Key, clusterv1.DeletedReason, "Revoked", "Object storage keys revoked")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeObjectStorageKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	linodeObjectStorageKeyMapper, err := kutil.ClusterToTypedObjectsMapper(r.Client, &infrav1alpha1.LinodeObjectStorageKeyList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeObjectStorageKeys: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeObjectStorageKey{}).
		Owns(&corev1.Secret{}).
		WithEventFilter(predicate.And(
			predicates.ResourceHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
			predicate.GenerationChangedPredicate{},
		)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeObjectStorageKeyMapper),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
		).Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}
