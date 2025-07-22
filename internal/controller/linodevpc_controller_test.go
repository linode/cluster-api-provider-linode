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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	rec "github.com/linode/cluster-api-provider-linode/util/reconciler"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("lifecycle", Ordered, Label("vpc", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	linodeVPC := infrav1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: "default",
		},
		Spec: infrav1alpha2.LinodeVPCSpec{
			Region: "us-east",
			Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
				{Label: "subnet1", IPv4: "10.0.0.0/8"},
			},
		},
	}

	objectKey := client.ObjectKeyFromObject(&linodeVPC)

	var reconciler LinodeVPCReconciler
	var vpcScope scope.VPCScope

	BeforeAll(func(ctx SpecContext) {
		vpcScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &linodeVPC)).To(Succeed())
	})

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		vpcScope.LinodeClient = mck.LinodeClient

		Expect(k8sClient.Get(ctx, objectKey, &linodeVPC)).To(Succeed())
		vpcScope.LinodeVPC = &linodeVPC

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		vpcScope.PatchHelper = patchHelper

		// Reset reconciler for each test
		reconciler = LinodeVPCReconciler{
			Recorder: mck.Recorder(),
		}
	})

	suite.Run(
		OneOf(
			Path(
				Call("unable to create", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().ListVPCs(ctx, gomock.Any()).Return([]linodego.VPC{}, nil)
					mck.LinodeClient.EXPECT().CreateVPC(ctx, gomock.Any()).Return(nil, errors.New("server error"))
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultVPCControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing VPC creation"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("server error"))
					})),
				),
			),
			Path(
				Call("able to create", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().ListVPCs(ctx, gomock.Any()).Return([]linodego.VPC{}, nil)
					mck.LinodeClient.EXPECT().CreateVPC(ctx, gomock.Any()).Return(&linodego.VPC{
						ID:     1,
						Region: "us-east",
						Subnets: []linodego.VPCSubnet{
							{Label: "subnet1", IPv4: "10.0.0.0/8", IPv6: []linodego.VPCIPv6Range{{Range: "2001:db8::/52"}}},
						},
					}, nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
					Expect(err).NotTo(HaveOccurred())

					Expect(k8sClient.Get(ctx, objectKey, &linodeVPC)).To(Succeed())
					Expect(*linodeVPC.Spec.VPCID).To(Equal(1))
					Expect(linodeVPC.Spec.Subnets[0].IPv4).To(Equal("10.0.0.0/8"))
					Expect(linodeVPC.Spec.Subnets[0].IPv6).To(ContainElement(linodego.VPCIPv6Range{Range: "2001:db8::/52"}))
					Expect(linodeVPC.Spec.Subnets[0].Label).To(Equal("subnet1"))
					Expect(mck.Logs()).NotTo(ContainSubstring("Failed to create VPC"))
				}),
			),
		),
		Once("update", func(ctx context.Context, _ Mock) {
			linodeVPC.Spec.Description = "update"
			Expect(k8sClient.Update(ctx, &linodeVPC)).To(Succeed())
			Expect(k8sClient.Get(ctx, objectKey, &linodeVPC)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("able to list VPC", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().ListVPCs(ctx, gomock.Any()).Return([]linodego.VPC{
						{
							ID:     1,
							Label:  "vpc1",
							Region: "us-east",
							Subnets: []linodego.VPCSubnet{
								{
									Label: "subnet1",
									IPv4:  "10.0.0.0/8",
									IPv6:  []linodego.VPCIPv6Range{{Range: "2001:db8::/52"}},
								},
							},
						},
					}, nil)
				}),
				Result("update success", func(ctx context.Context, mck Mock) {
					err = vpcScope.Client.Get(ctx, client.ObjectKeyFromObject(vpcScope.LinodeVPC), vpcScope.LinodeVPC)
					Expect(err).NotTo(HaveOccurred())

					_, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(mck.Logs()).NotTo(ContainSubstring("Failed to update VPC"))
				}),
			),
			Path(
				Call("unable to list VPC", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().ListVPCs(ctx, gomock.Any()).Return(nil, errors.New("server error"))
				}),
				OneOf(
					Path(Result("update requeues", func(ctx context.Context, mck Mock) {
						conditions.Set(vpcScope.LinodeVPC, metav1.Condition{
							Type:    string(clusterv1.ReadyCondition),
							Status:  metav1.ConditionFalse,
							Reason:  "test",
							Message: "test",
						})
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultVPCControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing VPC update"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("server error"))
					})),
				),
			),
		),
		Once("delete", func(ctx context.Context, _ Mock) {
			Expect(k8sClient.Delete(ctx, &linodeVPC)).To(Succeed())
			Expect(k8sClient.Get(ctx, objectKey, &linodeVPC)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("unable to get", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetVPC(ctx, gomock.Any()).Return(nil, errors.New("server error"))
				}),
				OneOf(
					Path(Result("delete requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultVPCControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to fetch VPC"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("server error"))
					})),
				),
			),
			Path(
				Call("unable to delete", func(ctx context.Context, mck Mock) {
					getVPC := mck.LinodeClient.EXPECT().GetVPC(ctx, gomock.Any()).Return(&linodego.VPC{
						ID:      1,
						Label:   "vpc1",
						Region:  "us-east",
						Updated: ptr.To(time.Now()),
						Subnets: []linodego.VPCSubnet{{}},
					}, nil)
					mck.LinodeClient.EXPECT().DeleteVPC(ctx, gomock.Any()).After(getVPC).Return(errors.New("server error"))
				}),
				OneOf(
					Path(Result("deletes are requeued", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultVPCControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to delete VPC"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("server error"))
					})),
				),
			),
			Path(
				Call("with nodes still attached", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetVPC(ctx, gomock.Any()).Return(&linodego.VPC{
						ID:      1,
						Label:   "vpc1",
						Region:  "us-east",
						Updated: ptr.To(time.Now()),
						Subnets: []linodego.VPCSubnet{
							{
								Linodes: []linodego.VPCSubnetLinode{{ID: 1}},
							},
						},
					}, nil)
				}),
				OneOf(
					Path(Result("delete requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultVPCControllerWaitForHasNodesDelay))
						Expect(mck.Logs()).To(ContainSubstring("VPC has node(s) attached, re-queuing VPC deletion"))
					})),
					Path(Result("timeout error", func(ctx context.Context, mck Mock) {
						reconciler.ReconcileTimeout = time.Nanosecond
						res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
						Expect(err).To(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
						Expect(mck.Events()).To(ContainSubstring("will not delete VPC with node(s) attached"))
					})),
				),
			),
			Path(
				Call("with no nodes attached", func(ctx context.Context, mck Mock) {
					getVPC := mck.LinodeClient.EXPECT().GetVPC(ctx, gomock.Any()).Return(&linodego.VPC{
						ID:      1,
						Label:   "vpc1",
						Region:  "us-east",
						Updated: ptr.To(time.Now()),
						Subnets: []linodego.VPCSubnet{{}},
					}, nil)
					mck.LinodeClient.EXPECT().DeleteVPC(ctx, gomock.Any()).After(getVPC).Return(nil)
				}),
				Result("delete success", func(ctx context.Context, mck Mock) {
					res, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
					k8sClient.Get(ctx, objectKey, &linodeVPC)
					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &linodeVPC))).To(BeTrue())
				}),
			),
		),
	)
})

var _ = Describe("retained VPC", Label("vpc", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	var reconciler LinodeVPCReconciler
	var vpcScope scope.VPCScope
	var linodeVPC infrav1alpha2.LinodeVPC

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		vpcScope.Client = k8sClient
		linodeVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "retained-vpc-",
				Namespace:    "default",
				Finalizers:   []string{infrav1alpha2.VPCFinalizer},
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				VPCID:     ptr.To(123),
				Region:    "us-east",
				IPv6Range: []infrav1alpha2.VPCCreateOptionsIPv6{{Range: ptr.To("/52")}},
				Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
					{Label: "subnet1", IPv4: "10.0.0.0/8", SubnetID: 1, Retain: true, IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{{Range: ptr.To("/56")}}},
					{Label: "subnet2", IPv4: "10.0.1.0/24", SubnetID: 2, IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{{Range: ptr.To("/56")}}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &linodeVPC)).To(Succeed())

		// Add deletion timestamp to trigger reconcileDelete
		Expect(k8sClient.Delete(ctx, &linodeVPC)).To(Succeed())

		vpcScope.LinodeClient = mck.LinodeClient

		reconciler = LinodeVPCReconciler{
			Recorder: mck.Recorder(),
		}

		// Get the resource back to ensure we have the latest state with the deletion timestamp.
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC)).To(Succeed())
		vpcScope.LinodeVPC = &linodeVPC

		// Initialize patch helper with the deleted object.
		patchHelper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		vpcScope.PatchHelper = patchHelper
	})

	AfterEach(func(ctx SpecContext) {
		err := k8sClient.Delete(ctx, &linodeVPC)
		if err != nil {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	})

	suite.Run(
		OneOf(
			Path(
				Call("retained VPC is not deleted", func(ctx context.Context, mck Mock) {
					vpcScope.LinodeVPC.Spec.Retain = true
					mck.LinodeClient.EXPECT().GetVPC(ctx, gomock.Any()).Return(&linodego.VPC{
						ID:      1,
						Label:   "vpc1",
						Region:  "us-east",
						Updated: ptr.To(time.Now()),
						IPv6:    []linodego.VPCIPv6Range{{Range: "2001:db8::/52"}},
						Subnets: []linodego.VPCSubnet{
							{ID: 1, Label: "subnet1", IPv4: "10.0.0.0/8", IPv6: []linodego.VPCIPv6Range{{Range: "2001:db8:8:1::/56"}}},
							{ID: 2, Label: "subnet2", IPv4: "10.0.1.0/24", IPv6: []linodego.VPCIPv6Range{{Range: "2001:db8:8:2::/56"}}},
						},
					}, nil)

					mck.LinodeClient.EXPECT().DeleteVPCSubnet(ctx, *vpcScope.LinodeVPC.Spec.VPCID, vpcScope.LinodeVPC.Spec.Subnets[1].SubnetID).Return(nil)

				}),
				Result("retained success", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
					Expect(err).NotTo(HaveOccurred())

					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC))).To(BeTrue())
					Expect(mck.Logs()).To(ContainSubstring("VPC has retain flag, skipping VPC deletion"))
				}),
			),
			Path(
				Call("retained VPC with subnet deletion disabled", func(ctx context.Context, mck Mock) {
					vpcScope.LinodeVPC.Spec.Retain = true
					vpcScope.LinodeVPC.Spec.Subnets[0].Retain = true
					vpcScope.LinodeVPC.Spec.Subnets[1].Retain = true

					mck.LinodeClient.EXPECT().GetVPC(ctx, gomock.Any()).Return(&linodego.VPC{
						ID:      1,
						Label:   "vpc1",
						Region:  "us-east",
						Updated: ptr.To(time.Now()),
						Subnets: []linodego.VPCSubnet{
							{ID: 1, Label: "subnet1", IPv4: "10.0.0.0/8"},
							{ID: 2, Label: "subnet2", IPv4: "10.0.1.0/24"},
						},
					}, nil)

				}),
				Result("unretained subnets are not deleted", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
					Expect(err).NotTo(HaveOccurred())

					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC))).To(BeTrue())
					Expect(mck.Logs()).NotTo(ContainSubstring("deleting subnet"))
				}),
			),
			Path(
				Call("retained VPC with unretained subnet deletion", func(ctx context.Context, mck Mock) {
					vpcScope.LinodeVPC.Spec.Retain = true
					vpcScope.LinodeVPC.Spec.Subnets[0].Retain = true
					vpcScope.LinodeVPC.Spec.Subnets[1].Retain = false

					mck.LinodeClient.EXPECT().GetVPC(ctx, *vpcScope.LinodeVPC.Spec.VPCID).Return(&linodego.VPC{
						ID: *vpcScope.LinodeVPC.Spec.VPCID,
						Subnets: []linodego.VPCSubnet{
							{ID: vpcScope.LinodeVPC.Spec.Subnets[0].SubnetID},
							{ID: vpcScope.LinodeVPC.Spec.Subnets[1].SubnetID},
						},
					}, nil)
					mck.LinodeClient.EXPECT().DeleteVPCSubnet(ctx, *vpcScope.LinodeVPC.Spec.VPCID, vpcScope.LinodeVPC.Spec.Subnets[1].SubnetID).Return(nil)
				}),
				Result("unretained subnets deleted", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
					Expect(err).NotTo(HaveOccurred())

					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC))).To(BeTrue())
					Expect(mck.Logs()).To(ContainSubstring("deleting subnet"))
				}),
			),
		),
	)
})

var _ = Describe("adopt existing VPC", Label("vpc", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	var reconciler LinodeVPCReconciler
	var vpcScope scope.VPCScope
	var linodeVPC infrav1alpha2.LinodeVPC

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		vpcScope.Client = k8sClient
		linodeVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "adopt-vpc-",
				Namespace:    "default",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "us-east",
				Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
					{Label: "adopted-subnet", IPv4: "10.0.0.0/8"},
					{Label: "created-subnet", IPv4: "10.0.1.0/24"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &linodeVPC)).To(Succeed())

		vpcScope.LinodeClient = mck.LinodeClient

		reconciler = LinodeVPCReconciler{
			Recorder: mck.Recorder(),
		}

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC)).To(Succeed())
		vpcScope.LinodeVPC = &linodeVPC

		patchHelper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		vpcScope.PatchHelper = patchHelper
	})

	AfterEach(func(ctx SpecContext) {
		err := k8sClient.Delete(ctx, &linodeVPC)
		if err != nil {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	})

	suite.Run(
		Path(
			Call("adopt existing VPC and create missing subnet", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().ListVPCs(ctx, gomock.Any()).Return([]linodego.VPC{
					{
						ID:     1,
						Label:  "adopt-vpc-",
						Region: "us-east",
						Subnets: []linodego.VPCSubnet{
							{
								ID:    1,
								Label: "adopted-subnet",
								IPv4:  "10.0.0.0/8",
							},
						},
					},
				}, nil)
				mck.LinodeClient.EXPECT().CreateVPCSubnet(ctx, gomock.Any(), 1).Return(&linodego.VPCSubnet{
					ID:    2,
					Label: "created-subnet",
					IPv4:  "10.0.1.0/24",
				}, nil)
			}),
			Result("adopt and create success", func(ctx context.Context, mck Mock) {
				_, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
				Expect(err).NotTo(HaveOccurred())

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC)).To(Succeed())
				Expect(len(linodeVPC.Spec.Subnets)).To(Equal(2))
				Expect(linodeVPC.Spec.Subnets[0].SubnetID).To(Equal(1))
				Expect(linodeVPC.Spec.Subnets[1].SubnetID).To(Equal(2))
			}),
		),
	)
})

var _ = Describe("name changing VPC", Label("vpc", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	var reconciler LinodeVPCReconciler
	var vpcScope scope.VPCScope
	var linodeVPC infrav1alpha2.LinodeVPC

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		vpcScope.Client = k8sClient
		linodeVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "changing-vpc-",
				Namespace:    "default",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region: "us-east",
				Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
					{Label: "changing-subnet", SubnetID: 1, IPv4: "10.0.0.0/8"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &linodeVPC)).To(Succeed())

		vpcScope.LinodeClient = mck.LinodeClient

		reconciler = LinodeVPCReconciler{
			Recorder: mck.Recorder(),
		}

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC)).To(Succeed())
		vpcScope.LinodeVPC = &linodeVPC

		patchHelper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		vpcScope.PatchHelper = patchHelper
	})

	AfterEach(func(ctx SpecContext) {
		err := k8sClient.Delete(ctx, &linodeVPC)
		if err != nil {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	})

	suite.Run(
		Path(
			Call("get existing VPC and adapt to name changes", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().ListVPCs(ctx, gomock.Any()).Return([]linodego.VPC{
					{
						ID:     1,
						Label:  "changed-vpc-",
						Region: "us-east",
						Subnets: []linodego.VPCSubnet{
							{
								ID:    1,
								Label: "changed-subnet-",
								IPv4:  "10.0.0.0/8",
							},
						},
					},
				}, nil)
			}),
			Result("reconcile VPC with changed name and create success", func(ctx context.Context, mck Mock) {
				_, err := reconciler.reconcile(ctx, mck.Logger(), &vpcScope)
				Expect(err).NotTo(HaveOccurred())

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC)).To(Succeed())
				Expect(len(linodeVPC.Spec.Subnets)).To(Equal(1))
				Expect(linodeVPC.Spec.Subnets[0].SubnetID).To(Equal(1))
				Expect(linodeVPC.Spec.Subnets[0].Label).To(Equal("changed-subnet-"))
			}),
		),
	)
})
