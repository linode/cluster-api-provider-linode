package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	kutil "sigs.k8s.io/cluster-api/util"

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

// AddNodeToNB adds a backend Node on the Node Balancer configuration
func AddNodeToNB(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) error {
	// Update the NB backend with the new instance if it's a control plane node
	if !kutil.IsControlPlaneMachine(machineScope.Machine) {
		return nil
	}

	instanceID, err := util.GetInstanceID(machineScope.LinodeMachine.Spec.ProviderID)
	if err != nil {
		logger.Error(err, "Failed to parse instance ID from provider ID")
		return err
	}
	// Get the private IP that was assigned
	addresses, err := machineScope.LinodeClient.GetInstanceIPAddresses(ctx, instanceID)
	if err != nil {
		logger.Error(err, "Failed get instance IP addresses")

		return err
	}
	if len(addresses.IPv4.Private) == 0 {
		err := errors.New("no private IP address")
		logger.Error(err, "no private IPV4 addresses set for LinodeInstance")

		return err
	}

	apiserverLBPort := DefaultApiserverLBPort
	if machineScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiserverLBPort = machineScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}

	if machineScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID == nil {
		err := errors.New("nil NodeBalancer Config ID")
		logger.Error(err, "config ID for NodeBalancer is nil")

		return err
	}

	_, err = machineScope.LinodeClient.CreateNodeBalancerNode(
		ctx,
		*machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*machineScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID,
		linodego.NodeBalancerNodeCreateOptions{
			Label:   machineScope.Cluster.Name,
			Address: fmt.Sprintf("%s:%d", addresses.IPv4.Private[0].Address, apiserverLBPort),
			Mode:    linodego.ModeAccept,
		},
	)
	if err != nil {
		logger.Error(err, "Failed to update Node Balancer")
		return err
	}

	for _, portConfig := range machineScope.LinodeCluster.Spec.Network.AdditionalPorts {
		_, err = machineScope.LinodeClient.CreateNodeBalancerNode(
			ctx,
			*machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
			*portConfig.NodeBalancerConfigID,
			linodego.NodeBalancerNodeCreateOptions{
				Label:   machineScope.Cluster.Name,
				Address: fmt.Sprintf("%s:%d", addresses.IPv4.Private[0].Address, portConfig.Port),
				Mode:    linodego.ModeAccept,
			},
		)
		if err != nil {
			logger.Error(err, "Failed to update Node Balancer")
			return err
		}
	}

	return nil
}

// DeleteNodeFromNB removes a backend Node from the Node Balancer configuration
func DeleteNodeFromNB(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) error {
	// Update the NB to remove the node if it's a control plane node
	if !kutil.IsControlPlaneMachine(machineScope.Machine) {
		return nil
	}

	if machineScope.LinodeCluster.Spec.ControlPlaneEndpoint.Host == "" {
		logger.Info("NodeBalancer already deleted, no NodeBalancer backend Node to remove")

		return nil
	}

	instanceID, err := util.GetInstanceID(machineScope.LinodeMachine.Spec.ProviderID)
	if err != nil {
		logger.Error(err, "Failed to parse instance ID from provider ID")
		return err
	}
	err = machineScope.LinodeClient.DeleteNodeBalancerNode(
		ctx,
		*machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*machineScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID,
		instanceID,
	)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		logger.Error(err, "Failed to update Node Balancer")

		return err
	}

	for _, portConfig := range machineScope.LinodeCluster.Spec.Network.AdditionalPorts {
		err = machineScope.LinodeClient.DeleteNodeBalancerNode(
			ctx,
			*machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
			*portConfig.NodeBalancerConfigID,
			instanceID,
		)
		if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
			logger.Error(err, "Failed to update Node Balancer")
			return err
		}
	}

	return nil
}
