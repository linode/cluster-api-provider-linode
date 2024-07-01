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

// log is for logging in this package.
var linodeclusterlog = logf.Log.WithName("linodecluster-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LinodeCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable updation and deletion validation.
//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters,verbs=create,versions=v1alpha2,name=v1alpha2.linodecluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &LinodeCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeCluster) ValidateCreate() (admission.Warnings, error) {
	linodeclusterlog.Info("validate create", "name", r.Name)

	ctx, cancel := context.WithTimeout(context.Background(), defaultWebhookTimeout)
	defer cancel()

	return nil, r.validateLinodeCluster(ctx, &defaultLinodeClient)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeCluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	linodeclusterlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeCluster) ValidateDelete() (admission.Warnings, error) {
	linodeclusterlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *LinodeCluster) validateLinodeCluster(ctx context.Context, client LinodeClient) error {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := r.validateLinodeClusterSpec(ctx, client); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeCluster"},
		r.Name, errs)
}

func (r *LinodeCluster) validateLinodeClusterSpec(ctx context.Context, client LinodeClient) field.ErrorList {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := validateRegion(ctx, client, r.Spec.Region, field.NewPath("spec").Child("region")); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}
