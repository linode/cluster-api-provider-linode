package scope

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

type MachineScopeParams struct {
	Client        K8sClient
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	LinodeCluster *infrav1alpha2.LinodeCluster
	LinodeMachine *infrav1alpha2.LinodeMachine
}

type MachineScope struct {
	Client        K8sClient
	PatchHelper   *patch.Helper
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	TokenHash     string
	LinodeClient  LinodeClient
	LinodeCluster *infrav1alpha2.LinodeCluster
	LinodeMachine *infrav1alpha2.LinodeMachine
}

func validateMachineScopeParams(params MachineScopeParams) error {
	if params.Cluster == nil {
		return errors.New("cluster is required when creating a MachineScope")
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

func NewMachineScope(ctx context.Context, linodeClientConfig ClientConfig, params MachineScopeParams) (*MachineScope, error) {
	if err := validateMachineScopeParams(params); err != nil {
		return nil, err
	}
	linodeClient, err := CreateLinodeClient(linodeClientConfig,
		WithRetryCount(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}
	helper, err := patch.NewHelper(params.LinodeMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &MachineScope{
		Client:        params.Client,
		PatchHelper:   helper,
		Cluster:       params.Cluster,
		Machine:       params.Machine,
		TokenHash:     GetHash(linodeClientConfig.Token),
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

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *MachineScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.LinodeMachine, infrav1alpha2.MachineFinalizer) {
		return s.Close(ctx)
	}

	return nil
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
	if err := m.Client.Get(ctx, key, secret); err != nil {
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

func (s *MachineScope) AddCredentialsRefFinalizer(ctx context.Context) error {
	// Only add the finalizer if the machine has an override for the credentials reference
	if s.LinodeMachine.Spec.CredentialsRef == nil {
		return nil
	}

	return addCredentialsFinalizer(ctx, s.Client,
		*s.LinodeMachine.Spec.CredentialsRef, s.LinodeMachine.GetNamespace(),
		toFinalizer(s.LinodeMachine))
}

func (s *MachineScope) RemoveCredentialsRefFinalizer(ctx context.Context) error {
	// Only remove the finalizer if the machine has an override for the credentials reference
	if s.LinodeMachine.Spec.CredentialsRef == nil {
		return nil
	}

	return removeCredentialsFinalizer(ctx, s.Client,
		*s.LinodeMachine.Spec.CredentialsRef, s.LinodeMachine.GetNamespace(),
		toFinalizer(s.LinodeMachine))
}

func (s *MachineScope) SetCredentialRefTokenForLinodeClients(ctx context.Context) error {
	var (
		credentialRef    *corev1.SecretReference
		defaultNamespace string
	)
	switch {
	case s.LinodeMachine.Spec.CredentialsRef != nil:
		credentialRef = s.LinodeMachine.Spec.CredentialsRef
		defaultNamespace = s.LinodeMachine.GetNamespace()
	default:
		credentialRef = s.LinodeCluster.Spec.CredentialsRef
		defaultNamespace = s.LinodeCluster.GetNamespace()
	}
	// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
	apiToken, err := getCredentialDataFromRef(ctx, s.Client, *credentialRef, defaultNamespace, "apiToken")
	if err != nil {
		return fmt.Errorf("credentials from secret ref: %w", err)
	}
	s.LinodeClient = s.LinodeClient.SetToken(string(apiToken))
	s.TokenHash = GetHash(string(apiToken))
	return nil
}
