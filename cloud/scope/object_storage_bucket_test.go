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
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestValidateObjectStorageBucketScopeParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      ObjectStorageBucketScopeParams
		expectedErr error
	}{
		{
			name: "Success - Valid ObjectStorageBucketScopeParams",
			params: ObjectStorageBucketScopeParams{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
				Logger: &logr.Logger{},
			},
			expectedErr: nil,
		},
		{
			name: "Failure - Invalid ObjectStorageBucketScopeParams. Logger is nil",
			params: ObjectStorageBucketScopeParams{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
				Logger: nil,
			},
			expectedErr: fmt.Errorf("logger is required when creating an ObjectStorageBucketScope"),
		},

		{
			name: "Failure - Invalid ObjectStorageBucketScopeParams. Bucket is nil",
			params: ObjectStorageBucketScopeParams{
				Bucket: nil,
				Logger: &logr.Logger{},
			},
			expectedErr: fmt.Errorf("object storage bucket is required when creating an ObjectStorageBucketScope"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			err := validateObjectStorageBucketScopeParams(testcase.params)
			if err != nil {
				assert.EqualError(t, err, testcase.expectedErr.Error())
			}
		})
	}
}

func TestNewObjectStorageBucketScope(t *testing.T) {
	t.Parallel()
	type args struct {
		apiKey string
		params ObjectStorageBucketScopeParams
	}
	tests := []struct {
		name            string
		args            args
		expectedErr     error
		expects         func(k8s *mock.MockK8sClient)
		clientBuildFunc func(apiKey string) (LinodeObjectStorageClient, error)
	}{
		{
			name: "Success - Pass in valid args and get a valid ObjectStorageBucketScope",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: nil,
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				})
			},
		},
		{
			name: "Success - Validate getCredentialDataFromRef() return some apiKey Data and we create a valid ClusterScope",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
						Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
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
					infrav1alpha1.AddToScheme(s)
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
			name: "Error - ValidateClusterScopeParams triggers error because ClusterScopeParams is empty",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{},
			},
			expectedErr: fmt.Errorf("object storage bucket is required when creating an ObjectStorageBucketScope"),
			expects:     func(k8s *mock.MockK8sClient) {},
		},
		{
			name: "Error - patchHelper returns error. Checking error handle for when new patchHelper is invoked",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("failed to init patch helper:"),
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().Return(runtime.NewScheme())
			},
		},
		{
			name: "Error - Using getCredentialDataFromRef(), func returns an error. Unable to create a valid ClusterScope",
			args: args{
				apiKey: "test-key",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
						Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
							CredentialsRef: &corev1.SecretReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("credentials from cluster secret ref: get credentials secret test/example: failed to get secret"),
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to get secret"))
			},
		},
		{
			name: "Error - createLinodeCluster throws an error for passing empty apiKey. Unable to create a valid ClusterScope",
			args: args{
				apiKey: "",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
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

			got, err := NewObjectStorageBucketScope(context.Background(), testcase.args.apiKey, testcase.args.params)

			if testcase.expectedErr != nil {
				assert.ErrorContains(t, err, testcase.expectedErr.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestObjectStorageBucketScopeMethods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		Bucket  *infrav1alpha1.LinodeObjectStorageBucket
		expects func(mock *mock.MockK8sClient)
	}{
		{
			name:   "Success - finalizer should be added to the Linode Object Storage Bucket object",
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				}).Times(2)
				mock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "Failure - finalizer should not be added to the Bucket object. Function returns nil since it was already present",
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{infrav1alpha1.GroupVersion.String()},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
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

			objScope, err := NewObjectStorageBucketScope(
				context.Background(),
				"test-key",
				ObjectStorageBucketScopeParams{
					Client: mockK8sClient,
					Bucket: testcase.Bucket,
					Logger: &logr.Logger{},
				})
			if err != nil {
				t.Errorf("NewObjectStorageBucketScope() error = %v", err)
			}

			if err := objScope.AddFinalizer(context.Background()); err != nil {
				t.Errorf("ClusterScope.AddFinalizer() error = %v", err)
			}

			if objScope.Bucket.Finalizers[0] != infrav1alpha1.GroupVersion.String() {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}

func TestGenerateKeySecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		Bucket      *infrav1alpha1.LinodeObjectStorageBucket
		keys        [NumAccessKeys]*linodego.ObjectStorageKey
		expectedErr error
		expects     func(mock *mock.MockK8sClient)
	}{
		{
			name: "happy path",
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					KeySecretName: ptr.To("test-bucket-bucket-details"),
				},
			},
			keys: [NumAccessKeys]*linodego.ObjectStorageKey{
				{
					ID:        1,
					Label:     "read_write",
					SecretKey: "read_write_key",
					AccessKey: "read_write_access_key",
					Limited:   false,
					BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
						{
							BucketName:  "bucket",
							Cluster:     "test-bucket",
							Permissions: "read_write",
						},
					},
				},
				{
					ID:        2,
					Label:     "read_only",
					SecretKey: "read_only_key",
					AccessKey: "read_only_access_key",
					Limited:   true,
					BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
						{
							BucketName:  "bucket",
							Cluster:     "test-bucket",
							Permissions: "read_only",
						},
					},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha1.AddToScheme(s)
					return s
				}).Times(1)
			},
			expectedErr: nil,
		},
		{
			name: "missing one or more keys",
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					KeySecretName: ptr.To("test-bucket-bucket-details"),
				},
			},
			keys: [NumAccessKeys]*linodego.ObjectStorageKey{
				{
					ID:        1,
					Label:     "read_write",
					SecretKey: "read_write_key",
					AccessKey: "read_write_access_key",
					Limited:   false,
					BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
						{
							BucketName:  "bucket",
							Cluster:     "test-bucket",
							Permissions: "read_write",
						},
					},
				},
			},
			expectedErr: errors.New("expected two non-nil object storage keys"),
		},
		{
			name: "client scheme does not have infrav1alpha1",
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bucket",
					Namespace: "test-namespace",
				},
			},
			keys: [NumAccessKeys]*linodego.ObjectStorageKey{
				{
					ID:        1,
					Label:     "read_write",
					SecretKey: "read_write_key",
					AccessKey: "read_write_access_key",
					Limited:   false,
					BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
						{
							BucketName:  "bucket",
							Cluster:     "test-bucket",
							Permissions: "read_write",
						},
					},
				},
				{
					ID:        2,
					Label:     "read_only",
					SecretKey: "read_only_key",
					AccessKey: "read_only_access_key",
					Limited:   true,
					BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
						{
							BucketName:  "bucket",
							Cluster:     "test-bucket",
							Permissions: "read_only",
						},
					},
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().Return(runtime.NewScheme())
			},
			expectedErr: fmt.Errorf("could not set owner ref on access key secret"),
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

			objScope := &ObjectStorageBucketScope{
				Client: mockClient,
				Bucket: testcase.Bucket,
			}

			secret, err := objScope.GenerateKeySecret(context.Background(), testcase.keys, &linodego.ObjectStorageBucket{})
			if testcase.expectedErr != nil {
				require.ErrorContains(t, err, testcase.expectedErr.Error())
				return
			}

			assert.Equal(t, "LinodeObjectStorageBucket", secret.OwnerReferences[0].Kind)
			assert.True(t, *secret.OwnerReferences[0].Controller)
		})
	}
}

func TestShouldInitKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		want   bool
		Bucket *infrav1alpha1.LinodeObjectStorageBucket
	}{
		{
			name: "should init keys",
			want: true,
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
					KeyGeneration: ptr.To(1),
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					LastKeyGeneration: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			objScope := &ObjectStorageBucketScope{
				Client:       nil,
				Bucket:       testcase.Bucket,
				Logger:       logr.Logger{},
				LinodeClient: &linodego.Client{},
				PatchHelper:  &patch.Helper{},
			}

			shouldInit := objScope.ShouldInitKeys()

			if shouldInit != testcase.want {
				t.Errorf("ObjectStorageBucketScope.ShouldInitKeys() = %v, want %v", shouldInit, testcase.want)
			}
		})
	}
}

func TestShouldRotateKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		want   bool
		Bucket *infrav1alpha1.LinodeObjectStorageBucket
	}{
		{
			name: "should rotate keys",
			want: true,
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
					KeyGeneration: ptr.To(1),
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					LastKeyGeneration: ptr.To(0),
				},
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			objScope := &ObjectStorageBucketScope{
				Client:       nil,
				Bucket:       testcase.Bucket,
				Logger:       logr.Logger{},
				LinodeClient: &linodego.Client{},
				PatchHelper:  &patch.Helper{},
			}

			rotate := objScope.ShouldRotateKeys()

			if rotate != testcase.want {
				t.Errorf("ObjectStorageBucketScope.ShouldRotateKeys() = %v, want %v", rotate, testcase.want)
			}
		})
	}
}

func TestShouldRestoreKeySecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		bucket      *infrav1alpha1.LinodeObjectStorageBucket
		expects     func(k8s *mock.MockK8sClient)
		want        bool
		expectedErr error
	}{
		{
			name: "status has no secret name",
			bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					KeySecretName: nil,
				},
			},
			want: false,
		},
		{
			name: "status has secret name and secret exists",
			bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bucket",
					Namespace: "ns",
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					KeySecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: "ns", Name: "secret"}, gomock.Any()).
					Return(nil)
			},
			want: false,
		},
		{
			name: "status has secret name and secret is missing",
			bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bucket",
					Namespace: "ns",
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					KeySecretName: ptr.To("secret"),
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
			bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bucket",
					Namespace: "ns",
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					KeySecretName: ptr.To("secret"),
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().
					Get(gomock.Any(), client.ObjectKey{Namespace: "ns", Name: "secret"}, gomock.Any()).
					Return(errors.New("unexpected error"))
			},
			want: false,
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

			objScope := &ObjectStorageBucketScope{
				Client: mockClient,
				Bucket: testcase.bucket,
			}

			restore, err := objScope.ShouldRestoreKeySecret(context.TODO())
			if testcase.expectedErr != nil {
				require.ErrorContains(t, err, testcase.expectedErr.Error())
			}

			assert.Equal(t, testcase.want, restore)
		})
	}
}
