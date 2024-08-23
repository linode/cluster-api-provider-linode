package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
)

func (r *LinodeClusterReconciler) addMachineToLB(ctx context.Context, clusterScope *scope.ClusterScope) error {
	logger := logr.FromContextOrDiscard(ctx)
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "dns" {
		if err := services.EnsureDNSEntries(ctx, clusterScope, "create"); err != nil {
			return err
		}
		return nil
	}
	nodeBalancerNodes, err := clusterScope.LinodeClient.ListNodeBalancerNodes(
		ctx,
		*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID,
		&linodego.ListOptions{},
	)
	if err != nil {
		return err
	}
	for _, eachMachine := range clusterScope.LinodeMachines.Items {
		err = services.AddNodesToNB(ctx, logger, clusterScope, eachMachine, nodeBalancerNodes)
		if err != nil {
			return err
		}
	}
	ipPortCombo := getIPPortCombo(clusterScope)
	for _, node := range nodeBalancerNodes {
		if !slices.Contains(ipPortCombo, node.Address) {
			if err := clusterScope.LinodeClient.DeleteNodeBalancerNode(ctx, node.NodeBalancerID, node.ConfigID, node.ID); err != nil {
				return err
			}
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

func getIPPortCombo(cscope *scope.ClusterScope) (ipPortComboList []string) {
	apiserverLBPort := services.DefaultApiserverLBPort
	if cscope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiserverLBPort = cscope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}
	for _, eachMachine := range cscope.LinodeMachines.Items {
		for _, IPs := range eachMachine.Status.Addresses {
			if IPs.Type != v1beta1.MachineInternalIP || !strings.Contains(IPs.Address, "192.168") {
				continue
			}
			ipPortComboList = append(ipPortComboList, fmt.Sprintf("%s:%d", IPs.Address, apiserverLBPort))
			for _, portConfig := range cscope.LinodeCluster.Spec.Network.AdditionalPorts {
				ipPortComboList = append(ipPortComboList, fmt.Sprintf("%s:%d", IPs.Address, portConfig.Port))
			}
		}
	}
	return ipPortComboList
}
