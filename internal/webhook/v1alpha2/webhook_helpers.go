/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/observability/wrappers/linodeclient"
)

const (
	// defaultClientTimeout is the default timeout for a client Linode API call
	defaultClientTimeout = time.Second * 10
)

var (
	// defaultLinodeClient is an unauthenticated Linode client
	defaultLinodeClient = linodeclient.NewLinodeClientWithTracing(
		ptr.To(linodego.NewClient(&http.Client{Timeout: defaultClientTimeout})),
		linodeclient.DefaultDecorator(),
	)
)

func validateRegion(ctx context.Context, linodegoclient clients.LinodeClient, id string, path *field.Path, capabilities ...string) *field.Error {
	region, err := linodegoclient.GetRegion(ctx, id)
	if err != nil {
		return field.NotFound(path, id)
	}

	for _, capability := range capabilities {
		if !slices.Contains(region.Capabilities, capability) {
			return field.Invalid(path, id, fmt.Sprintf("no capability: %s", capability))
		}
	}

	return nil
}

func validateLinodeType(ctx context.Context, linodegoclient clients.LinodeClient, id string, path *field.Path) (*linodego.LinodeType, *field.Error) {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	plan, err := linodegoclient.GetType(ctx, id)
	if err != nil {
		return nil, field.NotFound(path, id)
	}

	return plan, nil
}

// validateObjectStorageRegion validates an Object Storage deployment's region ID via the following rules:
//   - The Region ID is in the form: REGION_ID.
//   - The region has Object Storage support.
//
// NOTE: This implementation intended to bypass the authentication requirement for the [Clusters List] and [Cluster
// View] endpoints in the Linode API, thereby reusing a [github.com/linode/linodego.Client] (and its caching if enabled)
// across many admission requests.
//
// [Clusters List]: https://www.linode.com/docs/api/object-storage/#clusters-list
// [Cluster View]: https://www.linode.com/docs/api/object-storage/#cluster-view

func validateObjectStorageRegion(ctx context.Context, linodegoclient clients.LinodeClient, id string, path *field.Path) *field.Error {
	// TODO: instrument with tracing, might need refactor to preserve readibility

	cexp := regexp.MustCompile("^(([[:lower:]]+-)*[[:lower:]]+)$")
	cexp1 := regexp.MustCompile(`^(([[:lower:]]+-)*[[:lower:]]+)-\d+$`)
	if !cexp.MatchString(id) && !cexp1.MatchString(id) {
		return field.Invalid(path, id, "must be in form: region_id or region_id-ordinal")
	}
	var region string
	if cexp.FindStringSubmatch(id) != nil {
		region = cexp.FindStringSubmatch(id)[0]
	} else {
		region = cexp1.FindStringSubmatch(id)[1]
	}
	return validateRegion(ctx, linodegoclient, region, path, LinodeObjectStorageCapability)
}

func getCredentialDataFromRef(ctx context.Context, crClient clients.K8sClient, credentialsRef corev1.SecretReference, defaultNamespace string) ([]byte, error) {
	credSecret, err := getCredentials(ctx, crClient, credentialsRef, defaultNamespace)
	if err != nil {
		return nil, err
	}
	rawData, ok := credSecret.Data["apiToken"]
	if !ok {
		return nil, fmt.Errorf("no %s key in credentials secret %s/%s", "apiToken", credentialsRef.Namespace, credentialsRef.Name)
	}

	return rawData, nil
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

// setupClientWithCredentials configures a Linode client with credentials from a secret reference
// Returns (skipAPIValidation, client) - skipAPIValidation will be true if credentials cannot be found
// and API validation should be skipped
func setupClientWithCredentials(ctx context.Context, crClient clients.K8sClient, credRef *corev1.SecretReference,
	resourceName, namespace string, logger logr.Logger) (bool, clients.LinodeClient) {
	linodeClient := defaultLinodeClient

	apiToken, err := getCredentialDataFromRef(ctx, crClient, *credRef, namespace)
	if err == nil {
		logger.Info("creating a verified linode client for create request", "name", resourceName)
		linodeClient.SetToken(string(apiToken))
		return false, linodeClient
	}

	// Handle error cases
	if apierrors.IsNotFound(err) {
		logger.Info("credentials secret not found, skipping API validation",
			"name", resourceName, "secret", credRef.Name)
		return true, linodeClient
	}

	// For other errors, log the error but return the default client
	// The caller should handle validation with the default client
	logger.Error(err, "failed getting credentials from secret ref", "name", resourceName)
	return false, linodeClient
}
