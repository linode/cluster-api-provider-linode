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
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

// log is for logging in this package.
//
//nolint:unused // Package logger variable is intended for future use in webhook implementation
var linodemachinetemplatelog = logf.Log.WithName("linodemachinetemplate-resource")

// SetupLinodeMachineTemplateWebhookWithManager registers the webhook for LinodeMachineTemplate in the manager.
func SetupLinodeMachineTemplateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&infrav1alpha2.LinodeMachineTemplate{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
