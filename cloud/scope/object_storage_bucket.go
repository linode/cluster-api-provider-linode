package scope

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

type ObjectStorageBucketScopeParams struct {
	Client K8sClient
	Bucket *infrav1alpha2.LinodeObjectStorageBucket
	Logger *logr.Logger
}

type ObjectStorageBucketScope struct {
	Client       K8sClient
	Bucket       *infrav1alpha2.LinodeObjectStorageBucket
	Logger       logr.Logger
	LinodeClient LinodeClient
	PatchHelper  *patch.Helper
}

const (
	clientTimeout = 20 * time.Second
)

func validateObjectStorageBucketScopeParams(params ObjectStorageBucketScopeParams) error {
	if params.Bucket == nil {
		return errors.New("object storage bucket is required when creating an ObjectStorageBucketScope")
	}
	if params.Logger == nil {
		return errors.New("logger is required when creating an ObjectStorageBucketScope")
	}

	return nil
}

// TODO: Remove fields related to key provisioning from the bucket resource.
func NewObjectStorageBucketScope(ctx context.Context, linodeClientConfig ClientConfig, params ObjectStorageBucketScopeParams) (*ObjectStorageBucketScope, error) {
	if err := validateObjectStorageBucketScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Cluster's Secret reference (if supplied).
	if params.Bucket.Spec.CredentialsRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, params.Client, *params.Bucket.Spec.CredentialsRef, params.Bucket.GetNamespace(), "apiToken")
		if err != nil {
			return nil, fmt.Errorf("credentials from secret ref: %w", err)
		}
		linodeClientConfig.Token = string(apiToken)
	}
	linodeClientConfig.Timeout = clientTimeout
	linodeClient, err := CreateLinodeClient(linodeClientConfig)
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
	if controllerutil.AddFinalizer(s.Bucket, infrav1alpha2.ObjectStorageBucketFinalizer) {
		return s.Close(ctx)
	}

	return nil
}
