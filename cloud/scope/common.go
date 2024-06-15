package scope

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/linode/cluster-api-provider-linode/observability/wrappers/linodeclient"
	"github.com/linode/cluster-api-provider-linode/version"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

const (
	// defaultClientTimeout is the default timeout for a client Linode API call
	defaultClientTimeout = time.Second * 10
)

type Option struct {
	set func(client *linodego.Client)
}

func WithRetryCount(c int) Option {
	return Option{
		set: func(client *linodego.Client) {
			client.SetRetryCount(c)
		},
	}
}

func CreateLinodeClient(apiKey string, timeout time.Duration, opts ...Option) (LinodeClient, error) {
	if apiKey == "" {
		return nil, errors.New("missing Linode API key")
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
		Timeout: timeout,
	}
	linodeClient := linodego.NewClient(oauth2Client)

	linodeClient.SetUserAgent(fmt.Sprintf("CAPL/%s", version.GetVersion()))

	for _, opt := range opts {
		opt.set(&linodeClient)
	}

	return linodeclient.NewLinodeClientWithTracing(
		&linodeClient,
	), nil
}

func getCredentialDataFromRef(ctx context.Context, crClient K8sClient, credentialsRef corev1.SecretReference, defaultNamespace string) ([]byte, error) {
	credSecret, err := getCredentials(ctx, crClient, credentialsRef, defaultNamespace)
	if err != nil {
		return nil, err
	}

	// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
	rawData, ok := credSecret.Data["apiToken"]
	if !ok {
		return nil, fmt.Errorf("no apiToken key in credentials secret %s/%s", credentialsRef.Namespace, credentialsRef.Name)
	}

	return rawData, nil
}

func addCredentialsFinalizer(ctx context.Context, crClient K8sClient, credentialsRef corev1.SecretReference, defaultNamespace, finalizer string) error {
	secret, err := getCredentials(ctx, crClient, credentialsRef, defaultNamespace)
	if err != nil {
		return err
	}

	controllerutil.AddFinalizer(secret, finalizer)
	if err := crClient.Update(ctx, secret); err != nil {
		return fmt.Errorf("add finalizer to credentials secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

func removeCredentialsFinalizer(ctx context.Context, crClient K8sClient, credentialsRef corev1.SecretReference, defaultNamespace, finalizer string) error {
	secret, err := getCredentials(ctx, crClient, credentialsRef, defaultNamespace)
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(secret, finalizer)
	if err := crClient.Update(ctx, secret); err != nil {
		return fmt.Errorf("remove finalizer from credentials secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

func getCredentials(ctx context.Context, crClient K8sClient, credentialsRef corev1.SecretReference, defaultNamespace string) (*corev1.Secret, error) {
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

// toFinalizer converts an object into a valid finalizer key representation
func toFinalizer(obj client.Object) string {
	var (
		gvk       = obj.GetObjectKind().GroupVersionKind()
		group     = gvk.Group
		kind      = strings.ToLower(gvk.Kind)
		namespace = obj.GetNamespace()
		name      = obj.GetName()
	)
	if namespace == "" {
		return fmt.Sprintf("%s.%s/%s", kind, group, name)
	}
	return fmt.Sprintf("%s.%s/%s.%s", kind, group, namespace, name)
}
