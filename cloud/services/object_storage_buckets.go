package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/linode/linodego"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

func EnsureObjectStorageBucket(ctx context.Context, bs *scope.ObjectStorageBucketScope) (*linodego.ObjectStorageBucket, error) {
	filter := map[string]string{
		"label": bs.Object.Name,
	}

	rawFilter, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal list buckets filter: %w", err)
	}

	var buckets []linodego.ObjectStorageBucket
	if buckets, err = bs.LinodeClient.ListObjectStorageBucketsInCluster(
		ctx,
		linodego.NewListOptions(1, string(rawFilter)),
		bs.Object.Spec.Cluster,
	); err != nil {
		bs.Logger.Error(err, "Failed to list buckets; unable to provision/confirm")

		return nil, fmt.Errorf("failed to list buckets in cluster %s: %w", bs.Object.Spec.Cluster, err)
	}
	if len(buckets) == 1 {
		bs.Logger.Info("Bucket exists")

		return &buckets[0], nil
	}

	opts := linodego.ObjectStorageBucketCreateOptions{
		Cluster: bs.Object.Spec.Cluster,
		Label:   bs.Object.Name,
		ACL:     linodego.ACLPrivate,
	}

	var bucket *linodego.ObjectStorageBucket
	if bucket, err = bs.LinodeClient.CreateObjectStorageBucket(ctx, opts); err != nil {
		bs.Logger.Error(err, "Failed to create bucket")

		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	bs.Logger.Info("Created bucket")

	return bucket, nil
}

func CreateOrRotateObjectStorageKeys(ctx context.Context, bs *scope.ObjectStorageBucketScope) ([scope.AccessKeySecretLength]linodego.ObjectStorageKey, error) {
	var newKeys [scope.AccessKeySecretLength]linodego.ObjectStorageKey

	for i, permission := range []struct {
		name   string
		suffix string
	}{
		{"read_write", "rw"},
		{"read_only", "ro"},
	} {
		keyLabel := fmt.Sprintf("%s-%s", bs.Object.Name, permission.suffix)
		key, err := createObjectStorageKey(ctx, bs, keyLabel, permission.name)
		if err != nil {
			return newKeys, err
		}

		newKeys[i] = *key
	}

	// If key revocation fails here, just log the errors since new keys have been created
	if bs.Object.Status.LastKeyGeneration != nil && bs.ShouldRotateKeys() {
		secret, err := bs.GetSecret(ctx)
		if err != nil {
			bs.Logger.Error(err, "Failed to read secret with access keys to revoke; keys must be manually revoked")
		}

		if err := RevokeObjectStorageKeys(ctx, bs, secret); err != nil {
			bs.Logger.Error(err, "Failed to revoke access keys; keys must be manually revoked")
		}
	}

	return newKeys, nil
}

func createObjectStorageKey(ctx context.Context, bs *scope.ObjectStorageBucketScope, label, permission string) (*linodego.ObjectStorageKey, error) {
	opts := linodego.ObjectStorageKeyCreateOptions{
		Label: label,
		BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
			{
				BucketName:  bs.Object.Name,
				Cluster:     bs.Object.Spec.Cluster,
				Permissions: permission,
			},
		},
	}

	key, err := bs.LinodeClient.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		bs.Logger.Error(err, "Failed to create access key", "label", label)

		return nil, fmt.Errorf("failed to create access key: %w", err)
	}

	bs.Logger.Info("Created access key", "id", key.ID)

	return key, nil
}

func RevokeObjectStorageKeys(ctx context.Context, bs *scope.ObjectStorageBucketScope, secret *corev1.Secret) error {
	if secret == nil {
		return errors.New("unable to read access keys from nil secret")
	}

	keyIDs, err := bs.GetAccessKeysFromSecret(ctx, secret)
	if err != nil {
		bs.Logger.Error(err, "Failed to read secret with access keys to revoke; must be manually revoked")

		return fmt.Errorf("failed to read secret %s with access keys: %w", secret.Name, err)
	}

	var errs []error
	for _, keyID := range keyIDs {
		if err := revokeObjectStorageKey(ctx, bs, keyID); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func revokeObjectStorageKey(ctx context.Context, bs *scope.ObjectStorageBucketScope, keyID int) error {
	if err := bs.LinodeClient.DeleteObjectStorageKey(ctx, keyID); err != nil {
		linodeErr := &linodego.Error{}
		if errors.As(err, linodeErr) && linodeErr.StatusCode() != http.StatusNotFound {
			bs.Logger.Error(err, "Failed to revoke access key", "id", keyID)

			return fmt.Errorf("failed to revoke access key: %w", err)
		}
	}

	bs.Logger.Info("Revoked access key", "id", keyID)

	return nil
}
