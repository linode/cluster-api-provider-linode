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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
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
			log.Error(err, "Failed to fetch Linode machine")
		}

		return ctrl.Result{}, err
	}

	objectStorageBucketScope, err := scope.NewObjectStorageBucketScope(
		ctx,
		scope.ObjectStorageBucketScopeParams{
			Client:              r.Client,
			ObjectStorageBucket: objectStorageBucket,
		},
	)
	if err != nil {
		log.Error(err, "Failed to create object storage bucket scope")

		return ctrl.Result{}, fmt.Errorf("failed to create object storage bucket scope: %w", err)
	}

	return r.reconcile(ctx, log, objectStorageBucketScope)
}

func (r *LinodeObjectStorageBucketReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	scope *scope.ObjectStorageBucketScope,
) (res ctrl.Result, reterr error) {
	scope.ObjectStorageBucket.Status.Ready = false

	// Always close the scope when exiting this function so we can persist any LinodeObjectStorageBucket changes.
	defer func() {
		// Filter out any IsNotFound message since client.IgnoreNotFound does not handle aggregate errors
		if err := scope.Close(ctx); utilerrors.FilterOut(err, apierrors.IsNotFound) != nil && reterr == nil {
			logger.Error(err, "failed to patch LinodeObjectStorageBucket")
			reterr = err
		}
	}()

	// Deleted
	if !scope.ObjectStorageBucket.DeletionTimestamp.IsZero() {
		return res, r.reconcileDelete(ctx, logger, scope)
	}

	if err := scope.AddFinalizer(ctx); err != nil {
		return res, err
	}
	// Created
	if scope.ObjectStorageBucket.Status.AccessKeySecretName == nil {
		if err := r.reconcileCreate(ctx, logger, scope); err != nil {
			return res, err
		}
		//r.Recorder.Event(scope.ObjectStorageBucket, corev1.EventTypeNormal, "Ready", "Object storage bucket has been created")
	} else {
		// Updated
		if err := r.reconcileUpdate(ctx, logger, scope); err != nil {
			return res, err
		}
		//r.Recorder.Event(scope.ObjectStorageBucket, corev1.EventTypeNormal, "Updated", "Object storage bucket has been created")
	}

	scope.ObjectStorageBucket.Status.Ready = true
	return res, nil
}

func (r *LinodeObjectStorageBucketReconciler) reconcileCreate(ctx context.Context, logger logr.Logger, scope *scope.ObjectStorageBucketScope) error {
	panic("unimplemented")
}

func (r *LinodeObjectStorageBucketReconciler) reconcileUpdate(ctx context.Context, logger logr.Logger, scope *scope.ObjectStorageBucketScope) error {
	panic("unimplemented")
}

func (r *LinodeObjectStorageBucketReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, scope *scope.ObjectStorageBucketScope) error {
	panic("unimplemented")
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinodeObjectStorageBucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LinodeObjectStorageBucket{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue)).
		Complete(r)
}
