package scope

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
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
	Client              K8sClient
	MachinePatchHelper  *patch.Helper
	ClusterPatchHelper  *patch.Helper
	Cluster             *clusterv1.Cluster
	Machine             *clusterv1.Machine
	LinodeClient        LinodeClient
	LinodeDomainsClient LinodeClient
	AkamaiDomainsClient AkamClient
	LinodeCluster       *infrav1alpha2.LinodeCluster
	LinodeMachine       *infrav1alpha2.LinodeMachine
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

func NewMachineScope(ctx context.Context, linodeClientConfig, dnsClientConfig ClientConfig, params MachineScopeParams) (*MachineScope, error) {
	if err := validateMachineScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Machine's Secret reference (if supplied).
	// Credentials will be used in the following order:
	//   1. LinodeMachine
	//   2. Owner LinodeCluster
	//   3. Controller
	var (
		credentialRef    *corev1.SecretReference
		defaultNamespace string
	)
	switch {
	case params.LinodeMachine.Spec.CredentialsRef != nil:
		credentialRef = params.LinodeMachine.Spec.CredentialsRef
		defaultNamespace = params.LinodeMachine.GetNamespace()
	case params.LinodeCluster.Spec.CredentialsRef != nil:
		credentialRef = params.LinodeCluster.Spec.CredentialsRef
		defaultNamespace = params.LinodeCluster.GetNamespace()
	default:
		// Use default (controller) credentials
	}

	if credentialRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, params.Client, *credentialRef, defaultNamespace, "apiToken")
		if err != nil {
			return nil, fmt.Errorf("credentials from secret ref: %w", err)
		}
		linodeClientConfig.Token = string(apiToken)

		dnsToken, err := getCredentialDataFromRef(ctx, params.Client, *credentialRef, defaultNamespace, "dnsToken")
		if err != nil || len(dnsToken) == 0 {
			dnsToken = apiToken
		}
		dnsClientConfig.Token = string(dnsToken)
	}

	linodeClient, err := CreateLinodeClient(linodeClientConfig,
		WithRetryCount(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}
	linodeDomainsClient, err := CreateLinodeClient(dnsClientConfig,
		WithRetryCount(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	akamDomainsClient, err := setUpEdgeDNSInterface()
	if err != nil {
		return nil, fmt.Errorf("failed to create akamai dns client: %w", err)
	}

	machineHelper, err := patch.NewHelper(params.LinodeMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init machine patch helper: %w", err)
	}

	clusterHelper, err := patch.NewHelper(params.LinodeCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init cluster patch helper: %w", err)
	}

	return &MachineScope{
		Client:              params.Client,
		MachinePatchHelper:  machineHelper,
		ClusterPatchHelper:  clusterHelper,
		Cluster:             params.Cluster,
		Machine:             params.Machine,
		LinodeClient:        linodeClient,
		LinodeDomainsClient: linodeDomainsClient,
		AkamaiDomainsClient: akamDomainsClient,
		LinodeCluster:       params.LinodeCluster,
		LinodeMachine:       params.LinodeMachine,
	}, nil
}

// CloseAll persists the linodemachine and linodecluster configuration and status.
func (s *MachineScope) CloseAll(ctx context.Context) error {
	if err := s.MachineClose(ctx); err != nil {
		return err
	}
	if err := s.ClusterClose(ctx); err != nil {
		return err
	}
	return nil
}

// MachineClose persists the linodemachine configuration and status.
func (s *MachineScope) MachineClose(ctx context.Context) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		return s.MachinePatchHelper.Patch(ctx, s.LinodeMachine)
	})
}

// ClusterClose persists the linodecluster configuration and status.
func (s *MachineScope) ClusterClose(ctx context.Context) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsConflict, func() error {
		return s.ClusterPatchHelper.Patch(ctx, s.LinodeCluster)
	})
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *MachineScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.LinodeMachine, infrav1alpha2.MachineFinalizer) {
		return s.MachineClose(ctx)
	}

	return nil
}

// AddLinodeClusterFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *MachineScope) AddLinodeClusterFinalizer(ctx context.Context) error {
	if !kutil.IsControlPlaneMachine(s.Machine) {
		return nil
	}
	if controllerutil.AddFinalizer(s.LinodeCluster, s.LinodeMachine.Name) {
		return s.ClusterClose(ctx)
	}

	return nil
}

// RemoveLinodeClusterFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *MachineScope) RemoveLinodeClusterFinalizer(ctx context.Context) error {
	if !kutil.IsControlPlaneMachine(s.Machine) {
		return nil
	}
	if controllerutil.RemoveFinalizer(s.LinodeCluster, s.LinodeMachine.Name) {
		return s.ClusterClose(ctx)
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
