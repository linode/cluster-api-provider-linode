/*
Copyright 2025 Akamai Technologies, Inc.

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

package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

type LinodeMachineTemplateReconciler struct {
	client.Client
	Logger logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachinetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachines,verbs=get;list;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=linodemachinetemplates/status,verbs=get;update;patch

func (lmtr *LinodeMachineTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = lmtr.Logger.WithValues("linodemachinetemplate", req.NamespacedName)

	lmt := &infrav1alpha2.LinodeMachineTemplate{}
	if err := lmtr.Get(ctx, req.NamespacedName, lmt); err != nil {
		// If the object is not found, we can return early.
		if client.IgnoreNotFound(err) != nil {
			lmtr.Logger.Error(err, "unable to fetch LinodeMachineTemplate")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	lmtr.Logger.Info("Reconcile called for LinodeMachineTemplate", "name", lmt.Name)

	lmtScope, err := scope.NewMachineTemplateScope(
		ctx,
		scope.MachineTemplateScopeParams{
			Client:                lmtr.Client,
			LinodeMachineTemplate: lmt,
		},
	)
	if err != nil {
		lmtr.Logger.Error(err, "Failed to create Machine Template scope")

		return ctrl.Result{}, fmt.Errorf("failed to create Machine Template scope: %w", err)
	}

	return lmtr.reconcile(ctx, lmtScope)
}

func (lmtr *LinodeMachineTemplateReconciler) reconcile(ctx context.Context, lmtScope *scope.MachineTemplateScope) (ctrl.Result, error) {
	var outErr error
	var failureReason string

	if lmtScope.LinodeMachineTemplate.DeletionTimestamp != nil {
		// If the LinodeMachineTemplate is being deleted, we should not reconcile it.
		lmtr.Logger.Info("LinodeMachineTemplate is being deleted, skipping reconciliation", "name", lmtScope.LinodeMachineTemplate.Name)
		return ctrl.Result{}, nil
	}

	//nolint:dupl // Code duplication is simplicity in this case.
	defer func() {
		if outErr != nil {
			conditions.Set(lmtScope.LinodeMachineTemplate, metav1.Condition{
				Type:    string(clusterv1.ReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  failureReason,
				Message: outErr.Error(),
			})
		} else {
			conditions.Set(lmtScope.LinodeMachineTemplate, metav1.Condition{
				Type:    string(clusterv1.ReadyCondition),
				Status:  metav1.ConditionTrue,
				Reason:  "Reconciled",
				Message: "LinodeMachineTemplate reconciled successfully",
			})
		}

		if patchErr := lmtScope.Close(ctx); patchErr != nil && utilerrors.FilterOut(util.UnwrapError(patchErr), apierrors.IsNotFound) != nil {
			lmtr.Logger.Error(patchErr, "failed to patch LinodeMachineTemplate")
			outErr = errors.Join(outErr, patchErr)
		}
	}()

	// filter LinodeMachines that are templated by the given LinodeMachineTemplate
	linodeMachines := &infrav1alpha2.LinodeMachineList{}
	if outErr := lmtr.List(ctx, linodeMachines, client.InNamespace(lmtScope.LinodeMachineTemplate.Namespace)); outErr != nil {
		lmtr.Logger.Error(outErr, "Failed to list LinodeMachines for template", "template", lmtScope.LinodeMachineTemplate.Name)
		failureReason = "FailedToListLinodeMachines"
		return ctrl.Result{}, outErr
	}

	machinesFoundForTemplate := false

	for _, machine := range linodeMachines.Items {
		if machine.Annotations[clusterv1.TemplateClonedFromNameAnnotation] != lmtScope.LinodeMachineTemplate.Name {
			continue // Skip machines that are not templated by this LinodeMachineTemplate
		}

		machinesFoundForTemplate = true
		err := lmtr.reconcileUpdates(ctx, lmtScope.LinodeMachineTemplate, &machine)
		if err != nil {
			lmtr.Logger.Error(err, "Failed to add tags to LinodeMachine", "template", lmtScope.LinodeMachineTemplate.Name, "machine", machine.Name)
			outErr = errors.Join(outErr, err)

			failureReason = "FailedToPatchLinodeMachine"
		}
	}

	if !machinesFoundForTemplate {
		lmtr.Logger.Info("No LinodeMachines found for the template", "template", lmtScope.LinodeMachineTemplate.Name)
		return ctrl.Result{}, nil
	}

	// update the LMT status.tags if all the linodeMachines spec.tags is successfully updated.
	if outErr == nil {
		lmtScope.LinodeMachineTemplate.Status.Tags = slices.Clone(lmtScope.LinodeMachineTemplate.Spec.Template.Spec.Tags)
		lmtr.Logger.Info("Successfully reconciled LinodeMachineTemplate", "name", lmtScope.LinodeMachineTemplate.Name)
	} else {
		lmtr.Logger.Error(outErr, "Error in reconciling LinodeMachineTemplate, retrying..", "name", lmtScope.LinodeMachineTemplate.Name)
	}
	return ctrl.Result{}, outErr
}

func (lmtr *LinodeMachineTemplateReconciler) reconcileUpdates(ctx context.Context, lmt *infrav1alpha2.LinodeMachineTemplate, machine *infrav1alpha2.LinodeMachine) error {
	helper, err := patch.NewHelper(machine, lmtr.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %w", err)
	}

	if !slices.Equal(lmt.Spec.Template.Spec.Tags, lmt.Status.Tags) {
		machine.Spec.Tags = lmt.Spec.Template.Spec.Tags
		lmtr.Logger.Info("Update LinodeMachine with new tags", "machine", machine.Name, "tags", lmt.Spec.Template.Spec.Tags)
	}

	if machine.Spec.LabelPrefix != lmt.Spec.Template.Spec.LabelPrefix {
		machine.Spec.LabelPrefix = lmt.Spec.Template.Spec.LabelPrefix
		lmtr.Logger.Info("Update LinodeMachine with new label prefix", "machine", machine.Name, "labelPrefix", lmt.Spec.Template.Spec.LabelPrefix)
	}

	if err := helper.Patch(ctx, machine); err != nil {
		return fmt.Errorf("failed to patch LinodeMachine %s with new tags: %w", machine.Name, err)
	}
	return nil
}

func (lmtr *LinodeMachineTemplateReconciler) SetupWithManager(mgr ctrl.Manager, options crcontroller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha2.LinodeMachineTemplate{}).
		WithOptions(options).
		Complete(lmtr)
}
