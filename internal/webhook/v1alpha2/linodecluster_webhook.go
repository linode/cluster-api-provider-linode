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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
)

const clusterFQDNIndexerKey = "linodecluster-fqdn"

var linodeclusterlog = logf.Log.WithName("linodecluster-resource")

type linodeClusterValidator struct {
	Client client.Client
}

// SetupLinodeClusterWebhookWithManager registers the webhook for LinodeCluster in the manager.
func SetupLinodeClusterWebhookWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetCache().IndexField(ctx, &infrav1alpha2.LinodeCluster{}, clusterFQDNIndexerKey, extractFQDN); err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr, &infrav1alpha2.LinodeCluster{}).
		WithValidator(&linodeClusterValidator{Client: mgr.GetClient()}).
		Complete()
}

// Indexes LinodeClusters with LoadBalancerType DNS to perform FQDN lookups for uniqueness checks.
func extractFQDN(obj client.Object) []string {
	cluster, ok := obj.(*infrav1alpha2.LinodeCluster)
	if !ok {
		return nil
	}

	if cluster.Spec.Network.DNSRootDomain == "" {
		return nil
	}

	return []string{services.BuildFQDN(cluster)}
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodeclusters,verbs=create,versions=v1alpha2,name=validation.linodecluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeClusterValidator) ValidateCreate(ctx context.Context, cluster *infrav1alpha2.LinodeCluster) (admission.Warnings, error) {
	linodeclusterlog.Info("validate create", "name", cluster.Name)

	// Reject invalid names before building a client or making any Linode API calls.
	if err := validateLabelLength(cluster.GetName(), field.NewPath("metadata").Child("name")); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: infrav1alpha2.GroupVersion.Group, Kind: "LinodeCluster"},
			cluster.Name, field.ErrorList{err})
	}

	skipAPIValidation, linodeClient, err := setupClientWithCredentials(ctx, r.Client, cluster.Spec.CredentialsRef,
		cluster.Name, cluster.GetNamespace(), linodeclusterlog)
	if err != nil {
		return admission.Warnings{}, err
	}

	// TODO: instrument with tracing, might need refactor to preserve readability
	var errs field.ErrorList
	if err := r.validateLinodeCluster(ctx, linodeClient, cluster, skipAPIValidation); err != nil {
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

func (r *linodeClusterValidator) validateLinodeCluster(ctx context.Context, linodeclient clients.LinodeClient, cluster *infrav1alpha2.LinodeCluster, skipAPIValidation bool) field.ErrorList {
	var errs field.ErrorList

	// Validate local fields first before making any API calls for validation
	if cluster.Spec.Network.LoadBalancerType == "dns" && cluster.Spec.Network.DNSRootDomain == "" {
		errs = append(errs, &field.Error{
			Field:  "spec.network.dnsRootDomain",
			Type:   field.ErrorTypeRequired,
			Detail: "dnsRootDomain needs to be set when LoadBalancer Type is DNS",
		})
	}

	if cluster.Spec.Network.UseVlan && cluster.Spec.VPCRef != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.network.useVlan/spec.vpcRef",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot use VLANs and VPCs together. Unset `network.useVlan` or remove `vpcRef`",
		})
	}

	if cluster.Spec.VPCID != nil && cluster.Spec.VPCRef != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.vpcID/spec.vpcRef",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot specify both VPCID and VPCRef",
		})
	}

	if cluster.Spec.Network.NodeBalancerFirewallID != nil && cluster.Spec.NodeBalancerFirewallRef != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.network.nodeBalancerFirewallID/spec.nodeBalancerFirewallRef",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot specify both NodeBalancerFirewallID and NodeBalancerFirewallRef",
		})
	}

	if !skipAPIValidation {
		if err := validateRegion(ctx, linodeclient, cluster.Spec.Region, field.NewPath("spec").Child("region")); err != nil {
			errs = append(errs, err)
		}

		if err := enforceUniqueFQDN(ctx, r.Client, cluster); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// enforceUniqueFQDN ensures there is no other LinodeCluster with the same derived FQDN.
// DNSSubDomainOverride and DNSUniqueIdentifier are mutually exclusive, with DNSSubDomainOverride taking precedence.
// If either are set, we need to validate that the incoming spec does not match an existing LinodeCluster.
func enforceUniqueFQDN(ctx context.Context, cl client.Client, cluster *infrav1alpha2.LinodeCluster) *field.Error {
	if cluster.Spec.Network.LoadBalancerType != "dns" || cluster.Spec.Network.DNSRootDomain == "" {
		return nil
	}

	fieldsSelectorSet := map[string]string{
		clusterFQDNIndexerKey: services.BuildFQDN(cluster),
	}

	var existingClusters infrav1alpha2.LinodeClusterList
	err := cl.List(ctx, &existingClusters, client.MatchingFieldsSelector{Selector: fields.SelectorFromSet(fieldsSelectorSet)})
	if err != nil {
		return &field.Error{
			Field:  "spec.network",
			Type:   field.ErrorTypeInternal,
			Detail: "failed to list existing LinodeClusters to validate unique FQDN from dnsRootDomain and dnsUniqueIdentifier/dnsSubDomainOverride fields",
		}
	}

	if len(existingClusters.Items) > 0 {
		return &field.Error{
			Field:  "spec.network",
			Type:   field.ErrorTypeInvalid,
			Detail: "A LinodeCluster with the same dnsRootDomain and dnsUniqueIdentifier/dnsSubDOmainOverride combination already exists",
		}
	}

	return nil
}
