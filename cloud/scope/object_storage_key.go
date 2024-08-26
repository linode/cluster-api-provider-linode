package scope

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusteraddonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

type ObjectStorageKeyScopeParams struct {
	Client K8sClient
	Key    *infrav1alpha2.LinodeObjectStorageKey
	Logger *logr.Logger
}

type ObjectStorageKeyScope struct {
	Client       K8sClient
	Key          *infrav1alpha2.LinodeObjectStorageKey
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

func NewObjectStorageKeyScope(ctx context.Context, linodeClientConfig ClientConfig, params ObjectStorageKeyScopeParams) (*ObjectStorageKeyScope, error) {
	if err := validateObjectStorageKeyScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Cluster's Secret reference (if supplied).
	if params.Key.Spec.CredentialsRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, params.Client, *params.Key.Spec.CredentialsRef, params.Key.GetNamespace(), "apiToken")
		if err != nil || len(apiToken) == 0 {
			return nil, fmt.Errorf("credentials from secret ref: %w", err)
		}
		linodeClientConfig.Token = string(apiToken)
	}
	linodeClientConfig.Timeout = clientTimeout
	linodeClient, err := CreateLinodeClient(linodeClientConfig)
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
	if controllerutil.AddFinalizer(s.Key, infrav1alpha2.ObjectStorageKeyFinalizer) {
		return s.Close(ctx)
	}

	return nil
}

// GenerateKeySecret returns a secret suitable for submission to the Kubernetes API.
// The secret is expected to contain keys for accessing the bucket, as well as owner and controller references.
func (s *ObjectStorageKeyScope) GenerateKeySecret(ctx context.Context, key *linodego.ObjectStorageKey) (*corev1.Secret, error) {
	if key == nil {
		return nil, errors.New("expected non-nil object storage key")
	}

	secretStringData := make(map[string]string)

	tmplData := map[string]string{
		"AccessKey": key.AccessKey,
		"SecretKey": key.SecretKey,
	}

	// If the desired secret is of ClusterResourceSet type, encapsulate the secret.
	// Bucket details are retrieved from the first referenced LinodeObjectStorageBucket in the access key.
	if s.Key.Spec.GeneratedSecret.Type == clusteraddonsv1.ClusterResourceSetSecretType {
		// This should never run since the CRD has a validation marker to ensure bucketAccess has at least one item.
		if len(s.Key.Spec.BucketAccess) == 0 {
			return nil, fmt.Errorf("unable to generate %s; spec.bucketAccess must not be empty", clusteraddonsv1.ClusterResourceSetSecretType)
		}

		bucketRef := s.Key.Spec.BucketAccess[0]
		bucket, err := s.LinodeClient.GetObjectStorageBucket(ctx, bucketRef.Region, bucketRef.BucketName)
		if err != nil {
			return nil, fmt.Errorf("unable to generate %s; failed to get bucket: %w", clusteraddonsv1.ClusterResourceSetSecretType, err)
		}

		tmplData["BucketEndpoint"] = bucket.Hostname
	} else if len(s.Key.Spec.GeneratedSecret.Format) == 0 {
		secretStringData = map[string]string{
			"access_key": key.AccessKey,
			"secret_key": key.SecretKey,
		}
	}

	for key, tmpl := range s.Key.Spec.GeneratedSecret.Format {
		goTmpl, err := template.New(key).Parse(tmpl)
		if err != nil {
			return nil, fmt.Errorf("unable to generate secret; failed to parse template in secret data format for key %s: %w", key, err)
		}

		var output bytes.Buffer
		if err := goTmpl.Execute(&output, tmplData); err != nil {
			return nil, fmt.Errorf("unable to generate secret; failed to exec template in secret data format for key %s: %w", key, err)
		}

		secretStringData[key] = output.String()
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Key.Spec.GeneratedSecret.Name,
			Namespace: s.Key.Spec.GeneratedSecret.Namespace,
		},
		Type:       s.Key.Spec.GeneratedSecret.Type,
		StringData: secretStringData,
	}

	// Set an owner reference on a Secret if it will exist in the same namespace as the Key resource.
	// Kubernetes does not allow cross-namespace ownership so modifications to a Secret in another namespace won't trigger reconciliation.
	if s.Key.Spec.GeneratedSecret.Namespace == s.Key.Namespace {
		if err := controllerutil.SetControllerReference(s.Key, &secret, s.Client.Scheme()); err != nil {
			return nil, fmt.Errorf("could not set controller ref on access key secret %s/%s: %w", s.Key.Spec.GeneratedSecret.Name, s.Key.Spec.GeneratedSecret.Namespace, err)
		}
	}

	return &secret, nil
}

func (s *ObjectStorageKeyScope) ShouldInitKey() bool {
	return s.Key.Status.LastKeyGeneration == nil
}

func (s *ObjectStorageKeyScope) ShouldRotateKey() bool {
	return s.Key.Status.LastKeyGeneration != nil &&
		s.Key.Spec.KeyGeneration != *s.Key.Status.LastKeyGeneration
}
