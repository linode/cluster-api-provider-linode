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
	"fmt"
	"regexp"
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
	// The capability string indicating a region supports PlacementGroups: [PlacementGroups Availability]
	//
	// [PlacementGroups Availability]:https://www.linode.com/docs/products/compute/compute-instances/guides/placement-groups/#availability
	LinodePlacementGroupCapability = "Placement Group"
)

// log is for logging in this package.
var linodepglog = logf.Log.WithName("linodeplacementgroup-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LinodePlacementGroup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodeplacementgroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups,verbs=create,versions=v1alpha2,name=validation.linodeplacementgroup.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &LinodePlacementGroup{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodePlacementGroup) ValidateCreate() (admission.Warnings, error) {
	linodepglog.Info("validate create", "name", r.Name)

	ctx, cancel := context.WithTimeout(context.Background(), defaultWebhookTimeout)
	defer cancel()

	return nil, r.validateLinodePlacementGroup(ctx, &defaultLinodeClient)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodePlacementGroup) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	linodepglog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinodePlacementGroup) ValidateDelete() (admission.Warnings, error) {
	linodepglog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *LinodePlacementGroup) validateLinodePlacementGroup(ctx context.Context, client LinodeClient) error {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := r.validateLinodePlacementGroupSpec(ctx, client); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodePlacementGroup"},
		r.Name, errs)
}

func (r *LinodePlacementGroup) validateLinodePlacementGroupSpec(ctx context.Context, client LinodeClient) field.ErrorList {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := validateRegion(ctx, client, r.Spec.Region, field.NewPath("spec").Child("region"), LinodePlacementGroupCapability); err != nil {
		errs = append(errs, err)
	}

	if err := validatePlacementGroupLabel(r.Name, field.NewPath("metadata").Child("name")); err != nil {
		errs = append(errs, err)
	}
	// PlacementGroupPolicy is immutable, no need to verify again.
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// validatePlacementGroupLabel validates a label string is a valid [Linode Placement Group Label].
//
// [Linode Placement Group Label]: https://techdocs.akamai.com/linode-api/reference/post-placement-group
func validatePlacementGroupLabel(label string, path *field.Path) *field.Error {
	var (
		minLen = 1
		maxLen = 64 // its not actually specified in the docs, but why risk it?
		errs   = []error{
			fmt.Errorf("%d..%d characters", minLen, maxLen),
			errors.New("can only contain ASCII letters, numbers, hyphens (-), underscores (_) and periods (.), must start and end with a alphanumeric character"),
		}
		regex = regexp.MustCompile(`^[[:alnum:]][-[:alnum:]_.]*[[:alnum:]]$|^[[:alnum:]]$`)
	)
	if len(label) < minLen || len(label) > maxLen {
		return field.Invalid(path, label, errs[0].Error())
	}
	if !regex.MatchString(label) {
		return field.Invalid(path, label, errs[1].Error())
	}
	return nil
}
