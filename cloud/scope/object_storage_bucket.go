package scope

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
)

type ObjectStorageBucketScopeParams struct {
	Client              client.Client
	ObjectStorageBucket *infrav1alpha1.LinodeObjectStorageBucket
}

type ObjectStorageBucketScope struct {
	client client.Client

	Object       *infrav1alpha1.LinodeObjectStorageBucket
	PatchHelper  *patch.Helper
	LinodeClient *linodego.Client
}

func validateObjectStorageBucketScopeParams(params ObjectStorageBucketScopeParams) error {
	if params.ObjectStorageBucket == nil {
		return errors.New("object storage bucket is required when creating an ObjectStorageBucketScope")
	}

	return nil
}

func NewObjectStorageBucketScope(ctx context.Context, params ObjectStorageBucketScopeParams) (*ObjectStorageBucketScope, error) {
	if err := validateObjectStorageBucketScopeParams(params); err != nil {
		return nil, err
	}

	helper, err := patch.NewHelper(params.ObjectStorageBucket, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	bucketScope := &ObjectStorageBucketScope{
		client:      params.Client,
		Object:      params.ObjectStorageBucket,
		PatchHelper: helper,
	}

	apiKey, err := bucketScope.GetApiKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	bucketScope.LinodeClient = createLinodeClient(apiKey)

	return bucketScope, nil
}

// PatchObject persists the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.Object)
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

// GetApiKey returns the Linode API key from the Secret referenced by the ObjectStorageBucket's apiKeySecretRef.
func (s *ObjectStorageBucketScope) GetApiKey(ctx context.Context) (string, error) {
	if s.Object.Spec.ApiKeySecretRef.Name == "" || s.Object.Spec.ApiKeySecretRef.Key == "" {
		return "", fmt.Errorf(
			"api key secret ref must specify a name and key for LinodeObjectStorageBucket %s/%s",
			s.Object.Namespace,
			s.Object.Name,
		)
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: s.Object.Namespace, Name: s.Object.Spec.ApiKeySecretRef.Name}
	if err := s.client.Get(ctx, key, secret); err != nil {
		return "", fmt.Errorf(
			"failed to retrieve api key secret for LinodeObjectStorageBucket %s/%s: %w",
			s.Object.Namespace,
			s.Object.Name,
			err,
		)
	}

	apiTokenBytes, ok := secret.Data[s.Object.Spec.ApiKeySecretRef.Key]
	if !ok {
		return "", fmt.Errorf(
			"api key secret ref key is invalid for LinodeObjectStorageBucket %s/%s",
			s.Object.Namespace,
			s.Object.Name,
		)
	}

	return string(apiTokenBytes), nil
}

// CreateAccessKeySecret creates a Secret containing keys created for accessing the bucket.
func (s *ObjectStorageBucketScope) CreateAccessKeySecret(ctx context.Context, keys [2]*linodego.ObjectStorageKey, secretName string) error {
	var err error

	accessKeys := make([]json.RawMessage, 2)
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
	if err := s.client.Create(ctx, secret); err != nil {
		return fmt.Errorf(
			"failed to create api key secret for LinodeObjectStorageBucket %s/%s: %w",
			s.Object.Namespace,
			s.Object.Name,
			err,
		)
	}

	return nil
}
