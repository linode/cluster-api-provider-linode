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

func CreateObjectStorageKeys(ctx context.Context, bucketScope *scope.ObjectStorageBucketScope, logger logr.Logger) ([2]linodego.ObjectStorageKey, error) {
	var newKeys [2]linodego.ObjectStorageKey

	for i, e := range []struct {
		permission string
		suffix     string
	}{
		{"read_write", "rw"},
		{"read_only", "ro"},
	} {
		keyLabel := fmt.Sprintf("%s-%s-%s", bucketScope.Object.Spec.Cluster, bucketScope.Object.Spec.Label, e.suffix)
		filter := map[string]string{
			"label": keyLabel,
		}

		rawFilter, err := json.Marshal(filter)
		if err != nil {
			return newKeys, err
		}

		var existingKeys []linodego.ObjectStorageKey
		if existingKeys, err = bucketScope.LinodeClient.ListObjectStorageKeys(
			ctx,
			linodego.NewListOptions(1, string(rawFilter)),
		); err != nil {
			logger.Info("Failed to list object storage keys", "error", err.Error())

			return newKeys, err
		}
		if len(existingKeys) == 1 {
			logger.Info(fmt.Sprintf("ObjectStorageBucket %s already exists", existingKeys[0].Label))

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
