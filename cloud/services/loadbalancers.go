package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/linodego"
	kutil "sigs.k8s.io/cluster-api/util"
)

const (
	defaultLBPort = 6443
)

// CreateNodeBalancer creates a new NodeBalancer if one doesn't exist
func CreateNodeBalancer(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (*linodego.NodeBalancer, error) {
	var linodeNBs []linodego.NodeBalancer
	var linodeNB *linodego.NodeBalancer
	NBLabel := fmt.Sprintf("%s-api-server", clusterScope.LinodeCluster.Name)
	clusterUID := string(clusterScope.LinodeCluster.UID)
	tags := []string{string(clusterScope.LinodeCluster.UID)}
	filter := map[string]string{
		"label": NBLabel,
	}

	rawFilter, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	if linodeNBs, err = clusterScope.LinodeClient.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(rawFilter))); err != nil {
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

	logger.Info(fmt.Sprintf("Creating NodeBalancer %s-api-server", clusterScope.LinodeCluster.Name))
	createConfig := linodego.NodeBalancerCreateOptions{
		Label:  util.Pointer(fmt.Sprintf("%s-api-server", clusterScope.LinodeCluster.Name)),
		Region: clusterScope.LinodeCluster.Spec.Region,
		Tags:   tags,
	}

	if linodeNB, err = clusterScope.LinodeClient.CreateNodeBalancer(ctx, createConfig); err != nil {
		logger.Info("Failed to create Linode NodeBalancer", "error", err.Error())

		// Already exists is not an error
		apiErr := linodego.Error{}
		if errors.As(err, &apiErr) && apiErr.Code != http.StatusFound {
			return nil, err
		}

		if linodeNB != nil {
			logger.Info("Linode NodeBalancer already exists", "existing", linodeNB.Label)
		}
	}

	return linodeNB, nil
}

// CreateNodeBalancerConfig creates NodeBalancer config if it does not exist
func CreateNodeBalancerConfig(
	ctx context.Context,
	clusterScope *scope.ClusterScope,
	logger logr.Logger,
) (*linodego.NodeBalancerConfig, error) {
	var linodeNBConfig *linodego.NodeBalancerConfig
	var err error

	lbPort := defaultLBPort
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerPort != 0 {
		lbPort = clusterScope.LinodeCluster.Spec.Network.LoadBalancerPort
	}
	createConfig := linodego.NodeBalancerConfigCreateOptions{
		Port:      lbPort,
		Protocol:  linodego.ProtocolTCP,
		Algorithm: linodego.AlgorithmRoundRobin,
		Check:     linodego.CheckConnection,
	}

	if linodeNBConfig, err = clusterScope.LinodeClient.CreateNodeBalancerConfig(
		ctx,
		clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
		createConfig,
	); err != nil {
		logger.Info("Failed to create Linode NodeBalancer config", "error", err.Error())

		return nil, err
	}

	return linodeNBConfig, nil
}

// AddNodeToNB adds a backend Node on the Node Balancer configuration
func AddNodeToNB(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	clusterScope *scope.ClusterScope,
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

	lbPort := defaultLBPort
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerPort != 0 {
		lbPort = clusterScope.LinodeCluster.Spec.Network.LoadBalancerPort
	}
	if machineScope.LinodeCluster.Spec.Network.NodeBalancerConfigID == nil {
		err := errors.New("nil NodeBalancer Config ID")
		logger.Error(err, "config ID for NodeBalancer is nil")

		return err
	}
	_, err = machineScope.LinodeClient.CreateNodeBalancerNode(
		ctx,
		machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*machineScope.LinodeCluster.Spec.Network.NodeBalancerConfigID,
		linodego.NodeBalancerNodeCreateOptions{
			Label:   machineScope.Cluster.Name,
			Address: fmt.Sprintf("%s:%d", addresses.IPv4.Private[0].Address, lbPort),
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

	if machineScope.LinodeMachine.Spec.InstanceID == nil {
		return errors.New("no InstanceID")
	}

	err := machineScope.LinodeClient.DeleteNodeBalancerNode(
		ctx,
		machineScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*machineScope.LinodeCluster.Spec.Network.NodeBalancerConfigID,
		*machineScope.LinodeMachine.Spec.InstanceID,
	)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		logger.Error(err, "Failed to update Node Balancer")

		return err
	}

	return nil
}
