package scope

import (
	"context"
	"errors"
	"testing"

	"github.com/linode/cluster-api-provider-linode/cloud/scope/mock"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test_createLinodeClient tests the createLinodeClient function. Checks if the client does not error out.
func Test_createLinodeClient(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			if got := createLinodeClient(tt.apiKey); got != nil {
				assert.EqualExportedValues(t, got, tt.want, "Checking is the objects are equal")
			}
		})
	}
}

func Test_getCredentialDataFromRef(t *testing.T) {
	type args struct {
		ctx              context.Context
		providedCredentialsRef   corev1.SecretReference
		expectedCredentialsRef   corev1.SecretReference
		defaultNamespace string
		funcBehavior     func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
	}
	tests := []struct {
		name    string
		args    args
		expectedByte    []byte
		expectedError string 
	}{
		{
			name: "Check is the function works correctly",
			args: args{
				ctx: context.Background(),
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				defaultNamespace: "default",
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
			expectedByte: []byte("example"),
			expectedError: "",
		},
		{
			name: "Empty namespace test case",
			args: args{
				ctx: context.Background(),
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "default",
				},
				defaultNamespace: "default",
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
			expectedByte: []byte("example"),
			expectedError: "",
		},
		{
			name: "Handle error from crClient",
			args: args{
				ctx: context.Background(),
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				defaultNamespace: "default",
				funcBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					return errors.New("Could not find the secret")
				},
			},
			expectedByte: []byte(nil),
			expectedError: "get credentials secret test/example: Could not find the secret",
		},
		{
			name: "Handle error after getting empty secret from crClient",
			args: args{
				ctx: context.Background(),
				providedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				expectedCredentialsRef: corev1.SecretReference{
					Name:      "example",
					Namespace: "test",
				},
				defaultNamespace: "default",
				funcBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					return nil
				},
			},
			expectedByte: []byte(nil),
			expectedError: "no apiToken key in credentials secret test/example",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create an instance of the mock crClient
			mockClient := mock.NewMockcrClient(ctrl)

			// Setup Expected behaviour
			expectedSecretRef := client.ObjectKey{
				Name:      tt.args.expectedCredentialsRef.Name,
				Namespace: tt.args.expectedCredentialsRef.Namespace,
			}
			mockClient.EXPECT().Get(gomock.Any(), expectedSecretRef, gomock.Any()).DoAndReturn(tt.args.funcBehavior)

			// Call the function under test with the mock client
			got, err := getCredentialDataFromRef(tt.args.ctx, mockClient, tt.args.providedCredentialsRef, tt.args.defaultNamespace)

			// Check that the function returned the expected result
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.Equal(t, tt.expectedByte, got)
			}
		})
	}
}
