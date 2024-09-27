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
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

type linodePlacementGroupValidator struct {
	Client client.Client
}

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LinodePlacementGroup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&linodePlacementGroupValidator{Client: mgr.GetClient()}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodeplacementgroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups,verbs=create,versions=v1alpha2,name=validation.linodeplacementgroup.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodePlacementGroupValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pg, ok := obj.(*LinodePlacementGroup)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodePlacementGroup Resource")
	}
	spec := pg.Spec
	linodepglog.Info("validate create", "name", pg.Name)

	var linodeclient LinodeClient = defaultLinodeClient

	if spec.CredentialsRef != nil {
		apiToken, err := getCredentialDataFromRef(ctx, r.Client, *spec.CredentialsRef, pg.GetNamespace())
		if err != nil {
			linodepglog.Info("credentials from secret ref error", "name", pg.Name)
			return nil, err
		}
		linodepglog.Info("creating a verfied linode client for create request", "name", pg.Name)
		linodeclient.SetToken(string(apiToken))
	}
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := r.validateLinodePlacementGroupSpec(ctx, linodeclient, spec, pg.Name); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodePlacementGroup"},
		pg.Name, errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *linodePlacementGroupValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old, ok := oldObj.(*LinodePlacementGroup)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodePlacementGroup Resource")
	}
	linodepglog.Info("validate update", "name", old.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *linodePlacementGroupValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*LinodePlacementGroup)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodePlacementGroup Resource")
	}
	linodepglog.Info("validate delete", "name", c.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *linodePlacementGroupValidator) validateLinodePlacementGroupSpec(ctx context.Context, linodeclient LinodeClient, spec LinodePlacementGroupSpec, label string) field.ErrorList {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region"), LinodePlacementGroupCapability); err != nil {
		errs = append(errs, err)
	}

	if err := validatePlacementGroupLabel(label, field.NewPath("metadata").Child("name")); err != nil {
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
