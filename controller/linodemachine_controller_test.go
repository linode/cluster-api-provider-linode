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
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
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

const defaultNamespace = "default"

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
				Devices: &linodego.InstanceConfigDeviceMap{
					SDA: &linodego.InstanceConfigDevice{DiskID: 100},
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

		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightReady)).To(BeTrue())

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
			Expect(res).NotTo(Equal(rutil.DefaultMachineControllerWaitForRunningDelay))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("time is up"))

			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeFalse())
			Expect(conditions.Get(&linodeMachine, ConditionPreflightCreated).Severity).To(Equal(clusterv1.ConditionSeverityError))
			Expect(conditions.Get(&linodeMachine, ConditionPreflightCreated).Message).To(ContainSubstring("time is up"))
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

	Context("creates a instance with disks", func() {
		It("in a single call when disks aren't delayed", func(ctx SpecContext) {
			machine.Labels[clusterv1.MachineControlPlaneLabel] = "true"
			linodeMachine.Spec.DataDisks = map[string]*infrav1alpha2.InstanceDisk{"sdb": ptr.To(infrav1alpha2.InstanceDisk{Label: "etcd-data", Size: resource.MustParse("10Gi")})}

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
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).MaxTimes(2)
			getInstDisk := mockLinodeClient.EXPECT().
				GetInstanceDisk(ctx, 123, 100).
				After(listInstConfs).
				Return(&linodego.InstanceDisk{ID: 100, Size: 15000}, nil)
			resizeInstDisk := mockLinodeClient.EXPECT().
				ResizeInstanceDisk(ctx, 123, 100, 4262).
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
			listInstConfsForProfile := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createEtcdDisk).
				Return([]linodego.InstanceConfig{{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).MaxTimes(2)
			createInstanceProfile := mockLinodeClient.EXPECT().
				UpdateInstanceConfig(ctx, 123, 0, linodego.InstanceConfigUpdateOptions{
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
			getAddrs = mockLinodeClient.EXPECT().
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
			mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(getAddrs).
				Return([]linodego.InstanceConfig{{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
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
			Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
			Expect(k8sClient.Create(ctx, &linodeMachine)).To(Succeed())

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())

			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightReady)).To(BeTrue())

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
			linodeMachine.Spec.DataDisks = map[string]*infrav1alpha2.InstanceDisk{"sdb": ptr.To(infrav1alpha2.InstanceDisk{Label: "etcd-data", Size: resource.MustParse("10Gi")})}

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
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil)
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

			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeFalse())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightAdditionalDisksCreated)).To(BeFalse())

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
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil)
			createInstanceProfile := mockLinodeClient.EXPECT().
				UpdateInstanceConfig(ctx, 123, 0, linodego.InstanceConfigUpdateOptions{
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
			getAddrs = mockLinodeClient.EXPECT().
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
			mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(getAddrs).
				Return([]linodego.InstanceConfig{{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
					Interfaces: []linodego.InstanceConfigInterface{{
						VPCID: ptr.To(1),
						IPv4:  &linodego.VPCIPv4{VPC: "10.0.0.2"},
					}},
				}}, nil)

			_, err = reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())

			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightReady)).To(BeTrue())

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
				Devices: &linodego.InstanceConfigDeviceMap{
					SDA: &linodego.InstanceConfigDevice{DiskID: 100},
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

		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightReady)).To(BeTrue())

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
			Type:  "g6-nanode-1",
			Image: rutil.DefaultMachineControllerLinodeImage,
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
				}),
				OneOf(
					Path(Result("create fails when failing to get referenced firewall", func(ctx context.Context, mck Mock) {
						getRegion := mck.LinodeClient.EXPECT().
							GetRegion(ctx, gomock.Any()).
							Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
						mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
							After(getRegion).
							Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
						_, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).To(HaveOccurred())
						Expect(mck.Logs()).To(ContainSubstring("nil firewallID"))
					})),
				),
			),
			Path(
				Call("machine is not created because there were too many requests", func(ctx context.Context, mck Mock) {
					linodeMachine.Spec.FirewallRef = nil
				}),
				OneOf(
					Path(Result("create requeues when failing to create instance config", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().
							GetRegion(ctx, gomock.Any()).
							Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
						mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
							Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultLinodeTooManyRequestsErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to create Linode machine InstanceCreateOptions"))
					})),
					Path(Result("create requeues when failing to create instance", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().
							GetRegion(ctx, gomock.Any()).
							Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
						getImage := mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
							Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
						mck.LinodeClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).
							After(getImage).
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
						mck.LinodeClient.EXPECT().
							GetRegion(ctx, gomock.Any()).
							Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
						getImage := mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
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

						Expect(rutil.ConditionTrue(linodeMachine, ConditionPreflightCreated)).To(BeTrue())
						Expect(rutil.ConditionTrue(linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
						Expect(rutil.ConditionTrue(linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
						Expect(rutil.ConditionTrue(linodeMachine, ConditionPreflightReady)).To(BeTrue())

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
							Status:  linodego.InstanceProvisioning,
							Updated: util.Pointer(time.Now()),
						}, nil)
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
							Status:  linodego.InstanceRunning,
							Updated: util.Pointer(time.Now()),
						}, nil)
					res, err = reconciler.reconcile(ctx, logr.Logger{}, mScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceRunning))
					Expect(rutil.ConditionTrue(linodeMachine, clusterv1.ReadyCondition)).To(BeTrue())
				})),
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
						mck.LinodeClient.EXPECT().DeleteInstance(gomock.Any(), gomock.Any()).
							Return(&linodego.Error{Code: http.StatusInternalServerError})
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
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, infrav1alpha2.LinodePlacementGroupCapability}}, nil)
		mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)

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

		createOpts, err := newCreateConfig(ctx, &mScope, logger)
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
				VPCID:   ptr.To(1),
				Region:  "us-ord",
				Subnets: []infrav1alpha2.VPCSubnetCreateOptions{},
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
			},
		}
		mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, infrav1alpha2.LinodePlacementGroupCapability}}, nil)
		mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1}, nil)
		mockLinodeClient.EXPECT().
			GetVPC(ctx, gomock.Any()).
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

		createOpts, err := newCreateConfig(ctx, &mScope, logger)
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
			}}))
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
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			Return(&linodego.Region{Capabilities: []string{linodego.CapabilityMetadata, infrav1alpha2.LinodePlacementGroupCapability}}, nil)
		mockLinodeClient.EXPECT().
			GetImage(ctx, gomock.Any()).
			After(getRegion).
			Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
		mockLinodeClient.EXPECT().
			ListVPCs(ctx, gomock.Any()).
			Return([]linodego.VPC{}, nil)
		mockLinodeClient.EXPECT().
			CreateVPC(ctx, gomock.Any()).
			Return(&linodego.VPC{ID: 1}, nil)
		mockLinodeClient.EXPECT().
			GetVPC(ctx, gomock.Any()).
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

		createOpts, err := newCreateConfig(ctx, &mScope, logger)
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
})
