package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	kutil "sigs.k8s.io/cluster-api/util"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

const (
	defaultApiserverLBPort    = 6443
	defaultKonnectivityLBPort = 8132
)

// CreateNodeBalancer creates a new NodeBalancer if one doesn't exist
func CreateNodeBalancer(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (*linodego.NodeBalancer, error) {
	var linodeNB *linodego.NodeBalancer

	NBLabel := clusterScope.LinodeCluster.Name
	clusterUID := string(clusterScope.LinodeCluster.UID)
	tags := []string{string(clusterScope.LinodeCluster.UID)}
	listFilter := util.Filter{
		ID:    clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
		Label: NBLabel,
		Tags:  tags,
	}
	filter, err := listFilter.String()
	if err != nil {
		return nil, err
	}
	linodeNBs, err := clusterScope.LinodeClient.ListNodeBalancers(ctx, linodego.NewListOptions(1, filter))
	if err != nil {
		logger.Info("Failed to list NodeBalancers", "error", err.Error())

		return nil, err
	}
	if len(linodeNBs) == 1 {
		logger.Info(fmt.Sprintf("NodeBalancer %s already exists", *linodeNBs[0].Label))
		if !slices.Contains(linodeNBs[0].Tags, clusterUID) {
			err = errors.New("NodeBalancer conflict")
			logger.Error(err, fmt.Sprintf("NodeBalancer %s is not associated with cluster UID %s. Owner cluster is %s", *linodeNBs[0].Label, clusterUID, linodeNBs[0].Tags[0]))

			return nil, err
		}

		return &linodeNBs[0], nil
	}

	logger.Info(fmt.Sprintf("Creating NodeBalancer %s", clusterScope.LinodeCluster.Name))
	createConfig := linodego.NodeBalancerCreateOptions{
		Label:  util.Pointer(clusterScope.LinodeCluster.Name),
		Region: clusterScope.LinodeCluster.Spec.Region,
		Tags:   tags,
	}

	linodeNB, err = clusterScope.LinodeClient.CreateNodeBalancer(ctx, createConfig)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		return nil, err
	}
	if linodeNB != nil {
		logger.Info("Linode NodeBalancer already exists", "existing", linodeNB.Label)
	}

	return linodeNB, nil
}

// CreateNodeBalancerConfigs creates NodeBalancer configs if it does not exist
func CreateNodeBalancerConfigs(
	ctx context.Context,
	clusterScope *scope.ClusterScope,
	logger logr.Logger,
) ([]*linodego.NodeBalancerConfig, error) {
	nbConfigs := []*linodego.NodeBalancerConfig{}
	apiLBPort := defaultApiserverLBPort
	if clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiLBPort = clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}
	apiserverCreateConfig := linodego.NodeBalancerConfigCreateOptions{
		Port:      apiLBPort,
		Protocol:  linodego.ProtocolTCP,
		Algorithm: linodego.AlgorithmRoundRobin,
		Check:     linodego.CheckConnection,
	}

	apiserverLinodeNBConfig, err := clusterScope.LinodeClient.CreateNodeBalancerConfig(
		ctx,
		*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
		apiserverCreateConfig,
	)
	if err != nil {
		logger.Info("Failed to create Linode NodeBalancer config", "error", err.Error())
		return nil, err
	}
	nbConfigs = append(nbConfigs, apiserverLinodeNBConfig)

	// return if konnectivity should not be configured
	if !clusterScope.LinodeCluster.Spec.Network.Konnectivity {
		return nbConfigs, nil
	}

	konnLBPort := defaultKonnectivityLBPort
	if clusterScope.LinodeCluster.Spec.Network.KonnectivityLoadBalancerPort != 0 {
		konnLBPort = clusterScope.LinodeCluster.Spec.Network.KonnectivityLoadBalancerPort
	}
	konnectivityCreateConfig := linodego.NodeBalancerConfigCreateOptions{
		Port:      konnLBPort,
		Protocol:  linodego.ProtocolTCP,
		Algorithm: linodego.AlgorithmRoundRobin,
		Check:     linodego.CheckConnection,
	}

	konnectivityLinodeNBConfig, err := clusterScope.LinodeClient.CreateNodeBalancerConfig(
		ctx,
		*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
		konnectivityCreateConfig,
	)
	if err != nil {
		logger.Info("Failed to create Linode NodeBalancer config", "error", err.Error())
		return nil, err
	}
	nbConfigs = append(nbConfigs, konnectivityLinodeNBConfig)

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

	// Get the private IP that was assigned
	addresses, err := machineScope.LinodeClient.GetInstanceIPAddresses(ctx, *machineScope.LinodeMachine.Spec.InstanceID)
	if err != nil {
		logger.Error(err, "Failed get instance IP addresses")

		return err
	}
	if len(addresses.IPv4.Private) == 0 {
		err := errors.New("no private IP address")
		logger.Error(err, "no private IPV4 addresses set for LinodeInstance")

		return err
	}

	apiserverLBPort := defaultApiserverLBPort
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

	// return if konnectivity should not be configured
	if !machineScope.LinodeCluster.Spec.Network.Konnectivity {
		return nil
	}

	konnectivityLBPort := defaultKonnectivityLBPort
	if machineScope.LinodeCluster.Spec.Network.KonnectivityLoadBalancerPort != 0 {
		konnectivityLBPort = machineScope.LinodeCluster.Spec.Network.KonnectivityLoadBalancerPort
	}

	if machineScope.LinodeCluster.Spec.Network.KonnectivityNodeBalancerConfigID == nil {
		err := errors.New("nil NodeBalancer Config ID")
		logger.Error(err, "config ID for NodeBalancer is nil")

		return err
	}

	_, err = machineScope.LinodeClient.CreateNodeBalancerNode(
		ctx,
		*machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*machineScope.LinodeCluster.Spec.Network.KonnectivityNodeBalancerConfigID,
		linodego.NodeBalancerNodeCreateOptions{
			Label:   machineScope.Cluster.Name,
			Address: fmt.Sprintf("%s:%d", addresses.IPv4.Private[0].Address, konnectivityLBPort),
			Mode:    linodego.ModeAccept,
		},
	)
	if err != nil {
		logger.Error(err, "Failed to update Node Balancer")
		return err
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

	err := machineScope.LinodeClient.DeleteNodeBalancerNode(
		ctx,
		*machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*machineScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID,
		*machineScope.LinodeMachine.Spec.InstanceID,
	)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		logger.Error(err, "Failed to update Node Balancer")

		return err
	}

	if !machineScope.LinodeCluster.Spec.Network.Konnectivity {
		return nil
	}

	err = machineScope.LinodeClient.DeleteNodeBalancerNode(
		ctx,
		*machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*machineScope.LinodeCluster.Spec.Network.KonnectivityNodeBalancerConfigID,
		*machineScope.LinodeMachine.Spec.InstanceID,
	)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		logger.Error(err, "Failed to update Node Balancer")
		return err
	}

	return nil
}
