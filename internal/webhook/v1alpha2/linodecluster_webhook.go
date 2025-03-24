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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

var linodeclusterlog = logf.Log.WithName("linodecluster-resource")

type linodeClusterValidator struct {
	Client client.Client
}

// SetupLinodeClusterWebhookWithManager registers the webhook for LinodeCluster in the manager.
func SetupLinodeClusterWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1alpha2.LinodeCluster{}).
		WithValidator(&linodeClusterValidator{Client: mgr.GetClient()}).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable update and deletion validation.
// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters,verbs=create,versions=v1alpha2,name=validation.linodecluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster, ok := obj.(*infrav1alpha2.LinodeCluster)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeCluster Resource")
	}
	spec := cluster.Spec
	linodeclusterlog.Info("validate create", "name", cluster.Name)

	var linodeclient LinodeClient = defaultLinodeClient
	skipAPIValidation := false

	// Handle credentials if provided
	if spec.CredentialsRef != nil {
		skipAPIValidation, linodeclient = setupClientWithCredentials(ctx, r.Client, spec.CredentialsRef,
			cluster.Name, cluster.GetNamespace(), linodeclusterlog)
	}

	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := r.validateLinodeClusterSpec(ctx, linodeclient, spec, skipAPIValidation); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeCluster"},
		cluster.Name, errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old, ok := oldObj.(*infrav1alpha2.LinodeCluster)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeCluster Resource")
	}
	linodeclusterlog.Info("validate update", "name", old.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *linodeClusterValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*infrav1alpha2.LinodeCluster)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeCluster Resource")
	}
	linodeclusterlog.Info("validate delete", "name", c.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *linodeClusterValidator) validateLinodeClusterSpec(ctx context.Context, linodeclient LinodeClient, spec infrav1alpha2.LinodeClusterSpec, skipAPIValidation bool) field.ErrorList {
	var errs field.ErrorList

	if !skipAPIValidation {
		if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region")); err != nil {
			errs = append(errs, err)
		}
	}

	if spec.Network.LoadBalancerType == "dns" {
		if spec.Network.DNSRootDomain == "" {
			errs = append(errs, &field.Error{
				Field: "dnsRootDomain needs to be set when LoadBalancer Type is DNS",
				Type:  field.ErrorTypeRequired,
			})
		}
	}

	if spec.Network.UseVlan && spec.VPCRef != nil {
		errs = append(errs, &field.Error{
			Field: "Cannot use VLANs and VPCs together. Unset `network.useVlan` or remove `vpcRef`",
			Type:  field.ErrorTypeInvalid,
		})
	}

	if spec.VPCID != nil && spec.VPCRef != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.vpcID/spec.vpcRef",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot specify both VPCID and VPCRef",
		})
	}

	if spec.Network.NodeBalancerFirewallID != nil && spec.NodeBalancerFirewallRef != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.network.nodeBalancerFirewallID/spec.nodeBalancerFirewallRef",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot specify both NodeBalancerFirewallID and NodeBalancerFirewallRef",
		})
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}
