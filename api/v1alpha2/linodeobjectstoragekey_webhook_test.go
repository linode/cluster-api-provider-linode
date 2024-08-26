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
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusteraddonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
)

func TestValidateLinodeObjectStorageKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec LinodeObjectStorageKeySpec
		err  error
	}{
		{
			name: "opaque",
			spec: LinodeObjectStorageKeySpec{
				GeneratedSecret: GeneratedSecret{
					Type: corev1.SecretTypeOpaque,
				},
			},
			err: nil,
		},
		{
			name: "resourceset with empty secret data format",
			spec: LinodeObjectStorageKeySpec{
				GeneratedSecret: GeneratedSecret{
					Type:   clusteraddonsv1.ClusterResourceSetSecretType,
					Format: map[string]string{},
				},
			},
			err: errors.New("must not be empty with Secret type"),
		},
		{
			name: "valid resourceset",
			spec: LinodeObjectStorageKeySpec{
				GeneratedSecret: GeneratedSecret{
					Type: clusteraddonsv1.ClusterResourceSetSecretType,
					Format: map[string]string{
						"file.yaml": "kind: Secret",
					},
				},
			},
			err: nil,
		},
	}

	for _, tt := range tests {
		testcase := tt

		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			key := LinodeObjectStorageKey{
				Spec: testcase.spec,
			}

			_, err := key.validateLinodeObjectStorageKey()
			if err != nil {
				if testcase.err == nil {
					t.Fatal(err)
				}
				if errStr := testcase.err.Error(); !strings.Contains(err.Error(), errStr) {
					t.Errorf("error did not contain substring '%s'", errStr)
				}
			} else if testcase.err != nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLinodeObjectStorageKeyDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		genSecret         GeneratedSecret
		expectedName      string
		expectedNamespace string
	}{
		{"already set", GeneratedSecret{Name: "secret", Namespace: "ns"}, "secret", "ns"},
		{"no name", GeneratedSecret{Namespace: "ns"}, "key-obj-key", "ns"},
		{"no namespace", GeneratedSecret{Name: "secret"}, "secret", "keyns"},
	}

	for _, tt := range tests {
		testcase := tt

		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			key := &LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "key",
					Namespace: "keyns",
				},
				Spec: LinodeObjectStorageKeySpec{
					GeneratedSecret: testcase.genSecret,
				},
			}

			key.Default()
			if key.Spec.GeneratedSecret.Name != testcase.expectedName {
				t.Errorf("name: expected %s but got %s", testcase.expectedName, key.Spec.GeneratedSecret.Name)
			}
			if key.Spec.GeneratedSecret.Namespace != testcase.expectedNamespace {
				t.Errorf("name: expected %s but got %s", testcase.expectedNamespace, key.Spec.GeneratedSecret.Namespace)
			}
		})
	}
}
