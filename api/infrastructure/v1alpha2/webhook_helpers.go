package v1alpha2

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/linode/linodego"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

const (
	// defaultWebhookTimeout is the default timeout for an admission request
	defaultWebhookTimeout = time.Second * 10
	// defaultClientTimeout is the default timeout for a client Linode API call
	defaultClientTimeout = time.Second * 10
)

var (
	// defaultLinodeClient is an unauthenticated Linode client
	defaultLinodeClient = linodego.NewClient(&http.Client{Timeout: defaultClientTimeout})
)

func validateRegion(ctx context.Context, client LinodeClient, id string, path *field.Path, capabilities ...string) *field.Error {
	region, err := client.GetRegion(ctx, id)
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
