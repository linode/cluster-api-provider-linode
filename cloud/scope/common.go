package scope

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/edgegrid"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/session"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/observability/wrappers/linodeclient"
	"github.com/linode/cluster-api-provider-linode/version"
)

const (
	// defaultClientTimeout is the default timeout for a client Linode API call
	defaultClientTimeout = time.Second * 10

	// MaxBodySize is the max payload size for Akamai edge dns client requests
	maxBody = 131072

	// defaultObjectStorageSignedUrlExpiry is the default expiration for Object Storage signed URls
	defaultObjectStorageSignedUrlExpiry = 15 * time.Minute
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

type ClientConfig struct {
	Token               string
	BaseUrl             string
	RootCertificatePath string

	Timeout time.Duration
}

func CreateLinodeClient(config ClientConfig, opts ...Option) (clients.LinodeClient, error) {
	if config.Token == "" {
		return nil, errors.New("token cannot be empty")
	}

	timeout := defaultClientTimeout
	if config.Timeout != 0 {
		timeout = config.Timeout
	}

	// Use system cert pool if root CA cert was not provided explicitly for this client.
	// Works around linodego not using system certs if LINODE_CA is provided,
	// which affects all clients spawned via linodego.NewClient
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if config.RootCertificatePath == "" {
		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to load system cert pool: %w", err)
		}
		tlsConfig.RootCAs = systemCertPool
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	newClient := linodego.NewClient(httpClient)
	newClient.SetToken(config.Token)
	if config.RootCertificatePath != "" {
		newClient.SetRootCertificate(config.RootCertificatePath)
	}
	if config.BaseUrl != "" {
		_, err := newClient.UseURL(config.BaseUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to set base URL: %w", err)
		}
	}
	newClient.SetUserAgent(fmt.Sprintf("CAPL/%s", version.GetVersion()))

	for _, opt := range opts {
		opt.set(&newClient)
	}

	return linodeclient.NewLinodeClientWithTracing(
		&newClient,
		linodeclient.DefaultDecorator(),
	), nil
}

func CreateS3Clients(ctx context.Context, crClient clients.K8sClient, cluster infrav1alpha2.LinodeCluster) (clients.S3Client, clients.S3PresignClient, error) {
	var (
		configOpts = []func(*awsconfig.LoadOptions) error{
			awsconfig.WithRegion("auto"),
			awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
			awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
		}

		clientOpts = []func(*s3.Options){}
	)

	// If we have a cluster object store bucket, get its configuration.
	if cluster.Spec.ObjectStore != nil {
		objSecret, err := getCredentials(ctx, crClient, cluster.Spec.ObjectStore.CredentialsRef, cluster.GetNamespace())
		if err == nil {
			var (
				access   = string(objSecret.Data["access"])
				secret   = string(objSecret.Data["secret"])
				endpoint = string(objSecret.Data["endpoint"])
			)

			configOpts = append(configOpts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, "")))
			clientOpts = append(clientOpts, func(opts *s3.Options) {
				opts.BaseEndpoint = aws.String(endpoint)
				opts.UsePathStyle = strings.EqualFold(os.Getenv("LINODE_OBJECT_STORAGE_USE_PATH_STYLE"), "true")
			})
		}
	}

	config, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("load s3 config: %w", err)
	}

	var (
		s3Client        = s3.NewFromConfig(config, clientOpts...)
		s3PresignClient = s3.NewPresignClient(s3Client, func(opts *s3.PresignOptions) {
			opts.Expires = defaultObjectStorageSignedUrlExpiry
		})
	)

	return s3Client, s3PresignClient, nil
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

func getCredentialDataFromRef(ctx context.Context, crClient clients.K8sClient, credentialsRef corev1.SecretReference, defaultNamespace, key string) ([]byte, error) {
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

func addCredentialsFinalizer(ctx context.Context, crClient clients.K8sClient, credentialsRef corev1.SecretReference, defaultNamespace, finalizer string) error {
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

func removeCredentialsFinalizer(ctx context.Context, crClient clients.K8sClient, credentialsRef corev1.SecretReference, defaultNamespace, finalizer string) error {
	secret, err := getCredentials(ctx, crClient, credentialsRef, defaultNamespace)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	controllerutil.RemoveFinalizer(secret, finalizer)
	if err := crClient.Update(ctx, secret); err != nil {
		return fmt.Errorf("remove finalizer from credentials secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

func getCredentials(ctx context.Context, crClient clients.K8sClient, credentialsRef corev1.SecretReference, defaultNamespace string) (*corev1.Secret, error) {
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

// GetHash returns sha256 hash of input string
func GetHash(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
