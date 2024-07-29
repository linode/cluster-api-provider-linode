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

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	wrappedruntimeclient "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimeclient"
	wrappedruntimereconciler "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimereconciler"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// LinodeObjectStorageBucketReconciler reconciles a LinodeObjectStorageBucket object
type LinodeObjectStorageBucketReconciler struct {
	client.Client
	Logger           logr.Logger
	Recorder         record.EventRecorder
	LinodeApiKey     string
	WatchFilterValue string
	ReconcileTimeout time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets/finalizers,verbs=update

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

	objectStorageBucket := &infrav1alpha2.LinodeObjectStorageBucket{}
	if err := r.TracedClient().Get(ctx, req.NamespacedName, objectStorageBucket); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			logger.Error(err, "Failed to fetch LinodeObjectStorageBucket", "name", req.NamespacedName.String())
		}

		return ctrl.Result{}, err
	}

	bScope, err := scope.NewObjectStorageBucketScope(
		ctx,
		r.LinodeApiKey,
		scope.ObjectStorageBucketScopeParams{
			Client: r.TracedClient(),
			Bucket: objectStorageBucket,
			Logger: &logger,
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

	if !bScope.Bucket.DeletionTimestamp.IsZero() {
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

func (r *LinodeObjectStorageBucketReconciler) setFailure(bScope *scope.ObjectStorageBucketScope, err error) {
	bScope.Bucket.Status.FailureMessage = util.Pointer(err.Error())
	r.Recorder.Event(bScope.Bucket, corev1.EventTypeWarning, "Failed", err.Error())
	conditions.MarkFalse(bScope.Bucket, clusterv1.ReadyCondition, "Failed", clusterv1.ConditionSeverityError, "%s", err.Error())
}

func (r *LinodeObjectStorageBucketReconciler) reconcileApply(ctx context.Context, bScope *scope.ObjectStorageBucketScope) error {
	bScope.Logger.Info("Reconciling apply")

	bScope.Bucket.Status.Ready = false

	bucket, err := services.EnsureObjectStorageBucket(ctx, bScope)
	if err != nil {
		bScope.Logger.Error(err, "Failed to ensure bucket exists")
		r.setFailure(bScope, err)

		return err
	}

	bScope.Bucket.Status.Hostname = util.Pointer(bucket.Hostname)
	bScope.Bucket.Status.CreationTime = &metav1.Time{Time: *bucket.Created}

	var keys [2]*linodego.ObjectStorageKey
	var secretDeleted bool

	switch {
	case bScope.ShouldInitKeys(), bScope.ShouldRotateKeys():
		newKeys, err := services.RotateObjectStorageKeys(ctx, bScope)
		if err != nil {
			bScope.Logger.Error(err, "Failed to provision new access keys")
			r.setFailure(bScope, err)

			return err
		}
		bScope.Bucket.Status.AccessKeyRefs = []int{newKeys[0].ID, newKeys[1].ID}
		keys = newKeys

		r.Recorder.Event(bScope.Bucket, corev1.EventTypeNormal, "KeysAssigned", "Object storage keys assigned")

	case bScope.Bucket.Status.AccessKeyRefs != nil:
		secretDeleted, err = bScope.ShouldRestoreKeySecret(ctx)
		if err != nil {
			bScope.Logger.Error(err, "Failed to ensure access key secret exists")
			r.setFailure(bScope, err)

			return err
		}

		if secretDeleted {
			sameKeys, err := services.GetObjectStorageKeys(ctx, bScope)
			if err != nil {
				bScope.Logger.Error(err, "Failed to restore access keys for deleted secret")
				r.setFailure(bScope, err)

				return err
			}
			keys = sameKeys
		}

		r.Recorder.Event(bScope.Bucket, corev1.EventTypeNormal, "KeysRetrieved", "Object storage keys retrieved")
	}

	if keys[0] != nil && keys[1] != nil {
		secret, err := bScope.GenerateKeySecret(ctx, keys, bucket)
		if err != nil {
			bScope.Logger.Error(err, "Failed to generate key secret")
			r.setFailure(bScope, err)

			return err
		}

		if secretDeleted {
			err = bScope.Client.Create(ctx, secret)
		} else {
			_, err = controllerutil.CreateOrUpdate(ctx, bScope.Client, secret, func() error { return nil })
		}
		if err != nil {
			bScope.Logger.Error(err, "Failed to apply key secret")
			r.setFailure(bScope, err)

			return err
		}
		bScope.Logger.Info(fmt.Sprintf("Secret %s was applied with new access keys", secret.Name))

		bScope.Bucket.Status.KeySecretName = util.Pointer(secret.Name)
		bScope.Bucket.Status.LastKeyGeneration = bScope.Bucket.Spec.KeyGeneration

		r.Recorder.Event(bScope.Bucket, corev1.EventTypeNormal, "KeysStored", "Object storage keys stored in secret")
	}

	r.Recorder.Event(bScope.Bucket, corev1.EventTypeNormal, "Synced", "Object storage bucket synced")

	bScope.Bucket.Status.Ready = true
	conditions.MarkTrue(bScope.Bucket, clusterv1.ReadyCondition)

	return nil
}

func (r *LinodeObjectStorageBucketReconciler) reconcileDelete(ctx context.Context, bScope *scope.ObjectStorageBucketScope) error {
	bScope.Logger.Info("Reconciling delete")

	if err := services.RevokeObjectStorageKeys(ctx, bScope); err != nil {
		bScope.Logger.Error(err, "failed to revoke access keys; keys must be manually revoked")
		r.setFailure(bScope, err)

		return err
	}

	if !controllerutil.RemoveFinalizer(bScope.Bucket, infrav1alpha2.ObjectStorageBucketFinalizer) {
		err := errors.New("failed to remove finalizer from bucket; unable to delete")
		bScope.Logger.Error(err, "controllerutil.RemoveFinalizer")
		r.setFailure(bScope, err)

		return err
	}
	// TODO: remove this check and removal later
	if controllerutil.ContainsFinalizer(bScope.Bucket, infrav1alpha2.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(bScope.Bucket, infrav1alpha2.GroupVersion.String())
	}

	r.Recorder.Event(bScope.Bucket, clusterv1.DeletedReason, "Revoked", "Object storage keys revoked")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeObjectStorageBucketReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	linodeObjectStorageBucketMapper, err := kutil.ClusterToTypedObjectsMapper(
		r.TracedClient(),
		&infrav1alpha2.LinodeObjectStorageBucketList{},
		mgr.GetScheme(),
	)

	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeObjectStorageBuckets: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodeObjectStorageBucket{}).
		WithOptions(options).
		Owns(&corev1.Secret{}).
		WithEventFilter(predicate.And(
			predicates.ResourceHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
			predicate.GenerationChangedPredicate{},
		)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeObjectStorageBucketMapper),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
		).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeObjectStorageBucketReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
