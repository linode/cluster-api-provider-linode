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

package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"

	"go4.org/netipx"
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
	// The capability string indicating a region supports VPCs: [VPC Availability]
	//
	// [VPC Availability]: https://www.linode.com/docs/products/networking/vpc/#availability
	LinodeVPCCapability = "VPCs"

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

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LinodeVPC) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable updation and deletion validation.
//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-linodevpc,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodevpcs,verbs=create,versions=v1alpha1,name=vlinodevpc.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &LinodeVPC{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeVPC) ValidateCreate() (admission.Warnings, error) {
	linodevpclog.Info("validate create", "name", r.Name)

	ctx, cancel := context.WithTimeout(context.Background(), defaultWebhookTimeout)
	defer cancel()

	return nil, r.validateLinodeVPC(ctx, &defaultLinodeClient)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeVPC) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	linodevpclog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinodeVPC) ValidateDelete() (admission.Warnings, error) {
	linodevpclog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *LinodeVPC) validateLinodeVPC(ctx context.Context, client LinodeClient) error {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := r.validateLinodeVPCSpec(ctx, client); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeVPC"},
		r.Name, errs)
}

func (r *LinodeVPC) validateLinodeVPCSpec(ctx context.Context, client LinodeClient) field.ErrorList {
	// TODO: instrument with tracing, might need refactor to preserve readibility
	var errs field.ErrorList

	if err := validateRegion(ctx, client, r.Spec.Region, field.NewPath("spec").Child("region"), LinodeVPCCapability); err != nil {
		errs = append(errs, err)
	}
	if err := r.validateLinodeVPCSubnets(); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (r *LinodeVPC) validateLinodeVPCSubnets() field.ErrorList {
	var (
		errs    field.ErrorList
		builder netipx.IPSetBuilder
		cidrs   = &netipx.IPSet{}
		labels  = []string{}
	)

	for i, subnet := range r.Spec.Subnets {
		var (
			label     = subnet.Label
			labelPath = field.NewPath("spec").Child("Subnets").Index(i).Child("Label")
			ip        = subnet.IPv4
			ipPath    = field.NewPath("spec").Child("Subnets").Index(i).Child("IPv4")
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
	var (
		minLen = 1
		maxLen = 64
		errs   = []error{
			fmt.Errorf("%d..%d characters", minLen, maxLen),
			errors.New("can only contain ASCII letters, numbers, and hyphens (-)"),
			errors.New("cannot contain two consecutive hyphens (--)"),
		}
		regex = regexp.MustCompile("^[-[:alnum:]]*$")
	)
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
	var (
		minPrefix = 1
		maxPrefix = 29
		errs      = []error{
			errors.New("must be IPv4 range in CIDR canonical form"),
			errors.New("range must belong to a private address space as defined in RFC1918"),
			fmt.Errorf("allowed prefix lengths: %d-%d", minPrefix, maxPrefix),
			fmt.Errorf("%s %s", "range must not overlap with", LinodeVPCSubnetReserved.Prefixes()),
		}
	)

	prefix, ferr := netip.ParsePrefix(cidr)
	if !(ferr == nil && prefix.Addr().Is4()) {
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
