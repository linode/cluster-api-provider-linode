package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/clients"
)

// GetCredentialDataFromRef fetches a secret value from a referenced credentials secret.
func GetCredentialDataFromRef(ctx context.Context, crClient clients.K8sClient, credentialsRef corev1.SecretReference, defaultNamespace, key string) ([]byte, error) {
	credSecret, err := GetCredentials(ctx, crClient, credentialsRef, defaultNamespace)
	if err != nil {
		return nil, err
	}
	rawData, ok := credSecret.Data[key]
	if !ok {
		return nil, fmt.Errorf("no %s key in credentials secret %s/%s", key, credentialsRef.Namespace, credentialsRef.Name)
	}

	return rawData, nil
}

// GetCredentials fetches a referenced credentials secret, defaulting the namespace when omitted.
func GetCredentials(ctx context.Context, crClient clients.K8sClient, credentialsRef corev1.SecretReference, defaultNamespace string) (*corev1.Secret, error) {
	secretRef := client.ObjectKey{
		Name:      credentialsRef.Name,
		Namespace: credentialsRef.Namespace,
	}
	if secretRef.Namespace == "" {
		secretRef.Namespace = defaultNamespace
	}

	var credSecret corev1.Secret
	if err := crClient.Get(ctx, secretRef, &credSecret); err != nil {
		return nil, fmt.Errorf("get credentials secret %s/%s: %w", secretRef.Namespace, secretRef.Name, err)
	}

	return &credSecret, nil
}
