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
	Bucket *infrav1alpha1.LinodeObjectStorageBucket
	Logger *logr.Logger
}

type ObjectStorageBucketScope struct {
	client            client.Client
	Bucket            *infrav1alpha1.LinodeObjectStorageBucket
	Logger            logr.Logger
	LinodeClient      *linodego.Client
	BucketPatchHelper *patch.Helper
}

const AccessKeyNameTemplate = "%s-access-keys"
const NumAccessKeys = 2

func validateObjectStorageBucketScopeParams(params ObjectStorageBucketScopeParams) error {
	if params.Bucket == nil {
		return errors.New("object storage bucket is required when creating an ObjectStorageBucketScope")
	}
	if params.Logger == nil {
		return errors.New("logger is required when creating an ObjectStorageBucketScope")
	}

	return nil
}

func NewObjectStorageBucketScope(ctx context.Context, apiKey string, params ObjectStorageBucketScopeParams) (*ObjectStorageBucketScope, error) {
	if err := validateObjectStorageBucketScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Cluster's Secret reference (if supplied).
	if params.Bucket.Spec.CredentialsRef != nil {
		data, err := getCredentialDataFromRef(ctx, params.Client, *params.Bucket.Spec.CredentialsRef, params.Bucket.GetNamespace())
		if err != nil {
			return nil, fmt.Errorf("credentials from cluster secret ref: %w", err)
		}
		apiKey = string(data)
	}
	linodeClient := createLinodeClient(apiKey)

	bucketPatchHelper, err := patch.NewHelper(params.Bucket, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ObjectStorageBucketScope{
		client:            params.Client,
		Bucket:            params.Bucket,
		Logger:            *params.Logger,
		LinodeClient:      linodeClient,
		BucketPatchHelper: bucketPatchHelper,
	}, nil
}

// PatchObject persists the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) PatchObject(ctx context.Context) error {
	return s.BucketPatchHelper.Patch(ctx, s.Bucket)
}

// Close closes the current scope persisting the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *ObjectStorageBucketScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.Bucket, infrav1alpha1.GroupVersion.String()) {
		return s.Close(ctx)
	}

	return nil
}

// ApplyAccessKeySecret applies a Secret containing keys created for accessing the bucket.
func (s *ObjectStorageBucketScope) ApplyAccessKeySecret(ctx context.Context, keys [NumAccessKeys]linodego.ObjectStorageKey, secretName string) error {
	var err error

	accessKeys := make([]json.RawMessage, NumAccessKeys)
	for i, key := range keys {
		accessKeys[i], err = json.Marshal(key)
		if err != nil {
			return fmt.Errorf("could not unmarshal access key %s: %w", key.Label, err)
		}
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: s.Bucket.Namespace,
		},
		StringData: map[string]string{
			"read_write": string(accessKeys[0]),
			"read_only":  string(accessKeys[1]),
		},
	}

	if err := controllerutil.SetOwnerReference(s.Bucket, secret, s.client.Scheme()); err != nil {
		return fmt.Errorf("could not set owner ref on access key secret %s: %w", secretName, err)
	}

	// Add finalizer to secret so it isn't deleted when bucket deletion is triggered
	controllerutil.AddFinalizer(secret, infrav1alpha1.GroupVersion.String())

	if s.Bucket.Status.KeySecretName == nil {
		if err := s.client.Create(ctx, secret); err != nil {
			return fmt.Errorf("could not create access key secret %s: %w", secretName, err)
		}

		return nil
	}

	if err := s.client.Update(ctx, secret); err != nil {
		return fmt.Errorf("could not update access key secret %s: %w", secretName, err)
	}

	return nil
}

func (s *ObjectStorageBucketScope) GetAccessKeySecret(ctx context.Context) (*corev1.Secret, error) {
	secretName := fmt.Sprintf(AccessKeyNameTemplate, *s.Bucket.Spec.Label)

	objKey := client.ObjectKey{
		Namespace: s.Bucket.Namespace,
		Name:      secretName,
	}
	var secret corev1.Secret
	if err := s.client.Get(ctx, objKey, &secret); err != nil {
		return nil, err
	}

	return &secret, nil
}

// GetAccessKeysFromSecret gets the access key IDs for the OBJ buckets from a Secret.
func (s *ObjectStorageBucketScope) GetAccessKeysFromSecret(ctx context.Context, secret *corev1.Secret) ([NumAccessKeys]int, error) {
	var keyIDs [NumAccessKeys]int

	permissions := [NumAccessKeys]string{"read_write", "read_only"}
	for idx, permission := range permissions {
		secretDataForKey, ok := secret.Data[permission]
		if !ok {
			return keyIDs, fmt.Errorf("secret %s missing data field %s", secret.Name, permission)
		}

		var key linodego.ObjectStorageKey
		if err := json.Unmarshal(secretDataForKey, &key); err != nil {
			return keyIDs, fmt.Errorf("error unmarshaling key: %w", err)
		}

		keyIDs[idx] = key.ID
	}

	return keyIDs, nil
}

func (s *ObjectStorageBucketScope) ShouldRotateKeys() bool {
	return *s.Bucket.Spec.KeyGeneration != *s.Bucket.Status.LastKeyGeneration
}
