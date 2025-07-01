package services

import (
	"context"
	"errors"
	"testing"

	awssigner "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestCreateObject(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockK8sClient{}, mock.MockS3Client{}, mock.MockS3PresignClient{}).Run(
		OneOf(
			Path(
				Result("nil machine scope", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, nil, []byte("fake"))
					assert.ErrorContains(t, err, "nil machine scope")
				}),
				Result("nil s3 client", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}}},
						[]byte("fake"))
					assert.ErrorContains(t, err, "nil machine scope")
				}),
				Result("nil cluster object store", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster:   &infrav1alpha2.LinodeCluster{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "nil cluster object store")
				}),
				Result("no data", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
					}, nil)
					assert.ErrorContains(t, err, "empty data")
				}),
			),
			Path(
				Call("fail to get bucket name", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("get credentials ref"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "get bucket name")
				}),
				Call("empty bucket name", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket":   nil,
							"endpoint": []byte("fake"),
							"access":   []byte("fake"),
							"secret":   []byte("fake"),
						}}
						*obj = secret
						return nil
					})
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "missing bucket name")
				}),
			),
		),
		OneOf(
			Path(
				Call("fail to put object", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket":   []byte("fake"),
							"endpoint": []byte("fake"),
							"access":   []byte("fake"),
							"secret":   []byte("fake"),
						}}
						*obj = secret
						return nil
					})
					mck.S3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("fail"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							ObjectMeta: metav1.ObjectMeta{Namespace: "fake"},
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "put object")
				}),
			),
			Path(
				Call("fail to generate presigned url", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket":   []byte("fake"),
							"endpoint": []byte("fake"),
							"access":   []byte("fake"),
							"secret":   []byte("fake"),
						}}
						*obj = secret
						return nil
					})
					mck.S3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
					mck.S3PresignClient.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("fail"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							ObjectMeta: metav1.ObjectMeta{Namespace: "fake"},
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "generate presigned url")
				}),
			),
			Path(
				Call("create object", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket":   []byte("fake"),
							"endpoint": []byte("fake"),
							"access":   []byte("fake"),
							"secret":   []byte("fake"),
						}}
						*obj = secret
						return nil
					})
					mck.S3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
					mck.S3PresignClient.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&awssigner.PresignedHTTPRequest{URL: "https://example.com"}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					url, err := CreateObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					require.NoError(t, err)
					assert.Equal(t, "https://example.com", url)
				}),
			),
		),
	)
}

func TestDeleteObject(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockK8sClient{}, mock.MockS3Client{}).Run(
		OneOf(
			Path(
				Result("nil machine scope", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, nil)
					assert.Error(t, err)
				}),
				Result("nil s3 client", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}}})
					assert.Error(t, err)
				}),
				Result("nil cluster object store", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						LinodeCluster: &infrav1alpha2.LinodeCluster{},
						S3Client:      &mock.MockS3Client{}})
					assert.ErrorContains(t, err, "nil cluster object store")
				}),
				Path(
					Call("empty bucket name", func(ctx context.Context, mck Mock) {
						mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("get credentials ref"))
					}),
					Result("error", func(ctx context.Context, mck Mock) {
						err := DeleteObject(ctx, &scope.MachineScope{
							Client: mck.K8sClient,
							LinodeCluster: &infrav1alpha2.LinodeCluster{
								Spec: infrav1alpha2.LinodeClusterSpec{
									ObjectStore: &infrav1alpha2.ObjectStore{
										CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						})
						assert.ErrorContains(t, err, "empty data")
					}),
				),
			),
			Path(
				Call("fail to get bucket name", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("get credentials ref"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
					})
					assert.ErrorContains(t, err, "get bucket name")
				}),
				Call("empty bucket name", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket":   nil,
							"endpoint": []byte("fake"),
							"access":   []byte("fake"),
							"secret":   []byte("fake"),
						}}
						*obj = secret
						return nil
					})
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
					})
					assert.ErrorContains(t, err, "missing bucket name")
				}),
			),
		),
		OneOf(
			Path(
				Call("fail to head object", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket":   []byte("fake"),
							"endpoint": []byte("fake"),
							"access":   []byte("fake"),
							"secret":   []byte("fake"),
						}}
						*obj = secret
						return nil
					})
					mck.S3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("fail"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							ObjectMeta: metav1.ObjectMeta{Namespace: "fake"},
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					assert.Error(t, err)
				}),
			),
			Path(
				Call("fail to delete object", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						secret := corev1.Secret{Data: map[string][]byte{
							"bucket":   []byte("fake"),
							"endpoint": []byte("fake"),
							"access":   []byte("fake"),
							"secret":   []byte("fake"),
						}}
						*obj = secret
						return nil
					})
					mck.S3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.HeadObjectOutput{}, nil)
					mck.S3Client.EXPECT().DeleteObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("fail"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							ObjectMeta: metav1.ObjectMeta{Namespace: "fake"},
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					assert.Error(t, err)
				}),
			),
			Path(
				OneOf(
					Path(Call("delete object (no such key)", func(ctx context.Context, mck Mock) {
						mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
							secret := corev1.Secret{Data: map[string][]byte{
								"bucket":   []byte("fake"),
								"endpoint": []byte("fake"),
								"access":   []byte("fake"),
								"secret":   []byte("fake"),
							}}
							*obj = secret
							return nil
						})
						mck.S3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &types.NoSuchKey{})
					})),
					Path(Call("delete object (no such bucket)", func(ctx context.Context, mck Mock) {
						mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
							secret := corev1.Secret{Data: map[string][]byte{
								"bucket":   []byte("fake"),
								"endpoint": []byte("fake"),
								"access":   []byte("fake"),
								"secret":   []byte("fake"),
							}}
							*obj = secret
							return nil
						})
						mck.S3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &types.NoSuchBucket{})
					})),
					Path(Call("delete object (not found)", func(ctx context.Context, mck Mock) {
						mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
							secret := corev1.Secret{Data: map[string][]byte{
								"bucket":   []byte("fake"),
								"endpoint": []byte("fake"),
								"access":   []byte("fake"),
								"secret":   []byte("fake"),
							}}
							*obj = secret
							return nil
						})
						mck.S3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &types.NotFound{})
					})),
					Path(Call("delete object", func(ctx context.Context, mck Mock) {
						mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
							secret := corev1.Secret{Data: map[string][]byte{
								"bucket":   []byte("fake"),
								"endpoint": []byte("fake"),
								"access":   []byte("fake"),
								"secret":   []byte("fake"),
							}}
							*obj = secret
							return nil
						})
						mck.S3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.HeadObjectOutput{}, nil)
						mck.S3Client.EXPECT().DeleteObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectOutput{}, nil)
					})),
				),
				Result("success", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client:          mck.K8sClient,
						S3Client:        mck.S3Client,
						S3PresignClient: mck.S3PresignClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							ObjectMeta: metav1.ObjectMeta{Namespace: "fake"},
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					assert.NoError(t, err)
				}),
			),
		),
	)
}

func TestDeleteAllObjects(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockK8sClient{}, mock.MockS3Client{}).Run(
		OneOf(
			Path(
				Call("fail to list objects", func(ctx context.Context, mck Mock) {
					mck.S3Client.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("fail"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteAllObjects(ctx, mck.S3Client, "test", true)
					assert.Error(t, err)
				}),
			),
			Path(
				Call("no objects", func(ctx context.Context, mck Mock) {
					mck.S3Client.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					err := DeleteAllObjects(ctx, mck.S3Client, "test", true)
					assert.NoError(t, err)
				}),
			),
		),
	)
}

func TestDeleteAllObjectVersionsAndDeleteMarkers(t *testing.T) {
	t.Parallel()

	NewSuite(t, mock.MockK8sClient{}, mock.MockS3Client{}).Run(
		OneOf(
			Path(
				Call("fail to list object versions", func(ctx context.Context, mck Mock) {
					mck.S3Client.EXPECT().ListObjectVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("fail"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteAllObjectVersionsAndDeleteMarkers(ctx, mck.S3Client, "test", "", true, false)
					assert.Error(t, err)
				}),
			),
			Path(
				Call("no objects", func(ctx context.Context, mck Mock) {
					mck.S3Client.EXPECT().ListObjectVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.ListObjectVersionsOutput{}, nil)
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteAllObjectVersionsAndDeleteMarkers(ctx, mck.S3Client, "test", "", true, false)
					assert.NoError(t, err)
				}),
			),
			Path(
				Call("with an object", func(ctx context.Context, mck Mock) {
					mck.S3Client.EXPECT().ListObjectVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.ListObjectVersionsOutput{
						Name: ptr.To("test"),
						Versions: []types.ObjectVersion{
							{
								IsLatest:  ptr.To(true),
								Key:       ptr.To("test"),
								VersionId: ptr.To("version2"),
							},
						},
						ResultMetadata: middleware.Metadata{},
					}, nil)
					mck.S3Client.EXPECT().DeleteObjects(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectsOutput{}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					err := DeleteAllObjectVersionsAndDeleteMarkers(ctx, mck.S3Client, "test", "", true, false)
					assert.NoError(t, err)
				}),
			),
			Path(
				Call("with versions and delete markers", func(ctx context.Context, mck Mock) {
					mck.S3Client.EXPECT().ListObjectVersions(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.ListObjectVersionsOutput{
						Name: ptr.To("test"),
						Versions: []types.ObjectVersion{
							{
								IsLatest:  ptr.To(false),
								Key:       ptr.To("test"),
								VersionId: ptr.To("version1"),
							},
							{
								IsLatest:  ptr.To(true),
								Key:       ptr.To("test"),
								VersionId: ptr.To("version2"),
							},
						},
						ResultMetadata: middleware.Metadata{},
						DeleteMarkers: []types.DeleteMarkerEntry{
							{
								IsLatest:  ptr.To(false),
								Key:       ptr.To("test"),
								VersionId: ptr.To("version1"),
							},
							{
								IsLatest:  ptr.To(true),
								Key:       ptr.To("test"),
								VersionId: ptr.To("version2"),
							},
						},
					}, nil)
					mck.S3Client.EXPECT().DeleteObjects(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectsOutput{}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					err := DeleteAllObjectVersionsAndDeleteMarkers(ctx, mck.S3Client, "test", "", true, false)
					assert.NoError(t, err)
				}),
			),
		),
	)
}
