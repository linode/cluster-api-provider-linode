package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

const bucketKeySecret = `kind: Secret
apiVersion: v1
metadata:
  name: %s
stringData:
  bucket_name: %s
  bucket_region: %s
  bucket_endpoint: %s
  access_key_rw: %s
  secret_key_rw: %s
  access_key_ro: %s
  secret_key_ro: %s`

type ObjectStorageKeyScopeParams struct {
	Client K8sClient
	Key    *infrav1alpha1.LinodeObjectStorageKey
	Logger *logr.Logger
}

type ObjectStorageKeyScope struct {
	Client       K8sClient
	Key          *infrav1alpha1.LinodeObjectStorageKey
	Logger       logr.Logger
	LinodeClient LinodeClient
	PatchHelper  *patch.Helper
}

func validateObjectStorageKeyScopeParams(params ObjectStorageKeyScopeParams) error {
	if params.Key == nil {
		return errors.New("object storage key is required when creating an ObjectStorageKeyScope")
	}
	if params.Logger == nil {
		return errors.New("logger is required when creating an ObjectStorageKeyScope")
	}

	return nil
}

func NewObjectStorageKeyScope(ctx context.Context, apiKey string, params ObjectStorageKeyScopeParams) (*ObjectStorageKeyScope, error) {
	if err := validateObjectStorageKeyScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Cluster's Secret reference (if supplied).
	if params.Key.Spec.CredentialsRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, params.Client, *params.Key.Spec.CredentialsRef, params.Key.GetNamespace(), "apiToken")
		if err != nil {
			return nil, fmt.Errorf("credentials from secret ref: %w", err)
		}
		apiKey = string(apiToken)
	}
	linodeClient, err := CreateLinodeClient(apiKey, clientTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	patchHelper, err := patch.NewHelper(params.Key, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ObjectStorageKeyScope{
		Client:       params.Client,
		Key:          params.Key,
		Logger:       *params.Logger,
		LinodeClient: linodeClient,
		PatchHelper:  patchHelper,
	}, nil
}

// PatchObject persists the object storage key configuration and status.
func (s *ObjectStorageKeyScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.Key)
}

// Close closes the current scope persisting the object storage key configuration and status.
func (s *ObjectStorageKeyScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *ObjectStorageKeyScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.Key, infrav1alpha1.ObjectStorageKeyFinalizer) {
		return s.Close(ctx)
	}

	return nil
}

// GenerateKeySecret returns a secret suitable for submission to the Kubernetes API.
// The secret is expected to contain keys for accessing the bucket, as well as owner and controller references.
func (s *ObjectStorageKeyScope) GenerateKeySecret(ctx context.Context, key *linodego.ObjectStorageKey) (*corev1.Secret, error) {
	// TODO
	return nil, nil
}

func (s *ObjectStorageKeyScope) ShouldInitKey() bool {
	return s.Key.Status.LastKeyGeneration == nil
}

func (s *ObjectStorageKeyScope) ShouldRotateKey() bool {
	return s.Key.Status.LastKeyGeneration != nil &&
		*s.Key.Spec.KeyGeneration != *s.Key.Status.LastKeyGeneration
}

func (s *ObjectStorageKeyScope) ShouldRestoreKeySecret(ctx context.Context) (bool, error) {
	// TODO
	return false, nil
}
