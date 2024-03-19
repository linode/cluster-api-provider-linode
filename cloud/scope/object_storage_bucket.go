package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

type ObjectStorageBucketScopeParams struct {
	Client              k8sClient
	LinodeClientBuilder LinodeClientBuilder
	Bucket              *infrav1alpha1.LinodeObjectStorageBucket
	Logger              *logr.Logger
}

type ObjectStorageBucketScope struct {
	client            k8sClient
	Bucket            *infrav1alpha1.LinodeObjectStorageBucket
	Logger            logr.Logger
	LinodeClient      LinodeClient
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
	if params.LinodeClientBuilder == nil {
		return errors.New("LinodeClientBuilder is required when creating an ObjectStorageBucketScope")
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
	linodeClient, err := params.LinodeClientBuilder(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

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
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: s.Bucket.Namespace,
		},
		StringData: map[string]string{
			"read_write": keys[0].AccessKey,
			"read_only":  keys[1].AccessKey,
		},
	}

	if err := controllerutil.SetOwnerReference(s.Bucket, secret, s.client.Scheme()); err != nil {
		return fmt.Errorf("could not set owner ref on access key secret %s: %w", secretName, err)
	}

	result, err := controllerutil.CreateOrPatch(ctx, s.client, secret, func() error { return nil })
	if err != nil {
		return fmt.Errorf("could not create/patch access key secret %s: %w", secretName, err)
	}

	s.Logger.Info(fmt.Sprintf("Secret %s was %s with new access keys", secret.Name, result))

	return nil
}

func (s *ObjectStorageBucketScope) ShouldRotateKeys() bool {
	return *s.Bucket.Spec.KeyGeneration != *s.Bucket.Status.LastKeyGeneration
}
