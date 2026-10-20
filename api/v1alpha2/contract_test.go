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

	patchPaths := readKustomizationPatchPaths(t)
	for _, plural := range []string{
		"linodeclusters",
		"linodeclustertemplates",
		"linodemachines",
		"linodemachinetemplates",
	} {
		t.Run(plural, func(t *testing.T) {
			t.Parallel()

			patchPath := filepath.Join("..", "..", "config", "crd", "patches", "capicontract_in_"+plural+".yaml")
			patch := readCRD(t, patchPath)
			patchPathForKustomize := filepath.ToSlash(filepath.Join("patches", "capicontract_in_"+plural+".yaml"))
			assert.Contains(t, patchPaths, patchPathForKustomize)

			basePath := filepath.Join("..", "..", "config", "crd", "bases", "infrastructure.cluster.x-k8s.io_"+plural+".yaml")
			base := readCRD(t, basePath)
			assertCAPIContractLabels(t, patch, base)
		})
	}
}

func readCRD(t *testing.T, path string) apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read CRD %q: %v", path, err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("unmarshal CRD %q: %v", path, err)
	}
	return crd
}

func readKustomizationPatchPaths(t *testing.T) map[string]struct{} {
	t.Helper()

	path := filepath.Join("..", "..", "config", "crd", "kustomization.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kustomization %q: %v", path, err)
	}

	var kustomization struct {
		Patches []struct {
			Path string `json:"path,omitempty" yaml:"path,omitempty"`
		} `json:"patches,omitempty" yaml:"patches,omitempty"`
	}
	if err := yaml.Unmarshal(data, &kustomization); err != nil {
		t.Fatalf("unmarshal kustomization %q: %v", path, err)
	}

	patchPaths := make(map[string]struct{}, len(kustomization.Patches))
	for _, patch := range kustomization.Patches {
		patchPaths[patch.Path] = struct{}{}
	}
	return patchPaths
}

func assertCAPIContractLabels(t *testing.T, patch, base apiextensionsv1.CustomResourceDefinition) {
	t.Helper()

	expected := map[string]string{
		"cluster.x-k8s.io/v1beta1": "v1alpha2",
		"cluster.x-k8s.io/v1beta2": "v1alpha2",
	}
	assert.Equal(t, expected, patch.Labels)
	for labelKey, labelValue := range expected {
		for _, versionName := range strings.Split(labelValue, "_") {
			assert.True(t, crdServesVersion(base, versionName), "label %q version %q must be served by %s", labelKey, versionName, base.Name)
		}
	}
}

func crdServesVersion(crd apiextensionsv1.CustomResourceDefinition, name string) bool {
	for _, version := range crd.Spec.Versions {
		if version.Name == name && version.Served {
			return true
		}
	}
	return false
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
