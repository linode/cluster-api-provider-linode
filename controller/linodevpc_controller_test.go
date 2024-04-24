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
	"bytes"
	"errors"
	"time"

	"github.com/go-logr/logr"
	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	rec "github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/linodego"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("lifecycle", Ordered, Label("vpc", "lifecycle"), func() {
	var linodeVPC infrav1alpha1.LinodeVPC
	var reconciler *LinodeVPCReconciler

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		reconciler = &LinodeVPCReconciler{
			Recorder: recorder,
		}

		vpcSpec := infrav1alpha1.LinodeVPCSpec{
			Region: "us-east",
			Subnets: []infrav1alpha1.VPCSubnetCreateOptions{
				{Label: "subnet1", IPv4: "10.0.0.0/8"},
			},
		}
		linodeVPC = infrav1alpha1.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Labels:    make(map[string]string),
			},
			Spec: vpcSpec,
		}

		mockCtrl = gomock.NewController(GinkgoT())
		testLogs = &bytes.Buffer{}
		logger = zap.New(
			zap.WriteTo(GinkgoWriter),
			zap.WriteTo(testLogs),
			zap.UseDevMode(true),
		)
	})

	AfterEach(func(ctx SpecContext) {
		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates a vpc", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

		listVPCs := mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			After(listVPCs).
			Return(&linodego.VPC{
				ID:     1,
				Label:  "vpc1",
				Region: "us-east",
				Subnets: []linodego.VPCSubnet{
					{ID: 123, Label: "subnet1", IPv4: "10.0.0.0/8"},
				},
			}, nil)

		vpcScope := scope.VPCScope{
			Client:       k8sClient,
			LinodeClient: mockLinodeClient,
			LinodeVPC:    &linodeVPC,
		}

		err := reconciler.reconcileCreate(ctx, logger, &vpcScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(*linodeVPC.Spec.VPCID).To(Equal(1))
		Expect(linodeVPC.Spec.Subnets[0].IPv4).To(Equal("10.0.0.0/8"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create VPC"))
	})

	Context("when doing update", func() {
		It("successfully updates a vpc", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			listVPCs := mockLinodeClient.EXPECT().
				ListVPCs(ctx, gomock.Any()).
				Return([]linodego.VPC{}, nil)
			mockLinodeClient.EXPECT().
				CreateVPC(ctx, gomock.Any()).
				After(listVPCs).
				Return(&linodego.VPC{
					ID:     1,
					Label:  "vpc1",
					Region: "us-east",
					Subnets: []linodego.VPCSubnet{
						{ID: 123, Label: "subnet1", IPv4: "10.0.0.0/8"},
					},
				}, nil)

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			err := reconciler.reconcileUpdate(ctx, logger, &vpcScope)
			Expect(err).NotTo(HaveOccurred())

			Expect(*linodeVPC.Spec.VPCID).To(Equal(1))
			Expect(linodeVPC.Spec.Subnets[0].IPv4).To(Equal("10.0.0.0/8"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create VPC"))
		})

		It("fails if it can't list VPCs", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			mockLinodeClient.EXPECT().
				ListVPCs(ctx, gomock.Any()).
				Return([]linodego.VPC{}, errors.New("failed to make call"))

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			err := reconciler.reconcileUpdate(ctx, logger, &vpcScope)
			Expect(err).To(HaveOccurred())
			Expect(testLogs.String()).To(ContainSubstring("Failed to list VPCs"))
		})

		It("fails if it can't create VPC", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			listVPCs := mockLinodeClient.EXPECT().
				ListVPCs(ctx, gomock.Any()).
				Return([]linodego.VPC{}, nil)
			mockLinodeClient.EXPECT().
				CreateVPC(ctx, gomock.Any()).
				After(listVPCs).
				Return(&linodego.VPC{}, errors.New("failed creating VPC"))

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			err := reconciler.reconcileCreate(ctx, logger, &vpcScope)
			Expect(err).To(HaveOccurred())
			Expect(testLogs.String()).To(ContainSubstring("Failed to create VPC"))
		})

		It("fails if empty VPC is returned", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			listVPCs := mockLinodeClient.EXPECT().
				ListVPCs(ctx, gomock.Any()).
				Return([]linodego.VPC{}, nil)
			mockLinodeClient.EXPECT().
				CreateVPC(ctx, gomock.Any()).
				After(listVPCs).
				Return(nil, nil)

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			err := reconciler.reconcileCreate(ctx, logger, &vpcScope)
			Expect(err).To(HaveOccurred())
			Expect(testLogs.String()).To(ContainSubstring("Panic! Failed to create VPC"))
		})
	})

	Context("when deleting a VPC", func() {
		It("succeeds if no error occurs", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			vpcSpec := infrav1alpha1.LinodeVPCSpec{
				VPCID:  ptr.To(1),
				Region: "us-east",
				Subnets: []infrav1alpha1.VPCSubnetCreateOptions{
					{Label: "subnet1", IPv4: "10.0.0.0/8"},
				},
			}
			linodeVPC = infrav1alpha1.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Labels:    make(map[string]string),
				},
				Spec: vpcSpec,
			}

			getVPC := mockLinodeClient.EXPECT().
				GetVPC(ctx, gomock.Any()).
				Return(&linodego.VPC{
					ID:    1,
					Label: "vpc1",
					Subnets: []linodego.VPCSubnet{
						{ID: 123, Linodes: []linodego.VPCSubnetLinode{}},
					},
				}, nil)
			mockLinodeClient.EXPECT().
				DeleteVPC(ctx, gomock.Any()).
				After(getVPC).
				Return(nil)

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			_, err := reconciler.reconcileDelete(ctx, logger, &vpcScope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails if GetVPC API errors out", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			vpcSpec := infrav1alpha1.LinodeVPCSpec{
				VPCID:  ptr.To(1),
				Region: "us-east",
				Subnets: []infrav1alpha1.VPCSubnetCreateOptions{
					{Label: "subnet1", IPv4: "10.0.0.0/8"},
				},
			}
			linodeVPC = infrav1alpha1.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Labels:    make(map[string]string),
				},
				Spec: vpcSpec,
			}

			mockLinodeClient.EXPECT().
				GetVPC(ctx, gomock.Any()).
				Return(&linodego.VPC{}, errors.New("service unavailable"))

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			_, err := reconciler.reconcileDelete(ctx, logger, &vpcScope)
			Expect(err).To(HaveOccurred())
			Expect(testLogs.String()).To(ContainSubstring("Failed to fetch VPC"))
		})

		It("fails if DeleteVPC API errors out", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			vpcSpec := infrav1alpha1.LinodeVPCSpec{
				VPCID:  ptr.To(1),
				Region: "us-east",
				Subnets: []infrav1alpha1.VPCSubnetCreateOptions{
					{Label: "subnet1", IPv4: "10.0.0.0/8"},
				},
			}
			linodeVPC = infrav1alpha1.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Labels:    make(map[string]string),
				},
				Spec: vpcSpec,
			}

			getVPC := mockLinodeClient.EXPECT().
				GetVPC(ctx, gomock.Any()).
				Return(&linodego.VPC{
					ID:    1,
					Label: "vpc1",
					Subnets: []linodego.VPCSubnet{
						{ID: 123, Linodes: []linodego.VPCSubnetLinode{}},
					},
				}, nil)
			mockLinodeClient.EXPECT().
				DeleteVPC(ctx, gomock.Any()).
				After(getVPC).
				Return(errors.New("service unavailable"))

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			_, err := reconciler.reconcileDelete(ctx, logger, &vpcScope)
			Expect(err).To(HaveOccurred())
			Expect(testLogs.String()).To(ContainSubstring("Failed to delete VPC"))
		})

		It("requeues for reconciliation if linodes are still attached to VPC", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeVPCClient(mockCtrl)

			vpcSpec := infrav1alpha1.LinodeVPCSpec{
				VPCID:  ptr.To(1),
				Region: "us-east",
				Subnets: []infrav1alpha1.VPCSubnetCreateOptions{
					{Label: "subnet1", IPv4: "10.0.0.0/8"},
				},
			}
			linodeVPC = infrav1alpha1.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Labels:    make(map[string]string),
				},
				Spec: vpcSpec,
			}

			mockLinodeClient.EXPECT().
				GetVPC(ctx, gomock.Any()).
				Return(&linodego.VPC{
					ID:      1,
					Label:   "vpc1",
					Updated: ptr.To(time.Now()),
					Subnets: []linodego.VPCSubnet{
						{
							ID: 123,
							Linodes: []linodego.VPCSubnetLinode{
								{ID: 2, Interfaces: []linodego.VPCSubnetLinodeInterface{}},
							}},
					},
				}, nil)

			vpcScope := scope.VPCScope{
				Client:       k8sClient,
				LinodeClient: mockLinodeClient,
				LinodeVPC:    &linodeVPC,
			}

			resp, err := reconciler.reconcileDelete(ctx, logger, &vpcScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.RequeueAfter).To(Equal(rec.DefaultVPCControllerWaitForHasNodesDelay))
		})
	})
})
