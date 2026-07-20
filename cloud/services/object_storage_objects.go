package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

const objectStoreAttemptTimeout = 30 * time.Second

func validateObjectScopeParams(mscope *scope.MachineScope) error {
	if mscope == nil {
		return errors.New("nil machine scope")
	}

	if mscope.Client == nil {
		return errors.New("nil Kubernetes client")
	}
	if mscope.S3Clients == nil {
		return errors.New("nil S3 client builder")
	}
	if mscope.LinodeCluster == nil || mscope.LinodeCluster.Spec.ObjectStore == nil {
		return errors.New("nil cluster object store")
	}
	if mscope.LinodeMachine == nil {
		return errors.New("nil LinodeMachine")
	}

	return nil
}

// objectStoreRefs returns the ordered credentials references to try: the primary,
// then the optional secondary, each defaulted to the cluster namespace when unset.
func objectStoreRefs(mscope *scope.MachineScope) []corev1.SecretReference {
	objectStore := mscope.LinodeCluster.Spec.ObjectStore
	refs := []corev1.SecretReference{objectStore.CredentialsRef}
	if objectStore.SecondaryCredentialsRef != nil {
		refs = append(refs, *objectStore.SecondaryCredentialsRef)
	}
	for i := range refs {
		if refs[i].Namespace == "" {
			refs[i].Namespace = mscope.LinodeCluster.Namespace
		}
	}
	return refs
}

func CreateObject(ctx context.Context, mscope *scope.MachineScope, data []byte) (string, error) {
	if err := validateObjectScopeParams(mscope); err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("empty data")
	}

	// Key by UUID for shared buckets.
	key := string(mscope.LinodeMachine.UID)
	refs := objectStoreRefs(mscope)
	attemptErrs := make([]error, 0, len(refs))
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("object store upload cancelled: %w", err)
		}

		url, err := createObjectWithCredentials(ctx, mscope, ref, data, key)
		if err == nil {
			return url, nil
		}
		qualifiedErr := fmt.Errorf("object store credentials %s/%s: %w", ref.Namespace, ref.Name, err)
		attemptErrs = append(attemptErrs, qualifiedErr)
		log.FromContext(ctx).Error(err, "Object Store attempt failed", "secretReference", ref.Namespace+"/"+ref.Name)
	}

	return "", fmt.Errorf("all Object Store attempts failed: %w", errors.Join(attemptErrs...))
}

func createObjectWithCredentials(
	ctx context.Context,
	mscope *scope.MachineScope,
	ref corev1.SecretReference,
	data []byte,
	key string,
) (string, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, objectStoreAttemptTimeout)
	defer cancel()

	credentials, err := mscope.GetObjectStoreCredentials(attemptCtx, ref)
	if err != nil {
		return "", fmt.Errorf("load credentials: %w", err)
	}

	s3Client, presignClient, err := mscope.S3Clients(attemptCtx, credentials)
	if err != nil {
		return "", fmt.Errorf("create clients: %w", err)
	}
	if s3Client == nil {
		return "", errors.New("create clients: S3 client builder returned nil S3 client")
	}
	if presignClient == nil {
		return "", errors.New("create clients: S3 client builder returned nil presign client")
	}

	bucket := string(credentials.Data["bucket"])
	if _, err = s3Client.PutObject(attemptCtx, &s3.PutObjectInput{
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
	req, err := presignClient.PresignGetObject(attemptCtx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, opts...)
	switch {
	case err != nil:
		err = fmt.Errorf("generate presigned URL: %w", err)
	case req == nil || req.URL == "":
		err = errors.New("empty presigned URL")
	default:
		return req.URL, nil
	}
	return "", err
}

func DeleteObject(ctx context.Context, mscope *scope.MachineScope) error {
	if err := validateObjectScopeParams(mscope); err != nil {
		return err
	}

	// Key by UUID for shared buckets.
	key := string(mscope.LinodeMachine.UID)
	refs := objectStoreRefs(mscope)
	attemptErrs := make([]error, 0, len(refs))
	for _, ref := range refs {
		if err := deleteObjectWithCredentials(ctx, mscope, ref, key); err != nil {
			attemptErrs = append(attemptErrs, fmt.Errorf("object store credentials %s/%s: %w", ref.Namespace, ref.Name, err))
			log.FromContext(ctx).Error(err, "Object Store cleanup attempt failed", "secretReference", ref.Namespace+"/"+ref.Name)
		}
	}
	return errors.Join(attemptErrs...)
}

func deleteObjectWithCredentials(
	ctx context.Context,
	mscope *scope.MachineScope,
	ref corev1.SecretReference,
	key string,
) error {
	attemptCtx, cancel := context.WithTimeout(ctx, objectStoreAttemptTimeout)
	defer cancel()

	credentials, err := mscope.GetObjectStoreCredentials(attemptCtx, ref)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	s3Client, _, err := mscope.S3Clients(attemptCtx, credentials)
	if err == nil && s3Client == nil {
		err = errors.New("S3 client builder returned nil client")
	}
	if err != nil {
		return fmt.Errorf("create clients: %w", err)
	}

	return deleteObject(attemptCtx, s3Client, string(credentials.Data["bucket"]), key)
}

func deleteObject(ctx context.Context, s3Client clients.S3Client, bucket, key string) error {
	_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiErr smithy.APIError
		switch {
		// In the case that the IAM policy does not have sufficient permissions to get the object, we will attempt to
		// delete it anyway for backwards compatibility reasons.
		case errors.As(err, &apiErr) && apiErr.ErrorCode() == "Forbidden":
			break
		case isObjectMissingError(err):
			return nil
		default:
			return fmt.Errorf("delete object: %w", err)
		}
	}

	if _, err = s3Client.DeleteObject(ctx,
		&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}); err != nil {
		if isObjectMissingError(err) {
			return nil
		}
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func isObjectMissingError(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.ErrorCode() {
	case "NoSuchBucket", "NoSuchKey", "NotFound":
		return true
	}
	return false
}

// PurgeAllObjects wipes out all versions and delete markers for versioned objects.
func PurgeAllObjects(
	ctx context.Context,
	bucket string,
	s3client clients.S3Client,
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
	s3client clients.S3Client,
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
func DeleteAllObjectVersionsAndDeleteMarkers(ctx context.Context, client clients.S3Client, bucket, prefix string, bypassRetention, ignoreNotFound bool) error {
	paginator := s3.NewListObjectVersionsPaginator(client, &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	var objectsToDelete []s3types.ObjectIdentifier
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			if !IsObjNotFoundErr(err) || !ignoreNotFound {
				return err
			}
		}
		if page == nil {
			continue
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
