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

	"github.com/go-logr/logr"
	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)


func (r *LinodeObjectStorageBucketReconciler) requeueLinodeObjectStorageBucketForUnpausedCluster(logger logr.Logger) handler.MapFunc {
	logger = logger.WithName("LinodeVPCReconciler").WithName("requeueLinodeVPCsForUnpausedCluster")

	return func(ctx context.Context, o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		cluster, ok := o.(*clusterv1.Cluster)
		if !ok {
			logger.Info("Failed to cast object to Cluster")

			return nil
		}

		if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Info("Cluster has a deletion timestamp, skipping mapping")

			return nil
		}

		requests, err := r.requestsForClusterObjectStorageBuckets(ctx, cluster.Namespace, cluster.Name)
		if err != nil {
			logger.Error(err, "Failed to create request for cluster")

			return nil
		}

		return requests
	}
}

func (r *LinodeObjectStorageBucketReconciler) requestsForClusterObjectStorageBuckets(ctx context.Context, namespace, name string) ([]ctrl.Request, error) {
	labels := map[string]string{clusterv1.ClusterNameLabel: name}

	objList := infrav1alpha1.LinodeObjectStorageBucketList{}
	if err := r.Client.List(ctx, &objList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	result := make([]ctrl.Request, 0, len(objList.Items))
	for _, item := range objList.Items {
		result = append(result, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: item.Namespace,
				Name:      item.Name,
			},
		})
	}

	return result, nil
}

