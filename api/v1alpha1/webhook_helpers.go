package v1alpha1

import (
	"context"
	"net/http"
	"time"

	"github.com/linode/linodego"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

const (
	// defaultWebhookTimeout is the default timeout for an admission request
	defaultWebhookTimeout = time.Minute
)

var (
	// defaultLinodeClient is an unauthenticated Linode client
	defaultLinodeClient = linodego.NewClient(http.DefaultClient)
)

func validateRegion(ctx context.Context, client LinodeClient, id string, path *field.Path) *field.Error {
	_, err := client.GetRegion(ctx, id)
	if err != nil {
		return field.NotFound(path, id)
	}

	return nil
}

func validateLinodeType(ctx context.Context, client LinodeClient, id string, path *field.Path) (*linodego.LinodeType, *field.Error) {
	plan, err := client.GetType(ctx, id)
	if err != nil {
		return nil, field.NotFound(path, id)
	}

	return plan, nil
}
