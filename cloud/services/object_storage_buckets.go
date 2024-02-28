package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func CreateObjectStorageBucket(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, logger logr.Logger) (*linodego.ObjectStorageBucket, error) {
	var buckets []linodego.ObjectStorageBucket
	var bucket *linodego.ObjectStorageBucket

	filter := map[string]string{
		"label": bucketScope.Object.Spec.Label,
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
		logger.Info(fmt.Sprintf("ObjectStorageBucket %s already exists", buckets[0].Label))

		return &buckets[0], nil
	}

	logger.Info(fmt.Sprintf("Creating Object Storage Bucket %s", bucketScope.Object.Spec.Label))
	opts := linodego.ObjectStorageBucketCreateOptions{
		Cluster: bucketScope.Object.Spec.Cluster,
		Label:   bucketScope.Object.Spec.Label,
		ACL:     linodego.ACLPrivate,
	}

	if bucket, err = bucketScope.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
		logger.Info("Failed to create Object Storage Bucket", "error", err.Error())

		return nil, err
	}

	return bucket, nil
}

func DeleteObjectStorageBucket(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, logger logr.Logger) error {
	// Delete the OBJ bucket.
	if err := bucketScope.LinodeClient.DeleteObjectStorageBucket(ctx, bucketScope.Object.Spec.Cluster, bucketScope.Object.Spec.Label); err != nil {
		return fmt.Errorf("delete object storage bucket: %w", err)
	}

	return nil
}

func CreateObjectStorageKeys(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, logger logr.Logger) ([2]linodego.ObjectStorageKey, error) {
	var newKeys [2]linodego.ObjectStorageKey
	var existingKeys []linodego.ObjectStorageKey
	var err error

	if existingKeys, err = bucketScope.LinodeClient.ListObjectStorageKeys(
		ctx,
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
		keyLabel := fmt.Sprintf("%s-%s-%s", bucketScope.Object.Spec.Cluster, bucketScope.Object.Spec.Label, e.suffix)

		if _, ok := keysSet[keyLabel]; ok {
			logger.Info(fmt.Sprintf("Object storage key %s already exists", keyLabel))

			newKeys[i] = existingKeys[0]
			continue
		}

		logger.Info(fmt.Sprintf("Creating Object Storage Key %s", keyLabel))
		opts := linodego.ObjectStorageKeyCreateOptions{
			Label: keyLabel,
			BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
				{
					BucketName:  bucketScope.Object.Spec.Label,
					Cluster:     bucketScope.Object.Spec.Cluster,
					Permissions: e.permission,
				},
			},
		}

		key, err := bucketScope.LinodeClient.CreateObjectStorageKey(ctx, opts)
		if err != nil {
			logger.Info("Failed to create Object Storage Bucket", "label", keyLabel, "error", err.Error())

			return newKeys, err
		}

		newKeys[i] = *key
	}

	return newKeys, nil
}
