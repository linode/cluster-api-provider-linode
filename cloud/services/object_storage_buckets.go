package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

// EnsureAndUpdateObjectStorageBucket ensures that the bucket exists and updates its access options if necessary.
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

	if bucketAccess.ACL == linodego.ObjectStorageACL(bScope.Bucket.Spec.ACL) && bucketAccess.CorsEnabled == bScope.Bucket.Spec.CorsEnabled {
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

// DeleteBucket deletes the bucket and all its objects.
func DeleteBucket(ctx context.Context, bScope *scope.ObjectStorageBucketScope) error {
	s3Client, err := createS3ClientWithAccessKey(ctx, bScope)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}
	if err := PurgeAllObjects(ctx, bScope.Bucket.Name, s3Client, true, true); err != nil {
		return fmt.Errorf("failed to purge all objects: %w", err)
	}
	bScope.Logger.Info("Purged all objects", "bucket", bScope.Bucket.Name)

	if err := bScope.LinodeClient.DeleteObjectStorageBucket(ctx, bScope.Bucket.Spec.Region, bScope.Bucket.Name); err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}
	bScope.Logger.Info("Deleted empty bucket", "bucket", bScope.Bucket.Name)

	return nil
}

// createS3ClientWithAccessKey creates a connection to s3 given k8s client and an access key reference.
func createS3ClientWithAccessKey(ctx context.Context, bScope *scope.ObjectStorageBucketScope) (*s3.Client, error) {
	if bScope.Bucket.Spec.AccessKeyRef == nil {
		return nil, fmt.Errorf("accessKeyRef is nil")
	}
	objSecret := &corev1.Secret{}
	if bScope.Bucket.Spec.AccessKeyRef.Namespace == "" {
		bScope.Bucket.Spec.AccessKeyRef.Namespace = bScope.Bucket.Namespace
	}
	if err := bScope.Client.Get(ctx, types.NamespacedName{Name: bScope.Bucket.Spec.AccessKeyRef.Name + "-obj-key", Namespace: bScope.Bucket.Spec.AccessKeyRef.Namespace}, objSecret); err != nil {
		return nil, fmt.Errorf("failed to get bucket secret: %w", err)
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				string(objSecret.Data["access"]),
				string(objSecret.Data["secret"]),
				""),
		),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsConfig, func(opts *s3.Options) {
		opts.BaseEndpoint = aws.String(string(objSecret.Data["endpoint"]))
		opts.DisableLogOutputChecksumValidationSkipped = true
	})

	return s3Client, nil
}
