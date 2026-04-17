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
	"errors"
	"time"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/paused"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	rec "github.com/linode/cluster-api-provider-linode/util/reconciler"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("lifecycle", Ordered, Label("placementgroup", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	linodePG := infrav1alpha2.LinodePlacementGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: "default",
		},
		Spec: infrav1alpha2.LinodePlacementGroupSpec{
			Region:               "us-ord",
			PlacementGroupType:   "anti_affinity:local",
			PlacementGroupPolicy: "strict",
		},
	}

	objectKey := client.ObjectKeyFromObject(&linodePG)

	var reconciler LinodePlacementGroupReconciler
	var pgScope scope.PlacementGroupScope

	BeforeAll(func(ctx SpecContext) {
		pgScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &linodePG)).To(Succeed())
	})

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		pgScope.LinodeClient = mck.LinodeClient

		Expect(k8sClient.Get(ctx, objectKey, &linodePG)).To(Succeed())
		pgScope.LinodePlacementGroup = &linodePG

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&linodePG, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		pgScope.PatchHelper = patchHelper

		// Reset reconciler for each test
		reconciler = LinodePlacementGroupReconciler{
			Recorder: mck.Recorder(),
		}
	})

	suite.Run(
		OneOf(
			Path(
				Call("unable to create", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().ListPlacementGroups(ctx, gomock.Any()).Return([]linodego.PlacementGroup{}, nil)
					mck.LinodeClient.EXPECT().CreatePlacementGroup(ctx, gomock.Any()).Return(nil, errors.New("server error"))
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultPGControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing Placement Group creation"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("server error"))
					})),
				),
			),
			Path(
				Call("able to create", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().ListPlacementGroups(ctx, gomock.Any()).Return([]linodego.PlacementGroup{}, nil)
					mck.LinodeClient.EXPECT().CreatePlacementGroup(ctx, gomock.Any()).Return(&linodego.PlacementGroup{
						ID:     1,
						Region: "us-ord",
					}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
					Expect(err).NotTo(HaveOccurred())

					Expect(k8sClient.Get(ctx, objectKey, &linodePG)).To(Succeed())
					Expect(*linodePG.Spec.PGID).To(Equal(1))
					Expect(mck.Logs()).NotTo(ContainSubstring("Failed to create Placement Group"))
				}),
			),
		),
		Once("delete", func(ctx context.Context, _ Mock) {
			Expect(k8sClient.Delete(ctx, &linodePG)).To(Succeed())
			Expect(k8sClient.Get(ctx, objectKey, &linodePG)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("unable to get", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetPlacementGroup(ctx, gomock.Any()).Return(nil, errors.New("server error"))
				}),
				OneOf(
					Path(Result("delete requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultPGControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to fetch Placement Group from API"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("server error"))
					})),
				),
			),
			Path(
				Call("unable to delete", func(ctx context.Context, mck Mock) {
					getpg := mck.LinodeClient.EXPECT().GetPlacementGroup(ctx, gomock.Any()).Return(&linodego.PlacementGroup{
						ID:     1,
						Label:  "pg1",
						Region: "us-ord",
					}, nil)
					mck.LinodeClient.EXPECT().DeletePlacementGroup(ctx, gomock.Any()).After(getpg).Return(errors.New("server error"))
				}),
				OneOf(
					Path(Result("deletes are requeued", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultPGControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to delete Placement Group via API"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("server error"))
					})),
				),
			),
			Path(
				Call("with nodes still assigned", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetPlacementGroup(ctx, gomock.Any()).Return(&linodego.PlacementGroup{
						ID:     1,
						Label:  "pg1",
						Region: "us-ord",
						Members: []linodego.PlacementGroupMember{
							{
								LinodeID:    1,
								IsCompliant: true,
							},
							{
								LinodeID:    2,
								IsCompliant: true,
							},
						},
					}, nil)
				}),
				OneOf(
					Path(Result("delete requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultPGControllerReconcilerDelay))
						Expect(mck.Logs()).To(ContainSubstring("Placement Group has node(s) attached, re-queuing deletion to wait for detachment"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("Will not delete Placement Group"))
					})),
				),
			),
			Path(
				Call("with no nodes attached", func(ctx context.Context, mck Mock) {
					getPG := mck.LinodeClient.EXPECT().GetPlacementGroup(ctx, gomock.Any()).Return(&linodego.PlacementGroup{
						ID:      1,
						Label:   "pg1",
						Region:  "us-east",
						Members: []linodego.PlacementGroupMember{},
					}, nil)
					mck.LinodeClient.EXPECT().DeletePlacementGroup(ctx, gomock.Any()).After(getPG).Return(nil)
				}),
				Result("delete success", func(ctx context.Context, mck Mock) {
					res, err := reconciler.reconcile(ctx, mck.Logger(), &pgScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &linodePG))).To(BeTrue())
				}),
			),
		),
	)
})

var _ = Describe("pause handling", Label("placementgroup", "pause"), func() {
	It("sets paused condition for LinodePlacementGroup when owner Cluster is paused", func(ctx SpecContext) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pause-pg-cluster",
				Namespace: defaultNamespace,
			},
			Spec: clusterv1.ClusterSpec{
				Paused: ptr.To(true),
			},
		}
		placementGroup := &infrav1alpha2.LinodePlacementGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pause-pg-object",
				Namespace: defaultNamespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: cluster.Name,
				},
			},
			Spec: infrav1alpha2.LinodePlacementGroupSpec{
				Region:               "us-ord",
				PlacementGroupType:   "anti_affinity:local",
				PlacementGroupPolicy: "strict",
			},
		}

		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		Expect(k8sClient.Create(ctx, placementGroup)).To(Succeed())

		reconciler := &LinodePlacementGroupReconciler{
			Client:             k8sClient,
			LinodeClientConfig: scope.ClientConfig{Token: "test-token"},
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(placementGroup)})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(placementGroup), placementGroup)).To(Succeed())
		pausedCondition := placementGroup.GetCondition(clusterv1.PausedCondition)
		Expect(pausedCondition).NotTo(BeNil())
		Expect(pausedCondition.Status).To(Equal(metav1.ConditionTrue))
		Expect(controllerutil.ContainsFinalizer(placementGroup, infrav1alpha2.PlacementGroupFinalizer)).To(BeFalse())
	})

	It("sets and clears paused condition from the upstream pause annotation path", func(ctx SpecContext) {
		placementGroup := &infrav1alpha2.LinodePlacementGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pause-pg-annotation",
				Namespace: defaultNamespace,
				Annotations: map[string]string{
					clusterv1.PausedAnnotation: "",
				},
			},
			Spec: infrav1alpha2.LinodePlacementGroupSpec{
				Region:               "us-ord",
				PlacementGroupType:   "anti_affinity:local",
				PlacementGroupPolicy: "strict",
			},
		}

		Expect(k8sClient.Create(ctx, placementGroup)).To(Succeed())

		isPaused, _, err := paused.EnsurePausedCondition(ctx, k8sClient, nil, placementGroup)
		Expect(err).NotTo(HaveOccurred())
		Expect(isPaused).To(BeTrue())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(placementGroup), placementGroup)).To(Succeed())
		pausedCondition := placementGroup.GetCondition(clusterv1.PausedCondition)
		Expect(pausedCondition).NotTo(BeNil())
		Expect(pausedCondition.Status).To(Equal(metav1.ConditionTrue))

		delete(placementGroup.Annotations, clusterv1.PausedAnnotation)
		Expect(k8sClient.Update(ctx, placementGroup)).To(Succeed())

		isPaused, _, err = paused.EnsurePausedCondition(ctx, k8sClient, nil, placementGroup)
		Expect(err).NotTo(HaveOccurred())
		Expect(isPaused).To(BeFalse())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(placementGroup), placementGroup)).To(Succeed())
		pausedCondition = placementGroup.GetCondition(clusterv1.PausedCondition)
		Expect(pausedCondition).NotTo(BeNil())
		Expect(pausedCondition.Status).To(Equal(metav1.ConditionFalse))
	})
})
