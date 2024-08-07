package scope

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
			name: "Error - Using getCredentialDataFromRef(), func returns an error. Unable to create a valid ClusterScope",
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
			name: "Error - createLinodeCluster throws an error for passing empty apiKey. Unable to create a valid ClusterScope",
			args: args{
				apiKey: "",
				params: ObjectStorageBucketScopeParams{
					Client: nil,
					Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
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
		Bucket  *infrav1alpha2.LinodeObjectStorageBucket
		expects func(mock *mock.MockK8sClient)
	}{
		{
			name:   "Success - finalizer should be added to the Linode Object Storage Bucket object",
			Bucket: &infrav1alpha2.LinodeObjectStorageBucket{},
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
			name: "Failure - finalizer should not be added to the Bucket object. Function returns nil since it was already present",
			Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{infrav1alpha2.ObjectStorageBucketFinalizer},
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

			if objScope.Bucket.Finalizers[0] != infrav1alpha2.ObjectStorageBucketFinalizer {
				t.Errorf("Finalizer was not added")
			}
		})
	}
}
