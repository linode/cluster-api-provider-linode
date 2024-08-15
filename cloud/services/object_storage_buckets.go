package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

func EnsureAndUpdateObjectStorageBucket(ctx context.Context, bScope *scope.ObjectStorageBucketScope) (*linodego.ObjectStorageBucket, error) {
	bucket, err := bScope.LinodeClient.GetObjectStorageBucket(
		ctx,
		bScope.Bucket.Spec.Region,
		bScope.Bucket.Name,
	)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		return nil, fmt.Errorf("failed to get bucket from region %s: %w", bScope.Bucket.Spec.Region, err)
	}
	if bucket == nil {
		opts := linodego.ObjectStorageBucketCreateOptions{
			Region:      bScope.Bucket.Spec.Region,
			Label:       bScope.Bucket.Name,
			ACL:         linodego.ObjectStorageACL(bScope.Bucket.Spec.ACL),
			CorsEnabled: &bScope.Bucket.Spec.CorsEnabled,
		}

		if bucket, err = bScope.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}

		bScope.Logger.Info("Created bucket")

		return bucket, nil
	}

	bucketAccess, err := bScope.LinodeClient.GetObjectStorageBucketAccess(
		ctx,
		bucket.Region,
		bucket.Label,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket access details for %s: %w", bScope.Bucket.Name, err)
	}

	if (bucketAccess.ACL == linodego.ObjectStorageACL(bScope.Bucket.Spec.ACL)) && bucketAccess.CorsEnabled == bScope.Bucket.Spec.CorsEnabled {
		return bucket, nil
	}

	opts := linodego.ObjectStorageBucketUpdateAccessOptions{
		ACL:         linodego.ObjectStorageACL(bScope.Bucket.Spec.ACL),
		CorsEnabled: &bScope.Bucket.Spec.CorsEnabled,
	}
	if err = bScope.LinodeClient.UpdateObjectStorageBucketAccess(ctx, bucket.Region, bucket.Label, opts); err != nil {
		return nil, fmt.Errorf("failed to update the bucket access options for %s: %w", bScope.Bucket.Name, err)
	}

	bScope.Logger.Info("Updated Bucket")

	return bucket, nil
}
