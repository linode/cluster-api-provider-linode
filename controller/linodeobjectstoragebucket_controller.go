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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// LinodeObjectStorageBucketReconciler reconciles a LinodeObjectStorageBucket object
type LinodeObjectStorageBucketReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	Scheme           *runtime.Scheme
	ReconcileTimeout time.Duration
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

	log := ctrl.LoggerFrom(ctx).WithName("LinodeObjectStorageBucketReconciler").WithValues("name", req.NamespacedName.String())

	objectStorageBucket := &infrav1alpha1.LinodeObjectStorageBucket{}
	if err := r.Client.Get(ctx, req.NamespacedName, objectStorageBucket); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch LinodeObjectStorageBucket")
		}

		return ctrl.Result{}, err
	}

	bucketScope, err := scope.NewObjectStorageBucketScope(
		ctx,
		r.LinodeApiKey,
		scope.ObjectStorageBucketScopeParams{
			Client: r.Client,
			Object: objectStorageBucket,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create object storage bucket scope")

		return ctrl.Result{}, fmt.Errorf("failed to create object storage bucket scope: %w", err)
	}

	return r.reconcile(ctx, log, bucketScope)
}

func (r *LinodeObjectStorageBucketReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	bucketScope *scope.ObjectStorageBucketScope,
) (res ctrl.Result, reterr error) {
	// Always close the scope when exiting this function so we can persist any LinodeObjectStorageBucket changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := bucketScope.Close(ctx); utilerrors.FilterOut(err, apierrors.IsNotFound) != nil && reterr == nil {
			logger.Error(err, "failed to patch LinodeObjectStorageBucket")
			reterr = err
		}
	}()

	// Delete
	if !bucketScope.Object.DeletionTimestamp.IsZero() {
		return res, r.reconcileDelete(ctx, logger, bucketScope)
	}

	// Apply
	if err := r.reconcileApply(ctx, logger, bucketScope); err != nil {
		return res, err
	}

	return res, nil
}

func (r *LinodeObjectStorageBucketReconciler) setFailure(bucketScope *scope.ObjectStorageBucketScope, err error) {
	bucketScope.Object.Status.FailureMessage = util.Pointer(err.Error())
	r.Recorder.Event(bucketScope.Object, corev1.EventTypeWarning, "Failed", err.Error())
	conditions.MarkFalse(bucketScope.Object, clusterv1.ReadyCondition, "Failed", clusterv1.ConditionSeverityError, "%s", err.Error())
}

func (r *LinodeObjectStorageBucketReconciler) reconcileApply(ctx context.Context, logger logr.Logger, bucketScope *scope.ObjectStorageBucketScope) error {
	logger.Info("Applying LinodeObjectStorageBucket")

	bucketScope.Object.Status.Ready = false

	if err := bucketScope.AddFinalizer(ctx); err != nil {
		return err
	}

	// Label should only ever be nil on creation since the value is immutable.
	if bucketScope.Object.Spec.Label == nil {
		bucketScope.Object.Spec.Label = util.Pointer(util.RenderObjectLabel(bucketScope.Object.UID))
	}

	bucket, err := services.EnsureObjectStorageBucket(ctx, bucketScope, logger)
	if err != nil {
		r.setFailure(bucketScope, err)

		return err
	}
	bucketScope.Object.Status.Hostname = util.Pointer(bucket.Hostname)
	bucketScope.Object.Status.CreationTime = &metav1.Time{Time: *bucket.Created}

	if bucketScope.Object.Status.KeySecretName == nil || bucketScope.ShouldGenerateAccessKeys() {
		keys, err := services.CreateOrRotateObjectStorageKeys(ctx, bucketScope, true, logger)
		if err != nil {
			r.setFailure(bucketScope, err)

			return err
		}

		secretName := fmt.Sprintf("%s-access-keys", bucketScope.Object.Name)
		if err := bucketScope.ApplyAccessKeySecret(ctx, keys, secretName); err != nil {
			r.setFailure(bucketScope, err)

			return err
		}
		bucketScope.Object.Status.KeySecretName = util.Pointer(secretName)
		bucketScope.Object.Status.LastKeyGeneration = bucketScope.Object.Spec.KeyGeneration
	}

	r.Recorder.Event(bucketScope.Object, corev1.EventTypeNormal, "Ready", "Object storage bucket configuration applied")

	bucketScope.Object.Status.Ready = true
	conditions.MarkTrue(bucketScope.Object, clusterv1.ReadyCondition)

	return nil
}

func (r *LinodeObjectStorageBucketReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, bucketScope *scope.ObjectStorageBucketScope) error {
	logger.Info("Deleting LinodeObjectStorageBucket")

	controllerutil.RemoveFinalizer(bucketScope.Object, infrav1alpha1.GroupVersion.String())

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeObjectStorageBucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeObjectStorageBucket{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue)).
		Complete(r)
}
