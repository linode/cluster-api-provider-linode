package controller

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

func addMachineToLB(ctx context.Context, clusterScope *scope.ClusterScope) error {
	logger := logr.FromContextOrDiscard(ctx)
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "external" {
		logger.Info("LoadBalancing is handled externally")
		return nil
	}
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == "dns" {
		if err := services.EnsureDNSEntries(ctx, clusterScope, "create"); err != nil {
			logger.Error(err, "Failed to ensure DNS entries")
			return err
		}
		return nil
	}
	// Reconcile clusters with Spec.Network = {} and ControlPlaneEndpoint.Host externally managed
	if clusterScope.LinodeCluster.Spec.Network.NodeBalancerID == nil || clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID == nil {
		logger.Info("NodeBalancerID or ApiserverNodeBalancerConfigID not set for Type NodeBalancer, this cluster is managed externally")
		clusterScope.LinodeCluster.Spec.Network.LoadBalancerType = "external"
		return nil
	}
	nodeBalancerNodes, err := clusterScope.LinodeClient.ListNodeBalancerNodes(
		ctx,
		*clusterScope.LinodeCluster.Spec.Network.NodeBalancerID,
		*clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID,
		&linodego.ListOptions{},
	)
	if err != nil {
		logger.Error(err, "Failed to list NB nodes")
		return err
	}
	for _, eachMachine := range clusterScope.LinodeMachines.Items {
		err = services.AddNodesToNB(ctx, logger, clusterScope, eachMachine, nodeBalancerNodes)
		if err != nil {
			logger.Error(err, "Failed to add nodes to NB")
			return err
		}
	}
	ipPortCombo := getIPPortCombo(clusterScope)
	for _, node := range nodeBalancerNodes {
		if !slices.Contains(ipPortCombo, node.Address) {
			if err := clusterScope.LinodeClient.DeleteNodeBalancerNode(ctx, node.NodeBalancerID, node.ConfigID, node.ID); err != nil {
				logger.Error(err, "Failed to delete NB node")
				return err
			}
		}
	}

	return nil
}

func removeMachineFromDNS(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	if err := services.EnsureDNSEntries(ctx, clusterScope, "delete"); err != nil {
		logger.Error(err, "Failed to remove IP from DNS")
		return err
	}
	return nil
}

func removeMachineFromNB(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	if err := services.DeleteNodesFromNB(ctx, logger, clusterScope); err != nil {
		logger.Error(err, "Failed to remove node from Node Balancer backend")
		return err
	}
	return nil
}

func getIPPortCombo(cscope *scope.ClusterScope) (ipPortComboList []string) {
	apiserverLBPort := services.DefaultApiserverLBPort
	if cscope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiserverLBPort = cscope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}

	// Check if we're using VPC
	useVPCIps := cscope.LinodeCluster.Spec.Network.NodeBalancerBackendIPv4Range != "" && cscope.LinodeCluster.Spec.VPCRef != nil

	for _, eachMachine := range cscope.LinodeMachines.Items {
		// First try to find VPC IPs if we're using VPC
		if useVPCIps {
			vpcIPFound := false
			for _, IPs := range eachMachine.Status.Addresses {
				// Look for internal IPs that are NOT 192.168.* (likely VPC IPs)
				if IPs.Type == clusterv1.MachineInternalIP && !util.IsLinodePrivateIP(IPs.Address) {
					vpcIPFound = true
					ipPortComboList = append(ipPortComboList, fmt.Sprintf("%s:%d", IPs.Address, apiserverLBPort))
					for _, portConfig := range cscope.LinodeCluster.Spec.Network.AdditionalPorts {
						ipPortComboList = append(ipPortComboList, fmt.Sprintf("%s:%d", IPs.Address, portConfig.Port))
					}
					break // Use first VPC IP found for this machine
				}
			}

			// If we found a VPC IP for this machine, continue to the next machine
			if vpcIPFound {
				continue
			}
		}

		// Fall back to original behavior for this machine if no VPC IP found or not using VPC
		for _, IPs := range eachMachine.Status.Addresses {
			if IPs.Type != clusterv1.MachineInternalIP || !util.IsLinodePrivateIP(IPs.Address) {
				continue
			}
			ipPortComboList = append(ipPortComboList, fmt.Sprintf("%s:%d", IPs.Address, apiserverLBPort))
			for _, portConfig := range cscope.LinodeCluster.Spec.Network.AdditionalPorts {
				ipPortComboList = append(ipPortComboList, fmt.Sprintf("%s:%d", IPs.Address, portConfig.Port))
			}
			break // Use first 192.168.* IP found for this machine
		}
	}
	return ipPortComboList
}

func linodeMachineToLinodeCluster(tracedClient client.Client, logger logr.Logger) handler.MapFunc {
	logger = logger.WithName("LinodeClusterReconciler").WithName("linodeMachineToLinodeCluster")

	return func(ctx context.Context, o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		linodeMachine, ok := o.(*infrav1alpha2.LinodeMachine)
		if !ok {
			logger.Info("Failed to cast object to LinodeMachine")
			return nil
		}

		// We only need control plane machines to trigger reconciliation
		machine, err := kutil.GetOwnerMachine(ctx, tracedClient, linodeMachine.ObjectMeta)
		if err != nil || machine == nil || !kutil.IsControlPlaneMachine(machine) {
			return nil
		}

		linodeCluster := infrav1alpha2.LinodeCluster{}
		if err := tracedClient.Get(
			ctx,
			types.NamespacedName{
				Name:      linodeMachine.ObjectMeta.Labels[clusterv1.ClusterNameLabel],
				Namespace: linodeMachine.Namespace,
			},
			&linodeCluster); err != nil {
			logger.Info("Failed to get LinodeCluster")
			return nil
		}

		result := make([]ctrl.Request, 0, 1)
		result = append(result, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: linodeCluster.Namespace,
				Name:      linodeCluster.Name,
			},
		})

		return result
	}
}

func handleDNS(clusterScope *scope.ClusterScope) {
	clusterSpec := clusterScope.LinodeCluster.Spec
	clusterMetadata := clusterScope.LinodeCluster.ObjectMeta
	uniqueID := ""
	if clusterSpec.Network.DNSUniqueIdentifier != "" {
		uniqueID = "-" + clusterSpec.Network.DNSUniqueIdentifier
	}
	subDomain := clusterMetadata.Name + uniqueID

	if clusterScope.LinodeCluster.Spec.Network.DNSSubDomainOverride != "" {
		subDomain = clusterScope.LinodeCluster.Spec.Network.DNSSubDomainOverride
	}
	dnsHost := subDomain + "." + clusterSpec.Network.DNSRootDomain
	apiLBPort := services.DefaultApiserverLBPort
	if clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort != 0 {
		apiLBPort = clusterScope.LinodeCluster.Spec.Network.ApiserverLoadBalancerPort
	}
	clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: dnsHost,
		Port: int32(apiLBPort), // #nosec G115: Integer overflow conversion is safe for port numbers
	}
}

func handleNBCreate(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	linodeNB, err := services.EnsureNodeBalancer(ctx, clusterScope, logger)
	if err != nil {
		logger.Error(err, "failed to ensure nodebalancer")
		return err
	}
	if linodeNB == nil {
		err = fmt.Errorf("nodeBalancer created was nil")
		return err
	}
	clusterScope.LinodeCluster.Spec.Network.NodeBalancerID = &linodeNB.ID

	// create the configs for the nodeabalancer if not already specified
	configs, err := services.EnsureNodeBalancerConfigs(ctx, clusterScope, logger)
	if err != nil {
		logger.Error(err, "failed to ensure nodebalancer configs")
		return err
	}

	clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID = util.Pointer(configs[0].ID)
	additionalPorts := make([]infrav1alpha2.LinodeNBPortConfig, 0)
	for _, config := range configs[1:] {
		portConfig := infrav1alpha2.LinodeNBPortConfig{
			Port:                 config.Port,
			NodeBalancerConfigID: &config.ID,
		}
		additionalPorts = append(additionalPorts, portConfig)
	}
	clusterScope.LinodeCluster.Spec.Network.AdditionalPorts = additionalPorts

	clusterScope.LinodeCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: *linodeNB.IPv4,
		Port: int32(configs[0].Port), // #nosec G115: Integer overflow conversion is safe for port numbers
	}

	return nil
}
