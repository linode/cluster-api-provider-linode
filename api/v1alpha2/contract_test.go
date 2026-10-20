/*
Copyright 2026 Akamai Technologies, Inc.

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func TestCAPIContractLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		plural string
	}{
		{plural: "linodeclusters"},
		{plural: "linodeclustertemplates"},
		{plural: "linodemachines"},
		{plural: "linodemachinetemplates"},
	}

	kustomizationData, err := os.ReadFile(filepath.Join("..", "..", "config", "crd", "kustomization.yaml"))
	if !assert.NoError(t, err) {
		return
	}
	var kustomization struct {
		Patches []struct {
			Path string `json:"path,omitempty" yaml:"path,omitempty"`
		} `json:"patches,omitempty" yaml:"patches,omitempty"`
	}
	if !assert.NoError(t, yaml.Unmarshal(kustomizationData, &kustomization)) {
		return
	}
	patchPaths := make(map[string]struct{}, len(kustomization.Patches))
	for _, patch := range kustomization.Patches {
		patchPaths[patch.Path] = struct{}{}
	}

	for _, tc := range tests {
		t.Run(tc.plural, func(t *testing.T) {
			t.Parallel()

			patchPath := filepath.Join("..", "..", "config", "crd", "patches", "capicontract_in_"+tc.plural+".yaml")
			patchData, err := os.ReadFile(patchPath)
			if !assert.NoError(t, err) {
				return
			}
			var patch apiextensionsv1.CustomResourceDefinition
			if !assert.NoError(t, yaml.Unmarshal(patchData, &patch)) {
				return
			}

			patchPathForKustomize := filepath.ToSlash(filepath.Join("patches", "capicontract_in_"+tc.plural+".yaml"))
			_, included := patchPaths[patchPathForKustomize]
			assert.True(t, included, "contract patch %q must be included by config/crd/kustomization.yaml", patchPathForKustomize)

			basePath := filepath.Join("..", "..", "config", "crd", "bases", "infrastructure.cluster.x-k8s.io_"+tc.plural+".yaml")
			baseData, err := os.ReadFile(basePath)
			if !assert.NoError(t, err) {
				return
			}
			var base apiextensionsv1.CustomResourceDefinition
			if !assert.NoError(t, yaml.Unmarshal(baseData, &base)) {
				return
			}

			assert.Equal(t, map[string]string{
				"cluster.x-k8s.io/v1beta1": "v1alpha2",
				"cluster.x-k8s.io/v1beta2": "v1alpha2",
			}, patch.Labels)

			for _, labelKey := range []string{"cluster.x-k8s.io/v1beta1", "cluster.x-k8s.io/v1beta2"} {
				labelValue := patch.Labels[labelKey]
				for _, versionName := range strings.Split(labelValue, "_") {
					served := false
					for _, version := range base.Spec.Versions {
						if version.Name == versionName && version.Served {
							served = true
							break
						}
					}
					assert.True(t, served, "label %q version %q must be served by %s", labelKey, versionName, base.Name)
				}
			}
		})
	}
}

func TestInitializationStatusSchema(t *testing.T) {
	t.Parallel()

	for _, plural := range []string{"linodeclusters", "linodemachines"} {
		t.Run(plural, func(t *testing.T) {
			t.Parallel()

			basePath := filepath.Join("..", "..", "config", "crd", "bases", "infrastructure.cluster.x-k8s.io_"+plural+".yaml")
			data, err := os.ReadFile(basePath)
			if !assert.NoError(t, err) {
				return
			}
			var crd apiextensionsv1.CustomResourceDefinition
			if !assert.NoError(t, yaml.Unmarshal(data, &crd)) {
				return
			}

			var storageVersion *apiextensionsv1.CustomResourceDefinitionVersion
			for i := range crd.Spec.Versions {
				if crd.Spec.Versions[i].Storage {
					storageVersion = &crd.Spec.Versions[i]
					break
				}
			}
			if !assert.NotNil(t, storageVersion, "CRD must have a storage version") || !assert.NotNil(t, storageVersion.Schema) {
				return
			}
			schema := storageVersion.Schema.OpenAPIV3Schema
			if !assert.NotNil(t, schema) {
				return
			}
			status, ok := schema.Properties["status"]
			if !assert.True(t, ok, "status schema must exist") {
				return
			}
			initialization, ok := status.Properties["initialization"]
			if !assert.True(t, ok, "initialization schema must exist") {
				return
			}
			provisioned, ok := initialization.Properties["provisioned"]
			if !assert.True(t, ok, "provisioned schema must exist") {
				return
			}
			assert.Equal(t, "boolean", provisioned.Type)
		})
	}
}
