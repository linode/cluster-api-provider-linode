package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssigner "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
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
				Result("nil Kubernetes client", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}}},
						[]byte("fake"))
					assert.ErrorContains(t, err, "nil Kubernetes client")
				}),
				Result("nil S3 client builder", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						Client: mck.K8sClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "nil S3 client builder")
				}),
				Result("nil cluster object store", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "nil cluster object store")
				}),
				Result("no data", func(ctx context.Context, mck Mock) {
					_, err := CreateObject(ctx, &scope.MachineScope{
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "get credentials secret")
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "empty or missing bucket")
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
							ObjectMeta: metav1.ObjectMeta{Namespace: "fake"},
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					}, []byte("fake"))
					assert.ErrorContains(t, err, "generate presigned URL")
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
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

func testS3Factory(s3Client clients.S3Client, presignClient clients.S3PresignClient) func(context.Context, *corev1.Secret) (clients.S3Client, clients.S3PresignClient, error) {
	return func(context.Context, *corev1.Secret) (clients.S3Client, clients.S3PresignClient, error) {
		return s3Client, presignClient, nil
	}
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
				Result("nil Kubernetes client", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}}})
					assert.Error(t, err)
				}),
				Result("nil S3 client builder", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client: mck.K8sClient,
						LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					assert.ErrorContains(t, err, "nil S3 client builder")
				}),
				Result("nil cluster object store", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client:        mck.K8sClient,
						S3Clients:     testS3Factory(&mock.MockS3Client{}, nil),
						LinodeCluster: &infrav1alpha2.LinodeCluster{},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					assert.ErrorContains(t, err, "nil cluster object store")
				}),
				Path(
					Call("empty bucket name", func(ctx context.Context, mck Mock) {
						mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("get credentials ref"))
					}),
					Result("error", func(ctx context.Context, mck Mock) {
						err := DeleteObject(ctx, &scope.MachineScope{
							Client:    mck.K8sClient,
							S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient),
							LinodeCluster: &infrav1alpha2.LinodeCluster{
								Spec: infrav1alpha2.LinodeClusterSpec{
									ObjectStore: &infrav1alpha2.ObjectStore{
										CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
							LinodeMachine: &infrav1alpha2.LinodeMachine{},
						})
						assert.Error(t, err)
					}),
				),
			),
			Path(
				Call("fail to get bucket name", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("get credentials ref"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					err := DeleteObject(ctx, &scope.MachineScope{
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					assert.ErrorContains(t, err, "get credentials secret")
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
							Spec: infrav1alpha2.LinodeClusterSpec{
								ObjectStore: &infrav1alpha2.ObjectStore{
									CredentialsRef: corev1.SecretReference{Name: "fake"}}}},
						LinodeMachine: &infrav1alpha2.LinodeMachine{},
					})
					assert.ErrorContains(t, err, "empty or missing bucket")
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
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
						Client: mck.K8sClient, S3Clients: testS3Factory(mck.S3Client, mck.S3PresignClient), LinodeCluster: &infrav1alpha2.LinodeCluster{
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

const firstBucket = "first-bucket"

// objectStoreSecret builds a credentials secret carrying the keys CAPL requires
// to reach a bucket.
func objectStoreSecret(name, namespace, bucket string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data: map[string][]byte{
			"bucket":   []byte(bucket),
			"endpoint": []byte("https://" + bucket + ".example.com"),
			"access":   []byte("access"),
			"secret":   []byte("secret"),
		},
	}
}

// stubSecretLookups wires a mock K8s client to serve the given secrets by name,
// returning a not-found error for anything else.
func stubSecretLookups(k8s *mock.MockK8sClient, secrets ...*corev1.Secret) {
	byRef := make(map[string]*corev1.Secret, len(secrets))
	for _, secret := range secrets {
		byRef[secret.Namespace+"/"+secret.Name] = secret
	}
	k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(_ context.Context, key client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
			secret, ok := byRef[key.Namespace+"/"+key.Name]
			if !ok {
				return fmt.Errorf("secret %q/%q not found", key.Namespace, key.Name)
			}
			*obj = *secret
			return nil
		})
}

type stubClients struct {
	s3      clients.S3Client
	presign clients.S3PresignClient
	err     error
}

// recordingS3Factory returns an S3ClientBuilder that serves clients keyed by
// bucket name, appending each requested bucket to recorder (when non-nil).
func recordingS3Factory(recorder *[]string, byBucket map[string]stubClients) scope.S3ClientBuilder {
	return func(_ context.Context, credentials *corev1.Secret) (clients.S3Client, clients.S3PresignClient, error) {
		bucket := string(credentials.Data["bucket"])
		if recorder != nil {
			*recorder = append(*recorder, bucket)
		}
		stub := byBucket[bucket]
		return stub.s3, stub.presign, stub.err
	}
}

func fallbackMachineScope(factory scope.S3ClientBuilder, k8s clients.K8sClient, objectStore *infrav1alpha2.ObjectStore) *scope.MachineScope {
	return &scope.MachineScope{
		Client:    k8s,
		S3Clients: factory,
		LinodeCluster: &infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: "cluster-ns"},
			Spec:       infrav1alpha2.LinodeClusterSpec{ObjectStore: objectStore},
		},
		LinodeMachine: &infrav1alpha2.LinodeMachine{ObjectMeta: metav1.ObjectMeta{UID: k8stypes.UID("machine-uid")}},
	}
}

func TestCreateObjectFallbackOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// configurePrimary and configureSecondary script the per-bucket clients.
		// A nil configureSecondary means the secondary must never be consulted.
		configurePrimary   func(s3Client *mock.MockS3Client, presign *mock.MockS3PresignClient)
		configureSecondary func(s3Client *mock.MockS3Client, presign *mock.MockS3PresignClient)
		wantURL            string
		wantCalls          []string
	}{
		{
			name: "primary success skips secondary",
			configurePrimary: func(s3Client *mock.MockS3Client, presign *mock.MockS3PresignClient) {
				s3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
						assert.Equal(t, firstBucket, aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &s3.PutObjectOutput{}, nil
					})
				presign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*awssigner.PresignedHTTPRequest, error) {
						assert.Equal(t, firstBucket, aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &awssigner.PresignedHTTPRequest{URL: "https://first.example.com"}, nil
					})
			},
			wantURL:   "https://first.example.com",
			wantCalls: []string{firstBucket},
		},
		{
			name: "primary put failure falls back to secondary",
			configurePrimary: func(s3Client *mock.MockS3Client, _ *mock.MockS3PresignClient) {
				s3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
						assert.Equal(t, firstBucket, aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, errors.New("endpoint unavailable")
					})
			},
			configureSecondary: func(s3Client *mock.MockS3Client, presign *mock.MockS3PresignClient) {
				s3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
						assert.Equal(t, "second-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &s3.PutObjectOutput{}, nil
					})
				presign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*awssigner.PresignedHTTPRequest, error) {
						assert.Equal(t, "second-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &awssigner.PresignedHTTPRequest{URL: "https://second.example.com"}, nil
					})
			},
			wantURL:   "https://second.example.com",
			wantCalls: []string{firstBucket, "second-bucket"},
		},
		{
			name: "primary presign error falls back to secondary",
			configurePrimary: func(s3Client *mock.MockS3Client, presign *mock.MockS3PresignClient) {
				s3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
						assert.Equal(t, firstBucket, aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &s3.PutObjectOutput{}, nil
					})
				presign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*awssigner.PresignedHTTPRequest, error) {
						assert.Equal(t, firstBucket, aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, errors.New("presign failed")
					})
			},
			configureSecondary: func(s3Client *mock.MockS3Client, presign *mock.MockS3PresignClient) {
				s3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
						assert.Equal(t, "second-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &s3.PutObjectOutput{}, nil
					})
				presign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*awssigner.PresignedHTTPRequest, error) {
						assert.Equal(t, "second-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &awssigner.PresignedHTTPRequest{URL: "https://second.example.com"}, nil
					})
			},
			wantURL:   "https://second.example.com",
			wantCalls: []string{firstBucket, "second-bucket"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			primary := mock.NewMockS3Client(ctrl)
			primaryPresign := mock.NewMockS3PresignClient(ctrl)
			test.configurePrimary(primary, primaryPresign)
			byBucket := map[string]stubClients{
				firstBucket: {s3: primary, presign: primaryPresign},
			}
			if test.configureSecondary != nil {
				secondary := mock.NewMockS3Client(ctrl)
				secondaryPresign := mock.NewMockS3PresignClient(ctrl)
				test.configureSecondary(secondary, secondaryPresign)
				byBucket["second-bucket"] = stubClients{s3: secondary, presign: secondaryPresign}
			}

			var calls []string
			factory := recordingS3Factory(&calls, byBucket)

			k8s := mock.NewMockK8sClient(ctrl)
			stubSecretLookups(k8s,
				objectStoreSecret("first", "cluster-ns", firstBucket),
				objectStoreSecret("second", "cluster-ns", "second-bucket"),
			)

			mscope := fallbackMachineScope(factory, k8s, &infrav1alpha2.ObjectStore{
				CredentialsRef:          corev1.SecretReference{Name: "first"},
				SecondaryCredentialsRef: &corev1.SecretReference{Name: "second"},
			})

			url, err := CreateObject(t.Context(), mscope, []byte("bootstrap"))
			require.NoError(t, err)
			assert.Equal(t, test.wantURL, url)
			assert.Equal(t, test.wantCalls, calls)
		})
	}
}

func TestCreateObjectPrimaryFailuresUseSecondary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		firstSecret *corev1.Secret
		firstStub   *stubClients // nil => the primary is expected to fail before the factory is consulted
		checkCtx    bool         // assert the per-attempt context carries a deadline
	}{
		{
			name: "secret lookup",
		},
		{
			name: "validation",
			firstSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: "cluster-ns"},
				Data:       map[string][]byte{"bucket": []byte(firstBucket)},
			},
		},
		{
			name:        "client construction",
			firstSecret: objectStoreSecret("first", "cluster-ns", firstBucket),
			firstStub:   &stubClients{err: errors.New("client construction failed")},
		},
		{
			name:        "attempt timeout",
			firstSecret: objectStoreSecret("first", "cluster-ns", firstBucket),
			firstStub:   &stubClients{err: context.DeadlineExceeded},
			checkCtx:    true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			second := mock.NewMockS3Client(ctrl)
			second.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
			secondPresign := mock.NewMockS3PresignClient(ctrl)
			secondPresign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&awssigner.PresignedHTTPRequest{URL: "https://success.example.com"}, nil)

			secondUsed := false
			factory := func(ctx context.Context, credentials *corev1.Secret) (clients.S3Client, clients.S3PresignClient, error) {
				if string(credentials.Data["bucket"]) == firstBucket {
					if test.checkCtx {
						if _, hasDeadline := ctx.Deadline(); !hasDeadline {
							return nil, nil, errors.New("attempt context has no deadline")
						}
					}
					if test.firstStub == nil {
						t.Fatal("primary factory should not have been reached")
					}
					return test.firstStub.s3, test.firstStub.presign, test.firstStub.err
				}
				secondUsed = true
				return second, secondPresign, nil
			}

			secrets := []*corev1.Secret{objectStoreSecret("second", "explicit-ns", "second-bucket")}
			if test.firstSecret != nil {
				secrets = append(secrets, test.firstSecret)
			}
			k8s := mock.NewMockK8sClient(ctrl)
			stubSecretLookups(k8s, secrets...)

			mscope := fallbackMachineScope(factory, k8s, &infrav1alpha2.ObjectStore{
				CredentialsRef:          corev1.SecretReference{Name: "first"},
				SecondaryCredentialsRef: &corev1.SecretReference{Name: "second", Namespace: "explicit-ns"},
			})

			url, err := CreateObject(t.Context(), mscope, []byte("bootstrap"))
			require.NoError(t, err)
			assert.Equal(t, "https://success.example.com", url)
			assert.True(t, secondUsed)
		})
	}
}

func TestCreateObjectPresignFailureFallsThroughWithoutCleanup(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	first := mock.NewMockS3Client(ctrl)
	first.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			assert.Equal(t, firstBucket, aws.ToString(input.Bucket))
			assert.Equal(t, "machine-uid", aws.ToString(input.Key))
			return &s3.PutObjectOutput{}, nil
		})
	firstPresign := mock.NewMockS3PresignClient(ctrl)
	firstPresign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*awssigner.PresignedHTTPRequest, error) {
			assert.Equal(t, firstBucket, aws.ToString(input.Bucket))
			assert.Equal(t, "machine-uid", aws.ToString(input.Key))
			return &awssigner.PresignedHTTPRequest{}, nil
		})

	second := mock.NewMockS3Client(ctrl)
	second.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			assert.Equal(t, "second-bucket", aws.ToString(input.Bucket))
			assert.Equal(t, "machine-uid", aws.ToString(input.Key))
			return &s3.PutObjectOutput{}, nil
		})
	secondPresign := mock.NewMockS3PresignClient(ctrl)
	secondPresign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*awssigner.PresignedHTTPRequest, error) {
			assert.Equal(t, "second-bucket", aws.ToString(input.Bucket))
			assert.Equal(t, "machine-uid", aws.ToString(input.Key))
			return &awssigner.PresignedHTTPRequest{URL: "https://second.example.com"}, nil
		})

	factory := func(_ context.Context, credentials *corev1.Secret) (clients.S3Client, clients.S3PresignClient, error) {
		if string(credentials.Data["bucket"]) == firstBucket {
			return first, firstPresign, nil
		}
		return second, secondPresign, nil
	}

	k8s := mock.NewMockK8sClient(ctrl)
	stubSecretLookups(k8s,
		objectStoreSecret("first", "cluster-ns", firstBucket),
		objectStoreSecret("second", "cluster-ns", "second-bucket"),
	)

	mscope := fallbackMachineScope(factory, k8s, &infrav1alpha2.ObjectStore{
		CredentialsRef:          corev1.SecretReference{Name: "first"},
		SecondaryCredentialsRef: &corev1.SecretReference{Name: "second"},
	})

	url, err := CreateObject(t.Context(), mscope, []byte("bootstrap"))
	require.NoError(t, err)
	assert.Equal(t, "https://second.example.com", url)
}

func TestCreateObjectPresignedURLDurationAndJoinedErrors(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	duration := metav1.Duration{Duration: 24 * time.Hour}
	newClient := func(bucket string) stubClients {
		s3Client := mock.NewMockS3Client(ctrl)
		s3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
		presign := mock.NewMockS3PresignClient(ctrl)
		presign.EXPECT().PresignGetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*awssigner.PresignedHTTPRequest, error) {
				opts := s3.PresignOptions{}
				for _, opt := range optFns {
					opt(&opts)
				}
				assert.Equal(t, duration.Duration, opts.Expires)
				return nil, fmt.Errorf("presign %s failed", bucket)
			})
		return stubClients{s3: s3Client, presign: presign}
	}

	factory := recordingS3Factory(nil, map[string]stubClients{
		firstBucket:     newClient(firstBucket),
		"second-bucket": newClient("second-bucket"),
	})

	k8s := mock.NewMockK8sClient(ctrl)
	stubSecretLookups(k8s,
		objectStoreSecret("first", "cluster-ns", firstBucket),
		objectStoreSecret("second", "cluster-ns", "second-bucket"),
	)

	mscope := fallbackMachineScope(factory, k8s, &infrav1alpha2.ObjectStore{
		CredentialsRef:          corev1.SecretReference{Name: "first"},
		SecondaryCredentialsRef: &corev1.SecretReference{Name: "second"},
		PresignedURLDuration:    &duration,
	})

	_, err := CreateObject(t.Context(), mscope, []byte("bootstrap"))
	require.Error(t, err)
	require.ErrorContains(t, err, "cluster-ns/first")
	require.ErrorContains(t, err, "cluster-ns/second")
}

func TestCreateObjectParentCancellationStopsFallback(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(t.Context())
	factoryCalls := 0
	factory := func(context.Context, *corev1.Secret) (clients.S3Client, clients.S3PresignClient, error) {
		factoryCalls++
		cancel()
		return nil, nil, errors.New("first failed")
	}

	k8s := mock.NewMockK8sClient(ctrl)
	stubSecretLookups(k8s,
		objectStoreSecret("first", "cluster-ns", firstBucket),
		objectStoreSecret("second", "cluster-ns", "second-bucket"),
	)

	mscope := fallbackMachineScope(factory, k8s, &infrav1alpha2.ObjectStore{
		CredentialsRef:          corev1.SecretReference{Name: "first"},
		SecondaryCredentialsRef: &corev1.SecretReference{Name: "second"},
	})

	_, err := CreateObject(ctx, mscope, []byte("bootstrap"))
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, factoryCalls)
}

func TestDeleteObjectFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// configurePrimary and configureSecondary script the per-bucket clients.
		// A nil configureSecondary means the Object Store has no secondary reference.
		configurePrimary   func(*mock.MockS3Client)
		configureSecondary func(*mock.MockS3Client)
		wantErrContains    []string
		wantAttempted      []string
	}{
		{
			name: "both references fail and errors are joined",
			configurePrimary: func(s3Client *mock.MockS3Client) {
				s3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
						assert.Equal(t, "primary-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, errors.New("network unavailable")
					})
			},
			configureSecondary: func(s3Client *mock.MockS3Client) {
				s3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
						assert.Equal(t, "secondary-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, &smithy.GenericAPIError{Code: "Forbidden", Message: "forbidden"}
					})
				s3Client.EXPECT().DeleteObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
						assert.Equal(t, "secondary-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, errors.New("service unavailable")
					})
			},
			wantErrContains: []string{"network unavailable", "service unavailable"},
			wantAttempted:   []string{"primary-bucket", "secondary-bucket"},
		},
		{
			name: "primary only tolerates a missing object",
			configurePrimary: func(s3Client *mock.MockS3Client) {
				s3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
						assert.Equal(t, "primary-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return &s3.HeadObjectOutput{}, nil
					})
				s3Client.EXPECT().DeleteObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
						assert.Equal(t, "primary-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, &types.NotFound{}
					})
			},
			wantAttempted: []string{"primary-bucket"},
		},
		{
			name: "both references tolerate missing objects",
			configurePrimary: func(s3Client *mock.MockS3Client) {
				s3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
						assert.Equal(t, "primary-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, &types.NoSuchBucket{}
					})
			},
			configureSecondary: func(s3Client *mock.MockS3Client) {
				s3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
						assert.Equal(t, "secondary-bucket", aws.ToString(input.Bucket))
						assert.Equal(t, "machine-uid", aws.ToString(input.Key))
						return nil, &types.NoSuchKey{}
					})
			},
			wantAttempted: []string{"primary-bucket", "secondary-bucket"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			primary := mock.NewMockS3Client(ctrl)
			test.configurePrimary(primary)
			byBucket := map[string]stubClients{
				"primary-bucket": {s3: primary},
			}
			objectStore := &infrav1alpha2.ObjectStore{
				CredentialsRef: corev1.SecretReference{Name: "primary"},
			}
			secrets := []*corev1.Secret{objectStoreSecret("primary", "cluster-ns", "primary-bucket")}
			if test.configureSecondary != nil {
				secondary := mock.NewMockS3Client(ctrl)
				test.configureSecondary(secondary)
				byBucket["secondary-bucket"] = stubClients{s3: secondary}
				objectStore.SecondaryCredentialsRef = &corev1.SecretReference{Name: "secondary"}
				secrets = append(secrets, objectStoreSecret("secondary", "cluster-ns", "secondary-bucket"))
			}

			var attempted []string
			factory := recordingS3Factory(&attempted, byBucket)

			k8s := mock.NewMockK8sClient(ctrl)
			stubSecretLookups(k8s, secrets...)

			mscope := fallbackMachineScope(factory, k8s, objectStore)

			err := DeleteObject(t.Context(), mscope)
			if len(test.wantErrContains) == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				for _, substr := range test.wantErrContains {
					require.ErrorContains(t, err, substr)
				}
			}
			assert.Equal(t, test.wantAttempted, attempted)
		})
	}
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
								IsLatest:  aws.Bool(true),
								Key:       aws.String("test"),
								VersionId: aws.String("version2"),
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
								IsLatest:  aws.Bool(false),
								Key:       aws.String("test"),
								VersionId: aws.String("version1"),
							},
							{
								IsLatest:  aws.Bool(true),
								Key:       aws.String("test"),
								VersionId: aws.String("version2"),
							},
						},
						ResultMetadata: middleware.Metadata{},
						DeleteMarkers: []types.DeleteMarkerEntry{
							{
								IsLatest:  aws.Bool(false),
								Key:       aws.String("test"),
								VersionId: aws.String("version1"),
							},
							{
								IsLatest:  aws.Bool(true),
								Key:       aws.String("test"),
								VersionId: aws.String("version2"),
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
