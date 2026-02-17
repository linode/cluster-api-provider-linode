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
	"sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

const (
	DefaultApiserverLBPort    = 6443
	DefaultKonnectivityLBPort = 8132
)

// DetermineAPIServerLBPort returns the configured API server load balancer port,
// or the provider default when not explicitly set.
func DetermineAPIServerLBPort(clusterScope *scope.ClusterScope) int {
	if clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		return clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}
	return DefaultApiserverLBPort
}

// ShouldUseVPC decides whether VPC IPs/backends should be preferred and a VPC-scoped
// NodeBalancer should be created. It requires both the feature flag and a VPC reference/ID.
func ShouldUseVPC(clusterScope *scope.ClusterScope) bool {
	return clusterScope.LinodeCluster.Spec.Network.EnableVPCBackends && (clusterScope.LinodeCluster.Spec.VPCRef != nil || clusterScope.LinodeCluster.Spec.VPCID != nil)
}

// FindSubnet selects a subnet from the provided subnets based on the subnet name
// It handles both direct VPC subnets and VPCRef subnets
// If subnet name is provided, it looks for a matching subnet; otherwise, it uses the first subnet
// Returns the subnet ID and any error encountered
func FindSubnet(subnetName string, isDirectVPC bool, subnets interface{}) (int, error) {
	var subnetID int
	var err error

	// Different handling based on whether we're dealing with a direct VPC or VPCRef
	if isDirectVPC {
		subnetID, err = findDirectVPCSubnet(subnetName, subnets)
	} else {
		subnetID, err = findVPCRefSubnet(subnetName, subnets)
	}

	if err != nil {
		return 0, err
	}

	// Validate the selected subnet ID
	if subnetID == 0 {
		return 0, errors.New("invalid subnet ID: selected subnet ID is 0")
	}

	return subnetID, nil
}

// findDirectVPCSubnet finds a subnet in direct VPC subnets
func findDirectVPCSubnet(subnetName string, subnets interface{}) (int, error) {
	vpcSubnets, ok := subnets.([]linodego.VPCSubnet)
	if !ok {
		return 0, fmt.Errorf("invalid subnet data type for direct VPC: expected []linodego.VPCSubnet")
	}

	if len(vpcSubnets) == 0 {
		return 0, errors.New("no subnets found in VPC")
	}

	return selectSubnet(subnetName, vpcSubnets, func(subnet linodego.VPCSubnet) (string, int) {
		return subnet.Label, subnet.ID
	})
}

// findVPCRefSubnet finds a subnet in VPCRef subnets
func findVPCRefSubnet(subnetName string, subnets interface{}) (int, error) {
	vpcRefSubnets, ok := subnets.([]v1alpha2.VPCSubnetCreateOptions)
	if !ok {
		return 0, fmt.Errorf("invalid subnet data type for VPC reference: expected []v1alpha2.VPCSubnetCreateOptions")
	}

	if len(vpcRefSubnets) == 0 {
		return 0, errors.New("no subnets found in LinodeVPC")
	}

	return selectSubnet(subnetName, vpcRefSubnets, func(subnet v1alpha2.VPCSubnetCreateOptions) (string, int) {
		return subnet.Label, subnet.SubnetID
	})
}

// selectSubnet is a generic helper to select a subnet by name or use the first one
func selectSubnet[T any](subnetName string, subnets []T, getProps func(T) (string, int)) (int, error) {
	if len(subnets) == 0 {
		return 0, errors.New("no subnets available in the VPC")
	}

	// If subnet name specified, find matching subnet
	if subnetName != "" {
		for _, subnet := range subnets {
			label, id := getProps(subnet)
			if label == subnetName {
				return id, nil
			}
		}
		// Keep the original error message format for compatibility with tests
		return 0, fmt.Errorf("subnet with label %s not found in VPC", subnetName)
	}

	// Use the first subnet when no specific name is provided
	_, id := getProps(subnets[0])
	return id, nil
}

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

	// if enableVPCBackends is true and vpcRef or vpcID is set, create the NodeBalancer in the specified VPC
	if ShouldUseVPC(clusterScope) {
		logger.Info("Creating NodeBalancer in VPC")
		subnetID, err := getSubnetID(ctx, clusterScope, logger)
		if err != nil {
			logger.Error(err, "Failed to fetch Linode Subnet ID")
			return nil, err
		}

		createConfig.VPCs = []linodego.NodeBalancerVPCOptions{{SubnetID: subnetID}}
		if clusterScope.LinodeCluster.Spec.Network.NodeBalancerBackendIPv4Range != "" {
			createConfig.VPCs[0].IPv4Range = clusterScope.LinodeCluster.Spec.Network.NodeBalancerBackendIPv4Range
		}
	}

	// First check if a direct NodeBalancerFirewallID is specified (prioritize direct ID)
	if clusterScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID != nil {
		firewallID := *clusterScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID
		logger.Info("Using direct NodeBalancerFirewallID", "firewallID", firewallID)
		firewall, err := clusterScope.LinodeClient.GetFirewall(ctx, firewallID)
		if err != nil {
			logger.Error(err, "Failed to fetch Linode Firewall from the Linode API")
			return nil, err
		}
		createConfig.FirewallID = firewall.ID
	} else if clusterScope.LinodeCluster.Spec.NodeBalancerFirewallRef != nil {
		// Only use NodeBalancerFirewallRef if no direct ID is provided
		firewallID, err := getFirewallID(ctx, clusterScope, logger)
		if err != nil {
			logger.Error(err, "Failed to fetch LinodeFirewall ID from reference")
			return nil, err
		}
		createConfig.FirewallID = firewallID
	}

	nb, err := clusterScope.LinodeClient.CreateNodeBalancer(ctx, createConfig)
	// Handle the edge case where API did create the NB eventually after timing out on the client side
	if linodego.ErrHasStatus(err, http.StatusBadRequest) && strings.Contains(err.Error(), "Label must be unique") {
		logger.Error(err, "Failed to create NodeBalancer, received [400 BadRequest] response.")

		return getNBFromLabel(ctx, clusterScope, logger)
	}
	return nb, err
}

func getNBFromLabel(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (*linodego.NodeBalancer, error) {
	listFilter := util.Filter{Label: clusterScope.LinodeCluster.Name}
	filter, errFilter := listFilter.String()
	if errFilter != nil {
		logger.Error(errFilter, "Failed to create filter to list NodeBalancers")
		return nil, errFilter
	}
	nbs, listErr := clusterScope.LinodeClient.ListNodeBalancers(ctx, linodego.NewListOptions(1, filter))
	if listErr != nil {
		return nil, listErr
	}
	if len(nbs) > 0 {
		return &nbs[0], nil
	}
	return nil, fmt.Errorf("no NodeBalancer found with label %s", clusterScope.LinodeCluster.Name)
}

// getSubnetID returns the subnetID of the first subnet in the LinodeVPC.
// If no subnets or subnetID is found, it returns an error.
func getSubnetID(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (int, error) {
	subnetName := clusterScope.LinodeCluster.Spec.Network.SubnetName

	// If direct VPCID is specified, get the VPC and subnets directly from Linode API
	if clusterScope.LinodeCluster.Spec.VPCID != nil {
		vpcID := *clusterScope.LinodeCluster.Spec.VPCID
		vpc, err := clusterScope.LinodeClient.GetVPC(ctx, vpcID)
		if err != nil {
			logger.Error(err, "Failed to fetch VPC from Linode API", "vpcID", vpcID)
			return 0, err
		}

		if len(vpc.Subnets) == 0 {
			return 0, errors.New("no subnets found in VPC")
		}

		subnetID, err := FindSubnet(subnetName, true, vpc.Subnets)
		if err != nil {
			logger.Error(err, "Failed to find subnet in VPC", "vpcID", vpcID, "subnetName", subnetName)
			return 0, err
		}
		return subnetID, nil
	}

	// Otherwise, use the VPCRef
	if clusterScope.LinodeCluster.Spec.VPCRef == nil {
		return 0, errors.New("neither VPCID nor VPCRef is specified in LinodeCluster")
	}

	name := clusterScope.LinodeCluster.Spec.VPCRef.Name
	namespace := clusterScope.LinodeCluster.Spec.VPCRef.Namespace
	if namespace == "" {
		namespace = clusterScope.LinodeCluster.Namespace
	}

	if name == "" {
		return 0, errors.New("VPCRef name is not specified in LinodeCluster")
	}

	linodeVPC := &v1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	objectKey := client.ObjectKeyFromObject(linodeVPC)
	if err := clusterScope.Client.Get(ctx, objectKey, linodeVPC); err != nil {
		logger.Error(err, "Failed to fetch LinodeVPC", "name", name, "namespace", namespace)
		return 0, fmt.Errorf("failed to fetch LinodeVPC %s/%s: %w", namespace, name, err)
	}

	if len(linodeVPC.Spec.Subnets) == 0 {
		return 0, errors.New("no subnets found in LinodeVPC")
	}

	return FindSubnet(subnetName, false, linodeVPC.Spec.Subnets)
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
	apiLBPort := DetermineAPIServerLBPort(clusterScope)

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
	apiserverLBPort := DetermineAPIServerLBPort(clusterScope)

	// Set the port number and NB config ID for standard ports
	portsToBeAdded := make([]map[string]int, 0, 1+len(clusterScope.LinodeCluster.Spec.Network.AdditionalPorts))
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

	subnetID := 0
	if ShouldUseVPC(clusterScope) {
		subnetID, err := getSubnetID(ctx, clusterScope, logger)
		if err != nil {
			logger.Error(err, "Failed to fetch Linode Subnet ID")
			return err
		}
		for _, IPs := range linodeMachine.Status.Addresses {
			// Look for internal IPs that are NOT linode private IPs (likely VPC IPs)
			if IPs.Type == v1beta2.MachineInternalIP && !util.IsLinodePrivateIP(IPs.Address) {
				if err := processAndCreateNodeBalancerNodes(ctx, IPs.Address, clusterScope, nodeBalancerNodes, subnetID); err != nil {
					logger.Error(err, "Failed to process and create NB nodes")
					return err
				}
				return nil // Return early if we found and used a VPC IP
			}
		}
	}

	// We will use private IP address as the default
	internalIPFound := false
	for _, IPs := range linodeMachine.Status.Addresses {
		if IPs.Type != v1beta2.MachineInternalIP || !util.IsLinodePrivateIP(IPs.Address) {
			continue
		}
		internalIPFound = true

		err := processAndCreateNodeBalancerNodes(ctx, IPs.Address, clusterScope, nodeBalancerNodes, subnetID)
		if err != nil {
			logger.Error(err, "Failed to process and create NB nodes")
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
			// Skip machines without a ProviderID - they never had an instance
			// created, so there's no corresponding node in the NodeBalancer
			logger.Info("Skipping machine without ProviderID", "machine", eachMachine.Name)
			continue
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
