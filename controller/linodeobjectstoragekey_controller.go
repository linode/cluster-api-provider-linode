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

// LinodeObjectStorageKeyReconciler reconciles a LinodeObjectStorageKey object
type LinodeObjectStorageKeyReconciler struct {
	client.Client
	Logger             logr.Logger
	Recorder           record.EventRecorder
	LinodeClientConfig scope.ClientConfig
	WatchFilterValue   string
	Scheme             *runtime.Scheme
	ReconcileTimeout   time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekeys/finalizers,verbs=update

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch;create;update;patch;delete

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

	tracedClient := r.TracedClient()

	objectStorageKey := &infrav1alpha2.LinodeObjectStorageKey{}
	if err := tracedClient.Get(ctx, req.NamespacedName, objectStorageKey); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			logger.Error(err, "Failed to fetch LinodeObjectStorageKey", "name", req.NamespacedName.String())
		}

		return ctrl.Result{}, err
	}

	keyScope, err := scope.NewObjectStorageKeyScope(
		ctx,
		r.LinodeClientConfig,
		scope.ObjectStorageKeyScopeParams{
			Client: tracedClient,
			Key:    objectStorageKey,
			Logger: &logger,
		},
	)
	if err != nil {
		logger.Error(err, "Failed to create object storage key scope")

		return ctrl.Result{}, fmt.Errorf("failed to create object storage key scope: %w", err)
	}

	return r.reconcile(ctx, keyScope)
}

func (r *LinodeObjectStorageKeyReconciler) reconcile(ctx context.Context, keyScope *scope.ObjectStorageKeyScope) (res ctrl.Result, reterr error) {
	// Always close the scope when exiting this function so we can persist any LinodeObjectStorageKey changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := keyScope.Close(ctx); utilerrors.FilterOut(err, apierrors.IsNotFound) != nil && reterr == nil {
			keyScope.Logger.Error(err, "failed to patch LinodeObjectStorageKey")
			reterr = err
		}
	}()

	if !keyScope.Key.DeletionTimestamp.IsZero() {
		return res, r.reconcileDelete(ctx, keyScope)
	}

	if err := keyScope.AddFinalizer(ctx); err != nil {
		return res, err
	}

	if err := r.reconcileApply(ctx, keyScope); err != nil {
		return res, err
	}

	return res, nil
}

func (r *LinodeObjectStorageKeyReconciler) setFailure(keyScope *scope.ObjectStorageKeyScope, err error) {
	keyScope.Key.Status.FailureMessage = util.Pointer(err.Error())
	r.Recorder.Event(keyScope.Key, corev1.EventTypeWarning, "Failed", err.Error())
	conditions.MarkFalse(keyScope.Key, clusterv1.ReadyCondition, "Failed", clusterv1.ConditionSeverityError, "%s", err.Error())
}

func (r *LinodeObjectStorageKeyReconciler) reconcileApply(ctx context.Context, keyScope *scope.ObjectStorageKeyScope) error {
	keyScope.Logger.Info("Reconciling apply")

	keyScope.Key.Status.Ready = false
	keyScope.Key.Status.FailureMessage = nil

	var keyForSecret *linodego.ObjectStorageKey

	switch {
	// If no access key exists or key rotation is requested, make a new key
	case keyScope.ShouldInitKey(), keyScope.ShouldRotateKey():
		key, err := services.RotateObjectStorageKey(ctx, keyScope)
		if err != nil {
			keyScope.Logger.Error(err, "Failed to provision new access key")
			r.setFailure(keyScope, err)

			return err
		}

		keyScope.Key.Status.AccessKeyRef = &key.ID
		keyForSecret = key

		if keyScope.Key.Status.LastKeyGeneration == nil {
			keyScope.Key.Status.CreationTime = &metav1.Time{Time: time.Now()}
		}

		r.Recorder.Event(keyScope.Key, corev1.EventTypeNormal, "KeyAssigned", "Object storage key assigned")

	// Ensure the generated secret still exists
	case keyScope.Key.Status.AccessKeyRef != nil:
		ok, err := keyScope.ShouldReconcileKeySecret(ctx)
		if err != nil {
			keyScope.Logger.Error(err, "Failed check for access key secret")
			r.setFailure(keyScope, err)

			return err
		}

		if ok {
			key, err := services.GetObjectStorageKey(ctx, keyScope)
			if err != nil {
				keyScope.Logger.Error(err, "Failed to restore access key for modified/deleted secret")
				r.setFailure(keyScope, err)

				return err
			}

			keyForSecret = key

			r.Recorder.Event(keyScope.Key, corev1.EventTypeNormal, "KeyRetrieved", "Object storage key retrieved")
		}
	}

	if keyForSecret != nil {
		secret, err := keyScope.GenerateKeySecret(ctx, keyForSecret)
		if err != nil {
			keyScope.Logger.Error(err, "Failed to generate key secret")
			r.setFailure(keyScope, err)

			return err
		}

		emptySecret := &corev1.Secret{ObjectMeta: secret.ObjectMeta}
		operation, err := controllerutil.CreateOrUpdate(ctx, keyScope.Client, emptySecret, func() error {
			emptySecret.Type = keyScope.Key.Spec.SecretType
			emptySecret.StringData = secret.StringData
			emptySecret.Data = nil

			return nil
		})
		if err != nil {
			keyScope.Logger.Error(err, "Failed to apply key secret")
			r.setFailure(keyScope, err)

			return err
		}

		keyScope.Key.Status.SecretName = util.Pointer(secret.Name)

		keyScope.Logger.Info(fmt.Sprintf("Secret %s was %s with access key", secret.Name, operation))
		r.Recorder.Event(keyScope.Key, corev1.EventTypeNormal, "KeyStored", "Object storage key stored in secret")
	}

	keyScope.Key.Status.LastKeyGeneration = &keyScope.Key.Spec.KeyGeneration
	keyScope.Key.Status.Ready = true

	conditions.MarkTrue(keyScope.Key, clusterv1.ReadyCondition)
	r.Recorder.Event(keyScope.Key, corev1.EventTypeNormal, "Synced", "Object storage key synced")

	return nil
}

func (r *LinodeObjectStorageKeyReconciler) reconcileDelete(ctx context.Context, keyScope *scope.ObjectStorageKeyScope) error {
	keyScope.Logger.Info("Reconciling delete")

	if err := services.RevokeObjectStorageKey(ctx, keyScope); err != nil {
		keyScope.Logger.Error(err, "failed to revoke access key; key must be manually revoked")
		r.setFailure(keyScope, err)

		return err
	}

	r.Recorder.Event(keyScope.Key, clusterv1.DeletedReason, "KeyRevoked", "Object storage key revoked")

	if !controllerutil.RemoveFinalizer(keyScope.Key, infrav1alpha2.ObjectStorageKeyFinalizer) {
		err := errors.New("failed to remove finalizer from key; unable to delete")
		keyScope.Logger.Error(err, "controllerutil.RemoveFinalizer")
		r.setFailure(keyScope, err)

		return err
	}
	// TODO: remove this check and removal later
	if controllerutil.ContainsFinalizer(keyScope.Key, infrav1alpha2.GroupVersion.String()) {
		controllerutil.RemoveFinalizer(keyScope.Key, infrav1alpha2.GroupVersion.String())
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl // This follows the pattern used for the LinodeObjectStorageBucket controller.
func (r *LinodeObjectStorageKeyReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	linodeObjectStorageKeyMapper, err := kutil.ClusterToTypedObjectsMapper(r.TracedClient(), &infrav1alpha2.LinodeObjectStorageKeyList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeObjectStorageKeys: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodeObjectStorageKey{}).
		WithOptions(options).
		Owns(&corev1.Secret{}).
		WithEventFilter(predicate.And(
			predicates.ResourceHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
			predicate.GenerationChangedPredicate{},
		)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeObjectStorageKeyMapper),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetLogger())),
		).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeObjectStorageKeyReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
