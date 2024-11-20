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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

// log is for logging in this package.
//
//nolint:unused // Package logger variable is intended for future use in webhook implementation
var linodefirewalllog = logf.Log.WithName("linodefirewall-resource")

// SetupLinodeFirewallWebhookWithManager registers the webhook for LinodeFirewall in the manager.
func SetupLinodeFirewallWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&infrastructurev1alpha2.LinodeFirewall{}).
		WithValidator(&LinodeFirewallCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha2-linodefirewall,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=linodefirewalls,verbs=create;update,versions=v1alpha2,name=vlinodefirewall-v1alpha2.kb.io,admissionReviewVersions=v1

// LinodeFirewallCustomValidator struct is responsible for validating the LinodeFirewall resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type LinodeFirewallCustomValidator struct {
	// TODO (user):  Add more fields as needed for validation
}

var _ webhook.CustomValidator = &LinodeFirewallCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type LinodeFirewall.
func (v *LinodeFirewallCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	linodefirewall, ok := obj.(*infrastructurev1alpha2.LinodeFirewall)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeFirewall object but got %T", obj)
	}
	linodefirewalllog.Info("Validation for LinodeFirewall upon creation", "name", linodefirewall.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type LinodeFirewall.
func (v *LinodeFirewallCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	linodefirewall, ok := newObj.(*infrastructurev1alpha2.LinodeFirewall)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeFirewall object for the newObj but got %T", newObj)
	}
	linodefirewalllog.Info("Validation for LinodeFirewall upon update", "name", linodefirewall.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type LinodeFirewall.
func (v *LinodeFirewallCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	linodefirewall, ok := obj.(*infrastructurev1alpha2.LinodeFirewall)
	if !ok {
		return nil, fmt.Errorf("expected a LinodeFirewall object but got %T", obj)
	}
	linodefirewalllog.Info("Validation for LinodeFirewall upon deletion", "name", linodefirewall.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
