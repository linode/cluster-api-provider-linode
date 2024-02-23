package scope

import (
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
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

	PatchHelper         *patch.Helper
	ObjectStorageBucket *infrav1alpha1.LinodeObjectStorageBucket
	LinodeClient        *linodego.Client
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

	scope := &ObjectStorageBucketScope{
		client:              params.Client,
		PatchHelper:         helper,
		ObjectStorageBucket: params.ObjectStorageBucket,
	}

	apiKey, err := scope.GetApiKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}

	scope.LinodeClient = createLinodeClient(apiKey)

	return scope, nil
}

// PatchObject persists the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.ObjectStorageBucket)
}

// Close closes the current scope persisting the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *ObjectStorageBucketScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.ObjectStorageBucket, infrav1alpha1.GroupVersion.String()) {
		return s.Close(ctx)
	}

	return nil
}

// GetApiKey returns the Linode API key from the Secret referenced by the ObjectStorageBucket's apiKeySecretRef.
func (s *ObjectStorageBucketScope) GetApiKey(ctx context.Context) (string, error) {
	if s.ObjectStorageBucket.Spec.ApiKeySecretRef == nil ||
		s.ObjectStorageBucket.Spec.ApiKeySecretRef.Name == "" ||
		s.ObjectStorageBucket.Spec.ApiKeySecretRef.Key == "" {
		return "", fmt.Errorf(
			"api key secret ref is nil or malformed for LinodeObjectStorageBucket %s/%s",
			s.ObjectStorageBucket.Namespace,
			s.ObjectStorageBucket.Name,
		)
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: s.ObjectStorageBucket.Namespace, Name: s.ObjectStorageBucket.Spec.ApiKeySecretRef.Name}
	if err := s.client.Get(ctx, key, secret); err != nil {
		return "", fmt.Errorf(
			"failed to retrieve api key secret for LinodeObjectStorageBucket %s/%s: %w",
			s.ObjectStorageBucket.Namespace,
			s.ObjectStorageBucket.Name,
			err,
		)
	}

	value, ok := secret.Data[s.ObjectStorageBucket.Spec.ApiKeySecretRef.Key]
	if !ok {
		return "", fmt.Errorf(
			"api key secret ref key is invalid for LinodeObjectStorageBucket %s/%s",
			s.ObjectStorageBucket.Namespace,
			s.ObjectStorageBucket.Name,
		)
	}

	var decoded []byte
	_, err := b64.StdEncoding.Decode(decoded, value)
	if err != nil {
		return "", fmt.Errorf(
			"error while decoding api key from secret for LinodeObjectStorageBucket %s/%s: %w",
			s.ObjectStorageBucket.Namespace,
			s.ObjectStorageBucket.Name,
			err,
		)
	}

	return string(decoded), nil
}
