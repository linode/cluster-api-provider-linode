package controller

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
)

func (r *LinodeClusterReconciler) addMachineToLB(ctx context.Context, clusterScope *scope.ClusterScope) error {
	logger := logr.FromContextOrDiscard(ctx)
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType != "dns" {
		for _, eachMachine := range clusterScope.LinodeMachines.Items {
			if err := services.AddNodesToNB(ctx, logger, clusterScope, eachMachine); err != nil {
				return err
			}
		}
	} else {
		if err := services.EnsureDNSEntries(ctx, clusterScope, "create"); err != nil {
			return err
		}
	}

	return nil
}

func (r *LinodeClusterReconciler) removeMachineFromLB(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "NodeBalancer" {
		if err := services.DeleteNodesFromNB(ctx, logger, clusterScope); err != nil {
			logger.Error(err, "Failed to remove node from Node Balancer backend")
			return err
		}
	} else if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "dns" {
		if err := services.EnsureDNSEntries(ctx, clusterScope, "delete"); err != nil {
			logger.Error(err, "Failed to remove IP from DNS")
			return err
		}
	}
	return nil
}
