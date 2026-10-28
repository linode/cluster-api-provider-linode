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
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	kutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/paused"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/linode/cluster-api-provider-linode/clients"
	wrappedruntimeclient "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimeclient"
	wrappedruntimereconciler "github.com/linode/cluster-api-provider-linode/observability/wrappers/runtimereconciler"
	"github.com/linode/cluster-api-provider-linode/util"
)

type Reconciler[S scope, O client.Object] interface {
	Name() string
	Object() O
	AddWatches(*builder.Builder, *runtime.Scheme)
	NewScope(context.Context, clients.K8sClient, O, *clusterv1.Cluster, logr.Logger) (S, error)
	Apply(context.Context, S) (ctrl.Result, error)
	Delete(context.Context, S) (ctrl.Result, error)
}

type scope interface {
	SetCredentialRefTokenForLinodeClients(context.Context) error
	AddFinalizer(context.Context) error
	AddCredentialsRefFinalizer(context.Context) error
	Close(context.Context) error
}

type Controller[S scope, O client.Object] struct {
	rec      Reconciler[S, O]
	cl       clients.K8sClient
	newScope func(context.Context, clients.K8sClient, O, *clusterv1.Cluster, logr.Logger) (S, error)
}

func NewControllerFor[S scope, O client.Object](rec Reconciler[S, O]) *Controller[S, O] {
	return &Controller[S, O]{rec: rec}
}

func (c *Controller[S, O]) WithNewScopeForTest(fn func(context.Context, clients.K8sClient, O, *clusterv1.Cluster, logr.Logger) (S, error)) *Controller[S, O] {
	c.newScope = fn
	return c
}

func (c *Controller[S, O]) WithClientForTest(cl clients.K8sClient) *Controller[S, O] {
	c.cl = cl
	return c
}

func (c *Controller[S, O]) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	c.cl = wrappedruntimeclient.NewRuntimeClientWithTracing(mgr.GetClient(), wrappedruntimeclient.DefaultDecorator())

	obj := c.rec.Object()

	bldr := ctrl.NewControllerManagedBy(mgr).For(obj).WithOptions(options)

	c.rec.AddWatches(bldr, c.cl.Scheme())

	objList, err := objectListKind(c.cl.Scheme(), obj)
	if err != nil {
		return err
	}

	mapper, err := kutil.ClusterToTypedObjectsMapper(c.cl, objList, c.cl.Scheme())
	if err != nil {
		return fmt.Errorf("failed to create mapper for LinodeVPCs: %w", err)
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

func (c *Controller[S, O]) Reconcile(ctx context.Context, obj O) (res ctrl.Result, err error) {
	logger := ctrl.LoggerFrom(ctx).WithName(c.rec.Name()).WithValues("name", obj.GetName(), "namespace", obj.GetNamespace())

	var cluster *clusterv1.Cluster
	if _, ok := obj.GetLabels()[clusterv1.ClusterNameLabel]; ok {
		metadata := metav1.ObjectMeta{Namespace: obj.GetNamespace(), Labels: obj.GetLabels()}
		cluster, err = kutil.GetClusterFromMetadata(ctx, c.cl, metadata)
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "failed to fetch cluster from metadata")
			return ctrl.Result{}, err
		}

		// Set the owner reference to the Linode cluster if it is not nil.
		// It internally handleds the case where the cluster is not found.
		if err := util.SetOwnerReferenceToLinodeCluster(ctx, c.cl, cluster, obj, c.cl.Scheme()); err != nil {
			return res, err
		}
	}

	// For objects that implement paused.ConditionSetter, check pause if not deleting or if cluster still exists.
	condObj, isCondObj := any(obj).(paused.ConditionSetter)
	if (obj.GetDeletionTimestamp().IsZero() || cluster != nil) && isCondObj {
		isPaused, _, err := paused.EnsurePausedCondition(ctx, c.cl, cluster, condObj)
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
	recScope, err := newScope(ctx, c.cl, obj, cluster, logger)
	if err != nil {
		return res, err
	}

	// Override the controller credentials with ones from the object's secret reference when supplied.
	if err = recScope.SetCredentialRefTokenForLinodeClients(ctx); err != nil {
		logger.Error(err, "failed to update Linode client token from credential ref")
		return res, err
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		return c.rec.Delete(ctx, recScope)
	}

	if err = recScope.AddCredentialsRefFinalizer(ctx); err != nil {
		logger.Error(err, "failed to add credentials ref finalizer")
		return res, err
	}

	if err = recScope.AddFinalizer(ctx); err != nil {
		logger.Error(err, "failed to add finalizer")
		return res, err
	}

	return c.rec.Apply(ctx, recScope)
}
