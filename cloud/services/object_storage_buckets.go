package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

func EnsureObjectStorageBucket(ctx context.Context, bScope *scope.ObjectStorageBucketScope) (*linodego.ObjectStorageBucket, error) {
	bucket, err := bScope.LinodeClient.GetObjectStorageBucket(
		ctx,
		bScope.Bucket.Spec.Region,
		bScope.Bucket.Name,
	)
	if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
		return nil, fmt.Errorf("failed to get bucket from region %s: %w", bScope.Bucket.Spec.Region, err)
	}
	if bucket != nil {
		bScope.Logger.Info("Bucket exists")

		return bucket, nil
	}

	opts := linodego.ObjectStorageBucketCreateOptions{
		Region: bScope.Bucket.Spec.Region,
		Label:  bScope.Bucket.Name,
		ACL:    linodego.ObjectStorageACL(bScope.Bucket.Spec.ACL),
	}

	if bucket, err = bScope.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	bScope.Logger.Info("Created bucket")

	return bucket, nil
}
