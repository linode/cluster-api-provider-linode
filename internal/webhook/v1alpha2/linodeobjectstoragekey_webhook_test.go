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
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusteraddonsv1 "sigs.k8s.io/cluster-api/api/addons/v1beta2"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestValidateLinodeObjectStorageKeyCreate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		lfw = infrav1alpha2.LinodeObjectStorageKey{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeObjectStorageKeySpec{},
		}
		lfwLongName = infrav1alpha2.LinodeObjectStorageKey{
			ObjectMeta: metav1.ObjectMeta{
				Name:      longName,
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeObjectStorageKeySpec{},
		}
		validator = &LinodeObjectStorageKeyCustomValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("name too long", func(ctx context.Context, mck Mock) {

				}),
				Result("error", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &lfwLongName)
					assert.ErrorContains(t, err, labelLengthDetail)
				}),
			),
		),
		OneOf(
			Path(
				Call("valid", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateCreate(ctx, &lfw)
					require.NoError(t, err)
				}),
			),
		),
	)
}

func TestValidateLinodeObjectStorageKeyUpdate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		oldKey = infrav1alpha2.LinodeObjectStorageKey{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeObjectStorageKeySpec{},
		}
		newKey = infrav1alpha2.LinodeObjectStorageKey{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeObjectStorageKeySpec{},
		}

		validator = &LinodeObjectStorageKeyCustomValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("update", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateUpdate(ctx, &oldKey, &newKey)
					assert.NoError(t, err)
				}),
			),
		),
	)
}

func TestValidateLinodeObjectStorageKeyDelete(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		key = infrav1alpha2.LinodeObjectStorageKey{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "example",
			},
			Spec: infrav1alpha2.LinodeObjectStorageKeySpec{},
		}

		validator = &LinodeObjectStorageKeyCustomValidator{}
	)

	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("delete", func(ctx context.Context, mck Mock) {

				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := validator.ValidateDelete(ctx, &key)
					assert.NoError(t, err)
				}),
			),
		),
	)
}

func TestValidateLinodeObjectStorageKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec infrav1alpha2.LinodeObjectStorageKeySpec
		err  error
	}{
		{
			name: "opaque",
			spec: infrav1alpha2.LinodeObjectStorageKeySpec{
				GeneratedSecret: infrav1alpha2.GeneratedSecret{
					Type: corev1.SecretTypeOpaque,
				},
			},
			err: nil,
		},
		{
			name: "resourceset with empty secret data format",
			spec: infrav1alpha2.LinodeObjectStorageKeySpec{
				GeneratedSecret: infrav1alpha2.GeneratedSecret{
					Type:   clusteraddonsv1.ClusterResourceSetSecretType,
					Format: map[string]string{},
				},
			},
			err: errors.New("must not be empty with Secret type"),
		},
		{
			name: "valid resourceset",
			spec: infrav1alpha2.LinodeObjectStorageKeySpec{
				GeneratedSecret: infrav1alpha2.GeneratedSecret{
					Type: clusteraddonsv1.ClusterResourceSetSecretType,
					Format: map[string]string{
						"file.yaml": "kind: Secret",
					},
				},
			},
			err: nil,
		},
	}

	validator := LinodeObjectStorageKeyCustomValidator{}

	for _, tt := range tests {
		testcase := tt

		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			key := &infrav1alpha2.LinodeObjectStorageKey{
				Spec: testcase.spec,
			}

			errs := validator.validateLinodeObjectStorageKey(key)
			if errs != nil {
				if testcase.err == nil {
					t.Fatal(errs)
				}
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), testcase.err.Error()) {
						found = true
						break
					}
				}
				if errStr := testcase.err.Error(); !found {
					t.Errorf("errors did not contain substring '%s'", errStr)
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
		genSecret         infrav1alpha2.GeneratedSecret
		expectedName      string
		expectedNamespace string
	}{
		{"already set", infrav1alpha2.GeneratedSecret{Name: "secret", Namespace: "ns"}, "secret", "ns"},
		{"no name", infrav1alpha2.GeneratedSecret{Namespace: "ns"}, "key-obj-key", "ns"},
		{"no namespace", infrav1alpha2.GeneratedSecret{Name: "secret"}, "secret", "keyns"},
	}

	defaulter := LinodeObjectStorageKeyDefaulter{}

	for _, tt := range tests {
		testcase := tt

		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			key := &infrav1alpha2.LinodeObjectStorageKey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "key",
					Namespace: "keyns",
				},
				Spec: infrav1alpha2.LinodeObjectStorageKeySpec{
					GeneratedSecret: testcase.genSecret,
				},
			}

			err := defaulter.Default(t.Context(), key)
			if err != nil {
				t.Fatal(err)
			}

			if key.Spec.Name != testcase.expectedName {
				t.Errorf("name: expected %s but got %s", testcase.expectedName, key.Spec.Name)
			}
			if key.Spec.Namespace != testcase.expectedNamespace {
				t.Errorf("name: expected %s but got %s", testcase.expectedNamespace, key.Spec.Namespace)
			}
		})
	}
}
