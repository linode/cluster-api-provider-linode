package scope

import (
	"context"
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
		credentialsRef   corev1.SecretReference
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
			name: "Simple test case if no error",
			args: args{
				ctx: context.Background(),
				credentialsRef: corev1.SecretReference{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create an instance of the mock crClient
			mockClient := mock.NewMockcrClient(ctrl)

			// Setup Expected behaviour
			secretRef := client.ObjectKey{
				Name:      tt.args.credentialsRef.Name,
				Namespace: tt.args.credentialsRef.Namespace,
			}
			mockClient.EXPECT().Get(gomock.Any(), secretRef, gomock.Any()).DoAndReturn(tt.args.funcBehavior)

			// Call the function under test with the mock client
			got, err := getCredentialDataFromRef(tt.args.ctx, mockClient, tt.args.credentialsRef, tt.args.defaultNamespace)

			// Check that the function returned the expected result
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.Equal(t, tt.expectedByte, got)
			}
		})
	}
}
