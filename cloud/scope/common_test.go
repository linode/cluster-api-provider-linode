package scope

import (
	"context"
	"errors"
	"testing"

	mock "github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test_createLinodeClient tests the createLinodeClient function. Checks if the client does not error out.
func TestCreateLinodeClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		apiKey string
		want   *linodego.Client
	}{
		{
			"Valid API Key",
			"test-key",
			createLinodeClient("test-key"),
		},
		{
			"Empty API Key",
			"",
			createLinodeClient(""),
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := createLinodeClient(testCase.apiKey); got != nil {
				assert.EqualExportedValues(t, testCase.want, got, "Checking is the objects are equal")
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
			name: "Check is the function works correctly",
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
			name: "Empty namespace test case",
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
			name: "Handle error from crClient",
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
			name: "Handle error after getting empty secret from crClient",
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
			mockClient := mock.NewMockk8sClient(ctrl)

			// Setup Expected behaviour
			expectedSecretRef := client.ObjectKey{
				Name:      testCase.args.expectedCredentialsRef.Name,
				Namespace: testCase.args.expectedCredentialsRef.Namespace,
			}
			mockClient.EXPECT().Get(gomock.Any(), expectedSecretRef, gomock.Any()).DoAndReturn(testCase.args.funcBehavior)

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
