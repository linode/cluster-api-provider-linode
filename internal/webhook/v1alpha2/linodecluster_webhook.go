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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
)

const (
	dnsRootDomainField        = "spec.network.dnsRootDomain"
	dnsSubDomainOverrideField = "spec.network.dnsSubDomainOverride"
	dnsUniqueIdentifierField  = "spec.network.dnsUniqueIdentifier"
)

var linodeclusterlog = logf.Log.WithName("linodecluster-resource")

type linodeClusterValidator struct {
	Client client.Client
}

// SetupLinodeClusterWebhookWithManager registers the webhook for LinodeCluster in the manager.
func SetupLinodeClusterWebhookWithManager(ctx context.Context, mgr ctrl.Manager) error {
	cache := mgr.GetCache()
	for _, fieldName := range []string{dnsRootDomainField, dnsSubDomainOverrideField, dnsUniqueIdentifierField} {
		if err := cache.IndexField(ctx, &infrav1alpha2.LinodeCluster{}, fieldName, extractValueFunc(fieldName)); err != nil {
			return err
		}
	}

	return ctrl.NewWebhookManagedBy(mgr, &infrav1alpha2.LinodeCluster{}).
		WithValidator(&linodeClusterValidator{Client: mgr.GetClient()}).
		Complete()
}

func extractValueFunc(fieldName string) func(obj client.Object) []string {
	return func(obj client.Object) []string {
		cluster, ok := obj.(*infrav1alpha2.LinodeCluster)
		if !ok {
			return nil
		}
		switch fieldName {
		case "spec.network.dnsRootDomain":
			if cluster.Spec.Network.DNSRootDomain == "" {
				return nil
			}
			return []string{cluster.Spec.Network.DNSRootDomain}
		case "spec.network.dnsSubDomainOverride":
			if cluster.Spec.Network.DNSSubDomainOverride == "" {
				return nil
			}
			return []string{cluster.Spec.Network.DNSSubDomainOverride}
		case "spec.network.dnsUniqueIdentifier":
			if cluster.Spec.Network.DNSUniqueIdentifier == "" {
				return nil
			}
			return []string{cluster.Spec.Network.DNSUniqueIdentifier}
		default:
			return nil
		}
	}
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters,verbs=create,versions=v1alpha2,name=validation.linodecluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeClusterValidator) ValidateCreate(ctx context.Context, cluster *infrav1alpha2.LinodeCluster) (admission.Warnings, error) {
	spec := cluster.Spec
	linodeclusterlog.Info("validate create", "name", cluster.Name)

	// Reject invalid names before building a client or making any Linode API calls.
	if err := validateLabelLength(cluster.GetName(), field.NewPath("metadata").Child("name")); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: infrav1alpha2.GroupVersion.Group, Kind: "LinodeCluster"},
			cluster.Name, field.ErrorList{err})
	}

	skipAPIValidation, linodeClient, err := setupClientWithCredentials(ctx, r.Client, spec.CredentialsRef,
		cluster.Name, cluster.GetNamespace(), linodeclusterlog)
	if err != nil {
		return admission.Warnings{}, err
	}

	// TODO: instrument with tracing, might need refactor to preserve readability
	var errs field.ErrorList
	if err := r.validateLinodeClusterSpec(ctx, linodeClient, spec, skipAPIValidation); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: infrav1alpha2.GroupVersion.Group, Kind: "LinodeCluster"},
		cluster.Name, errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeClusterValidator) ValidateUpdate(_ context.Context, _, newCluster *infrav1alpha2.LinodeCluster) (admission.Warnings, error) {
	linodeclusterlog.Info("validate update", "name", newCluster.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *linodeClusterValidator) ValidateDelete(_ context.Context, cluster *infrav1alpha2.LinodeCluster) (admission.Warnings, error) {
	linodeclusterlog.Info("validate delete", "name", cluster.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *linodeClusterValidator) validateLinodeClusterSpec(ctx context.Context, linodeclient clients.LinodeClient, spec infrav1alpha2.LinodeClusterSpec, skipAPIValidation bool) field.ErrorList {
	var errs field.ErrorList

	// Validate local fields first before making any API calls for validation
	if spec.Network.LoadBalancerType == "dns" {
		if spec.Network.DNSRootDomain == "" {
			errs = append(errs, &field.Error{
				Field: "dnsRootDomain needs to be set when LoadBalancer Type is DNS",
				Type:  field.ErrorTypeRequired,
			})
		}

		if err := enforceUniqueDNSFields(ctx, r.Client, spec.Network); err != nil {
			errs = append(errs, err)
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

	if !skipAPIValidation {
		if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region")); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// enforceUniqueDNSFields ensures there is no other LinodeCluster with the same DNSRootDomain and either DNSSubDomainOverride or DNSUniqueIdentifier.
// DNSSubDomainOverride and DNSUniqueIdentifier are mutually exclusive, with DNSSubDomainOverride taking precedence.
// If either are set, we need to validate that the incominmg spec does not match an existing LinodeCluster.
func enforceUniqueDNSFields(ctx context.Context, cl client.Client, networkSpec infrav1alpha2.NetworkSpec) *field.Error {
	if networkSpec.DNSRootDomain == "" {
		return nil
	}

	fieldsSelectorSet := map[string]string{
		"spec.network.dnsRootDomain": networkSpec.DNSRootDomain,
	}

	var fieldPath string
	if networkSpec.DNSSubDomainOverride != "" {
		fieldPath = "spec.network.dnsSubDomainOverride"
		fieldsSelectorSet["spec.network.dnsSubDomainOverride"] = networkSpec.DNSSubDomainOverride
	} else if networkSpec.DNSUniqueIdentifier != "" {
		fieldPath = "spec.network.dnsUniqueIdentifier"
		fieldsSelectorSet["spec.network.dnsUniqueIdentifier"] = networkSpec.DNSUniqueIdentifier
	} else {
		return nil
	}

	var existingClusters infrav1alpha2.LinodeClusterList
	err := cl.List(ctx, &existingClusters, client.MatchingFieldsSelector{Selector: fields.SelectorFromSet(fieldsSelectorSet)})
	if err != nil {
		return &field.Error{
			Field: fmt.Sprintf("failed to list existing LinodeClusters to validate spec.network.dnsRootDomain and %s uniqueness", fieldPath),
			Type:  field.ErrorTypeInternal,
		}
	}

	if len(existingClusters.Items) > 0 {
		return &field.Error{
			Field:  fieldPath,
			Type:   field.ErrorTypeInvalid,
			Detail: fmt.Sprintf("A LinodeCluster with the same spec.network.dnsRootDomain and %s already exists", fieldPath),
		}
	}

	return nil
}
