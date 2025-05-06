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

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func validateObjectScopeParams(mscope *scope.MachineScope) error {
	if mscope == nil {
		return errors.New("nil machine scope")
	}

	if mscope.S3Client == nil {
		return errors.New("nil S3 client")
	}

	if mscope.LinodeCluster.Spec.ObjectStore == nil {
		return errors.New("nil cluster object store")
	}

	return nil
}

func CreateObject(ctx context.Context, mscope *scope.MachineScope, data []byte) (string, error) {
	if err := validateObjectScopeParams(mscope); err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("empty data")
	}

	bucket, err := mscope.GetBucketName(ctx)
	if err != nil {
		return "", err
	}
	if bucket == "" {
		return "", errors.New("missing bucket name")
	}

	// Key by UUID for shared buckets.
	key := string(mscope.LinodeMachine.UID)

	if _, err := mscope.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   s3manager.ReadSeekCloser(bytes.NewReader(data)),
	}); err != nil {
		return "", fmt.Errorf("put object: %w", err)
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
		return "", fmt.Errorf("generate presigned url: %w", err)
	}

	return req.URL, nil
}

func DeleteObject(ctx context.Context, mscope *scope.MachineScope) error {
	if err := validateObjectScopeParams(mscope); err != nil {
		return err
	}

	bucket, err := mscope.GetBucketName(ctx)
	if err != nil {
		return err
	}
	if bucket == "" {
		return errors.New("missing bucket name")
	}

	// Key by UUID for shared buckets.
	key := string(mscope.LinodeMachine.UID)

	_, err = mscope.S3Client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		var (
			ae  smithy.APIError
			bne *types.NoSuchBucket
			kne *types.NoSuchKey
			nf  *types.NotFound
		)
		switch {
		// In the case that the IAM policy does not have sufficient permissions to get the object, we will attempt to
		// delete it anyway for backwards compatibility reasons.
		case errors.As(err, &ae) && ae.ErrorCode() == "Forbidden":
			break
		// Specified bucket does not exist.
		case errors.As(err, &bne):
			return nil
		// Specified key does not exist.
		case errors.As(err, &kne):
			return nil
		// Object not found.
		case errors.As(err, &nf):
			return nil
		default:
			return fmt.Errorf("delete object: %w", err)
		}
	}

	if _, err = mscope.S3Client.DeleteObject(ctx,
		&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}
