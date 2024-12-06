package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func CreateObject(ctx context.Context, mscope *scope.MachineScope, data []byte, logger logr.Logger) (string, error) {
	logger.Info("Create Object Storage object")

	if mscope == nil {
		return "", errors.New("machine scope can't be nil")
	}

	if mscope.S3Client == nil {
		return "", errors.New("nil S3 client in machine scope")
	}

	if mscope.LinodeCluster.Spec.ObjectStore == nil {
		return "", errors.New("nil cluster object store")
	}

	if len(data) == 0 {
		return "", errors.New("got empty data")
	}

	bucket, err := mscope.GetBucketName(ctx)
	if err != nil || bucket == "" {
		return "", errors.New("no bucket name")
	}
	// Key by UUID for shared buckets.
	key := string(mscope.LinodeMachine.ObjectMeta.UID)

	if _, err := mscope.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   s3manager.ReadSeekCloser(bytes.NewReader(data)),
	}); err != nil {
		return "", fmt.Errorf("put object (%s) in bucket (%s)", key, bucket)
	}

	var opts []func(*s3.PresignOptions)
	if mscope.LinodeCluster.Spec.ObjectStore.PresignedURLDuration != nil {
		opts = append(opts, func(opts *s3.PresignOptions) {
			opts.Expires = mscope.LinodeCluster.Spec.ObjectStore.PresignedURLDuration.Duration
		})
	}

	req, err := mscope.S3PresignClient.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
		opts...)
	if err != nil {
		return "", fmt.Errorf("get presigned url: %w", err)
	}

	return req.URL, nil
}

func DeleteObject(ctx context.Context, mscope *scope.MachineScope) error {
	if mscope == nil {
		return errors.New("machine scope can't be nil")
	}

	if mscope.S3Client == nil {
		return errors.New("nil S3 client in machine scope")
	}

	if mscope.LinodeCluster.Spec.ObjectStore == nil {
		return errors.New("nil cluster object store")
	}

	bucket, err := mscope.GetBucketName(ctx)
	if err != nil || bucket == "" {
		return errors.New("got empty bucket name")
	}

	// Key by UUID for shared buckets.
	key := string(mscope.LinodeMachine.ObjectMeta.UID)

	// TODO: Just ignore errors in the caller?
	_, err = mscope.S3Client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		var (
			ae  smithy.APIError
			kne *types.NoSuchKey
			bne *types.NoSuchBucket
		)
		switch {
		// TODO: Check if this edge-case is also present with Linode Object Storage.
		// In the case that the IAM policy does not have sufficient
		// permissions to get the object, we will attempt to delete it
		// anyway for backwards compatibility reasons.
		case errors.As(err, &ae) && ae.ErrorCode() == "Forbidden":
			break
		// Object already deleted.
		case errors.As(err, &bne):
			return nil
		// Bucket does not exist.
		case errors.As(err, &kne):
			return nil
		default:
			return fmt.Errorf("delete S3 object: %w", err)
		}
	}

	if _, err = mscope.S3Client.DeleteObject(ctx,
		&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}); err != nil {
		return fmt.Errorf("delete S3 object: %w", err)
	}

	return nil
}
