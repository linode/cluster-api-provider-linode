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

package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/paused"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/linode/cluster-api-provider-linode/clients"
	wrappedruntimeclient "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimeclient"
	wrappedruntimereconciler "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimereconciler"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

type Reconciler[S scope, O client.Object] interface {
	// Name returns the name of the object being reconciled.
	Name() string
	// Object returns a pointer to the object kind being reconciled.
	Object() O
	// RequeueAfter tells the controller how long to wait before triggering another reconciliation.
	RequeueAfter() time.Duration
	// Timeout returns the maximum duration for a single reconciliation.
	Timeout() time.Duration
	// AddWatches adds watches to the controller builder.
	AddWatches(*builder.Builder, clients.K8sClient)
	// NewScope creates a new scope for each reconciliation.
	NewScope(context.Context, clients.K8sClient, O, *clusterv1.Cluster) (S, error)
	// Apply applies the desired state to the object.
	Apply(context.Context, S, logr.Logger) (ctrl.Result, error)
	// Delete handles the deletion of the object.
	Delete(context.Context, S, logr.Logger) (ctrl.Result, error)
	// HandleFailureCondition handles the failure condition for a single reconciliation.
	HandleFailureCondition(S, error)
}

type scope interface {
	AddFinalizer(context.Context) error
	RemoveFinalizer()
	AddCredentialsRefFinalizer(context.Context) error
	RemoveCredentialsRefFinalizer(context.Context) error
	Close(context.Context) error
	SetCredentialRefTokenForLinodeClients(context.Context) error
}

type object interface {
	client.Object
	paused.ConditionSetter
	SetCondition(metav1.Condition)
	GetCondition(string) *metav1.Condition
}

type controller[S scope, O object] struct {
	rec          Reconciler[S, O]
	cl           clients.K8sClient
	newScope     func(context.Context, clients.K8sClient, O, *clusterv1.Cluster) (S, error)
	logrOverride *logr.Logger
}

// NewControllerFor creates a new controller for the given Reconciler implementation.
func NewControllerFor[S scope, O object](rec Reconciler[S, O]) *controller[S, O] {
	return &controller[S, O]{rec: rec}
}

// WithClientForTest sets the Kubernetes client for the controller, primarily used for testing.
func (c *controller[S, O]) WithClientForTest(cl clients.K8sClient) *controller[S, O] {
	c.cl = cl
	return c
}

// WithNewScopeForTest sets the function to create a new scope for the controller, primarily used for testing.
func (c *controller[S, O]) WithNewScopeForTest(fn func(context.Context, clients.K8sClient, O, *clusterv1.Cluster) (S, error)) *controller[S, O] {
	c.newScope = fn
	return c
}

// WithLoggerForTest sets the logger for the controller, primarily used for testing.
func (c *controller[S, O]) WithLoggerForTest(logger logr.Logger) *controller[S, O] {
	c.logrOverride = &logger
	return c
}

// SetupWithManager sets up the controller with the given manager and options.
func (c *controller[S, O]) SetupWithManager(mgr ctrl.Manager, options ctrlcontroller.Options) error {
	c.cl = wrappedruntimeclient.NewRuntimeClientWithTracing(mgr.GetClient(), wrappedruntimeclient.DefaultDecorator())

	obj := c.rec.Object()
	bldr := ctrl.NewControllerManagedBy(mgr).For(obj).WithOptions(options)
	c.rec.AddWatches(bldr, c.cl)

	objList, err := objectListKind(c.cl.Scheme(), obj)
	if err != nil {
		return err
	}

	mapper, err := kutil.ClusterToTypedObjectsMapper(c.cl, objList, c.cl.Scheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for %s: %w", c.rec.Name(), err)
	}

	return bldr.
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(mapper),
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureProvisioned(c.cl.Scheme(), mgr.GetLogger())),
		).
		Complete(wrappedruntimereconciler.NewRuntimeReconcilerWithTracing(
			reconcile.AsReconciler(c.cl, c),
			wrappedruntimereconciler.DefaultDecorator(),
		))
}

// Reconcile is the main reconciliation loop for the controller.
// It handles cluster ownership, pause checks, scope creation, credential secret management,
// and delegates to the Reconciler implementation for actual reconciliation logic.
func (c *controller[S, O]) Reconcile(ctx context.Context, obj O) (res ctrl.Result, err error) {
	var logger logr.Logger
	if c.logrOverride != nil {
		logger = *c.logrOverride
	} else {
		logger = ctrl.LoggerFrom(ctx)
	}
	logger = logger.WithName(c.rec.Name()).WithValues("name", obj.GetName(), "namespace", obj.GetNamespace())

	var cluster *clusterv1.Cluster
	if _, ok := obj.GetLabels()[clusterv1.ClusterNameLabel]; ok {
		cluster, err = c.setClusterOwner(ctx, obj, logger)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check pause if not deleting or if cluster still exists.
	if obj.GetDeletionTimestamp().IsZero() || cluster != nil {
		isPaused, _, err := paused.EnsurePausedCondition(ctx, c.cl, cluster, obj)
		if err != nil {
			return res, err
		}
		if isPaused {
			logger.Info("resource or linked cluster is paused, skipping reconciliation")
			return res, nil
		}
	}

	newScope := c.rec.NewScope
	if c.newScope != nil {
		newScope = c.newScope
	}
	recScope, err := newScope(ctx, c.cl, obj, cluster)
	if err != nil {
		return res, err
	}

	// Override the controller credentials with ones from the object's secret reference when supplied.
	if err = recScope.SetCredentialRefTokenForLinodeClients(ctx); err != nil {
		logger.Error(err, "failed to update Linode client token from credential ref")
		return res, err
	}

	// Defer function to handle failure conditions for either Delete/Apply, requeue logic, and patching the object.
	defer func() {
		if err != nil {
			c.rec.HandleFailureCondition(recScope, err)
		}
		if patchErr := recScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			logger.Error(patchErr, "failed to patch")
			err = errors.Join(err, patchErr)
		}
	}()

	if !obj.GetDeletionTimestamp().IsZero() {
		return c.handleDelete(ctx, recScope, obj, logger)
	}

	return c.handleApply(ctx, recScope, obj, logger)
}

func (c *controller[S, O]) setClusterOwner(ctx context.Context, obj O, logger logr.Logger) (*clusterv1.Cluster, error) {
	cluster, err := kutil.GetClusterFromMetadata(ctx, c.cl, metav1.ObjectMeta{Namespace: obj.GetNamespace(), Labels: obj.GetLabels()})
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "failed to fetch cluster from metadata")
		return nil, err
	}

	// Set the owner reference to the Linode cluster if it is not nil.
	// It internally handleds the case where the cluster is not found.
	if err := util.SetOwnerReferenceToLinodeCluster(ctx, c.cl, cluster, obj, c.cl.Scheme()); err != nil {
		return nil, err
	}

	return cluster, nil
}

func (c *controller[S, O]) handleDelete(ctx context.Context, recScope S, obj O, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("deleting")

	res, err := c.rec.Delete(ctx, recScope, logger)
	if err != nil {
		return res, err
	}
	if res.RequeueAfter > 0 {
		return res, nil
	}

	err = recScope.RemoveCredentialsRefFinalizer(ctx)
	if err != nil {
		logger.Error(err, "failed to remove credentials ref finalizer")
		if obj.GetDeletionTimestamp().Add(c.rec.Timeout()).After(time.Now()) {
			logger.Info("re-queuing deletion")
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(c.rec.RequeueAfter())}, nil
		}
		// TODO: shouldn't this return nil? i.e. reconciling has timed out
		return ctrl.Result{}, err
	}

	recScope.RemoveFinalizer()

	return ctrl.Result{}, nil
}

func (c *controller[S, O]) handleApply(ctx context.Context, recScope S, obj O, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("applying")

	if err := recScope.AddCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "failed to add credentials ref finalizer")
		return ctrl.Result{}, err
	}

	if err := recScope.AddFinalizer(ctx); err != nil {
		logger.Error(err, "failed to add finalizer")
		return ctrl.Result{}, err
	}

	obj.SetCondition(metav1.Condition{
		Type:    string(clusterv1.ReadyCondition),
		Status:  metav1.ConditionUnknown,
		Reason:  "Reconciling",
		Message: fmt.Sprintf("%s is being reconciled", c.rec.Name()),
	})

	res, err := c.rec.Apply(ctx, recScope, logger)
	if err != nil {
		if !reconciler.HasStaleCondition(obj.GetCondition(string(clusterv1.ReadyCondition)), c.rec.Timeout()) {
			logger.Info("re-queuing reconciliation")
			return ctrl.Result{RequeueAfter: reconciler.WithJitter(c.rec.RequeueAfter())}, nil
		}
		return res, err
	}
	if res.RequeueAfter > 0 {
		return res, nil
	}

	obj.SetCondition(metav1.Condition{
		Type:    string(clusterv1.ReadyCondition),
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: fmt.Sprintf("%s is successfully reconciled", c.rec.Name()),
	})

	return ctrl.Result{}, nil
}
