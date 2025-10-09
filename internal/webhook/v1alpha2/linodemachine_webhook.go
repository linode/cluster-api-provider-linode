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

	"github.com/linode/linodego"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

var linodemachinelog = logf.Log.WithName("linodemachine-resource")

type linodeMachineValidator struct {
	Client client.Client
}

// SetupLinodeMachineWebhookWithManager registers the webhook for LinodeMachine in the manager.
func SetupLinodeMachineWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1alpha2.LinodeMachine{}).
		WithValidator(&linodeMachineValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodemachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodemachines,verbs=create,versions=v1alpha2,name=validation.linodemachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeMachineValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	machine, ok := obj.(*infrav1alpha2.LinodeMachine)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeMachine Resource")
	}
	spec := machine.Spec
	linodemachinelog.Info("validate create", "name", machine.Name)

	skipAPIValidation, linodeClient := setupClientWithCredentials(ctx, r.Client, spec.CredentialsRef,
		machine.Name, machine.GetNamespace(), linodemachinelog)

	var errs field.ErrorList
	if err := validateLabelLength(machine.GetName(), field.NewPath("metadata").Child("name")); err != nil {
		errs = append(errs, err)
	}
	if err := r.validateLinodeMachineSpec(ctx, linodeClient, spec, skipAPIValidation); err != nil {
		errs = slices.Concat(errs, err)
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "LinodeMachine"},
		machine.Name, errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeMachineValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old, ok := oldObj.(*infrav1alpha2.LinodeMachine)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeMachine Resource")
	}
	linodemachinelog.Info("validate update", "name", old.Name)

	new, ok := newObj.(*infrav1alpha2.LinodeMachine)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeMachine Resource")
	}

	// ensure that spec.Image is immutable
	if old.Spec.Image != new.Spec.Image {
		return nil, &field.Error{
			Field:  "spec.image",
			Type:   field.ErrorTypeInvalid,
			Detail: "Field is immutable",
		}
	}

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *linodeMachineValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*infrav1alpha2.LinodeMachine)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeCluster Resource")
	}
	linodemachinelog.Info("validate delete", "name", c.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *linodeMachineValidator) validateLinodeMachineSpec(ctx context.Context, linodeclient clients.LinodeClient, spec infrav1alpha2.LinodeMachineSpec, skipAPIValidation bool) field.ErrorList {
	var errs field.ErrorList

	if !skipAPIValidation { //nolint:nestif // too simple for switch
		if spec.LinodeInterfaces != nil {
			if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region"), linodego.CapabilityLinodeInterfaces); err != nil {
				errs = append(errs, err)
			}
		} else {
			if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region")); err != nil {
				errs = append(errs, err)
			}
		}

		plan, err := validateLinodeType(ctx, linodeclient, spec.Type, field.NewPath("spec").Child("type"))
		if err != nil {
			errs = append(errs, err)
		}
		if err := r.validateLinodeMachineDisks(plan, spec); err != nil {
			errs = append(errs, err)
		}
	}

	if spec.VPCID != nil && spec.VPCRef != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.vpcID/spec.vpcRef",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot specify both VPCID and VPCRef",
		})
	}

	if spec.LinodeInterfaces != nil {
		if ifaceErrs := r.validateLinodeInterfaces(spec); ifaceErrs != nil {
			errs = append(errs, ifaceErrs...)
		}
	}

	if spec.FirewallID != 0 && spec.FirewallRef != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.firewallID/spec.firewallRef",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot specify both FirewallID and FirewallRef",
		})
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (r *linodeMachineValidator) validateLinodeInterfaces(spec infrav1alpha2.LinodeMachineSpec) field.ErrorList {
	var errs field.ErrorList

	if spec.Interfaces != nil {
		errs = append(errs, &field.Error{
			Field:  "spec.linodeInterfaces/spec.interfaces",
			Type:   field.ErrorTypeInvalid,
			Detail: "Cannot specify both LinodeInterfaces and Interfaces",
		})
	}

	if spec.PrivateIP != nil && *spec.PrivateIP {
		errs = append(errs, &field.Error{
			Field:  "spec.linodeInterfaces/spec.privateIP",
			Type:   field.ErrorTypeInvalid,
			Detail: "Linode Interfaces do not support private IPs",
		})
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (r *linodeMachineValidator) validateLinodeMachineDisks(plan *linodego.LinodeType, spec infrav1alpha2.LinodeMachineSpec) *field.Error {
	// The Linode plan information is required to perform disk validation
	if plan == nil {
		return nil
	}

	var (
		// The Linode API represents storage sizes in megabytes (MB)
		// https://www.linode.com/docs/api/linode-types/#type-view
		planSize   = resource.MustParse(fmt.Sprintf("%d%s", plan.Disk, "M"))
		remainSize = &resource.Quantity{}
	)
	planSize.DeepCopyInto(remainSize)

	if err := validateDisk(spec.OSDisk, field.NewPath("spec").Child("osDisk"), remainSize, &planSize); err != nil {
		return err
	}
	if err := validateDataDisks(spec.DataDisks, field.NewPath("spec").Child("dataDisks"), remainSize, &planSize); err != nil {
		return err
	}

	return nil
}

func validateDataDisks(disks *infrav1alpha2.InstanceDisks, path *field.Path, remainSize, planSize *resource.Quantity) *field.Error {
	if disks == nil {
		return nil
	}
	if err := validateDisk(disks.SDB, path.Child("SDB"), remainSize, planSize); err != nil {
		return err
	}
	if err := validateDisk(disks.SDC, path.Child("SDC"), remainSize, planSize); err != nil {
		return err
	}
	if err := validateDisk(disks.SDD, path.Child("SDD"), remainSize, planSize); err != nil {
		return err
	}
	if err := validateDisk(disks.SDE, path.Child("SDE"), remainSize, planSize); err != nil {
		return err
	}
	if err := validateDisk(disks.SDF, path.Child("SDF"), remainSize, planSize); err != nil {
		return err
	}
	if err := validateDisk(disks.SDG, path.Child("SDG"), remainSize, planSize); err != nil {
		return err
	}
	if err := validateDisk(disks.SDH, path.Child("SDH"), remainSize, planSize); err != nil {
		return err
	}

	return nil
}

func validateDisk(disk *infrav1alpha2.InstanceDisk, path *field.Path, remainSize, planSize *resource.Quantity) *field.Error {
	if disk == nil {
		return nil
	}

	if disk.Size.Sign() < 1 {
		return field.Invalid(path, disk.Size.String(), "invalid size")
	}
	if remainSize.Cmp(disk.Size) == -1 {
		return field.Invalid(path, disk.Size.String(), fmt.Sprintf("sum disk sizes exceeds plan storage: %s", planSize.String()))
	}

	// Decrement the remaining amount of space available
	remainSize.Sub(disk.Size)
	return nil
}
