package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func CreateObjectStorageBucket(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, label string, logger logr.Logger) (*linodego.ObjectStorageBucket, error) {
	var buckets []linodego.ObjectStorageBucket
	var bucket *linodego.ObjectStorageBucket

	filter := map[string]string{
		"label": label,
	}

	rawFilter, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	if buckets, err = bucketScope.LinodeClient.ListObjectStorageBuckets(ctx, linodego.NewListOptions(1, string(rawFilter))); err != nil {
		logger.Info("Failed to list object storage buckets", "error", err.Error())

		return nil, err
	}
	if len(buckets) == 1 {
		logger.Info(fmt.Sprintf("ObjectStorageBucket %s already exists", buckets[0].Label))

		return &buckets[0], nil
	}

	logger.Info(fmt.Sprintf("Creating Object Storage Bucket %s", label))
	opts := linodego.ObjectStorageBucketCreateOptions{
		Cluster: bucketScope.Object.Spec.Region,
		Label:   label,
		ACL:     linodego.ACLPrivate,
	}

	if bucket, err = bucketScope.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
		logger.Info("Failed to create Object Storage Bucket", "error", err.Error())

		return nil, err
	}

	return bucket, nil
}

func CreateObjectStorageKeys(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, bucketLabel string, logger logr.Logger) ([2]*linodego.ObjectStorageKey, error) {
	var keys [2]*linodego.ObjectStorageKey

	for i, permission := range []string{"read_write", "read_only"} {
		keyLabel := fmt.Sprintf("%s-%s", bucketLabel, permission)

		logger.Info(fmt.Sprintf("Creating Object Storage Key %s", keyLabel))
		opts := linodego.ObjectStorageKeyCreateOptions{
			Label: keyLabel,
			BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
				{
					Cluster:     bucketScope.Object.Spec.Region,
					BucketName:  bucketLabel,
					Permissions: permission,
				},
			},
		}

		key, err := bucketScope.LinodeClient.CreateObjectStorageKey(ctx, opts)
		if err != nil {
			logger.Info("Failed to create Object Storage Bucket", "label", keyLabel, "error", err.Error())

			return keys, err
		}

		keys[i] = key
	}

	return keys, nil
}
