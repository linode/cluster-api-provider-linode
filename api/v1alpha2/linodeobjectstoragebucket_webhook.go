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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

var (
	// The capability string indicating a region supports Object Storage: [Object Storage Availability]
	//
	// [Object Storage Availability]: https://www.linode.com/docs/products/storage/object-storage/#availability
	LinodeObjectStorageCapability = "Object Storage"
)

// log is for logging in this package.
var linodeobjectstoragebucketlog = logf.Log.WithName("linodeobjectstoragebucket-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LinodeObjectStorageBucket) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable update and deletion validation.
//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodeobjectstoragebucket,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeobjectstoragebuckets,verbs=create,versions=v1alpha2,name=validation.linodeobjectstoragebucket.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1;v1alpha1;v1alpha2

var _ webhook.Validator = &LinodeObjectStorageBucket{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeObjectStorageBucket) ValidateCreate() (admission.Warnings, error) {
	linodeobjectstoragebucketlog.Info("validate create", "name", r.Name)

	ctx, cancel := context.WithTimeout(context.Background(), defaultWebhookTimeout)
	defer cancel()

	return nil, r.validateLinodeObjectStorageBucket(ctx, &defaultLinodeClient)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeObjectStorageBucket) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	linodeobjectstoragebucketlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeObjectStorageBucket) ValidateDelete() (admission.Warnings, error) {
	linodeobjectstoragebucketlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *LinodeObjectStorageBucket) validateLinodeObjectStorageBucket(ctx context.Context, client LinodeClient) error {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := r.validateLinodeObjectStorageBucketSpec(ctx, client); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeObjectStorageBucket"},
		r.Name, errs)
}

func (r *LinodeObjectStorageBucket) validateLinodeObjectStorageBucketSpec(ctx context.Context, client LinodeClient) field.ErrorList {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	// Handle this
	var errs field.ErrorList

	if err := validateObjectStorageRegion(ctx, client, r.Spec.Region, field.NewPath("spec").Child("cluster")); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}
