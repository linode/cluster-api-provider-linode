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

package runtimereconciler

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/linode/cluster-api-provider-linode/observability/wrappers"
)

const (
	RequestParam = "r1"
	ResultParam  = "r2"
)

func DefaultDecorator() func(span trace.Span, params, results map[string]interface{}) {
	return func(span trace.Span, params, results map[string]interface{}) {
		attr := []attribute.KeyValue{}

		if req, ok := wrappers.GetValue[reconcile.Request](params, RequestParam); ok {
			attr = append(attr,
				attribute.String("request.name", req.Name),
				attribute.String("request.namespace", req.Namespace),
			)
		}

		if res, ok := wrappers.GetValue[reconcile.Result](params, RequestParam); ok {
			attr = append(attr,
				attribute.String("result.requeue_after", res.RequeueAfter.String()),
			)
		}

		span.SetAttributes(attr...)
	}
}
