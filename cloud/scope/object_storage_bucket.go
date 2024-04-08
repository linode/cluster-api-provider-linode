package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

const bucketDataSecret = `kind: Secret
apiVersion: v1
metadata:
  name: etcd-backup
  namespace: kube-system
stringData:
  bucket_name: %s
  bucket_region: %s
  bucket_endpoint: %s
  access_key_rw: %s
  secret_key_rw: %s
  access_key_ro: %s
  secret_key_ro: %s`

type ObjectStorageBucketScopeParams struct {
	Client              K8sClient
	LinodeClientBuilder LinodeObjectStorageClientBuilder
	Bucket              *infrav1alpha1.LinodeObjectStorageBucket
	Logger              *logr.Logger
}

type ObjectStorageBucketScope struct {
	Client       K8sClient
	Bucket       *infrav1alpha1.LinodeObjectStorageBucket
	Logger       logr.Logger
	LinodeClient LinodeObjectStorageClient
	PatchHelper  *patch.Helper
}

const AccessKeyNameTemplate = "%s-bucket-details"
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

	patchHelper, err := patch.NewHelper(params.Bucket, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ObjectStorageBucketScope{
		Client:       params.Client,
		Bucket:       params.Bucket,
		Logger:       *params.Logger,
		LinodeClient: linodeClient,
		PatchHelper:  patchHelper,
	}, nil
}

// PatchObject persists the object storage bucket configuration and status.
func (s *ObjectStorageBucketScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.Bucket)
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

// GenerateKeySecret returns a secret suitable for submission to the Kubernetes API.
// The secret is expected to contain keys for accessing the bucket, as well as owner and controller references.
func (s *ObjectStorageBucketScope) GenerateKeySecret(ctx context.Context, keys [NumAccessKeys]*linodego.ObjectStorageKey, bucket *linodego.ObjectStorageBucket) (*corev1.Secret, error) {
	for _, key := range keys {
		if key == nil {
			return nil, errors.New("expected two non-nil object storage keys")
		}
	}
	var secretStringData map[string]string
	secretName := fmt.Sprintf(AccessKeyNameTemplate, s.Bucket.Name)
	// If the secret is of ClusterResourceSet type, encapsulate real data in the outer data
	if s.Bucket.Spec.SecretType == "addons.cluster.x-k8s.io/resource-set" {
		secretStringData = map[string]string{
			"bucket-details-secret.yaml": fmt.Sprintf(bucketDataSecret,
				bucket.Label,
				bucket.Cluster,
				bucket.Hostname,
				keys[0].AccessKey,
				keys[0].SecretKey,
				keys[1].AccessKey,
				keys[1].SecretKey,
			),
		}
	} else {
		secretStringData = map[string]string{
			"bucket_name":     bucket.Label,
			"bucket_region":   bucket.Cluster,
			"bucket_endpoint": bucket.Hostname,
			"access_key_rw":   keys[0].AccessKey,
			"secret_key_rw":   keys[0].SecretKey,
			"access_key_ro":   keys[1].AccessKey,
			"secret_key_ro":   keys[1].SecretKey,
		}
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: s.Bucket.Namespace,
		},
		Type:       corev1.SecretType(s.Bucket.Spec.SecretType),
		StringData: secretStringData,
	}

	scheme := s.Client.Scheme()
	if err := controllerutil.SetOwnerReference(s.Bucket, secret, scheme); err != nil {
		return nil, fmt.Errorf("could not set owner ref on access key secret %s: %w", secretName, err)
	}
	if err := controllerutil.SetControllerReference(s.Bucket, secret, scheme); err != nil {
		return nil, fmt.Errorf("could not set controller ref on access key secret %s: %w", secretName, err)
	}

	return secret, nil
}

func (s *ObjectStorageBucketScope) ShouldInitKeys() bool {
	return s.Bucket.Status.LastKeyGeneration == nil
}

func (s *ObjectStorageBucketScope) ShouldRotateKeys() bool {
	return s.Bucket.Status.LastKeyGeneration != nil &&
		*s.Bucket.Spec.KeyGeneration != *s.Bucket.Status.LastKeyGeneration
}

func (s *ObjectStorageBucketScope) ShouldRestoreKeySecret(ctx context.Context) (bool, error) {
	if s.Bucket.Status.KeySecretName == nil {
		return false, nil
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: s.Bucket.Namespace, Name: *s.Bucket.Status.KeySecretName}
	err := s.Client.Get(ctx, key, secret)

	return apierrors.IsNotFound(err), client.IgnoreNotFound(err)
}
