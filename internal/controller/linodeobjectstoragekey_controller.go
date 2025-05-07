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
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
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

	logger := r.Logger.WithValues("name", req.String())

	tracedClient := r.TracedClient()

	objectStorageKey := &infrav1alpha2.LinodeObjectStorageKey{}
	if err := tracedClient.Get(ctx, req.NamespacedName, objectStorageKey); err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			logger.Error(err, "Failed to fetch LinodeObjectStorageKey", "name", req.String())
		}

		return ctrl.Result{}, err
	}

	if _, ok := objectStorageKey.Labels[clusterv1.ClusterNameLabel]; ok {
		cluster, err := kutil.GetClusterFromMetadata(ctx, r.TracedClient(), objectStorageKey.ObjectMeta)
		if err != nil {
			if client.IgnoreNotFound(err) != nil {
				logger.Error(err, "failed to fetch cluster from metadata")
				return ctrl.Result{}, err
			}
			logger.Info("Cluster not found but LinodeObjectStorageKey is being deleted, continuing with deletion")
		}

		// It will handle the case where the cluster is not found
		if err := util.SetOwnerReferenceToLinodeCluster(ctx, r.TracedClient(), cluster, objectStorageKey, r.Scheme); err != nil {
			logger.Error(err, "Failed to set owner reference to LinodeCluster")
			return ctrl.Result{}, err
		}
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

	// Override the controller credentials with ones from the Key's Secret reference (if supplied).
	if err := keyScope.SetCredentialRefTokenForLinodeClients(ctx); err != nil {
		keyScope.Logger.Error(err, "failed to update linode client token from Credential Ref")
		return res, err
	}

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
	conditions.Set(keyScope.Key, metav1.Condition{
		Type:    string(clusterv1.ReadyCondition),
		Status:  metav1.ConditionFalse,
		Reason:  "Failed",
		Message: err.Error(),
	})
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
		secret := &corev1.Secret{}
		key := client.ObjectKey{
			Namespace: keyScope.Key.Spec.Namespace,
			Name:      keyScope.Key.Spec.Name,
		}

		if err := keyScope.Client.Get(ctx, key, secret); err != nil {
			if apierrors.IsNotFound(err) {
				key, err := services.GetObjectStorageKey(ctx, keyScope)
				if err != nil {
					keyScope.Logger.Error(err, "Failed to restore access key for modified/deleted secret")
					r.setFailure(keyScope, err)

					return err
				}

				keyForSecret = key

				r.Recorder.Event(keyScope.Key, corev1.EventTypeNormal, "KeyRetrieved", "Object storage key retrieved")
			} else {
				keyScope.Logger.Error(err, "Failed check for access key secret")
				r.setFailure(keyScope, fmt.Errorf("failed check for access key secret: %w", err))

				return err
			}
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
			emptySecret.Type = keyScope.Key.Spec.Type
			emptySecret.StringData = secret.StringData
			emptySecret.Data = nil

			return nil
		})
		if err != nil {
			keyScope.Logger.Error(err, "Failed to apply key secret")
			r.setFailure(keyScope, err)

			return err
		}

		keyScope.Logger.Info(fmt.Sprintf("Secret %s/%s was %s with access key", secret.Namespace, secret.Name, operation))
		r.Recorder.Event(keyScope.Key, corev1.EventTypeNormal, "KeyStored", "Object storage key stored in secret")
	}

	keyScope.Key.Status.LastKeyGeneration = &keyScope.Key.Spec.KeyGeneration
	keyScope.Key.Status.Ready = true

	conditions.Set(keyScope.Key, metav1.Condition{
		Type:   string(clusterv1.ReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: "LinodeObjectStorageKeySynced", // We have to set the reason to not fail object patching
	})
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

	// If this key's Secret was generated in another namespace, manually delete it since it has no owner reference.
	if keyScope.Key.Spec.Namespace != keyScope.Key.Namespace {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      keyScope.Key.Spec.Name,
				Namespace: keyScope.Key.Spec.Namespace,
			},
		}
		if err := keyScope.Client.Delete(ctx, &secret); err != nil {
			err := errors.New("failed to delete generated secret; unable to delete")
			keyScope.Logger.Error(err, "client.Delete")
			r.setFailure(keyScope, err)

			return err
		}
	}

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
			predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), mgr.GetLogger(), r.WatchFilterValue),
			predicate.GenerationChangedPredicate{},
		)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(linodeObjectStorageKeyMapper),
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureReady(mgr.GetScheme(), mgr.GetLogger())),
		).Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(r, wrappedruntimereconciler.DefaultDecorator()))
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

func (r *LinodeObjectStorageKeyReconciler) TracedClient() client.Client {
	return wrappedruntimeclient.NewRuntimeClientWithTracing(r.Client, wrappedruntimeclient.DefaultDecorator())
}
