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
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/linode/linodego"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
)

// log is for logging in this package.
var linodeplacementgrouplog = logf.Log.WithName("linodeplacementgroup-resource")

// SetupLinodePlacementGroupWebhookWithManager registers the webhook for LinodePlacementGroup in the manager.
func SetupLinodePlacementGroupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1alpha2.LinodePlacementGroup{}).
		WithValidator(&LinodePlacementGroupCustomValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodeplacementgroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeplacementgroups,verbs=create;update,versions=v1alpha2,name=validation.linodeplacementgroup.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// LinodePlacementGroupCustomValidator struct is responsible for validating the LinodePlacementGroup resource
type LinodePlacementGroupCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &LinodePlacementGroupCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type LinodePlacementGroup.
func (v *LinodePlacementGroupCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pg, ok := obj.(*infrav1alpha2.LinodePlacementGroup)
	if !ok {
		return nil, fmt.Errorf("expected a LinodePlacementGroup object but got %T", obj)
	}
	linodeplacementgrouplog.Info("Validation for LinodePlacementGroup upon creation", "name", pg.GetName())

	skipAPIValidation, linodeClient := setupClientWithCredentials(ctx, v.Client, pg.Spec.CredentialsRef,
		pg.Name, pg.GetNamespace(), linodeplacementgrouplog)

	var errs field.ErrorList

	if err := v.validateLinodePlacementGroupSpec(ctx, linodeClient, pg.Spec, pg.Name, skipAPIValidation); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodePlacementGroup"},
		pg.Name, errs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type LinodePlacementGroup.
func (v *LinodePlacementGroupCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	linodeplacementgroup, ok := newObj.(*infrav1alpha2.LinodePlacementGroup)
	if !ok {
		return nil, fmt.Errorf("expected a LinodePlacementGroup object for the newObj but got %T", newObj)
	}
	linodeplacementgrouplog.Info("Validation for LinodePlacementGroup upon update", "name", linodeplacementgroup.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type LinodePlacementGroup.
func (v *LinodePlacementGroupCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	linodeplacementgroup, ok := obj.(*infrav1alpha2.LinodePlacementGroup)
	if !ok {
		return nil, fmt.Errorf("expected a LinodePlacementGroup object but got %T", obj)
	}
	linodeplacementgrouplog.Info("Validation for LinodePlacementGroup upon deletion", "name", linodeplacementgroup.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

func (v *LinodePlacementGroupCustomValidator) validateLinodePlacementGroupSpec(ctx context.Context, linodeclient clients.LinodeClient, spec infrav1alpha2.LinodePlacementGroupSpec, label string, skipAPIValidation bool) field.ErrorList {
	var errs field.ErrorList

	if !skipAPIValidation {
		if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region"), linodego.CapabilityPlacementGroup); err != nil {
			errs = append(errs, err)
		}
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
