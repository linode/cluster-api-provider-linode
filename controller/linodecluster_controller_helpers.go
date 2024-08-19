package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	cerrs "sigs.k8s.io/cluster-api/errors"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
)

func (r *LinodeClusterReconciler) handleNBCreate(ctx context.Context, logger logr.Logger, clusterScope *scope.ClusterScope) error {
	linodeNB, err := services.EnsureNodeBalancer(ctx, clusterScope, logger)
	if err != nil {
		logger.Error(err, "failed to ensure nodebalancer")
		setFailureReason(clusterScope, cerrs.CreateClusterError, err, r)
		return err
	}
	if linodeNB == nil {
		err = fmt.Errorf("nodeBalancer created was nil")
		setFailureReason(clusterScope, cerrs.CreateClusterError, err, r)
		return err
	}
	clusterScope.LinodeCluster.Spec.Network.NodeBalancerID = &linodeNB.ID

	// create the configs for the nodeabalancer if not already specified
	configs, err := services.EnsureNodeBalancerConfigs(ctx, clusterScope, logger)
	if err != nil {
		logger.Error(err, "failed to ensure nodebalancer configs")
		setFailureReason(clusterScope, cerrs.CreateClusterError, err, r)
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
		Port: int32(configs[0].Port),
	}

	return nil
}

func (r *LinodeClusterReconciler) handleDNS(clusterScope *scope.ClusterScope) {
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
		Port: int32(apiLBPort),
	}
}
