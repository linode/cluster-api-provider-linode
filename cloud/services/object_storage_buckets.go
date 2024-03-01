package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func EnsureObjectStorageBucket(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, logger logr.Logger) (*linodego.ObjectStorageBucket, error) {
	var buckets []linodego.ObjectStorageBucket
	var bucket *linodego.ObjectStorageBucket

	filter := map[string]string{
		"label": bucketScope.Object.Name,
	}

	rawFilter, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}

	if buckets, err = bucketScope.LinodeClient.ListObjectStorageBucketsInCluster(
		ctx,
		linodego.NewListOptions(1, string(rawFilter)),
		bucketScope.Object.Spec.Cluster,
	); err != nil {
		logger.Info("Failed to list object storage buckets", "error", err.Error())

		return nil, err
	}
	if len(buckets) == 1 {
		return &buckets[0], nil
	}

	logger.Info(fmt.Sprintf("Creating object storage bucket %s", bucketScope.Object.Name))
	opts := linodego.ObjectStorageBucketCreateOptions{
		Cluster: bucketScope.Object.Spec.Cluster,
		Label:   bucketScope.Object.Name,
		ACL:     linodego.ACLPrivate,
	}

	if bucket, err = bucketScope.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
		logger.Info("Failed to create object storage bucket", "error", err.Error())

		return nil, err
	}

	return bucket, nil
}

func CreateOrRotateObjectStorageKeys(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, shouldRotate bool, logger logr.Logger) ([2]linodego.ObjectStorageKey, error) {
	var newKeys [2]linodego.ObjectStorageKey
	var existingKeys []linodego.ObjectStorageKey
	var err error

	if existingKeys, err = bucketScope.LinodeClient.ListObjectStorageKeys(
		ctx,
		// TODO: What if there are keys exceeding page 1?
		linodego.NewListOptions(1, "{}"),
	); err != nil {
		logger.Info("Failed to list object storage keys", "error", err.Error())

		return newKeys, err
	}

	keysSet := make(map[string]struct{})
	for _, key := range existingKeys {
		keysSet[key.Label] = struct{}{}
	}

	for i, e := range []struct {
		permission string
		suffix     string
	}{
		{"read_write", "rw"},
		{"read_only", "ro"},
	} {
		keyLabel := fmt.Sprintf("%s-%s", bucketScope.Object.Name, e.suffix)

		if _, ok := keysSet[keyLabel]; ok {
			logger.Info(fmt.Sprintf("Found existing object storage key %s", keyLabel))

			// If keys are not being rotated, store the existing key
			if !shouldRotate {
				newKeys[i] = existingKeys[0]
				continue
			}

			// Keys are being rotated, so we should revoke this key before making a new one
			oldKeyID := existingKeys[0].ID
			if err := revokeObjectStorageKey(ctx, bucketScope, oldKeyID, logger); err != nil {
				logger.Info("Failed to revoke object storage key for rotation", "id", oldKeyID, "error", err.Error())
			}
		}

		key, err := createObjectStorageKey(ctx, bucketScope, keyLabel, e.permission, logger)
		if err != nil {
			return newKeys, err
		}

		newKeys[i] = *key
	}

	return newKeys, nil
}

func createObjectStorageKey(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, label, permission string, logger logr.Logger) (*linodego.ObjectStorageKey, error) {
	logger.Info(fmt.Sprintf("Creating object storage key %s", label))
	opts := linodego.ObjectStorageKeyCreateOptions{
		Label: label,
		BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
			{
				BucketName:  bucketScope.Object.Name,
				Cluster:     bucketScope.Object.Spec.Cluster,
				Permissions: permission,
			},
		},
	}

	key, err := bucketScope.LinodeClient.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		logger.Info("Failed to create object storage key", "label", label, "error", err.Error())

		return nil, err
	}

	return key, nil
}

func RevokeObjectStorageKeys(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, keyIDs [2]int, logger logr.Logger) error {
	for _, keyID := range keyIDs {
		if err := revokeObjectStorageKey(ctx, bucketScope, keyID, logger); err != nil {
			logger.Info("Failed to revoke object storage key", "id", keyID, "error", err.Error())
		}
	}

	return nil
}

func revokeObjectStorageKey(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, keyID int, logger logr.Logger) error {
	if err := bucketScope.LinodeClient.DeleteObjectStorageKey(ctx, keyID); err != nil {
		return err
	}

	logger.Info("Revoked object storage key", "id", keyID)

	return nil
}
