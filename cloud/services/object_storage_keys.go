package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

func RotateObjectStorageKey(ctx context.Context, keyScope *scope.ObjectStorageKeyScope) (*linodego.ObjectStorageKey, error) {
	key, err := createObjectStorageKey(ctx, keyScope)
	if err != nil {
		return nil, err
	}

	// If key revocation is necessary and fails, just log the error since the new key has been created
	if !keyScope.ShouldInitKey() && keyScope.ShouldRotateKey() {
		if err := RevokeObjectStorageKey(ctx, keyScope); err != nil {
			keyScope.Logger.Error(err, "Failed to revoke access key; key must be manually revoked")
		}
	}

	return key, nil
}

func createObjectStorageKey(ctx context.Context, keyScope *scope.ObjectStorageKeyScope) (*linodego.ObjectStorageKey, error) {
	bucketAccess := make([]linodego.ObjectStorageKeyBucketAccess, len(keyScope.Key.Spec.BucketAccess))
	for idx, bucket := range keyScope.Key.Spec.BucketAccess {
		bucketAccess[idx] = linodego.ObjectStorageKeyBucketAccess{
			Region:      bucket.Region,
			BucketName:  bucket.BucketName,
			Permissions: bucket.Permissions,
		}
	}
	opts := linodego.ObjectStorageKeyCreateOptions{
		Label:        keyScope.Key.Name,
		BucketAccess: &bucketAccess,
	}

	key, err := keyScope.LinodeClient.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		keyScope.Logger.Error(err, "Failed to create access key", "label", opts.Label)

		return nil, fmt.Errorf("failed to create access key: %w", err)
	}

	keyScope.Logger.Info("Created access key", "id", key.ID)

	return key, nil
}

func RevokeObjectStorageKey(ctx context.Context, keyScope *scope.ObjectStorageKeyScope) error {
	err := keyScope.LinodeClient.DeleteObjectStorageKey(ctx, *keyScope.Key.Status.AccessKeyRef)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		keyScope.Logger.Error(err, "Failed to revoke access key", "id", *keyScope.Key.Status.AccessKeyRef)
		return fmt.Errorf("failed to revoke access key: %w", err)
	}

	keyScope.Logger.Info("Revoked access key", "id", *keyScope.Key.Status.AccessKeyRef)

	return nil
}

func GetObjectStorageKey(ctx context.Context, keyScope *scope.ObjectStorageKeyScope) (*linodego.ObjectStorageKey, error) {
	key, err := keyScope.LinodeClient.GetObjectStorageKey(ctx, *keyScope.Key.Status.AccessKeyRef)
	if err != nil {
		return nil, err
	}

	return key, nil
}
