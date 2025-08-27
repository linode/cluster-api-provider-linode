/*
Copyright 2024.

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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusteraddonsv1 "sigs.k8s.io/cluster-api/api/addons/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

// log is for logging in this package.
var linodeobjectstoragekeylog = logf.Log.WithName("linodeobjectstoragekey-resource")

const defaultKeySecretNameTemplate = "%s-obj-key"

// SetupLinodeObjectStorageKeyWebhookWithManager registers the webhook for LinodeObjectStorageKey in the manager.
func SetupLinodeObjectStorageKeyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1alpha2.LinodeObjectStorageKey{}).
		WithValidator(&LinodeObjectStorageKeyCustomValidator{}).
		WithDefaulter(&LinodeObjectStorageKeyDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodeobjectstoragekey,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekeys,verbs=create;update,versions=v1alpha2,name=validation.linodeobjectstoragekey.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// LinodeObjectStorageKeyCustomValidator struct is responsible for validating the LinodeObjectStorageKey resource
type LinodeObjectStorageKeyCustomValidator struct{}

var _ webhook.CustomValidator = &LinodeObjectStorageKeyCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type LinodeObjectStorageKey.
func (v *LinodeObjectStorageKeyCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	key, ok := obj.(*infrav1alpha2.LinodeObjectStorageKey)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeObjectStorageKey object but got %T", obj)
	}
	linodeobjectstoragekeylog.Info("Validation for LinodeObjectStorageKey upon creation", "name", key.GetName())

	var errs field.ErrorList
	if err := validateLabelLength(key.GetName(), field.NewPath("metadata").Child("name")); err != nil {
		errs = append(errs, err)
	}
	if err := v.validateLinodeObjectStorageKey(key); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeObjectStorageKey"},
		key.Name, errs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type LinodeObjectStorageKey.
func (v *LinodeObjectStorageKeyCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	key, ok := newObj.(*infrav1alpha2.LinodeObjectStorageKey)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeObjectStorageKey object but got %T", newObj)
	}
	linodeobjectstoragekeylog.Info("Validation for LinodeObjectStorageKey upon update", "name", key.GetName())

	errs := v.validateLinodeObjectStorageKey(key)

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeObjectStorageKey"},
		key.Name, errs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type LinodeObjectStorageKey.
func (v *LinodeObjectStorageKeyCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	key, ok := obj.(*infrav1alpha2.LinodeObjectStorageKey)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeObjectStorageKey object but got %T", obj)
	}
	linodeobjectstoragekeylog.Info("Validation for LinodeObjectStorageKey upon deletion", "name", key.GetName())

	return nil, nil
}

func (v *LinodeObjectStorageKeyCustomValidator) validateLinodeObjectStorageKey(key *infrav1alpha2.LinodeObjectStorageKey) field.ErrorList {
	var errs field.ErrorList

	if key.Spec.Type == clusteraddonsv1.ClusterResourceSetSecretType && len(key.Spec.Format) == 0 {
		errs = append(errs, field.Invalid(
			field.NewPath("spec").Child("generatedSecret").Child("format"),
			key.Spec.Format,
			fmt.Sprintf("must not be empty with Secret type %s", clusteraddonsv1.ClusterResourceSetSecretType),
		))
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha2-linodeobjectstoragekey,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragekeys,verbs=create;update,versions=v1alpha2,name=mutation.linodeobjectstoragekey.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// LinodeObjectStorageKeyDefaulter struct is responsible for defaulting the LinodeObjectStorageKey resource
type LinodeObjectStorageKeyDefaulter struct{}

var _ webhook.CustomDefaulter = &LinodeObjectStorageKeyDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type LinodeObjectStorageKey.
func (d *LinodeObjectStorageKeyDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	key, ok := obj.(*infrav1alpha2.LinodeObjectStorageKey)
	if !ok {
		return fmt.Errorf("expected a LinodeObjectStorageKey object but got %T", obj)
	}
	linodeobjectstoragekeylog.Info("Defaulting for LinodeObjectStorageKey", "name", key.GetName())

	// Default name and namespace derived from object metadata.
	if key.Spec.Name == "" {
		key.Spec.Name = fmt.Sprintf(defaultKeySecretNameTemplate, key.Name)
	}
	if key.Spec.Namespace == "" {
		key.Spec.Namespace = key.Namespace
	}

	// Support deprecated fields when specified and updated fields are empty.
	if key.Spec.SecretType != "" && key.Spec.Type == "" {
		key.Spec.Type = key.Spec.SecretType
	}
	if len(key.Spec.SecretDataFormat) > 0 && len(key.Spec.Format) == 0 {
		key.Spec.Format = key.Spec.SecretDataFormat
	}

	return nil
}
