package services

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestRotateObjectStorageKey(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(Call("create key", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().CreateObjectStorageKey(ctx, gomock.Any()).Return(&linodego.ObjectStorageKey{ID: 1, Label: "key"}, nil)
			})),
			Path(
				Call("create key fail", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().CreateObjectStorageKey(ctx, gomock.Any()).Return(nil, errors.New("unable to create"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := RotateObjectStorageKey(ctx, &scope.ObjectStorageKeyScope{
						LinodeClient: mck.LinodeClient,
						Key: &infrav1alpha2.LinodeObjectStorageKey{
							ObjectMeta: metav1.ObjectMeta{Name: "key"},
							Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
								BucketAccess: []infrav1alpha2.BucketAccessRef{
									{
										BucketName:  "mybucket",
										Region:      "us-ord",
										Permissions: "read_write",
									},
								},
							},
						},
					})
					assert.ErrorContains(t, err, "unable to create")
				}),
			),
		),
		OneOf(
			Path(Result("rotate not needed", func(ctx context.Context, mck Mock) {
				key, err := RotateObjectStorageKey(ctx, &scope.ObjectStorageKeyScope{
					LinodeClient: mck.LinodeClient,
					Key: &infrav1alpha2.LinodeObjectStorageKey{
						ObjectMeta: metav1.ObjectMeta{Name: "key"},
					},
				})
				require.NoError(t, err)
				assert.Equal(t, 1, key.ID)
				assert.Equal(t, "key", key.Label)
			})),
			Path(
				Call("delete old key", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().DeleteObjectStorageKey(ctx, 0).Return(nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					key, err := RotateObjectStorageKey(ctx, &scope.ObjectStorageKeyScope{
						LinodeClient: mck.LinodeClient,
						Key: &infrav1alpha2.LinodeObjectStorageKey{
							ObjectMeta: metav1.ObjectMeta{Name: "key"},
							Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
								KeyGeneration: ptr.To(1),
							},
							Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
								LastKeyGeneration: ptr.To(0),
								AccessKeyRef:      ptr.To(0),
							},
						},
					})
					require.NoError(t, err)
					assert.Equal(t, 1, key.ID)
				}),
			),
			Path(
				Call("delete old key fail", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().DeleteObjectStorageKey(ctx, 0).Return(errors.New("fail"))
				}),
				Result("error logged", func(ctx context.Context, mck Mock) {
					logs := &bytes.Buffer{}

					key, err := RotateObjectStorageKey(ctx, &scope.ObjectStorageKeyScope{
						LinodeClient: mck.LinodeClient,
						Logger:       zap.New(zap.WriteTo(logs)),
						Key: &infrav1alpha2.LinodeObjectStorageKey{
							ObjectMeta: metav1.ObjectMeta{Name: "key"},
							Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
								KeyGeneration: ptr.To(1),
							},
							Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
								LastKeyGeneration: ptr.To(0),
								AccessKeyRef:      ptr.To(0),
							},
						},
					})
					require.NoError(t, err)
					assert.Equal(t, 1, key.ID)

					assert.Contains(t, logs.String(), "Failed to revoke access key; key must be manually revoked")
				}),
			),
		),
	)
}

func TestGetObjectStorageKey(t *testing.T) {
	t.Parallel()

	key := infrav1alpha2.LinodeObjectStorageKey{
		Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
			AccessKeyRef: ptr.To(0),
		},
	}

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("get key", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageKey(ctx, gomock.Any()).Return(&linodego.ObjectStorageKey{ID: 0, Label: "key"}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					resp, err := GetObjectStorageKey(ctx, &scope.ObjectStorageKeyScope{
						LinodeClient: mck.LinodeClient,
						Key:          &key,
					})
					require.NoError(t, err)
					assert.Equal(t, 0, resp.ID)
					assert.Equal(t, "key", resp.Label)
				}),
			),
			Path(
				Call("get key fail", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageKey(ctx, gomock.Any()).Return(nil, errors.New("fail"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := GetObjectStorageKey(ctx, &scope.ObjectStorageKeyScope{
						LinodeClient: mck.LinodeClient,
						Key:          &key,
					})
					assert.ErrorContains(t, err, "fail")
				}),
			),
		),
	)
}
