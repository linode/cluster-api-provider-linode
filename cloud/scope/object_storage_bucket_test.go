package scope

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/clients"
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
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
				Logger: &logr.Logger{},
			},
			expectedErr: nil,
		},
		{
			name: "Failure - Invalid ObjectStorageBucketScopeParams. Logger is nil",
			params: ObjectStorageBucketScopeParams{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
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
		clientBuildFunc func(apiKey string) (LinodeClient, error)
	}{
		{
			name: "Success - Pass in valid args and get a valid ObjectStorageBucketScope",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
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
			name: "Success - Validate getCredentialDataFromRef() return some apiKey Data and we create a valid ClusterScope",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
						Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
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
			name: "Error - ValidateObjectStorageBucketScopeParams triggers error because ObjectStorageBucketScopeParams is empty",
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
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("failed to init patch helper:"),
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().Return(runtime.NewScheme())
			},
		},
		{
			name: "Error - Using getCredentialDataFromRef(), func returns an error. Unable to create a valid ObjectStorageBucketScope",
			args: args{
				apiKey: "test-key",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
						Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
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
			name: "Error - createObjectStorageBucket throws an error for passing empty apiKey. Unable to create a valid ObjectStorageBucketScope",
			args: args{
				apiKey: "",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("failed to create linode client: token cannot be empty"),
			expects:     func(mock *mock.MockK8sClient) {},
		},
		{
			name: "Error - kind is not registered",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("no kind is registered for the type v1alpha2.LinodeObjectStorageBucket"),
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().Return(runtime.NewScheme())
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

			testcase.args.params.Client = mockK8sClient

			got, err := NewObjectStorageBucketScope(t.Context(), ClientConfig{Token: testcase.args.apiKey}, testcase.args.params)

			if testcase.expectedErr != nil {
				assert.ErrorContains(t, err, testcase.expectedErr.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestAddFinalizer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockK8sClient := mock.NewMockK8sClient(ctrl)
	mockK8sClient.EXPECT().Scheme().AnyTimes().DoAndReturn(func() *runtime.Scheme {
		s := runtime.NewScheme()
		infrav1alpha2.AddToScheme(s)
		return s
	})
	mockK8sClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	bucket := &infrav1alpha2.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bucket",
			Namespace: "test-namespace",
		},
	}
	scope, err := NewObjectStorageBucketScope(t.Context(), ClientConfig{Token: "test-token"}, ObjectStorageBucketScopeParams{
		Client: mockK8sClient,
		Bucket: bucket,
		Logger: &logr.Logger{},
	})
	require.NoError(t, err)

	err = scope.AddFinalizer(t.Context())
	require.NoError(t, err)
}

func TestAddAccessKeyRefFinalizer(t *testing.T) {
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
		clientBuildFunc func(apiKey string) (LinodeClient, error)
	}{
		{
			name: "Success - no AccessKeyRef",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
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
			name: "Success - valid AccessKeyRef",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
						Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
							AccessKeyRef: &corev1.ObjectReference{
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
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, name types.NamespacedName, obj *infrav1alpha2.LinodeObjectStorageKey, opts ...client.GetOption) error {
					cred := infrav1alpha2.LinodeObjectStorageKey{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example",
							Namespace: "test",
						},
						Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
							BucketAccess: []infrav1alpha2.BucketAccessRef{{
								BucketName:  "test-bucket",
								Permissions: "read_write",
								Region:      "region",
							}},
						},
					}
					*obj = cred
					return nil
				})
				k8s.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					// Simulate adding a finalizer
					controllerutil.AddFinalizer(obj, "test-bucket")
					return nil
				})
			},
		},
		{
			name: "Error - accessKeyRef doesn't exist",
			args: args{
				apiKey: "test-key",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
						Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
							AccessKeyRef: &corev1.ObjectReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("not found"),
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				})
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
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

			testcase.args.params.Client = mockK8sClient

			scope, err := NewObjectStorageBucketScope(t.Context(), ClientConfig{Token: testcase.args.apiKey}, testcase.args.params)
			require.NoError(t, err)

			err = scope.AddAccessKeyRefFinalizer(t.Context(), tt.args.params.Bucket.Name)

			if testcase.expectedErr != nil {
				assert.ErrorContains(t, err, testcase.expectedErr.Error())
			}
		})
	}
}

func TestRemoveAccessKeyRefFinalizer(t *testing.T) {
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
		clientBuildFunc func(apiKey string) (LinodeClient, error)
	}{
		{
			name: "Success - valid AccessKeyRef",
			args: args{
				apiKey: "apikey",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
						Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
							AccessKeyRef: &corev1.ObjectReference{
								Name: "example",
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
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, name types.NamespacedName, obj *infrav1alpha2.LinodeObjectStorageKey, opts ...client.GetOption) error {
					cred := infrav1alpha2.LinodeObjectStorageKey{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "example",
							Namespace:  "test",
							Finalizers: []string{"test-bucket"},
						},
						Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
							BucketAccess: []infrav1alpha2.BucketAccessRef{{
								BucketName:  "test-bucket",
								Permissions: "read_write",
								Region:      "region",
							}},
						},
					}
					*obj = cred
					return nil
				})
				k8s.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					// Simulate adding a finalizer
					controllerutil.AddFinalizer(obj, "test-bucket")
					return nil
				})
			},
		},
		{
			name: "Error - accessKeyRef doesn't exist",
			args: args{
				apiKey: "test-key",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
						Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
							AccessKeyRef: &corev1.ObjectReference{
								Name:      "example",
								Namespace: "test",
							},
						},
					},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("not found"),
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				})
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
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

			testcase.args.params.Client = mockK8sClient

			scope, err := NewObjectStorageBucketScope(t.Context(), ClientConfig{Token: testcase.args.apiKey}, testcase.args.params)
			require.NoError(t, err)

			err = scope.RemoveAccessKeyRefFinalizer(t.Context(), tt.args.params.Bucket.Name)

			if testcase.expectedErr != nil {
				assert.ErrorContains(t, err, testcase.expectedErr.Error())
			}
		})
	}
}
