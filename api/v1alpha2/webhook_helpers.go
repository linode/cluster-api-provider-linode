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
	"slices"
	"time"

	"github.com/linode/linodego"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/linode/cluster-api-provider-linode/observability/wrappers/linodeclient"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

const (
	// defaultWebhookTimeout is the default timeout for an admission request
	defaultWebhookTimeout = time.Second * 10
	// defaultClientTimeout is the default timeout for a client Linode API call
	defaultClientTimeout = time.Second * 10
)

func mkptr[T any](v T) *T {
	return &v
}

var (
	// defaultLinodeClient is an unauthenticated Linode client
	defaultLinodeClient = linodeclient.NewLinodeClientWithTracing(
		mkptr(linodego.NewClient(&http.Client{Timeout: defaultClientTimeout})),
	)
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

func validateLinodeType(ctx context.Context, client LinodeClient, id string, path *field.Path) (*linodego.LinodeType, *field.Error) {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	plan, err := client.GetType(ctx, id)
	if err != nil {
		return nil, field.NotFound(path, id)
	}

	return plan, nil
}
