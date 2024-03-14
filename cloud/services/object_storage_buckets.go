package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linode/linodego"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

func EnsureObjectStorageBucket(ctx context.Context, bScope *scope.ObjectStorageBucketScope) (*linodego.ObjectStorageBucket, error) {
	bucket, err := bScope.LinodeClient.GetObjectStorageBucket(
		ctx,
		bScope.Bucket.Spec.Cluster,
		bScope.Bucket.Name,
	)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		return nil, fmt.Errorf("failed to get bucket from cluster %s: %w", bScope.Bucket.Spec.Cluster, err)
	}
	if bucket != nil {
		bScope.Logger.Info("Bucket exists")

		return bucket, nil
	}

	opts := linodego.ObjectStorageBucketCreateOptions{
		Cluster: bScope.Bucket.Spec.Cluster,
		Label:   bScope.Bucket.Name,
		ACL:     linodego.ACLPrivate,
	}

	if bucket, err = bScope.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	bScope.Logger.Info("Created bucket")

	return bucket, nil
}

func RotateObjectStorageKeys(ctx context.Context, bScope *scope.ObjectStorageBucketScope) ([scope.NumAccessKeys]linodego.ObjectStorageKey, error) {
	var newKeys [scope.NumAccessKeys]linodego.ObjectStorageKey

	for idx, permission := range []struct {
		name   string
		suffix string
	}{
		{"read_write", "rw"},
		{"read_only", "ro"},
	} {
		keyLabel := fmt.Sprintf("%s-%s", bScope.Bucket.Name, permission.suffix)
		key, err := createObjectStorageKey(ctx, bScope, keyLabel, permission.name)
		if err != nil {
			return newKeys, err
		}

		newKeys[idx] = *key
	}

	// If key revocation fails here, just log the errors since new keys have been created
	if bScope.Bucket.Status.LastKeyGeneration != nil && bScope.ShouldRotateKeys() {
		if err := RevokeObjectStorageKeys(ctx, bScope); err != nil {
			bScope.Logger.Error(err, "Failed to revoke access keys; keys must be manually revoked")
		}
	}

	return newKeys, nil
}

func createObjectStorageKey(ctx context.Context, bScope *scope.ObjectStorageBucketScope, label, permission string) (*linodego.ObjectStorageKey, error) {
	opts := linodego.ObjectStorageKeyCreateOptions{
		Label: label,
		BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
			{
				BucketName:  bScope.Bucket.Name,
				Cluster:     bScope.Bucket.Spec.Cluster,
				Permissions: permission,
			},
		},
	}

	key, err := bScope.LinodeClient.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		bScope.Logger.Error(err, "Failed to create access key", "label", label)

		return nil, fmt.Errorf("failed to create access key: %w", err)
	}

	bScope.Logger.Info("Created access key", "id", key.ID)

	return key, nil
}

func RevokeObjectStorageKeys(ctx context.Context, bScope *scope.ObjectStorageBucketScope) error {
	var errs []error
	for _, keyID := range bScope.Bucket.Status.AccessKeyRefs {
		if err := revokeObjectStorageKey(ctx, bScope, keyID); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func revokeObjectStorageKey(ctx context.Context, bScope *scope.ObjectStorageBucketScope, keyID int) error {
	err := bScope.LinodeClient.DeleteObjectStorageKey(ctx, keyID)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		bScope.Logger.Error(err, "Failed to revoke access key", "id", keyID)
		return fmt.Errorf("failed to revoke access key: %w", err)
	}

	bScope.Logger.Info("Revoked access key", "id", keyID)

	return nil
}
