package scope

import (
	"fmt"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MachineScopeParams struct {
	Client        client.Client
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	LinodeCluster *infrav1.LinodeCluster
	LinodeMachine *infrav1.LinodeMachine
}

type MachineScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	LinodeCluster *infrav1.LinodeCluster
	LinodeMachine *infrav1.LinodeMachine
}

func validateMachineScopeParams(params MachineScopeParams) error {
	if params.Cluster == nil {
		return fmt.Errorf("Cluster is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return fmt.Errorf("Machine is required when creating a MachineScope")
	}
	if params.LinodeCluster == nil {
		return fmt.Errorf("LinodeCluster is required when creating a MachineScope")
	}
	if params.LinodeMachine == nil {
		return fmt.Errorf("LinodeMachine is required when creating a MachineScope")
	}
	return nil
}

func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if err := validateMachineScopeParams(params); err != nil {
		return nil, err
	}

	helper, err := patch.NewHelper(params.LinodeMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &MachineScope{
		client:        params.Client,
		patchHelper:   helper,
		Cluster:       params.Cluster,
		Machine:       params.Machine,
		LinodeCluster: params.LinodeCluster,
		LinodeMachine: params.LinodeMachine,
	}, nil
}
