package scope

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"
)

// Test_createLinodeClient tests the createLinodeClient function. Checks if the client does not error out.
func TestCreateLinodeClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		apiKey      string
		expectedErr error
	}{
		{
			"Success - Valid API Key",
			"test-key",
			nil,
		},
		{
			"Error - Empty API Key",
			"",
			errors.New("missing Linode API key"),
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := CreateLinodeClient(testCase.apiKey, defaultClientTimeout)

			if testCase.expectedErr != nil {
				assert.EqualError(t, err, testCase.expectedErr.Error())
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}

// Test_getCredentialDataFromRef tests the getCredentialDataFromRef function.
func TestGetCredentialDataFromRef(t *testing.T) {
	t.Parallel()

	type args struct {
		providedCredentialsRef corev1.SecretReference
		expectedCredentialsRef corev1.SecretReference
		funcBehavior           func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
	}

	tests := []struct {
		name          string
		args          args
		expectedByte  []byte
		expectedError string
	}{
		{
			name: "Testing functionality using valid/good data. No error should be returned",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				funcBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"apiToken": []byte("example"),
						},
					}
					*obj = cred

					return nil
				},
			},
			expectedByte:  []byte("example"),
			expectedError: "",
		},
		{
			name: "Empty namespace provided and default namespace is used. No error should be returned",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "default",
				},
				funcBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"apiToken": []byte("example"),
						},
					}
					*obj = cred

					return nil
				},
			},
			expectedByte:  []byte("example"),
			expectedError: "",
		},
		{
			name: "Handle error from crClient. Error should be returned.",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				funcBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					return errors.New("Could not find the secret")
				},
			},
			expectedByte:  []byte(nil),
			expectedError: "get credentials secret test/example: Could not find the secret",
		},
		{
			name: "Handle error after getting empty secret from crClient. Error should be returned.",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				funcBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					return nil
				},
			},
			expectedByte:  []byte(nil),
			expectedError: "no apiToken key in credentials secret test/example",
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create an instance of the mock K8sClient
			mockClient := mock.NewMockK8sClient(ctrl)

			// Setup Expected behaviour
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testCase.args.funcBehavior)

			// Call getCredentialDataFromRef using the mock client
			got, err := getCredentialDataFromRef(context.Background(), mockClient, testCase.args.providedCredentialsRef, "default")

			// Check that the function returned the expected result
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.Equal(t, testCase.expectedByte, got)
			}
		})
	}
}

// Test_addCredentialsFinalizer tests the addCredentialsFinalizer function.
func Test_addCredentialsFinalizer(t *testing.T) {
	t.Parallel()

	type clientBehavior struct {
		Get    func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
		Update func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	}

	type args struct {
		providedCredentialsRef corev1.SecretReference
		clientBehavior         clientBehavior
	}

	tests := []struct {
		name          string
		args          args
		expectedError string
	}{
		{
			name: "Testing functionality using valid/good data. No error should be returned",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				clientBehavior: clientBehavior{
					Get: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
						cred := corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "example",
								Namespace: "test",
							},
						}
						*obj = cred

						return nil
					},
					Update: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Handle error from crClient Get. Error should be returned.",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				clientBehavior: clientBehavior{
					Get: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
						return errors.New("client get error")
					},
					Update: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
			},
			expectedError: "get credentials secret test/example: client get error",
		},
		{
			name: "Handle error from crClient Update. Error should be returned.",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				clientBehavior: clientBehavior{
					Get: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
						cred := corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "example",
								Namespace: "test",
							},
						}
						*obj = cred

						return nil
					},
					Update: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return errors.New("client update error")
					},
				},
			},
			expectedError: "add finalizer to credentials secret test/example: client update error",
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create an instance of the mock K8sClient
			mockClient := mock.NewMockK8sClient(ctrl)

			// Setup Expected behaviour
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testCase.args.clientBehavior.Get).AnyTimes()
			mockClient.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(testCase.args.clientBehavior.Update).AnyTimes()

			// Call addCredentialsFinalizer using the mock client
			err := addCredentialsFinalizer(context.Background(), mockClient, testCase.args.providedCredentialsRef, "default", "test.test/test.test")

			// Check that the function returned the expected result
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test_removeCredentialsFinalizer tests the removeCredentialsFinalizer function.
func Test_removeCredentialsFinalizer(t *testing.T) {
	t.Parallel()

	type clientBehavior struct {
		Get    func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
		Update func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	}

	type args struct {
		providedCredentialsRef corev1.SecretReference
		clientBehavior         clientBehavior
	}

	tests := []struct {
		name          string
		args          args
		expectedError string
	}{
		{
			name: "Testing functionality using valid/good data. No error should be returned",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				clientBehavior: clientBehavior{
					Get: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
						cred := corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "example",
								Namespace: "test",
							},
						}
						*obj = cred

						return nil
					},
					Update: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Handle error from crClient Get. Error should be returned.",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				clientBehavior: clientBehavior{
					Get: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
						return errors.New("client get error")
					},
					Update: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
			},
			expectedError: "get credentials secret test/example: client get error",
		},
		{
			name: "Handle error from crClient Update. Error should be returned.",
			args: args{
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				clientBehavior: clientBehavior{
					Get: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
						cred := corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "example",
								Namespace: "test",
							},
						}
						*obj = cred

						return nil
					},
					Update: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return errors.New("client update error")
					},
				},
			},
			expectedError: "remove finalizer from credentials secret test/example: client update error",
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create an instance of the mock K8sClient
			mockClient := mock.NewMockK8sClient(ctrl)

			// Setup Expected behaviour
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testCase.args.clientBehavior.Get).AnyTimes()
			mockClient.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(testCase.args.clientBehavior.Update).AnyTimes()

			// Call removeCredentialsFinalizer using the mock client
			err := removeCredentialsFinalizer(context.Background(), mockClient, testCase.args.providedCredentialsRef, "default", "test.test/test.test")

			// Check that the function returned the expected result
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test_toFinalizer tests the toFinalizer function.
func Test_toFinalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		gvk    schema.GroupVersionKind
		object client.Object
		want   string
	}{
		{
			"Namespaced Resources",
			schema.GroupVersionKind{
				Group:   infrav1alpha1.GroupVersion.Group,
				Version: infrav1alpha1.GroupVersion.Version,
				Kind:    "LinodeCluster",
			},
			&infrav1alpha2.LinodeCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "example",
				},
			},
			"linodecluster.infrastructure.cluster.x-k8s.io/test.example",
		},
		{
			"Cluster Resources",
			schema.GroupVersionKind{
				Group:   infrav1alpha1.GroupVersion.Group,
				Version: infrav1alpha1.GroupVersion.Version,
				Kind:    "LinodeCluster",
			},
			&infrav1alpha2.LinodeCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example",
					// NOTE: Fake a cluster resource by setting Namespace to the default value
					// See: https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Object
					Namespace: "",
				},
			},
			"linodecluster.infrastructure.cluster.x-k8s.io/example",
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Inject the unambiguous kind identity into the object
			testCase.object.GetObjectKind().SetGroupVersionKind(testCase.gvk)

			got := toFinalizer(testCase.object)
			if testCase.want != got {
				assert.Equal(t, testCase.want, got)
			}
		})
	}
}
