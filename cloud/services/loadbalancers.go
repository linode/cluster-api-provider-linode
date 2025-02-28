package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	// if NodeBalancerBackendIPv4Range is set, create the NodeBalancer in the specified VPC
	if clusterScope.LinodeCluster.Spec.Network.NodeBalancerBackendIPv4Range != "" && clusterScope.LinodeCluster.Spec.VPCRef != nil {
		logger.Info("Creating NodeBalancer in VPC", "NodeBalancerBackendIPv4Range", clusterScope.LinodeCluster.Spec.Network.NodeBalancerBackendIPv4Range)
		subnetID, err := getSubnetID(ctx, clusterScope, logger)
		if err != nil {
			logger.Error(err, "Failed to fetch Linode Subnet ID")
			return nil, err
		}
		createConfig.VPCs = []linodego.NodeBalancerVPCOptions{
			{
				IPv4Range: clusterScope.LinodeCluster.Spec.Network.NodeBalancerBackendIPv4Range,
				SubnetID:  subnetID,
			},
		}
	}

	// get firewall ID from firewallRef if it exists
	if clusterScope.LinodeCluster.Spec.NodeBalancerFirewallRef != nil {
		firewallID, err := getFirewallID(ctx, clusterScope, logger)
		if err != nil {
			logger.Error(err, "Failed to fetch LinodeFirewall ID")
			return nil, err
		}
		createConfig.FirewallID = firewallID
		clusterScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID = &firewallID
	}

	// Use a firewall created outside of the CAPL ecosystem
	// get & validate firewall ID from .Spec.Network.FirewallID if it exists
	if clusterScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID != nil {
		firewallID := *clusterScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID
		firewall, err := clusterScope.LinodeClient.GetFirewall(ctx, firewallID)
		if err != nil {
			logger.Error(err, "Failed to fetch Linode Firewall from the Linode API")
			return nil, err
		}
		createConfig.FirewallID = firewall.ID
	}

	return clusterScope.LinodeClient.CreateNodeBalancer(ctx, createConfig)
}

// getSubnetID returns the subnetID of the first subnet in the LinodeVPC.
// If no subnets or subnetID is found, it returns an error.
func getSubnetID(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (int, error) {
	name := clusterScope.LinodeCluster.Spec.VPCRef.Name
	namespace := clusterScope.LinodeCluster.Spec.VPCRef.Namespace
	if namespace == "" {
		namespace = clusterScope.LinodeCluster.Namespace
	}

	logger = logger.WithValues("vpcName", name, "vpcNamespace", namespace)

	linodeVPC := &v1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	objectKey := client.ObjectKeyFromObject(linodeVPC)
	err := clusterScope.Client.Get(ctx, objectKey, linodeVPC)
	if err != nil {
		logger.Error(err, "Failed to fetch LinodeVPC")
		return -1, err
	}
	if len(linodeVPC.Spec.Subnets) == 0 {
		err = errors.New("No subnets found in LinodeVPC")
		logger.Error(err, "Failed to fetch LinodeVPC")
		return -1, err
	}

	subnetID := 0
	subnetName := clusterScope.LinodeCluster.Spec.Network.SubnetName

	// If subnet name specified, find matching subnet; otherwise use first subnet
	if subnetName != "" {
		for _, subnet := range linodeVPC.Spec.Subnets {
			if subnet.Label == subnetName {
				subnetID = subnet.SubnetID
				break
			}
		}
		if subnetID == 0 {
			return -1, fmt.Errorf("subnet with label %s not found in VPC", subnetName)
		}
	} else {
		subnetID = linodeVPC.Spec.Subnets[0].SubnetID
	}

	// Validate the selected subnet ID
	if subnetID == 0 {
		return -1, errors.New("selected subnet ID is 0")
	}

	return subnetID, nil
}

func getFirewallID(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (int, error) {
	name := clusterScope.LinodeCluster.Spec.NodeBalancerFirewallRef.Name
	namespace := clusterScope.LinodeCluster.Spec.NodeBalancerFirewallRef.Namespace
	if namespace == "" {
		namespace = clusterScope.LinodeCluster.Namespace
	}

	logger = logger.WithValues("firewallName", name, "firewallNamespace", namespace)

	linodeFirewall := &v1alpha2.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	objectKey := client.ObjectKeyFromObject(linodeFirewall)
	err := clusterScope.Client.Get(ctx, objectKey, linodeFirewall)
	if err != nil {
		logger.Error(err, "Failed to fetch LinodeFirewall")
		return -1, err
	}
	if linodeFirewall.Spec.FirewallID == nil {
		err = errors.New("nil firewallID")
		logger.Error(err, "Failed to fetch LinodeFirewall")
		return -1, err
	}

	return *linodeFirewall.Spec.FirewallID, nil
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

func processAndCreateNodeBalancerNodes(ctx context.Context, ipAddress string, clusterScope *scope.ClusterScope, nodeBalancerNodes []linodego.NodeBalancerNode, subnetID int) error {
	apiserverLBPort := DefaultApiserverLBPort
	if clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiserverLBPort = clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}

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
		ipPortCombo := fmt.Sprintf("%s:%d", ipAddress, ports["port"])
		ipPortComboExists := false

		for _, nodes := range nodeBalancerNodes {
			// Create the node if the IP:Port combination does not exist
			if nodes.Address == ipPortCombo {
				ipPortComboExists = true
				break
			}
		}

		if !ipPortComboExists {
			createConfig := linodego.NodeBalancerNodeCreateOptions{
				Label:   clusterScope.Cluster.Name,
				Address: ipPortCombo,
				Mode:    linodego.ModeAccept,
			}
			if subnetID != 0 {
				createConfig.SubnetID = subnetID
			}
			_, err := clusterScope.LinodeClient.CreateNodeBalancerNode(
				ctx,
				*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
				ports["configID"],
				createConfig,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// AddNodesToNB adds backend Nodes on the Node Balancer configuration.
func AddNodesToNB(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope, linodeMachine v1alpha2.LinodeMachine, nodeBalancerNodes []linodego.NodeBalancerNode) error {
	if clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID == nil {
		return errors.New("nil NodeBalancer Config ID")
	}

	// if NodeBalancerBackendIPv4Range is set, we want to prioritize finding the VPC IP address
	// otherwise, we will use the private IP address
	subnetID := 0
	useVPCIps := clusterScope.LinodeCluster.Spec.Network.NodeBalancerBackendIPv4Range != "" && clusterScope.LinodeCluster.Spec.VPCRef != nil
	if useVPCIps {
		// Get subnetID
		subnetID, err := getSubnetID(ctx, clusterScope, logger)
		if err != nil {
			logger.Error(err, "Failed to fetch Linode Subnet ID")
			return err
		}
		for _, IPs := range linodeMachine.Status.Addresses {
			// Look for internal IPs that are NOT 192.168.* (likely VPC IPs)
			if IPs.Type == v1beta1.MachineInternalIP && !strings.Contains(IPs.Address, "192.168") {
				if err := processAndCreateNodeBalancerNodes(ctx, IPs.Address, clusterScope, nodeBalancerNodes, subnetID); err != nil {
					return err
				}
				return nil // Return early if we found and used a VPC IP
			}
		}
	}

	// We will use private IP address as the default
	internalIPFound := false
	for _, IPs := range linodeMachine.Status.Addresses {
		if IPs.Type != v1beta1.MachineInternalIP || !strings.Contains(IPs.Address, "192.168") {
			continue
		}
		internalIPFound = true

		err := processAndCreateNodeBalancerNodes(ctx, IPs.Address, clusterScope, nodeBalancerNodes, subnetID)
		if err != nil {
			return err
		}
		break // Use the first matching IP
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
		instanceID, errorInstanceID := util.GetInstanceID(eachMachine.Spec.ProviderID)
		if errorInstanceID != nil {
			return errorInstanceID
		}

		err := clusterScope.LinodeClient.DeleteNodeBalancerNode(
			ctx,
			*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
			*clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID,
			instanceID,
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
				instanceID,
			)
			if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
				logger.Error(err, "Failed to update Node Balancer")
				return err
			}
		}
	}

	return nil
}
