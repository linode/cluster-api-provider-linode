package services

import (
	"context"
	"fmt"

	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func RotateObjectStorageKey(ctx context.Context, bScope *scope.ObjectStorageKeyScope) (*linodego.ObjectStorageKey, error) {
	return nil, nil
}

func CreateObjectStorageKey(ctx context.Context, bScope *scope.ObjectStorageKeyScope) (*linodego.ObjectStorageKey, error) {
	label := bScope.Key.ObjectMeta.Name
	bucketAccess := []linodego.ObjectStorageKeyBucketAccess{}
	for _, bucket := range bScope.Key.Spec.Buckets {
		access := linodego.ObjectStorageKeyBucketAccess{
			Cluster:     bucket.Cluster,
			BucketName:  bucket.Name,
			Permissions: bucket.Permission,
		}
		bucketAccess = append(bucketAccess, access)
	}
	opts := linodego.ObjectStorageKeyCreateOptions{
		Label:        label,
		BucketAccess: &bucketAccess,
	}

	key, err := bScope.LinodeClient.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		bScope.Logger.Error(err, "Failed to create access key", "label", label)

		return nil, fmt.Errorf("failed to create access key: %w", err)
	}

	bScope.Logger.Info("Created access key", "id", key.ID)

	return key, nil
}

func RevokeObjectStorageKey(ctx context.Context, bScope *scope.ObjectStorageKeyScope) error {
	// TODO
	return nil
}

func GetObjectStorageKey(ctx context.Context, bScope *scope.ObjectStorageKeyScope) (*linodego.ObjectStorageKey, error) {
	// TODO
	return nil, nil
}
