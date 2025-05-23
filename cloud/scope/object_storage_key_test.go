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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusteraddonsv1 "sigs.k8s.io/cluster-api/api/addons/v1beta1"
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
			name: "empty apiKey",
			args: args{
				apiKey: "",
				params: ObjectStorageKeyScopeParams{
					Client: nil,
					Key:    &infrav1alpha2.LinodeObjectStorageKey{},
					Logger: &logr.Logger{},
				},
			},
			expectedErr: fmt.Errorf("failed to create linode client: token cannot be empty"),
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

			got, err := NewObjectStorageKeyScope(t.Context(), ClientConfig{Token: testcase.args.apiKey}, testcase.args.params)

			if testcase.expectedErr != nil {
				assert.ErrorContains(t, err, testcase.expectedErr.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestObjectStorageKeyAddFinalizer(t *testing.T) {
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
				t.Context(),
				ClientConfig{Token: "test-key"},
				ObjectStorageKeyScopeParams{
					Client: mockK8sClient,
					Key:    testcase.Key,
					Logger: &logr.Logger{},
				})
			if err != nil {
				t.Errorf("NewObjectStorageBucketScope() error = %v", err)
			}

			if err := keyScope.AddFinalizer(t.Context()); err != nil {
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
		expectK8s    func(*mock.MockK8sClient)
		expectLinode func(*mock.MockLinodeClient)
		expectedData map[string]string
		expectedErr  error
	}{
		{
			name: "opaque secret",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-key",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					GeneratedSecret: infrav1alpha2.GeneratedSecret{
						Name:      "test-key-obj-key",
						Namespace: "test-namespace",
					},
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "test-key",
				AccessKey: "access",
				SecretKey: "secret",
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "region",
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
			expectedData: map[string]string{
				"access": "access",
				"secret": "secret",
			},
			expectedErr: nil,
		},
		{
			name: "invalid template",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-key",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					BucketAccess: []infrav1alpha2.BucketAccessRef{
						{
							BucketName:  "bucket",
							Region:      "region",
							Permissions: "read_write",
						},
					},
					GeneratedSecret: infrav1alpha2.GeneratedSecret{
						Name:      "test-key-obj-key",
						Namespace: "test-namespace",
						Format: map[string]string{
							"key": "{{ .AccessKey",
						},
					},
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "test-key",
				AccessKey: "access",
				SecretKey: "secret",
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "region",
						Permissions: "read_write",
					},
				},
			},
			expectLinode: func(mck *mock.MockLinodeClient) {
				mck.EXPECT().GetObjectStorageBucket(gomock.Any(), "region", "bucket").Return(&linodego.ObjectStorageBucket{
					Label:    "bucket",
					Region:   "region",
					Hostname: "hostname",
				}, nil)
			},
			expectedErr: errors.New("unable to generate secret; failed to parse template in secret data format for key"),
		},
		{
			name: "cluster resource-set",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-key",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					BucketAccess: []infrav1alpha2.BucketAccessRef{
						{
							BucketName:  "bucket",
							Region:      "region",
							Permissions: "read_write",
						},
					},
					GeneratedSecret: infrav1alpha2.GeneratedSecret{
						Name:      "test-key-obj-key",
						Namespace: "test-namespace",
						Type:      clusteraddonsv1.ClusterResourceSetSecretType,
						Format: map[string]string{
							"key": "{{ .AccessKey }},{{ .SecretKey }},{{ .BucketEndpoint }}",
						},
					},
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "test-key",
				AccessKey: "access",
				SecretKey: "secret",
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "region",
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
				mck.EXPECT().GetObjectStorageBucket(gomock.Any(), "region", "bucket").Return(&linodego.ObjectStorageBucket{
					Label:    "bucket",
					Region:   "region",
					Hostname: "hostname",
				}, nil)
			},
			expectedData: map[string]string{
				"key": "access,secret,hostname",
			},
			expectedErr: nil,
		},
		{
			name: "cluster resource-set get bucket fail",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-key",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					BucketAccess: []infrav1alpha2.BucketAccessRef{
						{
							BucketName:  "bucket",
							Region:      "region",
							Permissions: "read_write",
						},
					},
					GeneratedSecret: infrav1alpha2.GeneratedSecret{
						Name:      "test-key-obj-key",
						Namespace: "test-namespace",
						Type:      clusteraddonsv1.ClusterResourceSetSecretType,
						Format: map[string]string{
							"key": "{{ .AccessKey }},{{ .SecretKey }},{{ .BucketEndpoint }}",
						},
					},
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "test-key",
				AccessKey: "access",
				SecretKey: "secret",
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "region",
						Permissions: "read_write",
					},
				},
			},
			expectLinode: func(mck *mock.MockLinodeClient) {
				mck.EXPECT().GetObjectStorageBucket(gomock.Any(), "region", "bucket").Return(nil, errors.New("api err"))
			},
			expectedErr: errors.New("unable to generate addons.cluster.x-k8s.io/resource-set; failed to get bucket: api err"),
		},
		{
			name: "cluster resource-set with empty buckets",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-key",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					GeneratedSecret: infrav1alpha2.GeneratedSecret{
						Name:      "test-key-obj-key",
						Namespace: "test-namespace",
						Type:      clusteraddonsv1.ClusterResourceSetSecretType,
						Format: map[string]string{
							"key": "{{ .AccessKey }},{{ .SecretKey }},{{ .BucketEndpoint }}",
						},
					},
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "test-key",
				AccessKey: "access",
				SecretKey: "secret",
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "region",
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
					Name:      "test-key",
					Namespace: "test-namespace",
				},
			},
			expectedErr: errors.New("expected non-nil object storage key"),
		},
		{
			name: "client scheme does not have infrav1alpha2",
			Key: &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-key",
					Namespace: "test-namespace",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					GeneratedSecret: infrav1alpha2.GeneratedSecret{
						Name:      "test-key-obj-key",
						Namespace: "test-namespace",
					},
				},
			},
			key: &linodego.ObjectStorageKey{
				ID:        1,
				Label:     "test-key",
				AccessKey: "access",
				SecretKey: "secret",
				BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
					{
						BucketName:  "bucket",
						Region:      "region",
						Permissions: "read_write",
					},
				},
			},
			expectK8s: func(mock *mock.MockK8sClient) {
				mock.EXPECT().Scheme().Return(runtime.NewScheme())
			},
			expectedErr: fmt.Errorf("could not set controller ref on access key secret"),
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

			secret, err := keyScope.GenerateKeySecret(t.Context(), testcase.key)
			if testcase.expectedErr != nil {
				require.ErrorContains(t, err, testcase.expectedErr.Error())
				return
			} else if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, testcase.expectedData, secret.StringData)
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
func TestObjectStorageKeySetCredentialRefTokenForLinodeClients(t *testing.T) {
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
				mock.EXPECT().Scheme().DoAndReturn(func() *runtime.Scheme {
					s := runtime.NewScheme()
					infrav1alpha2.AddToScheme(s)
					return s
				})
				mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to get secret"))
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

			kscope, err := NewObjectStorageKeyScope(t.Context(), ClientConfig{Token: testcase.args.apiKey}, testcase.args.params)

			if err != nil {
				t.Errorf("NewObjectStorageKeyScope() error = %v", err)
			}

			if err := kscope.SetCredentialRefTokenForLinodeClients(t.Context()); err != nil {
				assert.ErrorContains(t, err, testcase.expectedErr.Error())
			}
		})
	}
}
