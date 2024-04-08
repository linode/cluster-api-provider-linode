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
	"bytes"
	"context"
	"encoding/gob"
	"errors"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

func (r *LinodeVPCReconciler) reconcileVPC(ctx context.Context, vpcScope *scope.VPCScope, logger logr.Logger) error {
	createConfig := linodeVPCSpecToVPCCreateConfig(vpcScope.LinodeVPC.Spec)
	if createConfig == nil {
		err := errors.New("failed to convert VPC spec to create VPC config")

		logger.Error(err, "Panic! Struct of LinodeVPCSpec is different than VPCCreateOptions")

		return err
	}

	createConfig.Label = vpcScope.LinodeVPC.Name
	listFilter := util.Filter{
		ID:    vpcScope.LinodeVPC.Spec.VPCID,
		Label: createConfig.Label,
		Tags:  nil,
	}
	filter, err := listFilter.String()
	if err != nil {
		return err
	}
	if vpcs, err := vpcScope.LinodeClient.ListVPCs(ctx, linodego.NewListOptions(1, filter)); err != nil {
		logger.Error(err, "Failed to list VPCs")

		return err
	} else if len(vpcs) != 0 {
		// Labels are unique
		vpcScope.LinodeVPC.Spec.VPCID = &vpcs[0].ID

		return nil
	}

	vpc, err := vpcScope.LinodeClient.CreateVPC(ctx, *createConfig)
	if err != nil {
		logger.Error(err, "Failed to create VPC")

		return err
	} else if vpc == nil {
		err = errors.New("missing VPC")

		logger.Error(err, "Panic! Failed to create VPC")

		return err
	}

	vpcScope.LinodeVPC.Spec.VPCID = &vpc.ID

	return nil
}

func linodeVPCSpecToVPCCreateConfig(vpcSpec infrav1alpha1.LinodeVPCSpec) *linodego.VPCCreateOptions {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(vpcSpec)
	if err != nil {
		return nil
	}

	var createConfig linodego.VPCCreateOptions
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&createConfig)
	if err != nil {
		return nil
	}

	return &createConfig
}

func (r *LinodeVPCReconciler) requeueLinodeVPCsForUnpausedCluster(logger logr.Logger) handler.MapFunc {
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

		requests, err := r.requestsForClusterVPC(ctx, cluster.Namespace, cluster.Name)
		if err != nil {
			logger.Error(err, "Failed to create request for cluster")

			return nil
		}

		return requests
	}
}

func (r *LinodeVPCReconciler) requestsForClusterVPC(ctx context.Context, namespace, name string) ([]ctrl.Request, error) {
	labels := map[string]string{clusterv1.ClusterNameLabel: name}

	vpcList := infrav1alpha1.LinodeVPCList{}
	if err := r.Client.List(ctx, &vpcList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	result := make([]ctrl.Request, 0, len(vpcList.Items))
	for _, item := range vpcList.Items {
		result = append(result, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: item.Namespace,
				Name:      item.Name,
			},
		})
	}

	return result, nil
}
