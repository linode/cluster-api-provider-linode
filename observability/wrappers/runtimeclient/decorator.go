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

package runtimeclient

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/observability/wrappers"
)

const (
	KeyParam   = "key"
	ListParam  = "list" // ignored
	ObjParam   = "obj"
	OptsParam  = "opts" // ignored
	PatchParam = "patch"
)

func DefaultDecorator() func(span trace.Span, params, results map[string]interface{}) {
	return func(span trace.Span, params, results map[string]interface{}) {
		attr := []attribute.KeyValue{}

		if obj, ok := wrappers.GetValue[client.Object](params, ObjParam); ok {
			attr = append(attr,
				attribute.String("object.metadata.name", obj.GetName()),
				attribute.String("object.metadata.namespace", obj.GetNamespace()),
				attribute.String("object.metadata.uid", string(obj.GetUID())),
				attribute.String("object.group", obj.GetObjectKind().GroupVersionKind().Group),
				attribute.String("object.version", obj.GetObjectKind().GroupVersionKind().Version),
				attribute.String("object.kind", obj.GetObjectKind().GroupVersionKind().Kind),
			)
		}

		if key, ok := wrappers.GetValue[types.NamespacedName](params, KeyParam); ok {
			attr = append(attr,
				attribute.String("key.metadata.name", key.Name),
				attribute.String("key.metadata.namespace", key.Namespace),
			)
		}

		if patch, ok := wrappers.GetValue[client.Patch](params, KeyParam); ok {
			attr = append(attr,
				attribute.String("patch.type", string(patch.Type())),
			)
		}

		span.SetAttributes(attr...)
	}
}
