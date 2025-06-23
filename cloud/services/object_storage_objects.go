package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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
			bne *s3types.NoSuchBucket
			kne *s3types.NoSuchKey
			nf  *s3types.NotFound
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

// PurgeAllObjects wipes out all versions and delete markers for versioned objects.
func PurgeAllObjects(
	ctx context.Context,
	bucket string,
	s3client *s3.Client,
	bypassRetention,
	ignoreNotFound bool,
) error {
	versioning, err := s3client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return err
	}

	if versioning.Status == s3types.BucketVersioningStatusEnabled {
		err = DeleteAllObjectVersionsAndDeleteMarkers(
			ctx,
			s3client,
			bucket,
			"",
			bypassRetention,
			ignoreNotFound,
		)
	} else {
		err = DeleteAllObjects(ctx, s3client, bucket, bypassRetention)
	}
	return err
}

// DeleteAllObjects sends delete requests for every object.
// Versioned objects will get a deletion marker instead of being fully purged.
func DeleteAllObjects(
	ctx context.Context,
	s3client *s3.Client,
	bucketName string,
	bypassRetention bool,
) error {
	objPaginator := s3.NewListObjectsV2Paginator(s3client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})

	var objectsToDelete []s3types.ObjectIdentifier
	for objPaginator.HasMorePages() {
		page, err := objPaginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, obj := range page.Contents {
			objectsToDelete = append(objectsToDelete, s3types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}

	if len(objectsToDelete) == 0 {
		return nil
	}

	_, err := s3client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket:                    aws.String(bucketName),
		Delete:                    &s3types.Delete{Objects: objectsToDelete},
		BypassGovernanceRetention: &bypassRetention,
	})

	return err
}

// DeleteAllObjectVersionsAndDeleteMarkers deletes all versions of a given object
func DeleteAllObjectVersionsAndDeleteMarkers(ctx context.Context, client *s3.Client, bucket, prefix string, bypassRetention, ignoreNotFound bool) error {
	paginator := s3.NewListObjectVersionsPaginator(client, &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	var objectsToDelete []s3types.ObjectIdentifier
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if page == nil {
			continue
		}
		if err != nil {
			if !IsObjNotFoundErr(err) || !ignoreNotFound {
				return err
			}
		}

		for _, version := range page.Versions {
			objectsToDelete = append(
				objectsToDelete,
				s3types.ObjectIdentifier{
					Key:       version.Key,
					VersionId: version.VersionId,
				},
			)
		}
		for _, marker := range page.DeleteMarkers {
			objectsToDelete = append(
				objectsToDelete,
				s3types.ObjectIdentifier{
					Key:       marker.Key,
					VersionId: marker.VersionId,
				},
			)
		}
	}

	if len(objectsToDelete) == 0 {
		return nil
	}

	_, err := client.DeleteObjects(
		ctx,
		&s3.DeleteObjectsInput{
			Bucket:                    aws.String(bucket),
			Delete:                    &s3types.Delete{Objects: objectsToDelete},
			BypassGovernanceRetention: &bypassRetention,
		},
	)
	if err != nil {
		if !IsObjNotFoundErr(err) || !ignoreNotFound {
			return err
		}
	}
	return nil
}

// IsObjNotFoundErr checks if the error is a NotFound or Forbidden error from the S3 API.
func IsObjNotFoundErr(err error) bool {
	var apiErr smithy.APIError
	// Error code is 'Forbidden' when the bucket has been removed
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "Forbidden"
	}
	return false
}
