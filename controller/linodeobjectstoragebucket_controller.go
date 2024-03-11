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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// LinodeObjectStorageBucketReconciler reconciles a LinodeObjectStorageBucket object
type LinodeObjectStorageBucketReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Logger              logr.Logger
	Recorder            record.EventRecorder
	LinodeApiKey        string
	LinodeClientFactory scope.LinodeObjectStorageClientFactory
	WatchFilterValue    string
	ReconcileTimeout    time.Duration
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets/finalizers,verbs=update

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LinodeObjectStorageBucket object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *LinodeObjectStorageBucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	logger := r.Logger.WithValues("name", req.NamespacedName.String())

	objectStorageBucket := &infrav1alpha1.LinodeObjectStorageBucket{}
	if err := r.Client.Get(ctx, req.NamespacedName, objectStorageBucket); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			logger.Error(err, "Failed to fetch LinodeObjectStorageBucket", "name", req.NamespacedName.String())
		}

		return ctrl.Result{}, err
	}

	bScope, err := scope.NewObjectStorageBucketScope(
		ctx,
		r.LinodeApiKey,
		scope.ObjectStorageBucketScopeParams{
			Client:              r.Client,
			LinodeClientFactory: r.LinodeClientFactory,
			Bucket:              objectStorageBucket,
			Logger:              &logger,
		},
	)
	if err != nil {
		logger.Error(err, "Failed to create object storage bucket scope")

		return ctrl.Result{}, fmt.Errorf("failed to create object storage bucket scope: %w", err)
	}

	return r.reconcile(ctx, bScope)
}

func (r *LinodeObjectStorageBucketReconciler) reconcile(ctx context.Context, bScope *scope.ObjectStorageBucketScope) (res ctrl.Result, reterr error) {
	// Always close the scope when exiting this function so we can persist any LinodeObjectStorageBucket changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := bScope.Close(ctx); utilerrors.FilterOut(err, apierrors.IsNotFound) != nil && reterr == nil {
			bScope.Logger.Error(err, "failed to patch LinodeObjectStorageBucket")
			reterr = err
		}
	}()

	// Delete
	if !bScope.Bucket.DeletionTimestamp.IsZero() {
		return res, r.reconcileDelete(ctx, bScope)
	}

	// Apply
	if err := r.reconcileApply(ctx, bScope); err != nil {
		return res, err
	}

	return res, nil
}

func (r *LinodeObjectStorageBucketReconciler) setFailure(bScope *scope.ObjectStorageBucketScope, err error) {
	bScope.Bucket.Status.FailureMessage = util.Pointer(err.Error())
	r.Recorder.Event(bScope.Bucket, corev1.EventTypeWarning, "Failed", err.Error())
	conditions.MarkFalse(bScope.Bucket, clusterv1.ReadyCondition, "Failed", clusterv1.ConditionSeverityError, "%s", err.Error())
}

func (r *LinodeObjectStorageBucketReconciler) reconcileApply(ctx context.Context, bScope *scope.ObjectStorageBucketScope) error {
	bScope.Logger.Info("Reconciling apply")

	bScope.Bucket.Status.Ready = false

	if err := bScope.AddFinalizer(ctx); err != nil {
		return err
	}

	if bScope.Bucket.Spec.Label == nil {
		bScope.Bucket.Spec.Label = util.Pointer(bScope.Bucket.Name)
	}

	bucket, err := services.EnsureObjectStorageBucket(ctx, bScope)
	if err != nil {
		bScope.Logger.Error(err, "Failed to ensure bucket exists")
		r.setFailure(bScope, err)

		return err
	}
	bScope.Bucket.Status.Hostname = util.Pointer(bucket.Hostname)
	bScope.Bucket.Status.CreationTime = &metav1.Time{Time: *bucket.Created}

	if bScope.Bucket.Status.LastKeyGeneration == nil || bScope.ShouldRotateKeys() {
		keys, err := services.RotateObjectStorageKeys(ctx, bScope)
		if err != nil {
			bScope.Logger.Error(err, "Failed to provision new access keys")
			r.setFailure(bScope, err)

			return err
		}

		secretName := fmt.Sprintf(scope.AccessKeyNameTemplate, *bScope.Bucket.Spec.Label)
		if err := bScope.ApplyAccessKeySecret(ctx, keys, secretName); err != nil {
			bScope.Logger.Error(err, "Failed to apply access key secret")
			r.setFailure(bScope, err)

			return err
		}
		bScope.Bucket.Status.KeySecretName = util.Pointer(secretName)
		bScope.Bucket.Status.LastKeyGeneration = bScope.Bucket.Spec.KeyGeneration
	}

	r.Recorder.Event(bScope.Bucket, corev1.EventTypeNormal, "Ready", "Object storage bucket applied")

	bScope.Bucket.Status.Ready = true
	conditions.MarkTrue(bScope.Bucket, clusterv1.ReadyCondition)

	return nil
}

func (r *LinodeObjectStorageBucketReconciler) reconcileDelete(ctx context.Context, bScope *scope.ObjectStorageBucketScope) error {
	bScope.Logger.Info("Reconciling delete")

	secret, err := bScope.GetAccessKeySecret(ctx)
	if err != nil {
		bScope.Logger.Error(err, "Failed to read secret with access keys to revoke")
		r.setFailure(bScope, err)

		return err
	}

	if err := services.RevokeObjectStorageKeys(ctx, bScope, secret); err != nil {
		bScope.Logger.Error(err, "Failed to revoke access keys; keys must be manually revoked")
		r.setFailure(bScope, err)

		return err
	}

	// Only permit Secret and LinodeObjectStorageBucket deletion if keys were revoked
	if !controllerutil.RemoveFinalizer(secret, infrav1alpha1.GroupVersion.String()) {
		bScope.Logger.Error(err, "Failed to remove finalizer from secret; will not be deleted")
		r.setFailure(bScope, err)

		return err
	}

	if err := r.Client.Update(ctx, secret); err != nil {
		bScope.Logger.Error(err, "Failed to remove finalizer from secret; will not be deleted")
		r.setFailure(bScope, err)

		return err
	}

	if !controllerutil.RemoveFinalizer(bScope.Bucket, infrav1alpha1.GroupVersion.String()) {
		bScope.Logger.Error(err, "Failed to remove finalizer from bucket; will not be deleted")
		r.setFailure(bScope, err)

		return err
	}

	r.Recorder.Event(bScope.Bucket, clusterv1.DeletedReason, "Ready", "Object storage bucket deleted")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeObjectStorageBucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeObjectStorageBucket{}).
		WithEventFilter(predicate.And(
			predicates.ResourceHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
			predicate.GenerationChangedPredicate{},
		)).
		Complete(r)
}
