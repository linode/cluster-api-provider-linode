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
		logger.Error(err, "Failed to list object storage buckets; unable to provision/confirm")

		return nil, err
	}
	if len(buckets) == 1 {
		logger.Info("Object storage bucket exists")

		return &buckets[0], nil
	}

	logger.Info("Creating object storage bucket")
	opts := linodego.ObjectStorageBucketCreateOptions{
		Cluster: bucketScope.Object.Spec.Cluster,
		Label:   bucketScope.Object.Name,
		ACL:     linodego.ACLPrivate,
	}

	if bucket, err = bucketScope.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
		logger.Error(err, "Failed to create object storage bucket")

		return nil, err
	}

	return bucket, nil
}

func CreateOrRotateObjectStorageKeys(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, shouldRotate bool, logger logr.Logger) ([scope.AccessKeySecretLength]linodego.ObjectStorageKey, error) {
	var newKeys [scope.AccessKeySecretLength]linodego.ObjectStorageKey

	for i, permission := range []struct {
		name   string
		suffix string
	}{
		{"read_write", "rw"},
		{"read_only", "ro"},
	} {
		keyLabel := fmt.Sprintf("%s-%s", bucketScope.Object.Name, permission.suffix)
		key, err := createObjectStorageKey(ctx, bucketScope, keyLabel, permission.name, logger)
		if err != nil {
			return newKeys, err
		}

		newKeys[i] = *key
	}

	if shouldRotate {
		secretName := fmt.Sprintf(scope.AccessKeyNameTemplate, bucketScope.Object.Name)
		keyIDs, err := bucketScope.GetAccessKeysFromSecret(ctx, secretName, logger)
		if err != nil {
			return newKeys, err
		}

		if err := RevokeObjectStorageKeys(ctx, bucketScope, keyIDs, logger); err != nil {
			logger.Info("previous access keys must be manually revoked by the account owner")
		}
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
		logger.Error(err, "Failed to create object storage key", "label", label)

		return nil, err
	}

	return key, nil
}

func RevokeObjectStorageKeys(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, keyIDs [scope.AccessKeySecretLength]int, logger logr.Logger) error {
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
