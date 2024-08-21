package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

const (
	DefaultApiserverLBPort    = 6443
	DefaultKonnectivityLBPort = 8132
)

// EnsureNodeBalancer creates a new NodeBalancer if one doesn't exist or returns the existing NodeBalancer
func EnsureNodeBalancer(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (*linodego.NodeBalancer, error) {
	nbID := clusterScope.LinodeCluster.Spec.Network.NodeBalancerID
	if nbID != nil && *nbID != 0 {
		res, err := clusterScope.LinodeClient.GetNodeBalancer(ctx, *nbID)
		if err != nil {
			logger.Info("Failed to get NodeBalancer", "error", err.Error())

			return nil, err
		}
		return res, nil
	}

	logger.Info(fmt.Sprintf("Creating NodeBalancer %s", clusterScope.LinodeCluster.Name))
	createConfig := linodego.NodeBalancerCreateOptions{
		Label:  util.Pointer(clusterScope.LinodeCluster.Name),
		Region: clusterScope.LinodeCluster.Spec.Region,
		Tags:   []string{string(clusterScope.LinodeCluster.UID)},
	}

	return clusterScope.LinodeClient.CreateNodeBalancer(ctx, createConfig)
}

// EnsureNodeBalancerConfigs creates NodeBalancer configs if it does not exist or returns the existing NodeBalancerConfig
func EnsureNodeBalancerConfigs(
	ctx context.Context,
	clusterScope *scope.ClusterScope,
	logger logr.Logger,
) ([]*linodego.NodeBalancerConfig, error) {
	nbConfigs := []*linodego.NodeBalancerConfig{}
	var apiserverLinodeNBConfig *linodego.NodeBalancerConfig
	var err error
	apiLBPort := DefaultApiserverLBPort
	if clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiLBPort = clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}

	if clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID != nil {
		apiserverLinodeNBConfig, err = clusterScope.LinodeClient.GetNodeBalancerConfig(
			ctx,
			*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
			*clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID)
		if err != nil {
			logger.Info("Failed to get Linode NodeBalancer config", "error", err.Error())
			return nil, err
		}
	} else {
		apiserverLinodeNBConfig, err = clusterScope.LinodeClient.CreateNodeBalancerConfig(
			ctx,
			*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
			linodego.NodeBalancerConfigCreateOptions{
				Port:      apiLBPort,
				Protocol:  linodego.ProtocolTCP,
				Algorithm: linodego.AlgorithmRoundRobin,
				Check:     linodego.CheckConnection,
			},
		)
		if err != nil {
			logger.Info("Failed to create Linode NodeBalancer config", "error", err.Error())
			return nil, err
		}
	}

	nbConfigs = append(nbConfigs, apiserverLinodeNBConfig)

	// return if additional ports should not be configured
	if len(clusterScope.LinodeCluster.Spec.Network.AdditionalPorts) == 0 {
		return nbConfigs, nil
	}

	for _, portConfig := range clusterScope.LinodeCluster.Spec.Network.AdditionalPorts {
		portCreateConfig := linodego.NodeBalancerConfigCreateOptions{
			Port:      portConfig.Port,
			Protocol:  linodego.ProtocolTCP,
			Algorithm: linodego.AlgorithmRoundRobin,
			Check:     linodego.CheckConnection,
		}
		nbConfig, err := clusterScope.LinodeClient.CreateNodeBalancerConfig(
			ctx,
			*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
			portCreateConfig,
		)
		if err != nil {
			logger.Info("Failed to create Linode NodeBalancer config", "error", err.Error())
			return nil, err
		}
		nbConfigs = append(nbConfigs, nbConfig)
	}

	return nbConfigs, nil
}

// AddNodesToNB adds backend Nodes on the Node Balancer configuration
func AddNodesToNB(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope, eachMachine v1alpha2.LinodeMachine) error {
	apiserverLBPort := DefaultApiserverLBPort
	if clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiserverLBPort = clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}

	if clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID == nil {
		return errors.New("nil NodeBalancer Config ID")
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
	internalIPFound := false
	for _, IPs := range eachMachine.Status.Addresses {
		if IPs.Type != v1beta1.MachineInternalIP || !strings.Contains(IPs.Address, "192.168") {
			continue
		}
		internalIPFound = true

		// Set the port number and NB config ID for standard ports
		portsToBeAdded := make([]map[string]int, 0)
		standardPort := map[string]int{"configID": *clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID, "port": apiserverLBPort}
		portsToBeAdded = append(portsToBeAdded, standardPort)

		// Set the port number and NB config ID for any additional ports
		for _, portConfig := range clusterScope.LinodeCluster.Spec.Network.AdditionalPorts {
			portsToBeAdded = append(portsToBeAdded, map[string]int{"configID": *portConfig.NodeBalancerConfigID, "port": portConfig.Port})
		}

		// Cycle through all ports to be added
		for _, ports := range portsToBeAdded {
			ipPortComboExists := false
			for _, nodes := range nodeBalancerNodes {
				// Create the node if the IP:Port combination does not exist
				if nodes.Address == fmt.Sprintf("%s:%d", IPs.Address, ports["port"]) {
					ipPortComboExists = true
					break
				}
			}
			if !ipPortComboExists {
				_, err := clusterScope.LinodeClient.CreateNodeBalancerNode(
					ctx,
					*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
					ports["configID"],
					linodego.NodeBalancerNodeCreateOptions{
						Label:   clusterScope.Cluster.Name,
						Address: fmt.Sprintf("%s:%d", IPs.Address, ports["port"]),
						Mode:    linodego.ModeAccept,
					},
				)
				if err != nil {
					return err
				}
			}
		}
	}
	if !internalIPFound {
		return errors.New("no private IP address")
	}

	return nil
}

// DeleteNodesFromNB removes backend Nodes from the Node Balancer configuration
func DeleteNodesFromNB(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	if clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint.Host == "" {
		logger.Info("NodeBalancer already deleted, no NodeBalancer backend Node to remove")
		return nil
	}

	for _, eachMachine := range clusterScope.LinodeMachines.Items {
		err := clusterScope.LinodeClient.DeleteNodeBalancerNode(
			ctx,
			*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
			*clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID,
			*eachMachine.Spec.InstanceID,
		)
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to update Node Balancer")

			return err
		}

		for _, portConfig := range clusterScope.LinodeCluster.Spec.Network.AdditionalPorts {
			err = clusterScope.LinodeClient.DeleteNodeBalancerNode(
				ctx,
				*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
				*portConfig.NodeBalancerConfigID,
				*eachMachine.Spec.InstanceID,
			)
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
				logger.Error(err, "Failed to update Node Balancer")
				return err
			}
		}
	}

	return nil
}
