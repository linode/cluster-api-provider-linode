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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
)

// log is for logging in this package.
var linodemachinetemplatelog = logf.Log.WithName("linodemachinetemplate-resource")

type linodeMachineTemplateValidator struct {
	Client client.Client
}

// SetupLinodeMachineTemplateWebhookWithManager registers the webhook for LinodeMachineTemplate in the manager.
func SetupLinodeMachineTemplateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1alpha2.LinodeMachineTemplate{}).
		WithValidator(&linodeMachineTemplateValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodemachinetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodemachinetemplates,verbs=create,versions=v1alpha2,name=validation.linodemachinetemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeMachineTemplateValidator) ValidateCreate(ctx context.Context, linodeMachineTemplate *infrav1alpha2.LinodeMachineTemplate) (admission.Warnings, error) {
	spec := linodeMachineTemplate.Spec
	linodemachinetemplatelog.Info("validate create", "name", linodeMachineTemplate.Name)

	skipAPIValidation, linodeClient, err := setupClientWithCredentials(ctx, r.Client, spec.Template.Spec.CredentialsRef,
		linodeMachineTemplate.Name, linodeMachineTemplate.GetNamespace(), linodemachinetemplatelog)
	if err != nil {
		return admission.Warnings{}, err
	}

	var errs field.ErrorList
	if !skipAPIValidation {
		if templateErrs := r.validateLinodeMachineTemplateSpec(ctx, linodeClient, spec); templateErrs != nil {
			errs = append(errs, templateErrs...)
		}
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeMachineTemplate"},
		linodeMachineTemplate.Name, errs)
}

func (r *linodeMachineTemplateValidator) validateLinodeMachineTemplateSpec(ctx context.Context, linodeClient clients.LinodeClient, spec infrav1alpha2.LinodeMachineTemplateSpec) field.ErrorList {
	var errs field.ErrorList
	usesLegacy := len(spec.Template.Spec.Interfaces) > 0
	usesLinode := len(spec.Template.Spec.LinodeInterfaces) > 0
	path := field.NewPath("spec", "template", "spec")
	if ferr, err := validateInterfaceAccountSettings(ctx, linodeClient, usesLegacy, usesLinode, path); err != nil {
		return append(errs, field.InternalError(path, err))
	} else if ferr != nil {
		errs = append(errs, ferr)
	}
	return errs
}

func (r *linodeMachineTemplateValidator) ValidateUpdate(_ context.Context, _, newLinodeMachineTemplate *infrav1alpha2.LinodeMachineTemplate) (admission.Warnings, error) {
	linodemachinetemplatelog.Info("validate update", "name", newLinodeMachineTemplate.Name)
	return nil, nil
}

func (r *linodeMachineTemplateValidator) ValidateDelete(_ context.Context, linodeMachineTemplate *infrav1alpha2.LinodeMachineTemplate) (admission.Warnings, error) {
	linodemachinetemplatelog.Info("validate delete", "name", linodeMachineTemplate.Name)
	return nil, nil
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
