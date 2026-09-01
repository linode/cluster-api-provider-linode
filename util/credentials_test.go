package util

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestGetCredentialDataFromRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		credentialsRef corev1.SecretReference
		getBehavior    func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
		key            string
		expectedByte   []byte
		expectedError  string
	}{
		{
			name: "returns data from explicit namespace",
			credentialsRef: corev1.SecretReference{
				Name:      "example",
				Namespace: "test",
			},
			getBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				*obj = corev1.Secret{Data: map[string][]byte{"apiToken": []byte("example")}}
				return nil
			},
			key:          "apiToken",
			expectedByte: []byte("example"),
		},
		{
			name: "uses default namespace when omitted",
			credentialsRef: corev1.SecretReference{
				Name: "example",
			},
			getBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				assert.Equal(t, types.NamespacedName{Name: "example", Namespace: "default"}, key)
				*obj = corev1.Secret{Data: map[string][]byte{"apiToken": []byte("example")}}
				return nil
			},
			key:          "apiToken",
			expectedByte: []byte("example"),
		},
		{
			name: "propagates client get error",
			credentialsRef: corev1.SecretReference{
				Name:      "example",
				Namespace: "test",
			},
			getBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return errors.New("Could not find the secret")
			},
			key:           "apiToken",
			expectedError: "get credentials secret test/example: Could not find the secret",
		},
		{
			name: "fails when key is missing",
			credentialsRef: corev1.SecretReference{
				Name:      "example",
				Namespace: "test",
			},
			getBehavior: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
			key:           "apiToken",
			expectedError: "no apiToken key in credentials secret test/example",
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockK8sClient(ctrl)
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testCase.getBehavior)

			got, err := GetCredentialDataFromRef(t.Context(), mockClient, testCase.credentialsRef, "default", testCase.key)

			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
				return
			}

			assert.Equal(t, testCase.expectedByte, got)
		})
	}
}
