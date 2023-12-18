package scope

import (
	"errors"
	"fmt"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
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
	client client.Client

	PatchHelper   *patch.Helper
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	LinodeClient  *linodego.Client
	LinodeCluster *infrav1.LinodeCluster
	LinodeMachine *infrav1.LinodeMachine
}

func validateMachineScopeParams(params MachineScopeParams) error {
	if params.Cluster == nil {
		return errors.New("custer is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return errors.New("machine is required when creating a MachineScope")
	}
	if params.LinodeCluster == nil {
		return errors.New("linodeCluster is required when creating a MachineScope")
	}
	if params.LinodeMachine == nil {
		return errors.New("linodeMachine is required when creating a MachineScope")
	}
	return nil
}

func NewMachineScope(apiKey string, params MachineScopeParams) (*MachineScope, error) {
	if err := validateMachineScopeParams(params); err != nil {
		return nil, err
	}

	linodeClient, err := createLinodeClient(apiKey)
	if err != nil {
		return nil, err
	}

	helper, err := patch.NewHelper(params.LinodeMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &MachineScope{
		client:        params.Client,
		PatchHelper:   helper,
		Cluster:       params.Cluster,
		Machine:       params.Machine,
		LinodeClient:  linodeClient,
		LinodeCluster: params.LinodeCluster,
		LinodeMachine: params.LinodeMachine,
	}, nil
}
