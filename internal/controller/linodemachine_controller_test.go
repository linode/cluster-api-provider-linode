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
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
	rutil "github.com/linode/cluster-api-provider-linode/util/reconciler"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("create", Label("machine", "create"), func() {
	var machine clusterv1.Machine
	var linodeMachine infrav1alpha2.LinodeMachine
	var secret corev1.Secret
	var reconciler *LinodeMachineReconciler

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-east",
			Network: infrav1alpha2.NetworkSpec{
				NodeBalancerID:                ptr.To(1),
				ApiserverNodeBalancerConfigID: ptr.To(2),
			},
		},
	}

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		machine = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Labels:    make(map[string]string),
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}
		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Region:         "us-east",
				Type:           "g6-nanode-1",
				Image:          rutil.DefaultMachineControllerLinodeImage,
				DiskEncryption: string(linodego.InstanceDiskEncryptionEnabled),
			},
		}
		reconciler = &LinodeMachineReconciler{
			Recorder: recorder,
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
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	Context("create machine with firewall", func() {
		It("firewall is not yet present", func(ctx SpecContext) {
			linodeMachine.Spec.FirewallRef = &corev1.ObjectReference{Name: "fwnone", Namespace: defaultNamespace, Kind: "LinodeFirewall", APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2"}
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}
			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(linodeMachine.GetCondition(ConditionPreflightLinodeFirewallReady).Status).To(Equal(metav1.ConditionFalse))
		})

		It("firewall present but status is not yet ready", func(ctx SpecContext) {
			linodeFirewall := &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fw1",
					Namespace: defaultNamespace,
				},
				Spec: infrav1alpha2.LinodeFirewallSpec{
					Enabled:    false,
					FirewallID: ptr.To(1),
				},
			}
			Expect(k8sClient.Create(ctx, linodeFirewall)).To(Succeed())
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			linodeMachine.Spec.FirewallRef = &corev1.ObjectReference{Name: "fw1"}
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}
			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(linodeMachine.GetCondition(ConditionPreflightLinodeFirewallReady).Status).To(Equal(metav1.ConditionFalse))
		})

		It("firewall present and status is ready", func(ctx SpecContext) {
			linodeFirewall := &infrav1alpha2.LinodeFirewall{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fw2",
					Namespace: defaultNamespace,
				},
				Spec: infrav1alpha2.LinodeFirewallSpec{
					Enabled:    false,
					FirewallID: ptr.To(1),
				},
			}
			Expect(k8sClient.Create(ctx, linodeFirewall)).To(Succeed())
			linodeFirewall.Status.Ready = true
			Expect(k8sClient.Status().Update(ctx, linodeFirewall)).To(Succeed())
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
			getImage := mockLinodeClient.EXPECT().
				GetImage(ctx, gomock.Any()).
				After(getRegion).
				Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
			mockLinodeClient.EXPECT().
				CreateInstance(ctx, gomock.Any()).
				After(getImage).
				Return(&linodego.Instance{
					ID:     123,
					IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
					IPv6:   "fd00::",
					Status: linodego.InstanceOffline,
				}, nil)
			createInst := mockLinodeClient.EXPECT().
				OnAfterResponse(gomock.Any()).
				Return()
			listInstConfs := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createInst).
				Return([]linodego.InstanceConfig{{
					ID: 1,
				}}, nil)
			mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
				Helpers: &linodego.InstanceConfigHelpers{Network: true},
			}).
				After(listInstConfs).
				Return(nil, nil)
			bootInst := mockLinodeClient.EXPECT().
				BootInstance(ctx, 123, 0).
				After(createInst).
				Return(nil)
			mockLinodeClient.EXPECT().
				GetInstanceIPAddresses(ctx, 123).
				After(bootInst).
				Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
				}, nil)
			linodeMachine.Spec.FirewallRef = &corev1.ObjectReference{Name: "fw2", Namespace: defaultNamespace, Kind: "LinodeFirewall", APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2"}
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}
			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(linodeMachine.GetCondition(ConditionPreflightLinodeFirewallReady).Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("create machine with vpc", func() {
		It("vpc is not yet present", func(ctx SpecContext) {
			linodeMachine.Spec.VPCRef = &corev1.ObjectReference{
				Name:       "vpcnone",
				Namespace:  defaultNamespace,
				Kind:       "LinodeVPC",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
			}
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}
			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady).Status).To(Equal(metav1.ConditionFalse))
		})

		It("vpc present but status is not yet ready", func(ctx SpecContext) {
			linodeVPC := &infrav1alpha2.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpc1",
					Namespace: defaultNamespace,
				},
				Spec: infrav1alpha2.LinodeVPCSpec{
					Region: "us-ord",
					VPCID:  ptr.To(1),
				},
				Status: infrav1alpha2.LinodeVPCStatus{
					Ready: false,
				},
			}
			Expect(k8sClient.Create(ctx, linodeVPC)).To(Succeed())

			linodeMachine.Spec.VPCRef = &corev1.ObjectReference{
				Name:       "vpc1",
				Namespace:  defaultNamespace,
				Kind:       "LinodeVPC",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
			}

			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}
			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			result, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(rutil.DefaultClusterControllerReconcileDelay))
			Expect(linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady).Status).To(Equal(metav1.ConditionFalse))

			Expect(k8sClient.Delete(ctx, linodeVPC)).To(Succeed())
		})

		It("vpc is ready and machine creation succeeds", func(ctx SpecContext) {
			linodeVPC := &infrav1alpha2.LinodeVPC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpc2",
					Namespace: defaultNamespace,
				},
				Spec: infrav1alpha2.LinodeVPCSpec{
					VPCID:  ptr.To(1),
					Region: "us-ord",
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							SubnetID: 1,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, linodeVPC)).To(Succeed())
			linodeVPC.Status.Ready = true
			Expect(k8sClient.Status().Update(ctx, linodeVPC)).To(Succeed())

			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
			getImage := mockLinodeClient.EXPECT().
				GetImage(ctx, gomock.Any()).
				After(getRegion).
				Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
			mockLinodeClient.EXPECT().
				CreateInstance(ctx, gomock.Any()).
				After(getImage).
				Return(&linodego.Instance{
					ID:     123,
					IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
					IPv6:   "fd00::",
					Status: linodego.InstanceOffline,
				}, nil)
			createInst := mockLinodeClient.EXPECT().
				OnAfterResponse(gomock.Any()).
				Return()
			listInstConfs := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createInst).
				Return([]linodego.InstanceConfig{{
					ID: 1,
				}}, nil)
			mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
				Helpers: &linodego.InstanceConfigHelpers{Network: true},
			}).
				After(listInstConfs).
				Return(nil, nil)
			bootInst := mockLinodeClient.EXPECT().
				BootInstance(ctx, 123, 0).
				After(createInst).
				Return(nil)
			mockLinodeClient.EXPECT().
				GetInstanceIPAddresses(ctx, 123).
				After(bootInst).
				Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
				}, nil)

			linodeMachine.Spec.VPCRef = &corev1.ObjectReference{
				Name:       "vpc2",
				Namespace:  defaultNamespace,
				Kind:       "LinodeVPC",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
			}

			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}
			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady).Status).To(Equal(metav1.ConditionTrue))

			Expect(k8sClient.Delete(ctx, linodeVPC)).To(Succeed())
		})
	})

	It("creates a worker instance", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
		getImage := mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
		createInst := mockLinodeClient.EXPECT().
			CreateInstance(ctx, gomock.Any()).
			After(getImage).
			Return(&linodego.Instance{
				ID:     123,
				IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
				IPv6:   "fd00::",
				Status: linodego.InstanceOffline,
			}, nil)
		mockLinodeClient.EXPECT().
			OnAfterResponse(gomock.Any()).
			Return()
		listInstConfs := mockLinodeClient.EXPECT().
			ListInstanceConfigs(ctx, 123, gomock.Any()).
			After(createInst).
			Return([]linodego.InstanceConfig{{
				ID: 1,
			}}, nil)
		mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
			Helpers: &linodego.InstanceConfigHelpers{Network: true},
		}).
			After(listInstConfs).
			Return(nil, nil)
		bootInst := mockLinodeClient.EXPECT().
			BootInstance(ctx, 123, 0).
			After(createInst).
			Return(nil)
		mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(ctx, 123).
			After(bootInst).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
					Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
				},
				IPv6: &linodego.InstanceIPv6Response{
					SLAAC: &linodego.InstanceIP{
						Address: "fd00::",
					},
				},
			}, nil)

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))

		Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
		Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
		Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{
			{Type: clusterv1.MachineExternalIP, Address: "172.0.0.2"},
			{Type: clusterv1.MachineExternalIP, Address: "fd00::"},
			{Type: clusterv1.MachineInternalIP, Address: "192.168.0.2"},
		}))

		Expect(testLogs.String()).To(ContainSubstring("creating machine"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to list Linode machine instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Linode instance already exists"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine InstanceCreateOptions"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to boot instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("multiple instances found"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to add instance to Node Balancer backend"))
	})

	It("adopts a worker instance which already exists", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
		getImage := mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
		createInst := mockLinodeClient.EXPECT().
			CreateInstance(ctx, gomock.Any()).
			After(getImage).
			Return(nil, &linodego.Error{Code: http.StatusBadRequest, Message: "[400] [label] Label must be unique among your linodes"})
		listInst := mockLinodeClient.EXPECT().
			ListInstances(ctx, gomock.Any()).
			After(createInst).
			Return([]linodego.Instance{{
				ID:     123,
				IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
				IPv6:   "fd00::",
				Status: linodego.InstanceOffline,
			}}, nil)
		mockLinodeClient.EXPECT().
			OnAfterResponse(gomock.Any()).
			Return()
		listInstConfs := mockLinodeClient.EXPECT().
			ListInstanceConfigs(ctx, 123, gomock.Any()).
			After(createInst).
			Return([]linodego.InstanceConfig{{
				ID: 1,
			}}, nil)
		mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
			Helpers: &linodego.InstanceConfigHelpers{Network: true},
		}).
			After(listInstConfs).
			Return(nil, nil)
		bootInst := mockLinodeClient.EXPECT().
			BootInstance(ctx, 123, 0).
			After(listInst).
			Return(nil)
		mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(ctx, 123).
			After(bootInst).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
					Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
				},
				IPv6: &linodego.InstanceIPv6Response{
					SLAAC: &linodego.InstanceIP{
						Address: "fd00::",
					},
				},
			}, nil)

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))

		Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
		Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
		Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{
			{Type: clusterv1.MachineExternalIP, Address: "172.0.0.2"},
			{Type: clusterv1.MachineExternalIP, Address: "fd00::"},
			{Type: clusterv1.MachineInternalIP, Address: "192.168.0.2"},
		}))

		Expect(testLogs.String()).To(ContainSubstring("creating machine"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to list Linode machine instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Linode instance already exists"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine InstanceCreateOptions"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to boot instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("multiple instances found"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to add instance to Node Balancer backend"))
	})

	Context("fails when a preflight condition is stale", func() {
		It("can't create an instance in time", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
			getImage := mockLinodeClient.EXPECT().
				GetImage(ctx, gomock.Any()).
				After(getRegion).
				Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
			mockLinodeClient.EXPECT().
				CreateInstance(ctx, gomock.Any()).
				After(getImage).
				DoAndReturn(func(_, _ any) (*linodego.Instance, error) {
					time.Sleep(time.Microsecond)
					return nil, errors.New("time is up")
				})
			mockLinodeClient.EXPECT().
				OnAfterResponse(gomock.Any()).
				Return()

			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			reconciler.ReconcileTimeout = time.Nanosecond

			res, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerRetryDelay))
			Expect(err).NotTo(HaveOccurred())

			Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionFalse))
			condition := linodeMachine.GetCondition(ConditionPreflightCreated)
			Expect(condition).ToNot(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal(util.CreateError))
			Expect(condition.Message).To(ContainSubstring("time is up"))
		})
	})

	Context("when a known error occurs", func() {
		It("requeues due to context deadline exceeded error", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
			getImage := mockLinodeClient.EXPECT().
				GetImage(ctx, gomock.Any()).
				After(getRegion).
				Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
			mockLinodeClient.EXPECT().
				CreateInstance(ctx, gomock.Any()).
				After(getImage).
				DoAndReturn(func(_, _ any) (*linodego.Instance, error) {
					return nil, linodego.NewError(errors.New("context deadline exceeded"))
				})
			mockLinodeClient.EXPECT().
				OnAfterResponse(gomock.Any()).
				Return()
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			res, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerRetryDelay))
		})
	})

	Context("when an unknown error occurs", func() {
		It("does not requeue due to a 400 error", func(ctx SpecContext) {
			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
			getImage := mockLinodeClient.EXPECT().
				GetImage(ctx, gomock.Any()).
				After(getRegion).
				Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
			mockLinodeClient.EXPECT().
				CreateInstance(ctx, gomock.Any()).
				After(getImage).
				DoAndReturn(func(_, _ any) (*linodego.Instance, error) {
					return nil, linodego.NewError(linodego.Error{Code: 400, Message: "bad configuration"})
				})
			mockLinodeClient.EXPECT().
				OnAfterResponse(gomock.Any()).
				Return()
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			res, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerRetryDelay))
		})
	})

	Context("creates a instance with disks", func() {
		It("in a single call when disks aren't delayed", func(ctx SpecContext) {
			machine.Labels[clusterv1.MachineControlPlaneLabel] = "true"
			extraDisk := resource.MustParse("128Mi")
			linodeMachine.Spec.DataDisks = &infrav1alpha2.InstanceDisks{
				SDB: ptr.To(infrav1alpha2.InstanceDisk{Label: "etcd-data", Size: resource.MustParse("10Gi")}),
				SDC: ptr.To(infrav1alpha2.InstanceDisk{Label: "disk2", Size: extraDisk}),
				SDD: ptr.To(infrav1alpha2.InstanceDisk{Label: "disk3", Size: extraDisk}),
				SDE: ptr.To(infrav1alpha2.InstanceDisk{Label: "disk4", Size: extraDisk}),
				SDF: ptr.To(infrav1alpha2.InstanceDisk{Label: "disk5", Size: extraDisk}),
				SDG: ptr.To(infrav1alpha2.InstanceDisk{Label: "disk6", Size: extraDisk}),
				SDH: ptr.To(infrav1alpha2.InstanceDisk{Label: "disk7", Size: extraDisk}),
			}
			extraDiskSize := int(extraDisk.ScaledValue(resource.Mega))

			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
			getImage := mockLinodeClient.EXPECT().
				GetImage(ctx, gomock.Any()).
				After(getRegion).
				Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
			mockLinodeClient.EXPECT().
				CreateInstance(ctx, gomock.Any()).
				After(getImage).
				Return(&linodego.Instance{
					ID:     123,
					IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
					IPv6:   "fd00::",
					Status: linodego.InstanceOffline,
				}, nil)
			mockLinodeClient.EXPECT().
				OnAfterResponse(gomock.Any()).
				Return()
			mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				Return([]linodego.InstanceConfig{{
					ID: 1,
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).MaxTimes(3)
			mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
				Helpers: &linodego.InstanceConfigHelpers{Network: true},
			}).Return(nil, nil)
			getInstDisk := mockLinodeClient.EXPECT().
				GetInstanceDisk(ctx, 123, 100).
				Return(&linodego.InstanceDisk{ID: 100, Size: 15000}, nil)
			resizeInstDisk := mockLinodeClient.EXPECT().
				ResizeInstanceDisk(ctx, 123, 100, 3452).
				After(getInstDisk).
				Return(nil)
			createEtcdDisk := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "etcd-data",
					Size:       10738,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(resizeInstDisk).
				Return(&linodego.InstanceDisk{ID: 101}, nil)
			createAdditionalDisk2 := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "disk2",
					Size:       extraDiskSize,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(createEtcdDisk).
				Return(&linodego.InstanceDisk{ID: 102}, nil)
			createAdditionalDisk3 := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "disk3",
					Size:       extraDiskSize,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(createAdditionalDisk2).
				Return(&linodego.InstanceDisk{ID: 103}, nil)
			createAdditionalDisk4 := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "disk4",
					Size:       extraDiskSize,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(createAdditionalDisk3).
				Return(&linodego.InstanceDisk{ID: 104}, nil)
			createAdditionalDisk5 := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "disk5",
					Size:       extraDiskSize,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(createAdditionalDisk4).
				Return(&linodego.InstanceDisk{ID: 105}, nil)
			createAdditionalDisk6 := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "disk6",
					Size:       extraDiskSize,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(createAdditionalDisk5).
				Return(&linodego.InstanceDisk{ID: 106}, nil)
			createAdditionalDisk7 := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "disk7",
					Size:       extraDiskSize,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(createAdditionalDisk6).
				Return(&linodego.InstanceDisk{ID: 107}, nil)
			listInstConfsForProfile := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createAdditionalDisk7).
				Return([]linodego.InstanceConfig{{
					ID: 1,
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).MaxTimes(3)
			createInstanceProfile := mockLinodeClient.EXPECT().
				UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
						SDB: &linodego.InstanceConfigDevice{DiskID: 101},
						SDC: &linodego.InstanceConfigDevice{DiskID: 102},
						SDD: &linodego.InstanceConfigDevice{DiskID: 103},
						SDE: &linodego.InstanceConfigDevice{DiskID: 104},
						SDF: &linodego.InstanceConfigDevice{DiskID: 105},
						SDG: &linodego.InstanceConfigDevice{DiskID: 106},
						SDH: &linodego.InstanceConfigDevice{DiskID: 107},
					}}).
				After(listInstConfsForProfile)
			bootInst := mockLinodeClient.EXPECT().
				BootInstance(ctx, 123, 0).
				After(createInstanceProfile).
				Return(nil)
			getAddrs := mockLinodeClient.EXPECT().
				GetInstanceIPAddresses(ctx, 123).
				After(bootInst).
				Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
				}, nil)
			createNB := mockLinodeClient.EXPECT().
				CreateNodeBalancerNode(ctx, 1, 2, linodego.NodeBalancerNodeCreateOptions{
					Label:   "mock",
					Address: "192.168.0.2:6443",
					Mode:    linodego.ModeAccept,
				}).
				After(getAddrs).MaxTimes(2).
				Return(nil, nil)
			mockLinodeClient.EXPECT().
				GetInstanceIPAddresses(ctx, 123).
				After(createNB).
				Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
				}, nil).MaxTimes(2)

			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper
			Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
			Expect(k8sClient.Create(ctx, &linodeMachine)).To(Succeed())

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())

			Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))

			Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
			Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{
				{Type: clusterv1.MachineExternalIP, Address: "172.0.0.2"},
				{Type: clusterv1.MachineExternalIP, Address: "fd00::"},
				{Type: clusterv1.MachineInternalIP, Address: "192.168.0.2"},
			}))

			Expect(testLogs.String()).To(ContainSubstring("creating machine"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Failed to list Linode machine instance"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Linode instance already exists"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine InstanceCreateOptions"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine instance"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Failed to configure instance profile"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Waiting for control plane disks to be ready"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Failed to boot instance"))
			Expect(testLogs.String()).NotTo(ContainSubstring("multiple instances found"))
			Expect(testLogs.String()).NotTo(ContainSubstring("Failed to add instance to Node Balancer backend"))
		})

		It("in multiple calls when disks are delayed", func(ctx SpecContext) {
			machine.Labels[clusterv1.MachineControlPlaneLabel] = "true"
			linodeMachine.Spec.DataDisks = &infrav1alpha2.InstanceDisks{SDB: ptr.To(infrav1alpha2.InstanceDisk{Label: "etcd-data", Size: resource.MustParse("10Gi")})}

			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
			getImage := mockLinodeClient.EXPECT().
				GetImage(ctx, gomock.Any()).
				After(getRegion).
				Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
			createInst := mockLinodeClient.EXPECT().
				CreateInstance(ctx, gomock.Any()).
				After(getImage).
				Return(&linodego.Instance{
					ID:     123,
					IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
					IPv6:   "fd00::",
					Status: linodego.InstanceOffline,
				}, nil)
			mockLinodeClient.EXPECT().
				OnAfterResponse(gomock.Any()).
				Return()
			listInstConfs := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createInst).
				Return([]linodego.InstanceConfig{{
					ID: 1,
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil)
			mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
				Helpers: &linodego.InstanceConfigHelpers{Network: true},
			}).
				After(listInstConfs).
				Return(nil, nil)
			getInstDisk := mockLinodeClient.EXPECT().
				GetInstanceDisk(ctx, 123, 100).
				After(listInstConfs).
				Return(&linodego.InstanceDisk{ID: 100, Size: 15000}, nil)
			resizeInstDisk := mockLinodeClient.EXPECT().
				ResizeInstanceDisk(ctx, 123, 100, 4262).
				After(getInstDisk).
				Return(nil)
			createFailedEtcdDisk := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "etcd-data",
					Size:       10738,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(resizeInstDisk).
				Return(nil, &linodego.Error{Code: 500})

			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

			patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			mScope.PatchHelper = patchHelper

			res, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerWaitForRunningDelay))
			Expect(err).ToNot(HaveOccurred())

			Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionFalse))
			Expect(linodeMachine.GetCondition(ConditionPreflightAdditionalDisksCreated).Status).To(Equal(metav1.ConditionFalse))

			mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				Return([]linodego.InstanceConfig{{
					ID: 1,
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).AnyTimes()
			mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
				Helpers: &linodego.InstanceConfigHelpers{Network: true},
			}).Return(nil, nil).AnyTimes()
			getInst := mockLinodeClient.EXPECT().
				GetInstance(ctx, 123).
				After(createFailedEtcdDisk).
				Return(&linodego.Instance{
					ID:     123,
					IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
					IPv6:   "fd00::",
					Status: linodego.InstanceOffline,
				}, nil).MaxTimes(2)
			createEtcdDisk := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "etcd-data",
					Size:       10738,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(getInst).
				Return(&linodego.InstanceDisk{ID: 101}, nil)
			listInstConfsForProfile := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createEtcdDisk).
				Return([]linodego.InstanceConfig{{
					ID: 1,
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).AnyTimes()
			createInstanceProfile := mockLinodeClient.EXPECT().
				UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
						SDB: &linodego.InstanceConfigDevice{DiskID: 101},
					}}).
				After(listInstConfsForProfile)
			bootInst := mockLinodeClient.EXPECT().
				BootInstance(ctx, 123, 0).
				After(createInstanceProfile).
				Return(nil)
			getAddrs := mockLinodeClient.EXPECT().
				GetInstanceIPAddresses(ctx, 123).
				After(bootInst).
				Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
						VPC:     []*linodego.VPCIP{{Address: ptr.To("10.0.0.2")}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
				}, nil).MaxTimes(2)
			createNB := mockLinodeClient.EXPECT().
				CreateNodeBalancerNode(ctx, 1, 2, linodego.NodeBalancerNodeCreateOptions{
					Label:   "mock",
					Address: "192.168.0.2:6443",
					Mode:    linodego.ModeAccept,
				}).
				After(getAddrs).
				Return(nil, nil).MaxTimes(2)
			mockLinodeClient.EXPECT().
				GetInstanceIPAddresses(ctx, 123).
				After(createNB).
				Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
						VPC:     []*linodego.VPCIP{{Address: ptr.To("10.0.0.2")}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{
							Address: "fd00::",
						},
					},
				}, nil).MaxTimes(2)

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())

			Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
			Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))

			Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
			Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
			Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{
				{Type: clusterv1.MachineExternalIP, Address: "172.0.0.2"},
				{Type: clusterv1.MachineExternalIP, Address: "fd00::"},
				{Type: clusterv1.MachineInternalIP, Address: "10.0.0.2"},
				{Type: clusterv1.MachineInternalIP, Address: "192.168.0.2"},
			}))
		})
	})
})

var _ = Describe("createDNS", Label("machine", "createDNS"), func() {
	var machine clusterv1.Machine
	var linodeMachine infrav1alpha2.LinodeMachine
	var secret corev1.Secret
	var reconciler *LinodeMachineReconciler

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		Spec: infrav1alpha2.LinodeClusterSpec{
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType:    "dns",
				DNSRootDomain:       "lkedevs.net",
				DNSUniqueIdentifier: "abc123",
				DNSTTLSec:           30,
			},
		},
	}

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		machine = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Labels:    make(map[string]string),
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}
		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Type:  "g6-nanode-1",
				Image: rutil.DefaultMachineControllerLinodeImage,
			},
		}
		reconciler = &LinodeMachineReconciler{
			Recorder: recorder,
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
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates a worker instance", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
		getImage := mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
		createInst := mockLinodeClient.EXPECT().
			CreateInstance(ctx, gomock.Any()).
			After(getImage).
			Return(&linodego.Instance{
				ID:     123,
				IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
				IPv6:   "fd00::",
				Status: linodego.InstanceOffline,
			}, nil)
		mockLinodeClient.EXPECT().
			OnAfterResponse(gomock.Any()).
			Return()
		listInstConfs := mockLinodeClient.EXPECT().
			ListInstanceConfigs(ctx, 123, gomock.Any()).
			After(createInst).
			Return([]linodego.InstanceConfig{{
				ID: 1,
			}}, nil)
		mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
			Helpers: &linodego.InstanceConfigHelpers{Network: true},
		}).
			After(listInstConfs).
			Return(nil, nil)
		bootInst := mockLinodeClient.EXPECT().
			BootInstance(ctx, 123, 0).
			After(createInst).
			Return(nil)
		mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(ctx, 123).
			After(bootInst).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
					Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
				},
				IPv6: &linodego.InstanceIPv6Response{
					SLAAC: &linodego.InstanceIP{
						Address: "fd00::",
					},
				},
			}, nil)

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))

		Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
		Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
		Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{
			{Type: clusterv1.MachineExternalIP, Address: "172.0.0.2"},
			{Type: clusterv1.MachineExternalIP, Address: "fd00::"},
			{Type: clusterv1.MachineInternalIP, Address: "192.168.0.2"},
		}))

		Expect(testLogs.String()).To(ContainSubstring("creating machine"))
	})

})

var _ = Describe("machine-lifecycle", Ordered, Label("machine", "machine-lifecycle"), func() {
	machineName := "machine-lifecycle"
	namespace := defaultNamespace
	ownerRef := metav1.OwnerReference{
		Name:       machineName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Machine",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            machineName,
		Namespace:       namespace,
		OwnerReferences: ownerRefs,
	}
	missingFW := &infrav1alpha2.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-missing-fw",
			Namespace: namespace,
		},
		Spec: infrav1alpha2.LinodeFirewallSpec{
			FirewallID: nil,
			Enabled:    true,
		},
	}
	linodeMachine := &infrav1alpha2.LinodeMachine{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeMachineSpec{
			Type:   "g6-nanode-1",
			Image:  rutil.DefaultMachineControllerLinodeImage,
			Region: "us-east",
		},
	}
	machineKey := client.ObjectKeyFromObject(linodeMachine)
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Labels:    make(map[string]string),
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: ptr.To("test-bootstrap-secret"),
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bootstrap-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value": []byte("userdata"),
		},
	}

	linodeCluster := &infrav1alpha2.LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-cluster",
			Labels:    make(map[string]string),
		},
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-east",
			Network: infrav1alpha2.NetworkSpec{
				NodeBalancerID:                ptr.To(1),
				ApiserverNodeBalancerConfigID: ptr.To(2),
			},
		},
	}
	clusterKey := client.ObjectKeyFromObject(linodeCluster)

	ctlrSuite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})
	reconciler := LinodeMachineReconciler{}
	mScope := &scope.MachineScope{}

	BeforeAll(func(ctx SpecContext) {
		mScope.Client = k8sClient
		reconciler.Client = k8sClient
		mScope.Cluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespace,
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Name:      "test-cluster",
					Namespace: namespace,
				},
			},
		}
		mScope.Machine = machine
		_ = k8sClient.Create(ctx, missingFW)
		Expect(k8sClient.Create(ctx, linodeCluster)).To(Succeed())
		Expect(k8sClient.Create(ctx, linodeMachine)).To(Succeed())
		_ = k8sClient.Create(ctx, secret)
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()

		Expect(k8sClient.Get(ctx, machineKey, linodeMachine)).To(Succeed())
		mScope.LinodeMachine = linodeMachine

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper
		Expect(k8sClient.Get(ctx, clusterKey, linodeCluster)).To(Succeed())
		mScope.LinodeCluster = linodeCluster

		mScope.LinodeClient = mck.LinodeClient
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("machine is not created because of too many requests", func(ctx context.Context, mck Mock) {
				}),
				Path(Result("create requeues when failing to create instance config", func(ctx context.Context, mck Mock) {
					getRegion := mck.LinodeClient.EXPECT().
						GetRegion(ctx, gomock.Any()).
						Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
					mck.LinodeClient.EXPECT().
						GetImage(ctx, gomock.Any()).
						After(getRegion).
						Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
					res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(res.RequeueAfter).To(Equal(rutil.DefaultLinodeTooManyRequestsErrorRetryDelay))
					Expect(mck.Logs()).To(ContainSubstring("Failed to fetch image"))
				})),
				Call("machine is not created because there was an error creating instance", func(ctx context.Context, mck Mock) {
				}),
				OneOf(
					Path(Result("create error", func(ctx context.Context, mck Mock) {
						linodeMachine.Spec.ProviderID = util.Pointer("linode://foo")
						_, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).To(HaveOccurred())
						Expect(mck.Logs()).To(ContainSubstring("Failed to parse instance ID from provider ID"))
					})),
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						linodeMachine.Spec.ProviderID = nil
						getRegion := mck.LinodeClient.EXPECT().
							GetRegion(ctx, gomock.Any()).
							Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
						getImage := mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
							After(getRegion).
							Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
						mck.LinodeClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).
							After(getImage).
							Return(nil, &linodego.Error{Code: http.StatusBadGateway})
						mck.LinodeClient.EXPECT().
							OnAfterResponse(gomock.Any()).
							Return()
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to create Linode machine instance"))
					})),
				),
			),
			Path(
				Call("machine is not created because it fails to get referenced firewall", func(ctx context.Context, mck Mock) {
					linodeMachine.Spec.FirewallRef = &corev1.ObjectReference{
						Name:      "test-missing-fw",
						Namespace: namespace,
					}
					mScope.LinodeMachine.SetCondition(metav1.Condition{
						Type:   ConditionPreflightMetadataSupportConfigured,
						Status: metav1.ConditionTrue,
						Reason: "LinodeMetadataSupportConfigured", // We have to set the reason to not fail object patching
					})
				}),
				OneOf(
					Path(Result("firewall ready condition is not set", func(ctx context.Context, mck Mock) {
						_, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(linodeMachine.GetCondition(ConditionPreflightLinodeFirewallReady).Status).To(Equal(metav1.ConditionFalse))
					})),
				),
			),
			Path(
				Call("machine is not created because there were too many requests", func(ctx context.Context, mck Mock) {
					linodeMachine.Spec.FirewallRef = nil
				}),
				OneOf(
					Path(Result("create requeues when failing to create instance", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).
							Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
						mck.LinodeClient.EXPECT().
							OnAfterResponse(gomock.Any()).
							Return()
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultLinodeTooManyRequestsErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to create Linode machine instance"))
					})),
					Path(Result("create requeues when failing to update instance config", func(ctx context.Context, mck Mock) {
						linodeMachine.Spec.Configuration = &infrav1alpha2.InstanceConfiguration{Kernel: "test"}
						createInst := mck.LinodeClient.EXPECT().
							CreateInstance(ctx, gomock.Any()).
							Return(&linodego.Instance{
								ID:     123,
								IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
								IPv6:   "fd00::",
								Tags:   []string{"test-cluster-2"},
								Status: linodego.InstanceOffline,
							}, nil)
						mck.LinodeClient.EXPECT().
							OnAfterResponse(gomock.Any()).
							Return()
						listInstConfigs := mck.LinodeClient.EXPECT().
							ListInstanceConfigs(ctx, 123, gomock.Any()).
							After(createInst).
							Return([]linodego.InstanceConfig{{
								Devices: &linodego.InstanceConfigDeviceMap{
									SDA: &linodego.InstanceConfigDevice{DiskID: 100},
								},
							}}, nil)
						mck.LinodeClient.EXPECT().
							UpdateInstanceConfig(ctx, 123, 0, gomock.Any()).
							After(listInstConfigs).
							Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultLinodeTooManyRequestsErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to update default instance configuration"))
						linodeMachine.Spec.Configuration = nil
					})),
					Path(Result("create requeues when failing to get instance config", func(ctx context.Context, mck Mock) {
						getAddrs := mck.LinodeClient.EXPECT().
							GetInstanceIPAddresses(ctx, 123).
							Return(&linodego.InstanceIPAddressResponse{
								IPv4: &linodego.InstanceIPv4Response{
									Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
									Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
								},
								IPv6: &linodego.InstanceIPv6Response{
									SLAAC: &linodego.InstanceIP{
										Address: "fd00::",
									},
								},
							}, nil).MaxTimes(2)
						mck.LinodeClient.EXPECT().
							ListInstanceConfigs(ctx, 123, gomock.Any()).
							After(getAddrs).
							Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultLinodeTooManyRequestsErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to get default instance configuration"))
					})),
				),
				Call("machine is created", func(ctx context.Context, mck Mock) {
				}),
				OneOf(
					Path(Result("creates a worker machine without disks", func(ctx context.Context, mck Mock) {
						linodeMachine = &infrav1alpha2.LinodeMachine{
							ObjectMeta: metadata,
							Spec: infrav1alpha2.LinodeMachineSpec{
								Type:          "g6-nanode-1",
								Image:         rutil.DefaultMachineControllerLinodeImage,
								Configuration: nil,
							},
							Status: infrav1alpha2.LinodeMachineStatus{},
						}
						getRegion := mck.LinodeClient.EXPECT().
							GetRegion(ctx, gomock.Any()).
							Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
						getImage := mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
							After(getRegion).
							Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
						createInst := mck.LinodeClient.EXPECT().
							CreateInstance(ctx, gomock.Any()).
							After(getImage).
							Return(&linodego.Instance{
								ID:     123,
								IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
								IPv6:   "fd00::",
								Status: linodego.InstanceOffline,
							}, nil)
						mck.LinodeClient.EXPECT().
							OnAfterResponse(gomock.Any()).
							Return()
						bootInst := mck.LinodeClient.EXPECT().
							BootInstance(ctx, 123, 0).
							After(createInst).
							Return(nil)
						getAddrs := mck.LinodeClient.EXPECT().
							GetInstanceIPAddresses(ctx, 123).
							After(bootInst).
							Return(&linodego.InstanceIPAddressResponse{
								IPv4: &linodego.InstanceIPv4Response{
									Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
									Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
								},
								IPv6: &linodego.InstanceIPv6Response{
									SLAAC: &linodego.InstanceIP{
										Address: "fd00::",
									},
								},
							}, nil)
						mck.LinodeClient.EXPECT().
							ListInstanceConfigs(ctx, 123, gomock.Any()).
							After(getAddrs).
							Return([]linodego.InstanceConfig{{
								Devices: &linodego.InstanceConfigDeviceMap{
									SDA: &linodego.InstanceConfigDevice{DiskID: 100},
								},
							}}, nil)
						_, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())

						Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
						Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
						Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
						Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))

						Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
						Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
						Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{
							{Type: clusterv1.MachineExternalIP, Address: "172.0.0.2"},
							{Type: clusterv1.MachineExternalIP, Address: "fd00::"},
							{Type: clusterv1.MachineInternalIP, Address: "192.168.0.2"},
						}))
					})),
				),
			),
		),
	)
})

var _ = Describe("machine-update", Ordered, Label("machine", "machine-update"), func() {
	machineName := "machine-update"
	namespace := defaultNamespace
	ownerRef := metav1.OwnerReference{
		Name:       machineName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Machine",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            machineName,
		Namespace:       namespace,
		OwnerReferences: ownerRefs,
	}
	linodeMachine := &infrav1alpha2.LinodeMachine{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeMachineSpec{
			Region:     "us-ord",
			Type:       "g6-nanode-1",
			Image:      rutil.DefaultMachineControllerLinodeImage,
			ProviderID: util.Pointer("linode://11111"),
		},
	}
	machineKey := client.ObjectKeyFromObject(linodeMachine)
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Labels:    make(map[string]string),
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: ptr.To("test-bootstrap-secret-2"),
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bootstrap-secret-2",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value": []byte("userdata"),
		},
	}

	linodeCluster := &infrav1alpha2.LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-cluster-2",
			Labels:    make(map[string]string),
		},
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-east",
			Network: infrav1alpha2.NetworkSpec{
				NodeBalancerID:                ptr.To(1),
				ApiserverNodeBalancerConfigID: ptr.To(2),
			},
		},
	}
	clusterKey := client.ObjectKeyFromObject(linodeCluster)

	ctlrSuite := NewControllerSuite(
		GinkgoT(),
		mock.MockLinodeClient{},
		mock.MockK8sClient{},
	)
	reconciler := LinodeMachineReconciler{}
	mScope := &scope.MachineScope{}

	BeforeAll(func(ctx SpecContext) {
		mScope.Client = k8sClient
		reconciler.Client = k8sClient
		mScope.Cluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-2",
				Namespace: namespace,
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Name:      "test-cluster-2",
					Namespace: namespace,
				},
			},
		}
		mScope.Machine = machine
		Expect(k8sClient.Create(ctx, linodeCluster)).To(Succeed())
		Expect(k8sClient.Create(ctx, linodeMachine)).To(Succeed())
		_ = k8sClient.Create(ctx, secret)
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()

		Expect(k8sClient.Get(ctx, machineKey, linodeMachine)).To(Succeed())
		mScope.LinodeMachine = linodeMachine

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper
		Expect(k8sClient.Get(ctx, clusterKey, linodeCluster)).To(Succeed())
		mScope.LinodeCluster = linodeCluster

		mScope.LinodeClient = mck.LinodeClient
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("machine status is not updated because there was an error updating instance", func(ctx context.Context, mck Mock) {
				}),
				OneOf(
					Path(Result("update error", func(ctx context.Context, mck Mock) {
						linodeMachine.Spec.ProviderID = util.Pointer("linode://foo")
						_, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).To(HaveOccurred())
						Expect(mck.Logs()).To(ContainSubstring("Failed to parse instance ID from provider ID"))
					})),
					Path(Result("update requeues on get error", func(ctx context.Context, mck Mock) {
						linodeMachine.Spec.ProviderID = util.Pointer("linode://11111")
						linodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)
						mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).
							Return(nil, &linodego.Error{Code: http.StatusInternalServerError})
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerRetryDelay))
					})),
				),
			),
			Path(
				Call("machine status updated", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
						&linodego.Instance{
							ID:      11111,
							IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
							IPv6:    "fd00::",
							Tags:    []string{"test-cluster-2"},
							Status:  linodego.InstanceProvisioning,
							Updated: util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().ListInstanceFirewalls(ctx, 11111, nil).Return(
						[]linodego.Firewall{}, nil)
				}),
				Result("machine status updated", func(ctx context.Context, mck Mock) {
					linodeMachine.Spec.ProviderID = util.Pointer("linode://11111")
					linodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceOffline)
					res, err := reconciler.reconcile(ctx, logr.Logger{}, mScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceProvisioning))
					Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerWaitForRunningDelay))

					mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
						&linodego.Instance{
							ID:      11111,
							IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
							IPv6:    "fd00::",
							Tags:    []string{"test-cluster-2"},
							Status:  linodego.InstanceRunning,
							Updated: util.Pointer(time.Now()),
						}, nil)
					res, err = reconciler.reconcile(ctx, logr.Logger{}, mScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceRunning))
					Expect(linodeMachine.GetCondition(string(clusterv1.ReadyCondition)).Status).To(Equal(metav1.ConditionTrue))
				})),
		),
		Path(
			Call("machine tag is updated", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().UpdateInstance(ctx, 11111, gomock.Any()).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2", "test-tag"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().ListInstanceFirewalls(ctx, 11111, nil).Return(
					[]linodego.Firewall{}, nil)
			}),
			Result("machine tag is updated", func(ctx context.Context, mck Mock) {
				linodeMachine.Spec.ProviderID = util.Pointer("linode://11111")
				linodeMachine.Status.InstanceState = util.Pointer(linodego.InstanceRunning)
				linodeMachine.Spec.Tags = []string{"test-tag"}
				_, err := reconciler.reconcile(ctx, logr.Logger{}, mScope)
				Expect(err).NotTo(HaveOccurred())
				Expect(linodeMachine.Status.Tags).To(Equal([]string{"test-tag"}))
			}),
		),
		Path(
			Call("machine firewall is updated", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().UpdateInstance(ctx, 11111, gomock.Any()).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2", "test-tag"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().ListInstanceFirewalls(ctx, 11111, nil).Return(
					[]linodego.Firewall{
						{ID: 5}, // Instance currently has firewall ID 5
					}, nil)
				mck.LinodeClient.EXPECT().UpdateInstanceFirewalls(ctx, 11111, linodego.InstanceFirewallUpdateOptions{
					FirewallIDs: []int{10}, // Update to firewall ID 10
				}).Return(nil, nil)
			}),
			Result("machine firewall is updated", func(ctx context.Context, mck Mock) {
				linodeMachine.Spec.FirewallID = 10 // Set new firewall ID
				_, err := reconciler.reconcile(ctx, logr.Logger{}, mScope)
				Expect(err).NotTo(HaveOccurred())
			}),
		),
		Path(
			Call("machine firewall update applied when multiple firewall already attached", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().UpdateInstance(ctx, 11111, gomock.Any()).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2", "test-tag"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().ListInstanceFirewalls(ctx, 11111, nil).Return(
					[]linodego.Firewall{
						{ID: 10}, // Instance already has the desired firewall ID 10
						{ID: 15}, // Additional firewall
					}, nil)
				mck.LinodeClient.EXPECT().UpdateInstanceFirewalls(ctx, 11111, linodego.InstanceFirewallUpdateOptions{
					FirewallIDs: []int{10}, // Update to firewall ID 10
				}).Return(nil, nil)
			}),
			Result("machine firewall update skipped when firewall already attached", func(ctx context.Context, mck Mock) {
				linodeMachine.Spec.FirewallID = 10 // Firewall ID already attached
				_, err := reconciler.reconcile(ctx, logr.Logger{}, mScope)
				Expect(err).NotTo(HaveOccurred())
			}),
		),
		Path(
			Call("machine firewall update called even when FirewallID is zero", func(ctx context.Context, mck Mock) {
				mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().UpdateInstance(ctx, 11111, gomock.Any()).Return(
					&linodego.Instance{
						ID:      11111,
						IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
						IPv6:    "fd00::",
						Tags:    []string{"test-cluster-2", "test-tag"},
						Status:  linodego.InstanceRunning,
						Updated: util.Pointer(time.Now()),
					}, nil)
				mck.LinodeClient.EXPECT().ListInstanceFirewalls(ctx, 11111, nil).Return(
					[]linodego.Firewall{
						{ID: 5}, // Instance has existing firewall
					}, nil)
				// UpdateInstanceFirewalls WILL be called since 0 is not in the attachedFirewalls list
				mck.LinodeClient.EXPECT().UpdateInstanceFirewalls(ctx, 11111, linodego.InstanceFirewallUpdateOptions{
					FirewallIDs: []int{},
				}).Return(nil, nil)
			}),
			Result("machine firewall gets cleared when firewallID is set to 0", func(ctx context.Context, mck Mock) {
				linodeMachine.Spec.FirewallID = 0
				_, err := reconciler.reconcile(ctx, logr.Logger{}, mScope)
				Expect(err).NotTo(HaveOccurred())
			}),
		),
		OneOf(
			Path(
				Call("machine firewall list fails", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
						&linodego.Instance{
							ID:      11111,
							IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
							IPv6:    "fd00::",
							Tags:    []string{"test-cluster-2"},
							Status:  linodego.InstanceRunning,
							Updated: util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().UpdateInstance(ctx, 11111, gomock.Any()).Return(
						&linodego.Instance{
							ID:      11111,
							IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
							IPv6:    "fd00::",
							Tags:    []string{"test-cluster-2", "test-tag"},
							Status:  linodego.InstanceRunning,
							Updated: util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().ListInstanceFirewalls(ctx, 11111, nil).Return(
						nil, &linodego.Error{Code: http.StatusInternalServerError})
				}),
				Result("machine firewall list error requeues", func(ctx context.Context, mck Mock) {
					linodeMachine.Spec.FirewallID = 10
					res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerWaitForRunningDelay))
					Expect(mck.Logs()).To(ContainSubstring("Failed to list firewalls for Linode instance"))
				}),
			),
			Path(
				Call("machine firewall update fails", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetInstance(ctx, 11111).Return(
						&linodego.Instance{
							ID:      11111,
							IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
							IPv6:    "fd00::",
							Tags:    []string{"test-cluster-2"},
							Status:  linodego.InstanceRunning,
							Updated: util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().UpdateInstance(ctx, 11111, gomock.Any()).Return(
						&linodego.Instance{
							ID:      11111,
							IPv4:    []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
							IPv6:    "fd00::",
							Tags:    []string{"test-cluster-2", "test-tag"},
							Status:  linodego.InstanceRunning,
							Updated: util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().ListInstanceFirewalls(ctx, 11111, nil).Return(
						[]linodego.Firewall{
							{ID: 5}, // Instance currently has firewall ID 5
						}, nil)
					mck.LinodeClient.EXPECT().UpdateInstanceFirewalls(ctx, 11111, linodego.InstanceFirewallUpdateOptions{
						FirewallIDs: []int{10},
					}).Return(nil, &linodego.Error{Code: http.StatusBadRequest})
				}),
				Result("machine firewall update error requeues", func(ctx context.Context, mck Mock) {
					linodeMachine.Spec.FirewallID = 10
					_, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
					Expect(err).To(HaveOccurred())
					Expect(mck.Logs()).To(ContainSubstring("Failed to update firewalls for Linode instance"))
				}),
			),
		),
	)
})

var _ = Describe("machine-delete", Ordered, Label("machine", "machine-delete"), func() {
	machineName := "cluster-delete"
	namespace := "default"
	ownerRef := metav1.OwnerReference{
		Name:       machineName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Machine",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:              machineName,
		Namespace:         namespace,
		OwnerReferences:   ownerRefs,
		DeletionTimestamp: &metav1.Time{Time: time.Now()},
	}

	linodeCluster := &infrav1alpha2.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region:  "us-ord",
			Network: infrav1alpha2.NetworkSpec{},
		},
	}
	providerID := "linode://12345"
	linodeMachine := &infrav1alpha2.LinodeMachine{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeMachineSpec{
			ProviderID: &providerID,
		},
	}
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Labels:    make(map[string]string),
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: ptr.To("test-bootstrap-secret"),
			},
		},
	}

	ctlrSuite := NewControllerSuite(
		GinkgoT(),
		mock.MockLinodeClient{},
		mock.MockK8sClient{},
	)
	reconciler := LinodeMachineReconciler{}

	mScope := &scope.MachineScope{
		LinodeCluster: linodeCluster,
		LinodeMachine: linodeMachine,
		Machine:       machine,
	}

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
		mScope.LinodeMachine = linodeMachine
		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper
		mScope.LinodeCluster = linodeCluster
		mScope.LinodeClient = mck.LinodeClient
		reconciler.Client = mck.K8sClient
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("machine is not deleted because there was an error deleting instance", func(ctx context.Context, mck Mock) {
				}),
				OneOf(
					Path(Result("delete error", func(ctx context.Context, mck Mock) {
						tmpProviderID := linodeMachine.Spec.ProviderID
						linodeMachine.Spec.ProviderID = util.Pointer("linode://foo")
						_, err := reconciler.reconcileDelete(ctx, mck.Logger(), mScope)
						Expect(err).To(HaveOccurred())
						Expect(mck.Logs()).To(ContainSubstring("Failed to parse instance ID from provider ID"))
						linodeMachine.Spec.ProviderID = tmpProviderID

					})),
					Path(Result("delete requeues", func(ctx context.Context, mck Mock) {
						linodeMachine.DeletionTimestamp = &metav1.Time{Time: time.Now()}
						mck.LinodeClient.EXPECT().DeleteInstance(gomock.Any(), gomock.Any()).
							Return(&linodego.Error{Code: http.StatusBadGateway})
						res, err := reconciler.reconcileDelete(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing Linode instance deletion"))
					})),
				),
			),
			Path(
				Call("machine deleted", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().DeleteInstance(gomock.Any(), gomock.Any()).Return(nil)
				}),
				Result("machine deleted", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					_, err := reconciler.reconcileDelete(ctx, logr.Logger{}, mScope)
					Expect(err).NotTo(HaveOccurred())
				})),
		),
	)
})

var _ = Describe("machine in PlacementGroup", Label("machine", "placementGroup"), func() {
	var machine clusterv1.Machine
	var linodeMachine infrav1alpha2.LinodeMachine
	var secret corev1.Secret
	var lpgReconciler *LinodePlacementGroupReconciler
	var linodePlacementGroup infrav1alpha2.LinodePlacementGroup
	var linodeFirewall infrav1alpha2.LinodeFirewall

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType:    "dns",
				DNSRootDomain:       "lkedevs.net",
				DNSUniqueIdentifier: "abc123",
				DNSTTLSec:           30,
			},
		},
	}

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		machine = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Labels:    make(map[string]string),
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}

		linodePlacementGroup = infrav1alpha2.LinodePlacementGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pg",
				Namespace: defaultNamespace,
				UID:       "5123122",
			},
			Spec: infrav1alpha2.LinodePlacementGroupSpec{
				PGID:                 ptr.To(1),
				Region:               "us-ord",
				PlacementGroupPolicy: "strict",
				PlacementGroupType:   "anti_affinity:local",
			},
			Status: infrav1alpha2.LinodePlacementGroupStatus{
				Ready: true,
			},
		}
		Expect(k8sClient.Create(ctx, &linodePlacementGroup)).To(Succeed())

		linodeFirewall = infrav1alpha2.LinodeFirewall{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-fw",
				Namespace: defaultNamespace,
				UID:       "5123123",
			},
			Spec: infrav1alpha2.LinodeFirewallSpec{
				FirewallID: ptr.To(2),
				Enabled:    true,
			},
			Status: infrav1alpha2.LinodeFirewallStatus{
				Ready: true,
			},
		}
		Expect(k8sClient.Create(ctx, &linodeFirewall)).To(Succeed())

		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				ProviderID: ptr.To("linode://0"),
				Type:       "g6-nanode-1",
				Image:      rutil.DefaultMachineControllerLinodeImage,
				PlacementGroupRef: &corev1.ObjectReference{
					Namespace: defaultNamespace,
					Name:      "test-pg",
				},
				FirewallRef: &corev1.ObjectReference{
					Namespace: defaultNamespace,
					Name:      "test-fw",
				},
			},
		}

		lpgReconciler = &LinodePlacementGroupReconciler{
			Recorder: recorder,
			Client:   k8sClient,
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
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates a instance in a PlacementGroup with a firewall", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		helper, err := patch.NewHelper(&linodePlacementGroup, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		_, err = lpgReconciler.reconcile(ctx, logger, &scope.PlacementGroupScope{
			PatchHelper:          helper,
			Client:               k8sClient,
			LinodeClient:         mockLinodeClient,
			LinodePlacementGroup: &linodePlacementGroup,
		})

		Expect(err).NotTo(HaveOccurred())
		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		createOpts, err := newCreateConfig(ctx, &mScope, gzipCompressionFlag, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.PlacementGroup.ID).To(Equal(1))
		Expect(createOpts.FirewallID).To(Equal(2))
	})
})

var _ = Describe("machine in VPC", Label("machine", "VPC"), Ordered, func() {
	var machine clusterv1.Machine
	var secret corev1.Secret
	var lvpcReconciler *LinodeVPCReconciler
	var linodeVPC infrav1alpha2.LinodeVPC

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType:    "dns",
				DNSRootDomain:       "lkedevs.net",
				DNSUniqueIdentifier: "abc123",
				DNSTTLSec:           30,
				SubnetName:          "test",
			},
			VPCRef: &corev1.ObjectReference{
				Namespace: "default",
				Kind:      "LinodeVPC",
				Name:      "test-cluster",
			},
		},
	}

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		machine = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Labels:    make(map[string]string),
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}

		linodeVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: defaultNamespace,
				UID:       "5123122",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				VPCID:  ptr.To(1),
				Region: "us-ord",
				Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
					{
						IPv4:     "10.0.0.0/8",
						SubnetID: 1,
						Label:    "test",
					},
				},
			},
			Status: infrav1alpha2.LinodeVPCStatus{
				Ready: true,
			},
		}
		Expect(k8sClient.Create(ctx, &linodeVPC)).To(Succeed())

		lvpcReconciler = &LinodeVPCReconciler{
			Recorder: recorder,
			Client:   k8sClient,
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
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
		var currentVPC infrav1alpha2.LinodeVPC
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &currentVPC)).To(Succeed())
		currentVPC.Finalizers = nil

		Expect(k8sClient.Update(ctx, &currentVPC)).To(Succeed())

		Expect(k8sClient.Delete(ctx, &currentVPC)).To(Succeed())

		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates a instance with vpc", func(ctx SpecContext) {
		linodeMachine := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				ProviderID: ptr.To("linode://0"),
				Type:       "g6-nanode-1",
				Interfaces: []infrav1alpha2.InstanceConfigInterfaceCreateOptions{
					{
						Primary: true,
					},
				},
				InterfaceGeneration: linodego.GenerationLegacyConfig,
			},
		}
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1, Subnets: []linodego.VPCSubnet{{
				ID:    1,
				Label: "test",
				IPv4:  "10.0.0.0/24",
			}}}, nil)
		helper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		_, err = lvpcReconciler.reconcile(ctx, logger, &scope.VPCScope{
			PatchHelper:  helper,
			Client:       k8sClient,
			LinodeClient: mockLinodeClient,
			LinodeVPC:    &linodeVPC,
		})

		Expect(err).NotTo(HaveOccurred())

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		createOpts, err := newCreateConfig(ctx, &mScope, gzipCompressionFlag, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.Interfaces).To(Equal([]linodego.InstanceConfigInterfaceCreateOptions{
			{
				Purpose:  linodego.InterfacePurposeVPC,
				Primary:  true,
				SubnetID: ptr.To(1),
				IPv4:     &linodego.VPCIPv4{NAT1To1: ptr.To("any")},
			},
			{
				Primary: true,
			},
		}))
	})
	It("creates a instance with pre defined vpc interface", func(ctx SpecContext) {
		linodeMachine := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				ProviderID: ptr.To("linode://0"),
				Type:       "g6-nanode-1",
				Interfaces: []infrav1alpha2.InstanceConfigInterfaceCreateOptions{
					{
						Purpose: linodego.InterfacePurposeVPC,
						Primary: false,
					},
					{
						Purpose: linodego.InterfacePurposePublic,
						Primary: true,
					},
				},
			},
		}
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1, Subnets: []linodego.VPCSubnet{{
				ID:    1,
				Label: "test",
				IPv4:  "10.0.0.0/24",
			}}}, nil)
		helper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		_, err = lvpcReconciler.reconcile(ctx, logger, &scope.VPCScope{
			PatchHelper:  helper,
			Client:       k8sClient,
			LinodeClient: mockLinodeClient,
			LinodeVPC:    &linodeVPC,
		})

		Expect(err).NotTo(HaveOccurred())

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		createOpts, err := newCreateConfig(ctx, &mScope, gzipCompressionFlag, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.Interfaces).To(Equal([]linodego.InstanceConfigInterfaceCreateOptions{
			{
				Purpose:  linodego.InterfacePurposeVPC,
				Primary:  false,
				SubnetID: ptr.To(1),
			},
			{
				Purpose: linodego.InterfacePurposePublic,
				Primary: true,
			}}))
	})
	It("creates an instance with vpc with a specific subnet", func(ctx SpecContext) {
		linodeMachine := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				ProviderID: ptr.To("linode://0"),
				Type:       "g6-nanode-1",
				Interfaces: []infrav1alpha2.InstanceConfigInterfaceCreateOptions{
					{
						Primary: true,
					},
				},
			},
		}
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1, Subnets: []linodego.VPCSubnet{{
				ID:    1,
				Label: "primary",
				IPv4:  "192.16.0.0/24",
			},
				{
					ID:    27,
					Label: "test",
					IPv4:  "10.0.0.0/24",
				},
			}}, nil)
		helper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		_, err = lvpcReconciler.reconcile(ctx, logger, &scope.VPCScope{
			PatchHelper:  helper,
			Client:       k8sClient,
			LinodeClient: mockLinodeClient,
			LinodeVPC:    &linodeVPC,
		})

		Expect(err).NotTo(HaveOccurred())

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		createOpts, err := newCreateConfig(ctx, &mScope, gzipCompressionFlag, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.Interfaces).To(Equal([]linodego.InstanceConfigInterfaceCreateOptions{
			{
				Purpose:  linodego.InterfacePurposeVPC,
				Primary:  true,
				SubnetID: ptr.To(27),
				IPv4:     &linodego.VPCIPv4{NAT1To1: ptr.To("any")},
			},
			{
				Primary: true,
			},
		}))
	})
})

var _ = Describe("machine in VPC with new network interfaces", Label("machine", "newNetworkInterfaces", "VPC"), Ordered, func() {
	var machine clusterv1.Machine
	var secret corev1.Secret
	var lvpcReconciler *LinodeVPCReconciler
	var linodeVPC infrav1alpha2.LinodeVPC

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType:    "dns",
				DNSRootDomain:       "lkedevs.net",
				DNSUniqueIdentifier: "abc123",
				DNSTTLSec:           30,
				SubnetName:          "test",
			},
			VPCRef: &corev1.ObjectReference{
				Namespace: "default",
				Kind:      "LinodeVPC",
				Name:      "test-cluster",
			},
		},
	}

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		machine = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Labels:    make(map[string]string),
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}

		linodeVPC = infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: defaultNamespace,
				UID:       "5123122",
			},
			Spec: infrav1alpha2.LinodeVPCSpec{
				VPCID:  ptr.To(1),
				Region: "us-ord",
				Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
					{
						IPv4:     "10.0.0.0/8",
						SubnetID: 1,
						Label:    "test",
					},
				},
			},
			Status: infrav1alpha2.LinodeVPCStatus{
				Ready: true,
			},
		}
		Expect(k8sClient.Create(ctx, &linodeVPC)).To(Succeed())

		lvpcReconciler = &LinodeVPCReconciler{
			Recorder: recorder,
			Client:   k8sClient,
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
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
		var currentVPC infrav1alpha2.LinodeVPC
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &currentVPC)).To(Succeed())
		currentVPC.Finalizers = nil

		Expect(k8sClient.Update(ctx, &currentVPC)).To(Succeed())

		Expect(k8sClient.Delete(ctx, &currentVPC)).To(Succeed())

		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates a instance with vpc", func(ctx SpecContext) {
		linodeMachine := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				ProviderID:          ptr.To("linode://0"),
				Type:                "g6-nanode-1",
				InterfaceGeneration: linodego.GenerationLinode,
			},
		}
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1, Subnets: []linodego.VPCSubnet{{
				ID:    1,
				Label: "test",
				IPv4:  "10.0.0.0/24",
			}}}, nil)
		helper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		_, err = lvpcReconciler.reconcile(ctx, logger, &scope.VPCScope{
			PatchHelper:  helper,
			Client:       k8sClient,
			LinodeClient: mockLinodeClient,
			LinodeVPC:    &linodeVPC,
		})

		Expect(err).NotTo(HaveOccurred())

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		createOpts, err := newCreateConfig(ctx, &mScope, gzipCompressionFlag, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.LinodeInterfaces).To(Equal([]linodego.LinodeInterfaceCreateOptions{
			{
				VPC: &linodego.VPCInterfaceCreateOptions{
					SubnetID: 1,
					IPv4: &linodego.VPCInterfaceIPv4CreateOptions{
						Addresses: []linodego.VPCInterfaceIPv4AddressCreateOptions{{
							NAT1To1Address: ptr.To("auto"),
							Primary:        ptr.To(true),
							Address:        ptr.To("auto"),
						}},
					},
				},
			},
		}))
	})
	It("creates a instance with pre defined vpc interface", func(ctx SpecContext) {
		linodeMachine := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				ProviderID:          ptr.To("linode://0"),
				Type:                "g6-nanode-1",
				InterfaceGeneration: linodego.GenerationLinode,
			},
		}
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1, Subnets: []linodego.VPCSubnet{{
				ID:    1,
				Label: "test",
				IPv4:  "10.0.0.0/24",
			}}}, nil)
		helper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		_, err = lvpcReconciler.reconcile(ctx, logger, &scope.VPCScope{
			PatchHelper:  helper,
			Client:       k8sClient,
			LinodeClient: mockLinodeClient,
			LinodeVPC:    &linodeVPC,
		})

		Expect(err).NotTo(HaveOccurred())

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		createOpts, err := newCreateConfig(ctx, &mScope, gzipCompressionFlag, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.LinodeInterfaces).To(Equal([]linodego.LinodeInterfaceCreateOptions{
			{
				VPC: &linodego.VPCInterfaceCreateOptions{
					SubnetID: 1,
					IPv4: &linodego.VPCInterfaceIPv4CreateOptions{
						Addresses: []linodego.VPCInterfaceIPv4AddressCreateOptions{{
							NAT1To1Address: ptr.To("auto"),
							Primary:        ptr.To(true),
							Address:        ptr.To("auto"),
						}},
					},
				},
			},
		}))
	})
	It("creates an instance with vpc with a specific subnet", func(ctx SpecContext) {
		linodeMachine := infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				ProviderID:          ptr.To("linode://0"),
				Type:                "g6-nanode-1",
				InterfaceGeneration: linodego.GenerationLinode,
			},
		}
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1, Subnets: []linodego.VPCSubnet{{
				ID:    1,
				Label: "primary",
				IPv4:  "192.16.0.0/24",
			},
				{
					ID:    27,
					Label: "test",
					IPv4:  "10.0.0.0/24",
				},
			}}, nil)
		helper, err := patch.NewHelper(&linodeVPC, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		_, err = lvpcReconciler.reconcile(ctx, logger, &scope.VPCScope{
			PatchHelper:  helper,
			Client:       k8sClient,
			LinodeClient: mockLinodeClient,
			LinodeVPC:    &linodeVPC,
		})

		Expect(err).NotTo(HaveOccurred())

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		createOpts, err := newCreateConfig(ctx, &mScope, gzipCompressionFlag, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.LinodeInterfaces).To(Equal([]linodego.LinodeInterfaceCreateOptions{
			{
				VPC: &linodego.VPCInterfaceCreateOptions{
					SubnetID: 27,
					IPv4: &linodego.VPCInterfaceIPv4CreateOptions{
						Addresses: []linodego.VPCInterfaceIPv4AddressCreateOptions{{
							NAT1To1Address: ptr.To("auto"),
							Primary:        ptr.To(true),
							Address:        ptr.To("auto"),
						}},
					},
				},
			},
		}))
	})
})

var _ = Describe("machine in vlan", Label("machine", "vlan"), Ordered, func() {
	var machine clusterv1.Machine
	var secret corev1.Secret

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	var reconciler *LinodeMachineReconciler
	var linodeMachine infrav1alpha2.LinodeMachine

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				UseVlan: true,
			},
		},
	}

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		machine = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Labels:    make(map[string]string),
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}

		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Type:           "g6-nanode-1",
				Image:          rutil.DefaultMachineControllerLinodeImage,
				DiskEncryption: string(linodego.InstanceDiskEncryptionEnabled),
				Interfaces: []infrav1alpha2.InstanceConfigInterfaceCreateOptions{
					{
						Purpose: linodego.InterfacePurposePublic,
					},
					{
						Purpose: linodego.InterfacePurposeVLAN,
					},
				},
			},
		}

		mockCtrl = gomock.NewController(GinkgoT())
		testLogs = &bytes.Buffer{}
		logger = zap.New(
			zap.WriteTo(GinkgoWriter),
			zap.WriteTo(testLogs),
			zap.UseDevMode(true),
		)
		reconciler = &LinodeMachineReconciler{
			Recorder: recorder,
		}
	})

	AfterEach(func(ctx SpecContext) {
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates an instance with vlan", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
		getImage := mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
		createInst := mockLinodeClient.EXPECT().
			CreateInstance(ctx, gomock.Any()).
			After(getImage).
			Return(&linodego.Instance{
				ID:     123,
				IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
				IPv6:   "fd00::",
				Status: linodego.InstanceOffline,
			}, nil)
		mockLinodeClient.EXPECT().
			OnAfterResponse(gomock.Any()).
			Return()
		listInstConfs := mockLinodeClient.EXPECT().
			ListInstanceConfigs(ctx, 123, gomock.Any()).
			After(createInst).
			Return([]linodego.InstanceConfig{{
				ID: 1,
			}}, nil)
		mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
			Helpers: &linodego.InstanceConfigHelpers{Network: true},
		}).
			After(listInstConfs).
			Return(nil, nil)
		bootInst := mockLinodeClient.EXPECT().
			BootInstance(ctx, 123, 0).
			After(createInst).
			Return(nil)
		getAddrs := mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(ctx, 123).
			After(bootInst).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
					Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					VPC:     []*linodego.VPCIP{},
				},
				IPv6: &linodego.InstanceIPv6Response{
					SLAAC: &linodego.InstanceIP{
						Address: "fd00::",
					},
				},
			}, nil)
		mockLinodeClient.EXPECT().
			ListInstanceConfigs(ctx, 123, gomock.Any()).
			After(getAddrs).
			Return([]linodego.InstanceConfig{{
				Interfaces: []linodego.InstanceConfigInterface{
					{
						Purpose: linodego.InterfacePurposePublic,
					},
					{
						Purpose:     linodego.InterfacePurposeVLAN,
						IPAMAddress: "10.0.0.2/11",
					},
				},
			}}, nil)

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))
	})
})

var _ = Describe("machine in vlan for new network interfaces", Label("machine", "newNetworkInterfaces", "vlan"), Ordered, func() {
	var machine clusterv1.Machine
	var secret corev1.Secret

	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	var reconciler *LinodeMachineReconciler
	var linodeMachine infrav1alpha2.LinodeMachine

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: defaultNamespace,
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				UseVlan: true,
			},
		},
	}

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func(ctx SpecContext) {
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		machine = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Labels:    make(map[string]string),
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret"),
				},
			},
		}

		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Type:           "g6-nanode-1",
				Image:          rutil.DefaultMachineControllerLinodeImage,
				DiskEncryption: string(linodego.InstanceDiskEncryptionEnabled),
				LinodeInterfaces: []infrav1alpha2.LinodeInterfaceCreateOptions{{
					VLAN: &infrav1alpha2.VLANInterface{},
				}},
			},
		}

		mockCtrl = gomock.NewController(GinkgoT())
		testLogs = &bytes.Buffer{}
		logger = zap.New(
			zap.WriteTo(GinkgoWriter),
			zap.WriteTo(testLogs),
			zap.UseDevMode(true),
		)
		reconciler = &LinodeMachineReconciler{
			Recorder: recorder,
		}
	})

	AfterEach(func(ctx SpecContext) {
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates an instance with vlan", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, linodego.CapabilityDiskEncryption}}, nil)
		getImage := mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
		createInst := mockLinodeClient.EXPECT().
			CreateInstance(ctx, gomock.Any()).
			After(getImage).
			Return(&linodego.Instance{
				ID:     123,
				IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
				IPv6:   "fd00::",
				Status: linodego.InstanceOffline,
			}, nil)
		mockLinodeClient.EXPECT().
			OnAfterResponse(gomock.Any()).
			Return()
		listInstConfs := mockLinodeClient.EXPECT().
			ListInstanceConfigs(ctx, 123, gomock.Any()).
			After(createInst).
			Return([]linodego.InstanceConfig{{
				ID: 1,
			}}, nil)
		mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 1, linodego.InstanceConfigUpdateOptions{
			Helpers: &linodego.InstanceConfigHelpers{Network: true},
		}).
			After(listInstConfs).
			Return(nil, nil)
		bootInst := mockLinodeClient.EXPECT().
			BootInstance(ctx, 123, 0).
			After(createInst).
			Return(nil)
		getAddrs := mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(ctx, 123).
			After(bootInst).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
					Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					VPC:     []*linodego.VPCIP{},
				},
				IPv6: &linodego.InstanceIPv6Response{
					SLAAC: &linodego.InstanceIP{
						Address: "fd00::",
					},
				},
			}, nil)
		mockLinodeClient.EXPECT().
			ListInterfaces(ctx, 123, gomock.Any()).
			After(getAddrs).
			Return([]linodego.LinodeInterface{{
				VLAN: &linodego.VLANInterface{
					IPAMAddress: ptr.To("10.0.0.2/11"),
				},
			}}, nil)

		mScope := scope.MachineScope{
			Client:        k8sClient,
			LinodeClient:  mockLinodeClient,
			Cluster:       &cluster,
			Machine:       &machine,
			LinodeCluster: &linodeCluster,
			LinodeMachine: &linodeMachine,
		}

		patchHelper, err := patch.NewHelper(mScope.LinodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = patchHelper

		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(linodeMachine.GetCondition(ConditionPreflightMetadataSupportConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightCreated).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightConfigured).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightBootTriggered).Status).To(Equal(metav1.ConditionTrue))
		Expect(linodeMachine.GetCondition(ConditionPreflightReady).Status).To(Equal(metav1.ConditionTrue))
	})
})

var _ = Describe("create machine with direct VPCID", Label("machine", "VPCID"), Ordered, func() {
	var (
		reconciler      LinodeMachineReconciler
		linodeMachine   infrav1alpha2.LinodeMachine
		machineKey      client.ObjectKey
		bootstrapSecret corev1.Secret
	)

	BeforeAll(func(ctx SpecContext) {
		reconciler = LinodeMachineReconciler{
			Client:   k8sClient,
			Recorder: record.NewFakeRecorder(100),
		}

		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-with-direct-vpcid",
				Namespace: defaultNamespace,
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Type:   "g6-nanode-1",
				Image:  "linode/ubuntu22.04",
				Region: "us-east",
				VPCID:  ptr.To(12345),
			},
		}
		machineKey = client.ObjectKeyFromObject(&linodeMachine)

		bootstrapSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret-vpcid",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}

		Expect(k8sClient.Create(ctx, &bootstrapSecret)).To(Succeed())
		Expect(k8sClient.Create(ctx, &linodeMachine)).To(Succeed())
	})

	AfterAll(func(ctx SpecContext) {
		Expect(k8sClient.Delete(ctx, &linodeMachine)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &bootstrapSecret)).To(Succeed())
	})

	It("creates a machine with direct VPCID", func(ctx SpecContext) {
		mockCtrl := gomock.NewController(GinkgoT())
		defer mockCtrl.Finish()

		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetRegion(gomock.Any(), gomock.Any()).
			Return(&linodego.Region{ID: "us-east", Capabilities: []string{"Metadata"}}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			GetImage(gomock.Any(), gomock.Any()).
			Return(&linodego.Image{ID: "linode/ubuntu22.04", Capabilities: []string{"cloud-init"}}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			GetVPC(gomock.Any(), gomock.Eq(12345)).
			Return(&linodego.VPC{
				ID:     12345,
				Label:  "test-vpc",
				Region: "us-east",
				Subnets: []linodego.VPCSubnet{
					{
						ID:    1001,
						Label: "subnet-1",
					},
				},
			}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			CreateInstance(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts linodego.InstanceCreateOptions) (*linodego.Instance, error) {
				// Verify that the instance is created with the correct VPC interface
				Expect(opts.Interfaces).To(HaveLen(1))
				Expect(opts.Interfaces[0].Purpose).To(Equal(linodego.InterfacePurposeVPC))
				Expect(*opts.Interfaces[0].SubnetID).To(Equal(1001))

				return &linodego.Instance{
					ID:     12345,
					Label:  opts.Label,
					Region: opts.Region,
					Status: linodego.InstanceRunning,
					IPv4:   []*net.IP{ptr.To(net.ParseIP("192.168.1.2"))},
					IPv6:   "2001:db8::2",
				}, nil
			}).
			AnyTimes()
		mockLinodeClient.EXPECT().
			OnAfterResponse(gomock.Any()).
			Return().
			AnyTimes()
		mockLinodeClient.EXPECT().
			ListInstanceConfigs(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]linodego.InstanceConfig{
				{
					ID:      1,
					Label:   "My Config",
					Devices: &linodego.InstanceConfigDeviceMap{},
				},
			}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			UpdateInstanceConfig(gomock.Any(), 12345, 1, gomock.Any()).
			Return(nil, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(gomock.Any(), gomock.Any()).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Public:  []*linodego.InstanceIP{{Address: "192.168.1.2"}},
					Private: []*linodego.InstanceIP{},
				},
				IPv6: &linodego.InstanceIPv6Response{
					SLAAC: &linodego.InstanceIP{Address: "2001:db8::2"},
				},
			}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			BootInstance(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		// Create a machine scope with the mock client
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-with-direct-vpcid",
				Namespace: defaultNamespace,
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret-vpcid"),
				},
			},
		}

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: defaultNamespace,
			},
		}

		// Get the LinodeMachine
		Expect(k8sClient.Get(ctx, machineKey, &linodeMachine)).To(Succeed())

		// Create a machine scope
		patchHelper, err := patch.NewHelper(&linodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		// Create a LinodeCluster for the machineScope
		linodeCluster := &infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: defaultNamespace,
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-east",
			},
		}

		// Set the VPC preflight check condition to true
		linodeMachine.SetCondition(metav1.Condition{
			Type:   ConditionPreflightLinodeVPCReady,
			Status: metav1.ConditionTrue,
			Reason: "VPCReady",
		})

		mScope := &scope.MachineScope{
			Client:        k8sClient,
			Cluster:       cluster,
			Machine:       machine,
			LinodeMachine: &linodeMachine,
			LinodeCluster: linodeCluster, // Add the LinodeCluster to the scope
			PatchHelper:   patchHelper,
			LinodeClient:  mockLinodeClient,
		}

		// Reconcile the machine
		result, err := reconciler.reconcile(ctx, logr.Discard(), mScope)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsZero()).To(BeTrue())

		// Verify that the preflight check for VPC is successful
		Expect(linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady).Status).To(Equal(metav1.ConditionTrue))
	})
})

var _ = Describe("create machine with direct VPCID with new network interfaces", Label("machine", "newNetworkInterfaces", "VPCID"), Ordered, func() {
	var (
		reconciler      LinodeMachineReconciler
		linodeMachine   infrav1alpha2.LinodeMachine
		machineKey      client.ObjectKey
		bootstrapSecret corev1.Secret
	)

	BeforeAll(func(ctx SpecContext) {
		reconciler = LinodeMachineReconciler{
			Client:   k8sClient,
			Recorder: record.NewFakeRecorder(100),
		}

		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-with-direct-vpcid-new-network-interfaces",
				Namespace: defaultNamespace,
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				Type:                "g6-nanode-1",
				Image:               "linode/ubuntu22.04",
				Region:              "us-east",
				VPCID:               ptr.To(12345),
				InterfaceGeneration: linodego.GenerationLinode,
			},
		}
		machineKey = client.ObjectKeyFromObject(&linodeMachine)

		bootstrapSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-secret-vpcid-new-network-interfaces",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"value": []byte("userdata"),
			},
		}

		Expect(k8sClient.Create(ctx, &bootstrapSecret)).To(Succeed())
		Expect(k8sClient.Create(ctx, &linodeMachine)).To(Succeed())
	})

	AfterAll(func(ctx SpecContext) {
		Expect(k8sClient.Delete(ctx, &linodeMachine)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &bootstrapSecret)).To(Succeed())
	})

	It("creates a machine with direct VPCID", func(ctx SpecContext) {
		mockCtrl := gomock.NewController(GinkgoT())
		defer mockCtrl.Finish()

		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetRegion(gomock.Any(), gomock.Any()).
			Return(&linodego.Region{ID: "us-east", Capabilities: []string{"Metadata"}}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			GetImage(gomock.Any(), gomock.Any()).
			Return(&linodego.Image{ID: "linode/ubuntu22.04", Capabilities: []string{"cloud-init"}}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			GetVPC(gomock.Any(), gomock.Eq(12345)).
			Return(&linodego.VPC{
				ID:     12345,
				Label:  "test-vpc",
				Region: "us-east",
				Subnets: []linodego.VPCSubnet{
					{
						ID:    1001,
						Label: "subnet-1",
					},
				},
			}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			CreateInstance(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, opts linodego.InstanceCreateOptions) (*linodego.Instance, error) {
				// Verify that the instance is created with the correct VPC interface
				Expect(opts.LinodeInterfaces).To(HaveLen(1))
				Expect(opts.LinodeInterfaces[0].VPC).ToNot(BeNil())
				Expect(opts.LinodeInterfaces[0].VPC.SubnetID).To(Equal(1001))

				return &linodego.Instance{
					ID:     12345,
					Label:  opts.Label,
					Region: opts.Region,
					Status: linodego.InstanceRunning,
					IPv4:   []*net.IP{ptr.To(net.ParseIP("192.168.1.2"))},
					IPv6:   "2001:db8::2",
				}, nil
			}).
			AnyTimes()
		mockLinodeClient.EXPECT().
			OnAfterResponse(gomock.Any()).
			Return().
			AnyTimes()
		mockLinodeClient.EXPECT().
			ListInstanceConfigs(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]linodego.InstanceConfig{
				{
					ID:      1,
					Label:   "My Config",
					Devices: &linodego.InstanceConfigDeviceMap{},
				},
			}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			UpdateInstanceConfig(gomock.Any(), 12345, 1, gomock.Any()).
			Return(nil, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(gomock.Any(), gomock.Any()).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Public:  []*linodego.InstanceIP{{Address: "192.168.1.2"}},
					Private: []*linodego.InstanceIP{},
				},
				IPv6: &linodego.InstanceIPv6Response{
					SLAAC: &linodego.InstanceIP{Address: "2001:db8::2"},
				},
			}, nil).
			AnyTimes()
		mockLinodeClient.EXPECT().
			BootInstance(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		// Create a machine scope with the mock client
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-with-direct-vpcid-new-network-interfaces",
				Namespace: defaultNamespace,
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("bootstrap-secret-vpcid-new-network-interfaces"),
				},
			},
		}

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: defaultNamespace,
			},
		}

		// Get the LinodeMachine
		Expect(k8sClient.Get(ctx, machineKey, &linodeMachine)).To(Succeed())

		// Create a machine scope
		patchHelper, err := patch.NewHelper(&linodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		// Create a LinodeCluster for the machineScope
		linodeCluster := &infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: defaultNamespace,
			},
			Spec: infrav1alpha2.LinodeClusterSpec{
				Region: "us-east",
			},
		}

		// Set the VPC preflight check condition to true
		linodeMachine.SetCondition(metav1.Condition{
			Type:   ConditionPreflightLinodeVPCReady,
			Status: metav1.ConditionTrue,
			Reason: "VPCReady",
		})

		mScope := &scope.MachineScope{
			Client:        k8sClient,
			Cluster:       cluster,
			Machine:       machine,
			LinodeMachine: &linodeMachine,
			LinodeCluster: linodeCluster, // Add the LinodeCluster to the scope
			PatchHelper:   patchHelper,
			LinodeClient:  mockLinodeClient,
		}

		// Reconcile the machine
		result, err := reconciler.reconcile(ctx, logr.Discard(), mScope)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsZero()).To(BeTrue())

		// Verify that the preflight check for VPC is successful
		Expect(linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady).Status).To(Equal(metav1.ConditionTrue))
	})
})

var _ = Describe("direct vpc functions", Label("machine", "vpc", "functions"), Ordered, func() {
	var mockCtrl *gomock.Controller
	var mockLinodeClient *mock.MockLinodeClient
	var mockK8sClient *mock.MockK8sClient
	var mockRecorder *record.FakeRecorder
	var reconciler *LinodeMachineReconciler
	var logger logr.Logger
	var machineScope *scope.MachineScope
	var linodeMachine *infrav1alpha2.LinodeMachine
	var linodeCluster *infrav1alpha2.LinodeCluster
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
		mockCtrl = gomock.NewController(GinkgoT())
		mockLinodeClient = mock.NewMockLinodeClient(mockCtrl)
		mockK8sClient = mock.NewMockK8sClient(mockCtrl)
		mockRecorder = record.NewFakeRecorder(10)
		logger = zap.New()

		linodeMachine = &infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-machine",
				Namespace: "default",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{},
		}

		linodeCluster = &infrav1alpha2.LinodeCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
			Spec: infrav1alpha2.LinodeClusterSpec{},
		}

		machineScope = &scope.MachineScope{
			LinodeClient:  mockLinodeClient,
			Client:        mockK8sClient,
			LinodeMachine: linodeMachine,
			LinodeCluster: linodeCluster,
		}

		reconciler = &LinodeMachineReconciler{
			Client:   mockK8sClient,
			Recorder: mockRecorder,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("validateVPC", func() {
		Context("when VPC exists with subnets", func() {
			BeforeEach(func() {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(123)).Return(&linodego.VPC{
					ID:     123,
					Label:  "test-vpc",
					Region: "us-east",
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			})

			It("should succeed and set condition to true", func() {
				err := reconciler.validateVPC(ctx, 123, machineScope, logger, "Test")
				Expect(err).NotTo(HaveOccurred())
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			})
		})

		Context("when VPC exists with no subnets", func() {
			BeforeEach(func() {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(123)).Return(&linodego.VPC{
					ID:      123,
					Label:   "test-vpc",
					Region:  "us-east",
					Subnets: []linodego.VPCSubnet{},
				}, nil)
			})

			It("should fail and set condition to false", func() {
				err := reconciler.validateVPC(ctx, 123, machineScope, logger, "Test")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Test VPC with ID 123 has no subnets"))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			})
		})

		Context("when VPC does not exist", func() {
			BeforeEach(func() {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(123)).Return(nil, errors.New("VPC not found"))
			})

			It("should fail and set condition to false", func() {
				err := reconciler.validateVPC(ctx, 123, machineScope, logger, "Test")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Test VPC with ID 123 not found"))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			})
		})
	})

	Describe("reconcilePreflightVPC", func() {
		Context("when machine has direct VPCID and it exists with subnets", func() {
			BeforeEach(func() {
				machineScope.LinodeMachine.Spec.VPCID = ptr.To(123)
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(123)).Return(&linodego.VPC{
					ID:     123,
					Label:  "test-vpc",
					Region: "us-east",
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			})

			It("should succeed", func() {
				vpcRef := &corev1.ObjectReference{Name: "test-vpc"}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			})
		})

		Context("when machine has direct VPCID but it does not exist", func() {
			BeforeEach(func() {
				machineScope.LinodeMachine.Spec.VPCID = ptr.To(123)
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(123)).Return(nil, errors.New("VPC not found"))
			})

			It("should fail", func() {
				vpcRef := &corev1.ObjectReference{Name: "test-vpc"}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Machine VPC with ID 123 not found"))
				Expect(result).To(Equal(ctrl.Result{}))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			})
		})

		Context("when cluster has direct VPCID and it exists with subnets", func() {
			BeforeEach(func() {
				machineScope.LinodeCluster.Spec.VPCID = ptr.To(456)
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(456)).Return(&linodego.VPC{
					ID:     456,
					Label:  "test-vpc",
					Region: "us-east",
					Subnets: []linodego.VPCSubnet{
						{
							ID:    789,
							Label: "subnet-1",
						},
					},
				}, nil)
			})

			It("should succeed", func() {
				vpcRef := &corev1.ObjectReference{Name: "test-vpc"}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			})
		})

		Context("when cluster has direct VPCID but it does not exist", func() {
			BeforeEach(func() {
				machineScope.LinodeCluster.Spec.VPCID = ptr.To(456)
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(456)).Return(nil, errors.New("VPC not found"))
			})

			It("should fail", func() {
				vpcRef := &corev1.ObjectReference{Name: "test-vpc"}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Cluster VPC with ID 456 not found"))
				Expect(result).To(Equal(ctrl.Result{}))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			})
		})

		Context("when using VPC reference and it exists and is ready", func() {
			BeforeEach(func() {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ interface{}, vpc *infrav1alpha2.LinodeVPC, _ ...interface{}) error {
						vpc.Status.Ready = true
						return nil
					})
			})

			It("should succeed and trigger an event", func() {
				vpcRef := &corev1.ObjectReference{
					Name:      "test-vpc",
					Namespace: "default",
				}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				// Check if an event was recorded
				select {
				case event := <-mockRecorder.Events:
					Expect(event).To(ContainSubstring("LinodeVPC is now available"))
				default:
					Fail("Expected event, but none was recorded")
				}

				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			})
		})

		Context("when using VPC reference with empty namespace", func() {
			BeforeEach(func() {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Namespace: "default", // Should use machine's namespace
					Name:      "test-vpc",
				}, gomock.Any()).DoAndReturn(
					func(_ context.Context, _ interface{}, vpc *infrav1alpha2.LinodeVPC, _ ...interface{}) error {
						vpc.Status.Ready = true
						return nil
					})
			})

			It("should succeed and use machine's namespace", func() {
				vpcRef := &corev1.ObjectReference{
					Name: "test-vpc",
					// Namespace intentionally omitted
				}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			})
		})

		Context("when using VPC reference and it exists but is not ready", func() {
			BeforeEach(func() {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ interface{}, vpc *infrav1alpha2.LinodeVPC, _ ...interface{}) error {
						vpc.Status.Ready = false
						return nil
					})
			})

			It("should requeue with delay", func() {
				vpcRef := &corev1.ObjectReference{
					Name:      "test-vpc",
					Namespace: "default",
				}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{RequeueAfter: rutil.DefaultClusterControllerReconcileDelay}))
			})
		})

		Context("when using VPC reference and it is not found with no stale condition", func() {
			BeforeEach(func() {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("VPC not found"))
			})

			It("should requeue with delay", func() {
				vpcRef := &corev1.ObjectReference{
					Name:      "test-vpc",
					Namespace: "default",
				}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{RequeueAfter: rutil.DefaultClusterControllerReconcileDelay}))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			})
		})

		Context("when using VPC reference and it is not found with stale condition", func() {
			BeforeEach(func() {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("VPC not found"))

				// Set stale condition
				oldTime := metav1.NewTime(time.Now().Add(-24 * time.Hour)) // 24 hours ago
				linodeMachine.SetCondition(metav1.Condition{
					Type:               ConditionPreflightLinodeVPCReady,
					Status:             metav1.ConditionFalse,
					Reason:             "TestReason",
					Message:            "Test message",
					LastTransitionTime: oldTime,
				})
			})

			It("should fail", func() {
				vpcRef := &corev1.ObjectReference{
					Name:      "test-vpc",
					Namespace: "default",
				}
				result, err := reconciler.reconcilePreflightVPC(ctx, logger, machineScope, vpcRef)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("VPC not found"))
				Expect(result).To(Equal(ctrl.Result{}))
				condition := linodeMachine.GetCondition(ConditionPreflightLinodeVPCReady)
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			})
		})
	})
})
