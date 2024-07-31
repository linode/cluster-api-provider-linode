package scope

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusteraddonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

func TestValidateObjectStorageKeyScopeParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      ObjectStorageKeyScopeParams
		expectedErr error
	}{
		{
			name: "valid",
			params: ObjectStorageKeyScopeParams{
				Key:    &infrav1alpha2.LinodeObjectStorageKey{},
				Logger: &logr.Logger{},
			},
			expectedErr: nil,
		},
		{
			name: "nil logger",
			params: ObjectStorageKeyScopeParams{
				Key:    &infrav1alpha2.LinodeObjectStorageKey{},
				Logger: nil,
			},
			expectedErr: fmt.Errorf("logger is required when creating an ObjectStorageKeyScope"),
		},

		{
			name: "nil key",
			params: ObjectStorageKeyScopeParams{
				Key:    nil,
				Logger: &logr.Logger{},
			},
			expectedErr: fmt.Errorf("object storage key is required when creating an ObjectStorageKeyScope"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			err := validateObjectStorageKeyScopeParams(testcase.params)
			if err != nil {
				assert.EqualError(t, err, testcase.expectedErr.Error())
			}
		})
	}
}

func TestNewObjectStorageKeyScope(t *testing.T) {
	t.Parallel()

	type args struct {
		apiKey string
		params ObjectStorageKeyScopeParams
	}
	tests := []struct {
		name            string
		args            args
		expectedErr     error
		expects         func(k8s *mock.MockK8sClient)
		clientBuildFunc func(apiKey string) (LinodeClient, error)
	}{
		{
			name: "success",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageKeyScopeParams{
					Key:    &infrav1alpha2.LinodeObjectStorageKey{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: nil,
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				})
			},
		},
		{
			name: "with credentials from secret",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageKeyScopeParams{
					Client: nil,
					Key: &infrav1alpha2.LinodeObjectStorageKey{
						Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: nil,
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				})
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, name types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"apiToken": []byte("example"),
						},
					}
					*obj = cred
					return nil
				})
			},
		},
		{
			name: "empty params",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageKeyScopeParams{},
			},
			expectedErr: fmt.Errorf("object storage key is required"),
			expects:     func(k8s *mock.MockK8sClient) {},
		},
		{
			name: "patch newHelper fail",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageKeyScopeParams{
					Client: nil,
					Key:    &infrav1alpha2.LinodeObjectStorageKey{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("failed to init patch helper:"),
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().Return(runtime.NewScheme())
			},
		},
		{
			name: "credentials from ref fail",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageKeyScopeParams{
					Client: nil,
					Key: &infrav1alpha2.LinodeObjectStorageKey{
						Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("credentials from secret ref: get credentials secret test/example: failed to get secret"),
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to get secret"))
			},
		},
		{
			name: "empty apiKey",
			args: args{
				apiKey: "",
				params: ObjectStorageKeyScopeParams{
					Client: nil,
					Key:    &infrav1alpha2.LinodeObjectStorageKey{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("failed to create linode client: missing Linode API key"),
			expects:     func(mock *mock.MockK8sClient) {},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)

			testcase.expects(mockK8sClient)

			testcase.args.params.Client = mockK8sClient

			got, err := NewObjectStorageKeyScope(context.Background(), testcase.args.apiKey, testcase.args.params)

			if testcase.expectedErr != nil {
				assert.ErrorContains(t, err, testcase.expectedErr.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestObjectStrorageKeyAddFinalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		Key     *infrav1alpha2.LinodeObjectStorageKey
		expects func(mock *mock.MockK8sClient)
	}{
		{
			name: "success",
			Key:  &infrav1alpha2.LinodeObjectStorageKey{},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).Times(2)
				mock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "fail",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{infrav1alpha2.ObjectStorageKeyFinalizer},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).Times(1)
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)

			testcase.expects(mockK8sClient)

			keyScope, err := NewObjectStorageKeyScope(
				context.Background(),
				"test-key",
				ObjectStorageKeyScopeParams{
					Client: mockK8sClient,
					Key:    testcase.Key,
					Logger: &logr.Logger{},
				})
			if err != nil {
				t.Errorf("NewObjectStorageBucketScope() error = %v", err)
			}

			if err := keyScope.AddFinalizer(context.Background()); err != nil {
				t.Errorf("ClusterScope.AddFinalizer() error = %v", err)
			}

			if keyScope.Key.Finalizers[0] != infrav1alpha2.ObjectStorageKeyFinalizer {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}

func TestGenerateKeySecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		Key          *infrav1alpha2.LinodeObjectStorageKey
		key          *linodego.ObjectStorageKey
		expectedErr  error
		expectK8s    func(*mock.MockK8sClient)
		expectLinode func(*mock.MockLinodeClient)
	}{
		{
			name: "opaque secret",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("test-bucket-obj-key"),
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "read_write",
				SecretKey: "read_write_key",
				AccessKey: "read_write_access_key",
				Limited:   false,
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "test-bucket",
						Permissions: "read_write",
					},
				},
			},
			expectK8s: func(mck *mock.MockK8sClient) {
				mck.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).Times(1)
			},
			expectedErr: nil,
		},
		{
			name: "cluster resource-set",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					BucketAccess: []infrav1alpha2.BucketAccessRef{
						{
							BucketName:  "bucket",
							Region:      "test-bucket",
							Permissions: "read_write",
						},
					},
					SecretType: clusteraddonsv1.ClusterResourceSetSecretType,
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("test-bucket-obj-key"),
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "read_write",
				SecretKey: "read_write_key",
				AccessKey: "read_write_access_key",
				Limited:   false,
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "test-bucket",
						Permissions: "read_write",
					},
				},
			},
			expectK8s: func(mck *mock.MockK8sClient) {
				mck.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				}).Times(1)
			},
			expectLinode: func(mck *mock.MockLinodeClient) {
				mck.EXPECT().GetObjectStorageBucket(gomock.Any(), "test-bucket", "bucket").Return(&linodego.ObjectStorageBucket{
					Label:    "bucket",
					Region:   "us-ord",
					Hostname: "hostname",
				}, nil)
			},
			expectedErr: nil,
		},
		{
			name: "cluster resource-set with empty buckets",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					SecretType: clusteraddonsv1.ClusterResourceSetSecretType,
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("test-bucket-obj-key"),
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "read_write",
				SecretKey: "read_write_key",
				AccessKey: "read_write_access_key",
				Limited:   false,
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "test-bucket",
						Permissions: "read_write",
					},
				},
			},
			expectedErr: errors.New("unable to generate addons.cluster.x-k8s.io/resource-set; spec.bucketAccess must not be empty"),
		},
		{
			name: "missing key",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("test-bucket-obj-key"),
				},
			},
			expectedErr: errors.New("expected non-nil object storage key"),
		},
		{
			name: "client scheme does not have infrav1alpha2",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("test-bucket-obj-key"),
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "read_write",
				SecretKey: "read_write_key",
				AccessKey: "read_write_access_key",
				Limited:   false,
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "test-bucket",
						Permissions: "read_write",
					},
				},
			},
			expectK8s: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().Return(runtime.NewScheme())
			},
			expectedErr: fmt.Errorf("could not set owner ref on access key secret"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)
			if testcase.expectK8s != nil {
				testcase.expectK8s(mockK8sClient)
			}

			mockLinodeClient := mock.NewMockLinodeClient(ctrl)
			if testcase.expectLinode != nil {
				testcase.expectLinode(mockLinodeClient)
			}

			keyScope := &ObjectStorageKeyScope{
				Client:       mockK8sClient,
				LinodeClient: mockLinodeClient,
				Key:          testcase.Key,
			}

			secret, err := keyScope.GenerateKeySecret(context.Background(), testcase.key)
			if testcase.expectedErr != nil {
				require.ErrorContains(t, err, testcase.expectedErr.Error())
				return
			} else if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, "LinodeObjectStorageKey", secret.OwnerReferences[0].Kind)
			assert.True(t, *secret.OwnerReferences[0].Controller)
		})
	}
}

func TestShouldInitKey(t *testing.T) {
	t.Parallel()

	assert.True(t, (&ObjectStorageKeyScope{
		Key: &infrav1alpha2.LinodeObjectStorageKey{
			Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
				LastKeyGeneration: nil,
			},
		},
	}).ShouldInitKey())
}

func TestShouldRotateKey(t *testing.T) {
	t.Parallel()

	assert.False(t, (&ObjectStorageKeyScope{
		Key: &infrav1alpha2.LinodeObjectStorageKey{
			Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
				LastKeyGeneration: nil,
			},
		},
	}).ShouldRotateKey())

	assert.False(t, (&ObjectStorageKeyScope{
		Key: &infrav1alpha2.LinodeObjectStorageKey{
			Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
				KeyGeneration: 0,
			},
			Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
				LastKeyGeneration: ptr.To(0),
			},
		},
	}).ShouldRotateKey())

	assert.True(t, (&ObjectStorageKeyScope{
		Key: &infrav1alpha2.LinodeObjectStorageKey{
			Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
				KeyGeneration: 1,
			},
			Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
				LastKeyGeneration: ptr.To(0),
			},
		},
	}).ShouldRotateKey())
}

func TestShouldReconcileKeySecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         *infrav1alpha2.LinodeObjectStorageKey
		expects     func(k8s *mock.MockK8sClient)
		want        bool
		expectedErr error
	}{
		{
			name: "status has no secret name",
			key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: nil,
				},
			},
			want: false,
		},
		{
			name: "secret has expected key",
			key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					SecretType: corev1.SecretTypeOpaque,
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						*obj = corev1.Secret{
							Data: map[string][]byte{
								"access_key": {},
							},
						}
						return nil
					}).AnyTimes()
			},
			want: false,
		},
		{
			name: "secret is missing expected key",
			key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					SecretType: corev1.SecretTypeOpaque,
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						*obj = corev1.Secret{
							Data: map[string][]byte{
								"not_access_key": {},
							},
						}
						return nil
					}).AnyTimes()
				k8s.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			want: true,
		},
		{
			name: "secret is missing",
			key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: "ns", Name: "secret"}, gomock.Any()).
					Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "secret"))
			},
			want: true,
		},
		{
			name: "non-404 api error",
			key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: "ns", Name: "secret"}, gomock.Any()).
					Return(errors.New("unexpected error"))
			},
			want:        false,
			expectedErr: errors.New("unexpected error"),
		},
		{
			name: "unsupported secret type",
			key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					SecretType: "unsupported secret type",
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						*obj = corev1.Secret{
							Data: map[string][]byte{
								"not_access_key": {},
							},
						}
						return nil
					}).AnyTimes()
			},
			want:        false,
			expectedErr: errors.New("unsupported secret type configured in LinodeObjectStorageKey"),
		},
		{
			name: "failed delete",
			key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					SecretType: corev1.SecretTypeOpaque,
				},
				Status: infrav1alpha2.LinodeObjectStorageKeyStatus{
					SecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret, opts ...client.GetOption) error {
						*obj = corev1.Secret{
							Data: map[string][]byte{
								"not_access_key": {},
							},
						}
						return nil
					}).AnyTimes()
				k8s.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed delete"))
			},
			want:        false,
			expectedErr: errors.New("failed delete"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			var mockClient *mock.MockK8sClient
			if testcase.expects != nil {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockClient = mock.NewMockK8sClient(ctrl)
				testcase.expects(mockClient)
			}

			keyScope := &ObjectStorageKeyScope{
				Client: mockClient,
				Key:    testcase.key,
			}

			restore, err := keyScope.ShouldReconcileKeySecret(context.TODO())
			if testcase.expectedErr != nil {
				require.ErrorContains(t, err, testcase.expectedErr.Error())
			}

			assert.Equal(t, testcase.want, restore)
		})
	}
}
