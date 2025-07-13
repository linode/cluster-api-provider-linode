// /*
// Copyright 2025 Akamai Technologies, Inc.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package controller

import (
	"context"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func generateMachine(templateName, label string) infrav1alpha2.LinodeMachine {
	out := infrav1alpha2.LinodeMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
			Namespace: "default",
			Annotations: map[string]string{
				clusterv1.TemplateClonedFromNameAnnotation: templateName,
			},
		},
		Spec: infrav1alpha2.LinodeMachineSpec{},
	}

	if label != "" {
		out.Spec.Label = label
	}

	return out
}

func generateMachineTemplate(templateName string, tags []string, label string, statusTags []string) infrav1alpha2.LinodeMachineTemplate {
	return infrav1alpha2.LinodeMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
			Namespace: "default",
		},
		Spec: infrav1alpha2.LinodeMachineTemplateSpec{
			Template: infrav1alpha2.LinodeMachineTemplateResource{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Tags:  tags,
					Label: label,
				},
			},
		},
		Status: infrav1alpha2.LinodeMachineTemplateStatus{
			Tags: statusTags,
		},
	}
}

var _ = Describe("lifecycle", Ordered, Label("LinodeMachineTemplateReconciler", "lifecycle"), func() {

	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	var reconciler LinodeMachineTemplateReconciler
	var lmtScope scope.MachineTemplateScope
	// var linodeMT infrav1alpha2.LinodeMachineTemplate

	machineTemplates := []infrav1alpha2.LinodeMachineTemplate{
		generateMachineTemplate("machine-template-no-machines", nil, "", nil),
		generateMachineTemplate("machine-template-with-spec-tags", []string{"test-tag"}, "", nil),
		generateMachineTemplate("machine-template-no-tags-change", []string{"test-tag1"}, "test-label", []string{"test-tag1"}),
		generateMachineTemplate("machine-template-label-change", nil, "test-label", nil),
		generateMachineTemplate("machine-template-label-change-to-empty", nil, "", nil),
	}

	linodeMachines := []infrav1alpha2.LinodeMachine{
		generateMachine("machine-template-with-spec-tags", ""),
		generateMachine("machine-template-no-tags-change", ""),
		generateMachine("machine-template-label-change", ""),
		generateMachine("machine-template-label-change-to-empty", "old-label"),
	}

	BeforeAll(func(ctx SpecContext) {

		// Create the machine templates
		for _, template := range machineTemplates {
			Expect(k8sClient.Create(context.Background(), &template)).To(Succeed())
		}

		// create the machines
		for _, machine := range linodeMachines {
			Expect(k8sClient.Create(context.Background(), &machine)).To(Succeed())
		}

		DeferCleanup(func() {
			// Delete the machine templates
			for _, template := range machineTemplates {
				Expect(k8sClient.Delete(context.Background(), &template)).To(Succeed())
			}

			// Delete the machines
			for _, machine := range linodeMachines {
				Expect(k8sClient.Delete(context.Background(), &machine)).To(Succeed())
			}
		})
	})

	suite.Run(OneOf(
		Path(
			Call("no machines found for template", func(ctx context.Context, mck Mock) {}),
			Result("success", func(ctx context.Context, mck Mock) {
				patchHelper, err := patch.NewHelper(&machineTemplates[0], k8sClient)
				Expect(err).NotTo(HaveOccurred())
				lmtScope = scope.MachineTemplateScope{
					PatchHelper:           patchHelper,
					LinodeMachineTemplate: &machineTemplates[0],
				}
				reconciler = LinodeMachineTemplateReconciler{
					Logger: mck.Logger(),
					Client: k8sClient,
				}

				res, err := reconciler.reconcile(ctx, &lmtScope)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(ctrl.Result{}))
				Expect(mck.Logs()).To(ContainSubstring("No LinodeMachines found for the template"))
			}),
		),
		Path(
			Call("machine template update tags", func(ctx context.Context, mck Mock) {}),
			Result("success", func(ctx context.Context, mck Mock) {
				Expect(err).NotTo(HaveOccurred())
				lmtScope, _ := scope.NewMachineTemplateScope(
					ctx,
					scope.MachineTemplateScopeParams{
						Client:                k8sClient,
						LinodeMachineTemplate: &machineTemplates[1],
					},
				)
				reconciler = LinodeMachineTemplateReconciler{
					Logger: mck.Logger(),
					Client: k8sClient,
				}

				res, err := reconciler.reconcile(ctx, lmtScope)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(ctrl.Result{}))
				Expect(mck.Logs()).To(ContainSubstring("Update LinodeMachine with new tags"))

				// get the updated machineTemplate
				updatedMachineTemplate := &infrav1alpha2.LinodeMachineTemplate{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      machineTemplates[1].Name,
					Namespace: machineTemplates[1].Namespace,
				}, updatedMachineTemplate)).To(Succeed())
				Expect(updatedMachineTemplate.Status.Tags).To(Equal(updatedMachineTemplate.Spec.Template.Spec.Tags))
			}),
		),
		Path(
			Call("machine template no tags update", func(ctx context.Context, mck Mock) {}),
			Result("success", func(ctx context.Context, mck Mock) {
				patchHelper, err := patch.NewHelper(&machineTemplates[2], k8sClient)
				Expect(err).NotTo(HaveOccurred())

				lmtScope = scope.MachineTemplateScope{
					PatchHelper:           patchHelper,
					LinodeMachineTemplate: &machineTemplates[2],
				}
				reconciler = LinodeMachineTemplateReconciler{
					Logger: mck.Logger(),
					Client: k8sClient,
				}

				res, err := reconciler.reconcile(ctx, &lmtScope)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(ctrl.Result{}))
				Expect(mck.Logs()).NotTo(ContainSubstring("Patched LinodeMachine with new tags"))
			}),
		),
		Path(
			Call("machine template machine label update to non-empty value", func(ctx context.Context, mck Mock) {}),
			Result("success", func(ctx context.Context, mck Mock) {
				patchHelper, err := patch.NewHelper(&machineTemplates[3], k8sClient)
				Expect(err).NotTo(HaveOccurred())

				lmtScope = scope.MachineTemplateScope{
					PatchHelper:           patchHelper,
					LinodeMachineTemplate: &machineTemplates[3],
				}
				reconciler = LinodeMachineTemplateReconciler{
					Logger: mck.Logger(),
					Client: k8sClient,
				}

				res, err := reconciler.reconcile(ctx, &lmtScope)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(ctrl.Result{}))
				Expect(mck.Logs()).To(ContainSubstring("Update LinodeMachine with new label"))
			}),
		),
		Path(
			Call("machine template machine label update to empty value", func(ctx context.Context, mck Mock) {}),
			Result("success", func(ctx context.Context, mck Mock) {
				patchHelper, err := patch.NewHelper(&machineTemplates[4], k8sClient)
				Expect(err).NotTo(HaveOccurred())

				lmtScope = scope.MachineTemplateScope{
					PatchHelper:           patchHelper,
					LinodeMachineTemplate: &machineTemplates[4],
				}
				reconciler = LinodeMachineTemplateReconciler{
					Logger: mck.Logger(),
					Client: k8sClient,
				}

				res, err := reconciler.reconcile(ctx, &lmtScope)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(ctrl.Result{}))
				Expect(mck.Logs()).To(ContainSubstring("Update LinodeMachine with new label"))

				// get the updated machine
				updatedMachine := &infrav1alpha2.LinodeMachine{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      linodeMachines[3].Name,
					Namespace: linodeMachines[3].Namespace,
				}, updatedMachine)).To(Succeed())
				Expect(updatedMachine.Spec.Label).To(Equal(""))
			}),
		),
	),
	)

})
