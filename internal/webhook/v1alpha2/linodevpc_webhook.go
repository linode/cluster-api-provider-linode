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
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/linode/linodego"
	"go4.org/netipx"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
)

var (
	// The IPv4 ranges that are excluded from VPC Subnets: [Valid IPv4 Ranges for a Subnet]
	//
	// [Valid IPv4 Ranges for a Subnet]: https://www.linode.com/docs/products/networking/vpc/guides/subnets/#valid-ipv4-ranges
	LinodeVPCSubnetReserved = mustParseIPSet("192.168.128.0/17")

	// IPv4 private address space as defined in [RFC 1918].
	//
	// [RFC 1918]: https://datatracker.ietf.org/doc/html/rfc1918
	privateIPv4 = mustParseIPSet("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")
)

// mustParseIPSet parses the given IP CIDRs as a [go4.org/netipx.IPSet]. It is intended for use with hard-coded strings.
//
//nolint:errcheck //^
func mustParseIPSet(cidrs ...string) *netipx.IPSet {
	var (
		builder netipx.IPSetBuilder
		set     *netipx.IPSet
	)
	for _, cidr := range cidrs {
		prefix, _ := netip.ParsePrefix(cidr)
		builder.AddPrefix(prefix)
	}
	set, _ = builder.IPSet()
	return set
}

// log is for logging in this package.
var linodevpclog = logf.Log.WithName("linodevpc-resource")

type linodeVPCValidator struct {
	Client client.Client
}

// SetupLinodeVPCWebhookWithManager will setup the manager to manage the webhooks
func SetupLinodeVPCWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1alpha2.LinodeVPC{}).
		WithValidator(&linodeVPCValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodevpc,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs,verbs=create,versions=v1alpha2,name=validation.linodevpc.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeVPCValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	vpc, ok := obj.(*infrav1alpha2.LinodeVPC)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeVPC Resource")
	}
	spec := vpc.Spec
	linodevpclog.Info("validate create", "name", vpc.Name)

	skipAPIValidation, linodeClient := setupClientWithCredentials(ctx, r.Client, spec.CredentialsRef,
		vpc.Name, vpc.GetNamespace(), linodevpclog)

	// TODO: instrument with tracing, might need refactor to preserve readability
	var errs field.ErrorList
	if err := validateLabelLength(vpc.GetName(), field.NewPath("metadata").Child("name")); err != nil {
		errs = append(errs, err)
	}
	if err := r.validateLinodeVPCSpec(ctx, linodeClient, spec, skipAPIValidation); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeVPC"},
		vpc.Name, errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeVPCValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old, ok := oldObj.(*infrav1alpha2.LinodeVPC)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeVPC Resource")
	}
	linodevpclog.Info("validate update", "name", old.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *linodeVPCValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*infrav1alpha2.LinodeVPC)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeVPC Resource")
	}
	linodevpclog.Info("validate delete", "name", c.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *linodeVPCValidator) validateLinodeVPCSpec(ctx context.Context, linodeclient clients.LinodeClient, spec infrav1alpha2.LinodeVPCSpec, skipAPIValidation bool) field.ErrorList {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if !skipAPIValidation {
		if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region"), linodego.CapabilityVPCs); err != nil {
			errs = append(errs, err)
		}
	}
	if err := r.validateLinodeVPCSubnets(spec); err != nil {
		errs = slices.Concat(errs, err)
	}

	// Validate VPC IPv6 Ranges
	for idx, ipv6Range := range spec.IPv6Range {
		ipv6RangePath := field.NewPath("spec").Child("IPv6Range").Index(idx).Child("Range")
		rangeErr := validateIPv6Range(ipv6Range.Range, ipv6RangePath)
		if rangeErr != nil {
			errs = append(errs, rangeErr)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (r *linodeVPCValidator) validateLinodeVPCSubnets(spec infrav1alpha2.LinodeVPCSpec) field.ErrorList {
	var (
		errs    field.ErrorList
		builder netipx.IPSetBuilder
		cidrs   = &netipx.IPSet{}
		labels  = []string{}
	)

	for idx, subnet := range spec.Subnets {
		var (
			label     = subnet.Label
			labelPath = field.NewPath("spec").Child("Subnets").Index(idx).Child("Label")
			ip        = subnet.IPv4
			ipPath    = field.NewPath("spec").Child("Subnets").Index(idx).Child("IPv4")
		)

		// Validate Subnet Label
		if err := validateVPCLabel(label, labelPath); err != nil {
			errs = append(errs, err)
		} else if slices.Contains(labels, label) {
			errs = append(errs, field.Invalid(labelPath, label, "must be unique among the vpc's subnets"))
		} else {
			labels = append(labels, label)
		}

		// Validate Subnet IP Address Range
		cidr, ferr := validateSubnetIPv4CIDR(ip, ipPath)
		if ferr != nil {
			errs = append(errs, ferr)
			continue
		}
		if cidrs.Overlaps(cidr) {
			errs = append(errs, field.Invalid(ipPath, ip, "range must not overlap with other subnets on the same vpc"))
		}
		var err error
		builder.AddSet(cidr)
		if cidrs, err = builder.IPSet(); err != nil {
			return append(field.ErrorList{}, field.InternalError(ipPath, fmt.Errorf("build ip set: %w", err)))
		}

		// Validate Subnet IPv6 Ranges
		for subnetIdx, ipv6Range := range spec.Subnets[idx].IPv6Range {
			ipv6RangePath := field.NewPath("spec").Child("Subnets").Index(idx).Child("IPv6Range").Index(subnetIdx).Child("Range")
			rangeErr := validateIPv6Range(ipv6Range.Range, ipv6RangePath)
			if rangeErr != nil {
				errs = append(errs, rangeErr)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// TODO: Replace the OpenAPI schema validation for .metadata.name.
//
// validateVPCLabel validates a label string is a valid [Linode VPC Label].
//
// [Linode VPC Label]: https://www.linode.com/docs/api/vpcs/#vpc-create__request-body-schema
func validateVPCLabel(label string, path *field.Path) *field.Error {
	minLen := 1
	maxLen := 64
	errs := []error{
		fmt.Errorf("%d..%d characters", minLen, maxLen),
		errors.New("can only contain ASCII letters, numbers, and hyphens (-)"),
		errors.New("cannot contain two consecutive hyphens (--)"),
	}
	regex := regexp.MustCompile("^[-[:alnum:]]*$")
	if len(label) < minLen || len(label) > maxLen {
		return field.Invalid(path, label, errs[0].Error())
	}
	if !regex.MatchString(label) {
		return field.Invalid(path, label, errs[1].Error())
	}
	if strings.Contains(label, "--") {
		return field.Invalid(path, label, errs[2].Error())
	}
	return nil
}

// validateSubnetIPv4CIDR validates a CIDR string is a valid [Linode VPC Subnet IPv4 Address Range].
//
// [Linode VPC Subnet IPv4 Address Range]: https://www.linode.com/docs/api/vpcs/#vpc-create__request-body-schema
func validateSubnetIPv4CIDR(cidr string, path *field.Path) (*netipx.IPSet, *field.Error) {
	minPrefix := 1
	maxPrefix := 29
	errs := []error{
		errors.New("must be IPv4 range in CIDR canonical form"),
		errors.New("range must belong to a private address space as defined in RFC1918"),
		fmt.Errorf("allowed prefix lengths: %d-%d", minPrefix, maxPrefix),
		fmt.Errorf("%s %s", "range must not overlap with", LinodeVPCSubnetReserved.Prefixes()),
	}

	prefix, ferr := netip.ParsePrefix(cidr)
	if ferr != nil || !prefix.Addr().Is4() {
		return nil, field.Invalid(path, cidr, errs[0].Error())
	}
	if netipx.ComparePrefix(prefix, prefix.Masked()) != 0 {
		return nil, field.Invalid(path, cidr, errs[0].Error())
	}
	if !privateIPv4.ContainsPrefix(prefix) {
		return nil, field.Invalid(path, cidr, errs[1].Error())
	}
	size, _ := netipx.PrefixIPNet(prefix).Mask.Size()
	if size < minPrefix || size > maxPrefix {
		return nil, field.Invalid(path, cidr, errs[2].Error())
	}
	if LinodeVPCSubnetReserved.OverlapsPrefix(prefix) {
		return nil, field.Invalid(path, cidr, errs[3].Error())
	}

	var (
		builder netipx.IPSetBuilder
		set     *netipx.IPSet
		err     error
	)
	builder.AddPrefix(prefix)
	if set, err = builder.IPSet(); err != nil {
		return nil, field.InternalError(path, fmt.Errorf("build ip set: %w", err))
	}
	return set, nil
}

func validateIPv6Range(ipv6Range *string, path *field.Path) *field.Error {
	const (
		errIPv6RangeFormat   = "IPv6 range must be either 'auto', valid IPv6 prefix or start with /. Example: auto, /52, 2001:db8::/52"
		errIPv6RangeNoNumber = "IPv6 range doesn't contain a valid number after /"
		errIPv6RangeBounds   = "IPv6 range must be between /0 and /128"
	)

	ipv6RangeStr := "auto"
	if ipv6Range != nil {
		ipv6RangeStr = *ipv6Range
	}

	// "auto" is valid
	if ipv6RangeStr == "auto" {
		return nil
	}

	// check for valid IPv6 prefix (e.g., 2001:db8::/52)
	if _, ipNet, err := net.ParseCIDR(ipv6RangeStr); err == nil {
		if ipNet.IP.To16() != nil && ipNet.IP.To4() == nil {
			ones, _ := ipNet.Mask.Size()
			if ones < 0 || ones > 128 {
				return field.Invalid(path, ipv6RangeStr, errIPv6RangeBounds)
			}
			return nil
		}
	}

	// Slash-prefixed format (e.g., /52)
	if numStr, ok := strings.CutPrefix(ipv6RangeStr, "/"); ok {
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return field.Invalid(path, ipv6RangeStr, errIPv6RangeNoNumber)
		}
		if num < 0 || num > 128 {
			return field.Invalid(path, ipv6RangeStr, errIPv6RangeBounds)
		}
		return nil
	}

	// Invalid
	return field.Invalid(path, ipv6RangeStr, errIPv6RangeFormat)
}
