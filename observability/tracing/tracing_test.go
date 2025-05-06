/*
Copyright 2024 Akamai Technologies, Inc.

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

package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestSetup(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		env      map[string]string
		resource *resource.Resource
	}{
		"smoke": {
			env:      make(map[string]string),
			resource: resource.Default(),
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			shutdown, err := Setup(ctx, tc.resource)
			require.NoError(t, err)

			err = shutdown(ctx)
			assert.NoError(t, err)
		})
	}
}
