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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
)

var (
	// The capability string indicating a region supports Object Storage: [Object Storage Availability]
	//
	// [Object Storage Availability]: https://www.linode.com/docs/products/storage/object-storage/#availability
	LinodeObjectStorageCapability = "Object Storage"
)

// log is for logging in this package.
var linodeobjectstoragebucketlog = logf.Log.WithName("linodeobjectstoragebucket-resource")

// SetupLinodeObjectStorageBucketWebhookWithManager registers the webhook for LinodeObjectStorageBucket in the manager.
func SetupLinodeObjectStorageBucketWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1alpha2.LinodeObjectStorageBucket{}).
		WithValidator(&LinodeObjectStorageBucketCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodeobjectstoragebucket,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets,verbs=create;update,versions=v1alpha2,name=validation.linodeobjectstoragebucket.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// LinodeObjectStorageBucketCustomValidator struct is responsible for validating the LinodeObjectStorageBucket resource
type LinodeObjectStorageBucketCustomValidator struct{}

var _ webhook.CustomValidator = &LinodeObjectStorageBucketCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type LinodeObjectStorageBucket.
func (v *LinodeObjectStorageBucketCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	bucket, ok := obj.(*infrav1alpha2.LinodeObjectStorageBucket)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeObjectStorageBucket object but got %T", obj)
	}
	linodeobjectstoragebucketlog.Info("validate create", "name", bucket.Name)

	return nil, v.validateLinodeObjectStorageBucket(ctx, bucket, &defaultLinodeClient)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type LinodeObjectStorageBucket.
func (v *LinodeObjectStorageBucketCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	bucket, ok := newObj.(*infrav1alpha2.LinodeObjectStorageBucket)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeObjectStorageBucket object but got %T", newObj)
	}
	linodeobjectstoragebucketlog.Info("validate update", "name", bucket.Name)

	return nil, v.validateLinodeObjectStorageBucket(ctx, bucket, &defaultLinodeClient)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type LinodeObjectStorageBucket.
func (v *LinodeObjectStorageBucketCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	bucket, ok := obj.(*infrav1alpha2.LinodeObjectStorageBucket)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeObjectStorageBucket object but got %T", obj)
	}
	linodeobjectstoragebucketlog.Info("validate delete", "name", bucket.Name)

	// No validation needed for deletion
	return nil, nil
}

func (v *LinodeObjectStorageBucketCustomValidator) validateLinodeObjectStorageBucket(ctx context.Context, bucket *infrav1alpha2.LinodeObjectStorageBucket, client clients.LinodeClient) error {
	var errs field.ErrorList

	if err := v.validateLinodeObjectStorageBucketSpec(ctx, bucket, client); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeObjectStorageBucket"},
		bucket.Name, errs)
}

func (v *LinodeObjectStorageBucketCustomValidator) validateLinodeObjectStorageBucketSpec(ctx context.Context, bucket *infrav1alpha2.LinodeObjectStorageBucket, client clients.LinodeClient) field.ErrorList {
	var errs field.ErrorList

	if err := validateObjectStorageRegion(ctx, client, bucket.Spec.Region, field.NewPath("spec").Child("region")); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}
