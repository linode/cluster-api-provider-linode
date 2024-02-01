package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

type MachineScopeParams struct {
	Client        client.Client
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	LinodeCluster *infrav1alpha1.LinodeCluster
	LinodeMachine *infrav1alpha1.LinodeMachine
}

type MachineScope struct {
	client client.Client

	PatchHelper   *patch.Helper
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	LinodeClient  *linodego.Client
	LinodeCluster *infrav1alpha1.LinodeCluster
	LinodeMachine *infrav1alpha1.LinodeMachine
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

	linodeClient := createLinodeClient(apiKey)

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

// PatchObject persists the machine configuration and status.
func (s *MachineScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.LinodeMachine)
}

// Close closes the current scope persisting the machine configuration and status.
func (s *MachineScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer and immediately patches the object to avoid any race conditions
func (s *MachineScope) AddFinalizer(ctx context.Context) error {
	controllerutil.AddFinalizer(s.LinodeMachine, infrav1alpha1.GroupVersion.String())

	return s.Close(ctx)
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetBootstrapData(ctx context.Context) ([]byte, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return []byte{}, fmt.Errorf(
			"bootstrap data secret is nil for LinodeMachine %s/%s",
			m.LinodeMachine.Namespace,
			m.LinodeMachine.Name,
		)
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.LinodeMachine.Namespace, Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(ctx, key, secret); err != nil {
		return []byte{}, fmt.Errorf(
			"failed to retrieve bootstrap data secret for LinodeMachine %s/%s",
			m.LinodeMachine.Namespace,
			m.LinodeMachine.Name,
		)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return []byte{}, fmt.Errorf(
			"bootstrap data secret value key is missing for LinodeMachine %s/%s",
			m.LinodeMachine.Namespace,
			m.LinodeMachine.Name,
		)
	}

	return value, nil
}
