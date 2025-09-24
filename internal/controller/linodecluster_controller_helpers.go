package controller

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == lbTypeExternal {
		logger.Info("LoadBalancing is handled externally")
		return nil
	}
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerType == lbTypeDNS {
		if err := services.EnsureDNSEntries(ctx, clusterScope, "create"); err != nil {
			logger.Error(err, "Failed to ensure DNS entries")
			return err
		}
		return nil
	}
	// Reconcile clusters with Spec.Network = {} and ControlPlaneEndpoint.Host externally managed
	if clusterScope.LinodeCluster.Spec.Network.NodeBalancerID == nil || clusterScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID == nil {
		logger.Info("NodeBalancerID or ApiserverNodeBalancerConfigID not set for Type NodeBalancer, this cluster is managed externally")
		clusterScope.LinodeCluster.Spec.Network.LoadBalancerType = lbTypeExternal
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
	apiServerLBPort := services.DetermineAPIServerLBPort(cscope)
	useVPCIPs := services.ShouldUseVPC(cscope)

	for _, eachMachine := range cscope.LinodeMachines.Items {
		var selectedIP string

		if useVPCIPs {
			if ip, ok := findFirstVPCInternalIP(eachMachine.Status.Addresses); ok {
				selectedIP = ip
			}
		}

		if selectedIP == "" {
			if ip, ok := findFirstPrivateInternalIP(eachMachine.Status.Addresses); ok {
				selectedIP = ip
			}
		}

		if selectedIP != "" {
			ipPortComboList = append(ipPortComboList, buildPortCombosForIP(selectedIP, apiServerLBPort, cscope.LinodeCluster.Spec.Network.AdditionalPorts)...)
		}
	}

	return ipPortComboList
}

// findFirstVPCInternalIP returns the first internal IP that is not in Linode's private 192.168.* range.
func findFirstVPCInternalIP(addresses []clusterv1.MachineAddress) (string, bool) {
	for _, addr := range addresses {
		if addr.Type == clusterv1.MachineInternalIP && !util.IsLinodePrivateIP(addr.Address) {
			return addr.Address, true
		}
	}
	return "", false
}

// findFirstPrivateInternalIP returns the first internal IP in Linode's private 192.168.* range.
func findFirstPrivateInternalIP(addresses []clusterv1.MachineAddress) (string, bool) {
	for _, addr := range addresses {
		if addr.Type == clusterv1.MachineInternalIP && util.IsLinodePrivateIP(addr.Address) {
			return addr.Address, true
		}
	}
	return "", false
}

// buildPortCombosForIP composes ip:port pairs for the API server port and any additional ports.
func buildPortCombosForIP(ip string, apiServerLBPort int, additionalPorts []infrav1alpha2.LinodeNBPortConfig) []string {
	results := make([]string, 0, 1+len(additionalPorts))
	results = append(results, fmt.Sprintf("%s:%d", ip, apiServerLBPort))
	for _, portConfig := range additionalPorts {
		results = append(results, fmt.Sprintf("%s:%d", ip, portConfig.Port))
	}
	return results
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
				Name:      linodeMachine.Labels[clusterv1.ClusterNameLabel],
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
	apiLBPort := services.DetermineAPIServerLBPort(clusterScope)
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
