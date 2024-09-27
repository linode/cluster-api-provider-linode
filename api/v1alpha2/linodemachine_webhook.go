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

	. "github.com/linode/cluster-api-provider-linode/clients"
)

var (
	// The list of valid device slots that data device disks may attach to.
	// NOTE: sda is reserved for the OS device disk.
	LinodeMachineDevicePaths = []string{"sdb", "sdc", "sdd", "sde", "sdf", "sdg", "sdh"}

	// The maximum number of device disks allowed per [Configuration Profile per Linode’s Instance].
	//
	// [Configuration Profile per Linode’s Instance]: https://www.linode.com/docs/api/linode-instances/#configuration-profile-view
	LinodeMachineMaxDisk = 8

	// The maximum number of data device disks allowed in a Linode’s Instance's configuration profile.
	// NOTE: The first device disk is reserved for the OS disk
	LinodeMachineMaxDataDisk = LinodeMachineMaxDisk - 1
)

// log is for logging in this package.
var linodemachinelog = logf.Log.WithName("linodemachine-resource")

type linodeMachineValidator struct {
	Client client.Client
}

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LinodeMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&linodeMachineValidator{Client: mgr.GetClient()}).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable update and deletion validation.
// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodemachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodemachines,verbs=create,versions=v1alpha2,name=validation.linodemachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *linodeMachineValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	machine, ok := obj.(*LinodeMachine)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeMachine Resource")
	}
	spec := machine.Spec
	linodemachinelog.Info("validate create", "name", machine.Name)

	var linodeclient LinodeClient = defaultLinodeClient

	if spec.CredentialsRef != nil {
		apiToken, err := getCredentialDataFromRef(ctx, r.Client, *spec.CredentialsRef, machine.GetNamespace())
		if err != nil {
			linodemachinelog.Info("credentials from secret ref error", "name", machine.Name)
			return nil, err
		}
		linodemachinelog.Info("creating a verfied linode client for create request", "name", machine.Name)
		linodeclient.SetToken(string(apiToken))
	}
	var errs field.ErrorList

	if err := r.validateLinodeMachineSpec(ctx, linodeclient, spec); err != nil {
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
	old, ok := oldObj.(*LinodeMachine)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeMachine Resource")
	}
	linodemachinelog.Info("validate update", "name", old.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *linodeMachineValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*LinodeMachine)
	if !ok {
		return nil, apierrors.NewBadRequest("expected a LinodeCluster Resource")
	}
	linodemachinelog.Info("validate delete", "name", c.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *linodeMachineValidator) validateLinodeMachineSpec(ctx context.Context, linodeclient LinodeClient, spec LinodeMachineSpec) field.ErrorList {
	var errs field.ErrorList

	if err := validateRegion(ctx, linodeclient, spec.Region, field.NewPath("spec").Child("region")); err != nil {
		errs = append(errs, err)
	}
	plan, err := validateLinodeType(ctx, linodeclient, spec.Type, field.NewPath("spec").Child("type"))
	if err != nil {
		errs = append(errs, err)
	}
	if err := r.validateLinodeMachineDisks(plan, spec); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (r *linodeMachineValidator) validateLinodeMachineDisks(plan *linodego.LinodeType, spec LinodeMachineSpec) *field.Error {
	// The Linode plan information is required to perform disk validation
	if plan == nil {
		return nil
	}

	var (
		// The Linode API represents storage sizes in megabytes (MB)
		// https://www.linode.com/docs/api/linode-types/#type-view
		planSize   = resource.MustParse(fmt.Sprintf("%d%s", plan.Disk, "M"))
		remainSize = &resource.Quantity{}
		err        *field.Error
	)
	planSize.DeepCopyInto(remainSize)

	if remainSize, err = validateDisk(spec.OSDisk, field.NewPath("spec").Child("osDisk"), remainSize, &planSize); err != nil {
		return err
	}
	if _, err := validateDataDisks(spec.DataDisks, field.NewPath("spec").Child("dataDisks"), remainSize, &planSize); err != nil {
		return err
	}

	return nil
}

func validateDataDisks(disks map[string]*InstanceDisk, path *field.Path, remainSize, planSize *resource.Quantity) (*resource.Quantity, *field.Error) {
	devs := []string{}

	for dev, disk := range disks {
		if !slices.Contains(LinodeMachineDevicePaths, dev) {
			return nil, field.Forbidden(path.Child(dev), fmt.Sprintf("allowed device paths: %v", LinodeMachineDevicePaths))
		}
		if slices.Contains(devs, dev) {
			return nil, field.Duplicate(path.Child(dev), "duplicate device path")
		}
		devs = append(devs, dev)
		if len(devs) > LinodeMachineMaxDataDisk {
			return nil, field.TooMany(path, len(devs), LinodeMachineMaxDataDisk)
		}

		var err *field.Error
		if remainSize, err = validateDisk(disk, path.Child(dev), remainSize, planSize); err != nil {
			return nil, err
		}
	}
	return remainSize, nil
}

func validateDisk(disk *InstanceDisk, path *field.Path, remainSize, planSize *resource.Quantity) (*resource.Quantity, *field.Error) {
	if disk == nil {
		return remainSize, nil
	}

	if disk.Size.Sign() < 1 {
		return nil, field.Invalid(path, disk.Size.String(), "invalid size")
	}
	if remainSize.Cmp(disk.Size) == -1 {
		return nil, field.Invalid(path, disk.Size.String(), fmt.Sprintf("sum disk sizes exceeds plan storage: %s", planSize.String()))
	}

	// Decrement the remaining amount of space available
	remainSize.Sub(disk.Size)
	return remainSize, nil
}
