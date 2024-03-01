package scope

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
)

type ObjectStorageBucketScopeParams struct {
	Client client.Client
	Object *infrav1alpha1.LinodeObjectStorageBucket
}

type ObjectStorageBucketScope struct {
	client            client.Client
	Object            *infrav1alpha1.LinodeObjectStorageBucket
	LinodeClient      *linodego.Client
	BucketPatchHelper *patch.Helper
	SecretPatchHelper *patch.Helper
}

const AccessKeyNameTemplate = "%s-access-keys"
const AccessKeySecretLength = 2

func validateObjectStorageBucketScopeParams(params ObjectStorageBucketScopeParams) error {
	if params.Object == nil {
		return errors.New("object storage bucket is required when creating an ObjectStorageBucketScope")
	}

	return nil
}

func NewObjectStorageBucketScope(ctx context.Context, apiKey string, params ObjectStorageBucketScopeParams) (*ObjectStorageBucketScope, error) {
	if err := validateObjectStorageBucketScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Cluster's Secret reference (if supplied).
	if params.Object.Spec.CredentialsRef != nil {
		credRef := *params.Object.Spec.CredentialsRef
		if credRef.Namespace == "" {
			credRef.Namespace = params.Object.Namespace
		}
		data, err := getCredentialDataFromRef(ctx, params.Client, &credRef)
		if err != nil {
			return nil, fmt.Errorf("credentials from cluster secret ref: %w", err)
		}
		apiKey = string(data)
	}
	linodeClient := createLinodeClient(apiKey)

	bucketPatchHelper, err := patch.NewHelper(params.Object, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	secretPatchHelper, err := patch.NewHelper(&corev1.Secret{}, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	bucketScope := &ObjectStorageBucketScope{
		client:            params.Client,
		Object:            params.Object,
		LinodeClient:      linodeClient,
		BucketPatchHelper: bucketPatchHelper,
		SecretPatchHelper: secretPatchHelper,
	}

	return bucketScope, nil
}

// PatchObject persists the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) PatchObject(ctx context.Context) error {
	return s.BucketPatchHelper.Patch(ctx, s.Object)
}

// Close closes the current scope persisting the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *ObjectStorageBucketScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.Object, infrav1alpha1.GroupVersion.String()) {
		return s.Close(ctx)
	}

	return nil
}

// ApplyAccessKeySecret applies a Secret containing keys created for accessing the bucket.
func (s *ObjectStorageBucketScope) ApplyAccessKeySecret(ctx context.Context, keys [AccessKeySecretLength]linodego.ObjectStorageKey, secretName string, logger logr.Logger) error {
	var err error

	accessKeys := make([]json.RawMessage, AccessKeySecretLength)
	for i, key := range keys {
		accessKeys[i], err = json.Marshal(key)
		if err != nil {
			return fmt.Errorf(
				"error while unmarshaling access key %s for LinodeObjectStorageBucket %s/%s: %w",
				key.Label,
				s.Object.Namespace,
				s.Object.Name,
				err,
			)
		}
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: s.Object.Namespace,
		},
		StringData: map[string]string{
			"read_write": string(accessKeys[0]),
			"read_only":  string(accessKeys[1]),
		},
	}

	if err := controllerutil.SetOwnerReference(s.Object, secret, s.client.Scheme()); err != nil {
		return fmt.Errorf(
			"error while creating access key secret %s for LinodeObjectStorageBucket %s/%s: failed to set owner ref: %w",
			secretName,
			s.Object.Namespace,
			s.Object.Name,
			err,
		)
	}

	// Add finalizer to secret so it isn't deleted when bucket deletion is triggered
	controllerutil.AddFinalizer(secret, infrav1alpha1.GroupVersion.String())

	if s.Object.Status.KeySecretName == nil {
		if err := s.client.Create(ctx, secret); err != nil {
			return fmt.Errorf(
				"failed to create access key secret %s for LinodeObjectStorageBucket %s/%s: %w",
				secretName,
				s.Object.Namespace,
				s.Object.Name,
				err,
			)
		}

		return nil
	}

	if err := s.SecretPatchHelper.Patch(ctx, secret); err != nil {
		return fmt.Errorf(
			"failed to patch access key secret %s for LinodeObjectStorageBucket %s/%s: %w",
			secretName,
			s.Object.Namespace,
			s.Object.Name,
			err,
		)
	}

	return nil
}

// GetAccessKeysFromSecret gets the access key IDs for the OBJ buckets from a Secret.
func (s *ObjectStorageBucketScope) GetAccessKeysFromSecret(ctx context.Context, secretName string, logger logr.Logger) ([AccessKeySecretLength]int, error) {
	var keyIDs [AccessKeySecretLength]int

	// Delete the access keys.
	objkey := client.ObjectKey{
		Namespace: s.Object.Namespace,
		Name:      secretName,
	}
	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, objkey, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return keyIDs, nil
		}

		return keyIDs, fmt.Errorf("failed to get access key secret %s; unable to revoke keys: %w", secretName, err)
	}

	// Allow deletion of the secret since we have the access key IDs to revoke
	controllerutil.RemoveFinalizer(secret, infrav1alpha1.GroupVersion.String())
	if err := s.SecretPatchHelper.Patch(ctx, secret); err != nil {
		logger.Info("Failed to patch secret; will not be deleted", "secret", secret.Name)
	}

	permissions := [AccessKeySecretLength]string{"read_write", "read_only"}
	for idx, permission := range permissions {
		secretDataForKey, ok := secret.Data[permission]
		if !ok {
			return keyIDs, fmt.Errorf("secret %s missing data field: %s", secretName, permission)
		}

		key := &linodego.ObjectStorageKey{}
		if err := json.Unmarshal(secretDataForKey, key); err != nil {
			return keyIDs, fmt.Errorf("error unmarshalling key: %w", err)
		}

		keyIDs[idx] = key.ID
	}

	return keyIDs, nil
}

func (s *ObjectStorageBucketScope) ShouldGenerateAccessKeys() bool {
	if s.Object.Status.LastKeyGeneration == nil {
		return true
	}

	return *s.Object.Spec.KeyGeneration != *s.Object.Status.LastKeyGeneration
}
