package scope

import (
	"context"
	"fmt"
	"net/http"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/version"
)

func CreateLinodeClient(apiKey string) *linodego.Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}
	linodeClient := linodego.NewClient(oauth2Client)

	linodeClient.SetUserAgent(fmt.Sprintf("CAPL/%s", version.GetVersion()))

	return &linodeClient
}

func getCredentialDataFromRef(ctx context.Context, crClient client.Client, credentialsRef *corev1.SecretReference) ([]byte, error) {
	secretRefName := client.ObjectKey{
		Name:      credentialsRef.Name,
		Namespace: credentialsRef.Namespace,
	}

	var credSecret corev1.Secret
	if err := crClient.Get(ctx, secretRefName, &credSecret); err != nil {
		return nil, fmt.Errorf("failed to retrieve configured credentials secret %s: %w", secretRefName.String(), err)
	}

	rawData, ok := credSecret.Data["apiToken"]
	if !ok {
		return nil, fmt.Errorf("credentials secret %s is missing an apiToken key", secretRefName.String())
	}

	return rawData, nil
}
