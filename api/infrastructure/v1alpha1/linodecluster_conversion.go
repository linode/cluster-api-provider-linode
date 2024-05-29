package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/infrastructure/v1alpha2"
)

// ConvertTo converts this LinodeCluster to the Hub version (v1alpha2).
func (src *LinodeCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrastructurev1alpha2.LinodeCluster)

	network := src.Spec.Network
	dst.Spec.Network = infrastructurev1alpha2.NetworkSpec{
		LoadBalancerType:                 network.LoadBalancerType,
		ApiserverLoadBalancerPort:        network.LoadBalancerPort,
		NodeBalancerID:                   network.NodeBalancerID,
		ApiserverNodeBalancerConfigID:    network.NodeBalancerConfigID,
		Konnectivity:                     false,
		KonnectivityLoadBalancerPort:     0,
		KonnectivityNodeBalancerConfigID: nil,
	}

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.ControlPlaneEndpoint = src.Spec.ControlPlaneEndpoint
	dst.Spec.Region = src.Spec.Region
	dst.Spec.VPCRef = src.Spec.VPCRef
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = src.Status.FailureReason

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha2) to this version.
func (dst *LinodeCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrastructurev1alpha2.LinodeCluster)

	dst.Spec.Network.LoadBalancerPort = src.Spec.Network.ApiserverLoadBalancerPort
	dst.Spec.Network.LoadBalancerType = src.Spec.Network.LoadBalancerType
	dst.Spec.Network.NodeBalancerID = src.Spec.Network.NodeBalancerID
	dst.Spec.Network.NodeBalancerConfigID = src.Spec.Network.ApiserverNodeBalancerConfigID

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.ControlPlaneEndpoint = src.Spec.ControlPlaneEndpoint
	dst.Spec.Region = src.Spec.Region
	dst.Spec.VPCRef = src.Spec.VPCRef
	dst.Spec.CredentialsRef = src.Spec.CredentialsRef

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.FailureMessage = src.Status.FailureMessage
	dst.Status.FailureReason = src.Status.FailureReason

	return nil
}
