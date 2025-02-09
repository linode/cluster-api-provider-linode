// /*
// Copyright 2023 Akamai Technologies, Inc.

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
	"fmt"
	"net/http"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"time"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
	rec "github.com/linode/cluster-api-provider-linode/util/reconciler"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("lifecycle", Ordered, Label("firewalls", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	addrSetRefs := []*corev1.ObjectReference{{Namespace: defaultNamespace, Name: "lifecycle"}}
	inboundRuleRefs := []*corev1.ObjectReference{{Namespace: defaultNamespace, Name: "lifecycle"}}
	inboundRules := []infrav1alpha2.FirewallRuleSpec{{
		Action:      "ACCEPT",
		Label:       "a-label-that-is-way-too-long-and-should-be-truncated",
		Description: "allow-ssh",
		Ports:       "22",
		Protocol:    "TCP",
		Addresses: &infrav1alpha2.NetworkAddresses{
			IPv4: &[]string{"0.0.0.0/0"},
			IPv6: &[]string{"::/0"},
		},
		AddressSetRefs: addrSetRefs,
	}}
	outboundRules := []infrav1alpha2.FirewallRuleSpec{{
		Action:      "DROP",
		Label:       "another-label-that-is-way-too-long-and-should-be-truncated",
		Description: "deny-foo",
		Ports:       "6435",
		Protocol:    "TCP",
		Addresses: &infrav1alpha2.NetworkAddresses{
			IPv4: &[]string{"1.2.3.4/32"},
			IPv6: &[]string{"::/0"},
		},
	}}
	linodeFW := infrav1alpha2.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: defaultNamespace,
		},
		Spec: infrav1alpha2.LinodeFirewallSpec{
			FirewallID:      nil,
			Enabled:         true,
			InboundRules:    inboundRules,
			InboundRuleRefs: inboundRuleRefs,
			OutboundRules:   outboundRules,
			InboundPolicy:   "DROP",
			OutboundPolicy:  "ACCEPT",
		},
	}
	addrSet := infrav1alpha2.AddressSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: defaultNamespace,
		},
		Spec: infrav1alpha2.AddressSetSpec{
			IPv4: &[]string{"10.0.0.0/11"},
			IPv6: &[]string{"::/0"},
		},
	}
	fwRule := infrav1alpha2.FirewallRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: defaultNamespace,
		},
		Spec: infrav1alpha2.FirewallRuleSpec{
			Action:      "ACCEPT",
			Label:       "fwrule-test",
			Description: "allow-ssh",
			Ports:       "22",
			Protocol:    "TCP",
			Addresses: &infrav1alpha2.NetworkAddresses{
				IPv4: &[]string{"192.168.0.0/16"},
			},
		},
	}

	fwObjectKey := client.ObjectKeyFromObject(&linodeFW)
	addrSetObjectKey := client.ObjectKeyFromObject(&addrSet)
	fwRuleObjectKey := client.ObjectKeyFromObject(&fwRule)

	var reconciler LinodeFirewallReconciler
	var fwScope scope.FirewallScope

	BeforeAll(func(ctx SpecContext) {
		fwScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &addrSet)).To(Succeed())
		Expect(k8sClient.Create(ctx, &fwRule)).To(Succeed())
		Expect(k8sClient.Create(ctx, &linodeFW)).To(Succeed())
	})

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		fwScope.LinodeClient = mck.LinodeClient

		Expect(k8sClient.Get(ctx, fwObjectKey, &linodeFW)).To(Succeed())
		fwScope.LinodeFirewall = &linodeFW
		Expect(k8sClient.Get(ctx, addrSetObjectKey, &addrSet)).To(Succeed())
		Expect(k8sClient.Get(ctx, fwRuleObjectKey, &fwRule)).To(Succeed())

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&linodeFW, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		fwScope.PatchHelper = patchHelper

		// Reset reconciler for each test
		reconciler = LinodeFirewallReconciler{
			Recorder: mck.Recorder(),
		}
		reconciler.Client = k8sClient
	})

	suite.Run(
		OneOf(
			Path(
				Call("unable to create", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().CreateFirewall(ctx, gomock.Any()).Return(nil, &linodego.Error{Code: http.StatusInternalServerError})
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						// first one is for pause
						res, err = reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultFWControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing Firewall create"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
					})),
				),
			),
			Path(Result("unable to create with too many rules", func(ctx context.Context, mck Mock) {
				for idx := 0; idx < 255; idx++ {
					linodeFW.Spec.InboundRules = append(linodeFW.Spec.InboundRules, infrav1alpha2.FirewallRuleSpec{
						Action:   "ACCEPT",
						Ports:    "22",
						Protocol: "TCP",
						Addresses: &infrav1alpha2.NetworkAddresses{
							IPv4: &[]string{fmt.Sprintf("192.168.%d.%d", idx, 0)},
						}})
				}
				res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
				Expect(err).ToNot(HaveOccurred())
				Expect(mck.Logs()).To(ContainSubstring("too many IPs in this ACL"))
				Expect(res.Requeue).To(BeFalse())
			})),
			Path(
				Call("able to create", func(ctx context.Context, mck Mock) {
					linodeFW.Spec.InboundRules = inboundRules
					mck.LinodeClient.EXPECT().CreateFirewall(ctx, gomock.Any()).Return(&linodego.Firewall{
						ID: 1,
					}, nil)
					mck.LinodeClient.EXPECT().UpdateFirewall(ctx, 1, gomock.Any()).Return(nil, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
					Expect(err).NotTo(HaveOccurred())
					// once more after pause
					_, err = reconciler.reconcile(ctx, mck.Logger(), &fwScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(k8sClient.Get(ctx, fwObjectKey, &linodeFW)).To(Succeed())
					Expect(*linodeFW.Spec.FirewallID).To(Equal(1))
					Expect(mck.Logs()).NotTo(ContainSubstring("failed to create Firewall"))
				}),
			),
			Path(
				Call("unable to update", func(ctx context.Context, mck Mock) {
					linodeFW.Spec.FirewallID = util.Pointer(1)
					mck.LinodeClient.EXPECT().GetFirewall(ctx, 1).Return(&linodego.Firewall{
						ID: 1,
					}, nil)
				}),
				OneOf(
					Path(Result("update requeues for update rules error", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().UpdateFirewallRules(ctx, 1, gomock.Any()).Return(nil, &linodego.Error{Code: http.StatusInternalServerError})
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())

						conditions.Set(fwScope.LinodeFirewall, metav1.Condition{
							Type:    string(clusterv1.ReadyCondition),
							Status:  metav1.ConditionFalse,
							Reason:  "test",
							Message: "test",
						})
						// after pause is done, do the real reconcile
						res, err = reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultFWControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing Firewall update"))
					})),
					Path(Result("update requeues for update error", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().UpdateFirewallRules(ctx, 1, gomock.Any()).Return(nil, nil)
						mck.LinodeClient.EXPECT().UpdateFirewall(ctx, 1, gomock.Any()).Return(nil, &linodego.Error{Code: http.StatusInternalServerError})
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultFWControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing Firewall update"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().UpdateFirewallRules(ctx, 1, gomock.Any()).Return(nil, nil)
						mck.LinodeClient.EXPECT().UpdateFirewall(ctx, 1, gomock.Any()).Return(nil, &linodego.Error{Code: http.StatusInternalServerError})
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
					})),
				),
			),
			Path(
				Call("able to update", func(ctx context.Context, mck Mock) {
					linodeFW.Spec.FirewallID = util.Pointer(1)
					ipv4s := []string{}
					for idx := 0; idx < 256; idx++ {
						ipv4s = append(ipv4s, fmt.Sprintf("192.168.%d.%d", idx, 0))
					}
					linodeFW.Spec.InboundRules = append(linodeFW.Spec.InboundRules, infrav1alpha2.FirewallRuleSpec{
						Action:   "ACCEPT",
						Ports:    "22",
						Protocol: "TCP",
						Addresses: &infrav1alpha2.NetworkAddresses{
							IPv4: &ipv4s,
						}})

					mck.LinodeClient.EXPECT().GetFirewall(ctx, 1).Return(&linodego.Firewall{
						ID: 1,
					}, nil)
					mck.LinodeClient.EXPECT().UpdateFirewallRules(ctx, 1, gomock.Any()).Return(nil, nil)
					mck.LinodeClient.EXPECT().UpdateFirewall(ctx, 1, gomock.Any()).Return(nil, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
					Expect(err).NotTo(HaveOccurred())

					Expect(k8sClient.Get(ctx, fwObjectKey, &linodeFW)).To(Succeed())
					Expect(*linodeFW.Spec.FirewallID).To(Equal(1))
					Expect(mck.Logs()).NotTo(ContainSubstring("failed to update Firewall"))
				}),
			),
		),
		OneOf(
			Path(
				Call("unable to delete", func(ctx context.Context, mck Mock) {
					Expect(k8sClient.Delete(ctx, &linodeFW)).To(Succeed())
					linodeFW.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					mck.LinodeClient.EXPECT().DeleteFirewall(ctx, 1).Return(&linodego.Error{Code: http.StatusInternalServerError})
				}),
				OneOf(
					Path(Result("deletes are requeued", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						// Now do it after the pause is done
						res, err = reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultFWControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("failed to delete Firewall"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						// Now pause is done
						res, err = reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
					})),
				),
			),
			Path(
				Call("able to delete", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().DeleteFirewall(ctx, 1).Return(nil)
				}),
				OneOf(
					Path(Result("success", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &fwScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Logs()).NotTo(ContainSubstring("failed to delete Firewall"))
					})),
				),
			),
		),
	)
})
