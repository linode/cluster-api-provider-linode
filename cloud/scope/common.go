package scope

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/edgegrid"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/session"
	"github.com/linode/linodego"
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

	// MaxBodySize is the max payload size for Akamai edge dns client requests
	maxBody = 131072
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

	linodeClient := linodego.NewClient(nil)
	linodeClient.SetToken(apiKey)
	linodeClient.SetUserAgent(fmt.Sprintf("CAPL/%s", version.GetVersion()))

	for _, opt := range opts {
		opt.set(&linodeClient)
	}

	return linodeclient.NewLinodeClientWithTracing(
		&linodeClient,
		linodeclient.DefaultDecorator(),
	), nil
}

func setUpEdgeDNSInterface() (dnsInterface dns.DNS, err error) {
	edgeRCConfig := edgegrid.Config{
		Host:         os.Getenv("AKAMAI_HOST"),
		AccessToken:  os.Getenv("AKAMAI_ACCESS_TOKEN"),
		ClientToken:  os.Getenv("AKAMAI_CLIENT_TOKEN"),
		ClientSecret: os.Getenv("AKAMAI_CLIENT_SECRET"),
		MaxBody:      maxBody,
	}
	sess, err := session.New(session.WithSigner(&edgeRCConfig))
	if err != nil {
		return nil, err
	}
	return dns.Client(sess), nil
}

func getCredentialDataFromRef(ctx context.Context, crClient K8sClient, credentialsRef corev1.SecretReference, defaultNamespace, key string) ([]byte, error) {
	credSecret, err := getCredentials(ctx, crClient, credentialsRef, defaultNamespace)
	if err != nil {
		return nil, err
	}
	rawData, ok := credSecret.Data[key]
	if !ok {
		return nil, fmt.Errorf("no %s key in credentials secret %s/%s", key, credentialsRef.Namespace, credentialsRef.Name)
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
