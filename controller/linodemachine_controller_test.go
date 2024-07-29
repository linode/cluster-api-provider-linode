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
				InstanceID:     ptr.To(0),
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
		listInst := mockLinodeClient.EXPECT().
			ListInstances(ctx, gomock.Any()).
			Return([]linodego.Instance{}, nil)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			After(listInst).
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
			}, nil).AnyTimes()
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

		_, err := reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightReady)).To(BeTrue())

		Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
		Expect(*linodeMachine.Spec.InstanceID).To(Equal(123))
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
			listInst := mockLinodeClient.EXPECT().
				ListInstances(ctx, gomock.Any()).
				Return([]linodego.Instance{}, nil)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				After(listInst).
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

			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

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
			listInst := mockLinodeClient.EXPECT().
				ListInstances(ctx, gomock.Any()).
				Return([]linodego.Instance{}, nil)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				After(listInst).
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
			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

			res, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerLinodeErrorRetryDelay))
		})
	})

	Context("creates a instance with disks", func() {
		It("in a single call when disks aren't delayed", func(ctx SpecContext) {
			machine.Labels[clusterv1.MachineControlPlaneLabel] = "true"
			linodeMachine.Spec.DataDisks = map[string]*infrav1alpha2.InstanceDisk{"sdb": ptr.To(infrav1alpha2.InstanceDisk{Label: "etcd-data", Size: resource.MustParse("10Gi")})}

			mockLinodeClient := mock.NewMockLinodeClient(mockCtrl)
			listInst := mockLinodeClient.EXPECT().
				ListInstances(ctx, gomock.Any()).
				Return([]linodego.Instance{}, nil)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				After(listInst).
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
			listInstConfs := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createInst).
				Return([]linodego.InstanceConfig{{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).AnyTimes()
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
				}}, nil).AnyTimes()
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
				}, nil).AnyTimes()
			createNB := mockLinodeClient.EXPECT().
				CreateNodeBalancerNode(ctx, 1, 2, linodego.NodeBalancerNodeCreateOptions{
					Label:   "mock",
					Address: "192.168.0.2:6443",
					Mode:    linodego.ModeAccept,
				}).
				After(getAddrs).
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
				}, nil).AnyTimes()
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

			_, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(err).NotTo(HaveOccurred())

			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightReady)).To(BeTrue())

			Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
			Expect(*linodeMachine.Spec.InstanceID).To(Equal(123))
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
			listInst := mockLinodeClient.EXPECT().
				ListInstances(ctx, gomock.Any()).
				Return([]linodego.Instance{}, nil)
			getRegion := mockLinodeClient.EXPECT().
				GetRegion(ctx, gomock.Any()).
				After(listInst).
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
			listInstConfs := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createInst).
				Return([]linodego.InstanceConfig{{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).AnyTimes()
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
				Return(nil, linodego.Error{Code: 400})

			mScope := scope.MachineScope{
				Client:        k8sClient,
				LinodeClient:  mockLinodeClient,
				Cluster:       &cluster,
				Machine:       &machine,
				LinodeCluster: &linodeCluster,
				LinodeMachine: &linodeMachine,
			}

			res, err := reconciler.reconcileCreate(ctx, logger, &mScope)
			Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerWaitForRunningDelay))
			Expect(err).ToNot(HaveOccurred())

			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
			Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeFalse())

			listInst = mockLinodeClient.EXPECT().
				ListInstances(ctx, gomock.Any()).
				After(createFailedEtcdDisk).
				Return([]linodego.Instance{{
					ID:     123,
					IPv4:   []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
					IPv6:   "fd00::",
					Status: linodego.InstanceOffline,
				}}, nil)
			createEtcdDisk := mockLinodeClient.EXPECT().
				CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
					Label:      "etcd-data",
					Size:       10738,
					Filesystem: string(linodego.FilesystemExt4),
				}).
				After(listInst).
				Return(&linodego.InstanceDisk{ID: 101}, nil)
			listInstConfsForProfile := mockLinodeClient.EXPECT().
				ListInstanceConfigs(ctx, 123, gomock.Any()).
				After(createEtcdDisk).
				Return([]linodego.InstanceConfig{{
					Devices: &linodego.InstanceConfigDeviceMap{
						SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					},
				}}, nil).AnyTimes()
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
				}, nil).AnyTimes()
			createNB := mockLinodeClient.EXPECT().
				CreateNodeBalancerNode(ctx, 1, 2, linodego.NodeBalancerNodeCreateOptions{
					Label:   "mock",
					Address: "192.168.0.2:6443",
					Mode:    linodego.ModeAccept,
				}).
				After(getAddrs).
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
				}, nil).AnyTimes()
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
			Expect(*linodeMachine.Spec.InstanceID).To(Equal(123))
			Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
			Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{
				{Type: clusterv1.MachineExternalIP, Address: "172.0.0.2"},
				{Type: clusterv1.MachineExternalIP, Address: "fd00::"},
				{Type: clusterv1.MachineInternalIP, Address: "10.0.0.2"},
				{Type: clusterv1.MachineInternalIP, Address: "192.168.0.2"},
			}))

			Expect(testLogs.String()).To(ContainSubstring("creating machine"))
			Expect(testLogs.String()).To(ContainSubstring("Linode instance already exists"))
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
				InstanceID: ptr.To(0),
				Type:       "g6-nanode-1",
				Image:      rutil.DefaultMachineControllerLinodeImage,
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
		listInst := mockLinodeClient.EXPECT().
			ListInstances(ctx, gomock.Any()).
			Return([]linodego.Instance{}, nil)
		getRegion := mockLinodeClient.EXPECT().
			GetRegion(ctx, gomock.Any()).
			After(listInst).
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
			}, nil).AnyTimes()
		mockLinodeClient.EXPECT().
			ListInstanceConfigs(ctx, 123, gomock.Any()).
			After(getAddrs).
			Return([]linodego.InstanceConfig{{
				Devices: &linodego.InstanceConfigDeviceMap{
					SDA: &linodego.InstanceConfigDevice{DiskID: 100},
				},
			}}, nil)

		mScope := scope.MachineScope{
			Client:              k8sClient,
			LinodeClient:        mockLinodeClient,
			LinodeDomainsClient: mockLinodeClient,
			Cluster:             &cluster,
			Machine:             &machine,
			LinodeCluster:       &linodeCluster,
			LinodeMachine:       &linodeMachine,
		}

		_, err := reconciler.reconcileCreate(ctx, logger, &mScope)
		Expect(err).NotTo(HaveOccurred())

		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightCreated)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightConfigured)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightBootTriggered)).To(BeTrue())
		Expect(rutil.ConditionTrue(&linodeMachine, ConditionPreflightReady)).To(BeTrue())

		Expect(*linodeMachine.Status.InstanceState).To(Equal(linodego.InstanceOffline))
		Expect(*linodeMachine.Spec.InstanceID).To(Equal(123))
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
	linodeMachine := &infrav1alpha2.LinodeMachine{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeMachineSpec{
			InstanceID:    ptr.To(0),
			Type:          "g6-nanode-1",
			Image:         rutil.DefaultMachineControllerLinodeImage,
			Configuration: &infrav1alpha2.InstanceConfiguration{Kernel: "test"},
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
		Expect(k8sClient.Create(ctx, linodeCluster)).To(Succeed())
		Expect(k8sClient.Create(ctx, linodeMachine)).To(Succeed())
		_ = k8sClient.Create(ctx, secret)
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()

		Expect(k8sClient.Get(ctx, machineKey, linodeMachine)).To(Succeed())
		mScope.LinodeMachine = linodeMachine

		machinePatchHelper, err := patch.NewHelper(linodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = machinePatchHelper

		Expect(k8sClient.Get(ctx, clusterKey, linodeCluster)).To(Succeed())
		mScope.LinodeCluster = linodeCluster

		mScope.LinodeClient = mck.LinodeClient
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("machine is not created because there was an error creating instance", func(ctx context.Context, mck Mock) {
					listInst := mck.LinodeClient.EXPECT().
						ListInstances(ctx, gomock.Any()).
						Return([]linodego.Instance{}, nil)
					getRegion := mck.LinodeClient.EXPECT().
						GetRegion(ctx, gomock.Any()).
						After(listInst).
						Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
					getImage := mck.LinodeClient.EXPECT().
						GetImage(ctx, gomock.Any()).
						After(getRegion).
						Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
					mck.LinodeClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).
						After(getImage).
						Return(nil, errors.New("failed to ensure instance"))
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerWaitForRunningDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to create Linode machine instance"))
					})),
					Path(Result("create machine error - timeout error", func(ctx context.Context, mck Mock) {
						tempTimeout := reconciler.ReconcileTimeout
						reconciler.ReconcileTimeout = time.Nanosecond
						_, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("failed to ensure instance"))
						reconciler.ReconcileTimeout = tempTimeout
					})),
				),
			),
			Path(
				Call("machine is not created because there were too many requests", func(ctx context.Context, mck Mock) {
					listInst := mck.LinodeClient.EXPECT().
						ListInstances(ctx, gomock.Any()).
						Return([]linodego.Instance{}, nil)
					mck.LinodeClient.EXPECT().
						GetRegion(ctx, gomock.Any()).
						After(listInst).
						Return(&linodego.Region{Capabilities: []string{"Metadata"}}, nil)
				}),
				OneOf(
					Path(Result("create requeues when failing to create instance config", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
							Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerLinodeErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to create Linode machine InstanceCreateOptions"))
					})),
					Path(Result("create requeues when failing to create instance", func(ctx context.Context, mck Mock) {
						getImage := mck.LinodeClient.EXPECT().
							GetImage(ctx, gomock.Any()).
							Return(&linodego.Image{Capabilities: []string{"cloud-init"}}, nil)
						mck.LinodeClient.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).
							After(getImage).
							Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerLinodeErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to create Linode instance due to API error"))
					})),
					Path(Result("create requeues when failing to update instance config", func(ctx context.Context, mck Mock) {
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
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerLinodeErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to update default instance configuration"))
					})),
					Path(Result("create requeues when failing to get instance config", func(ctx context.Context, mck Mock) {
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
						updateInstConfig := mck.LinodeClient.EXPECT().
							UpdateInstanceConfig(ctx, 123, 0, gomock.Any()).
							After(createInst).
							Return(nil, nil).AnyTimes()
						getAddrs := mck.LinodeClient.EXPECT().
							GetInstanceIPAddresses(ctx, 123).
							After(updateInstConfig).
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
							}, nil).AnyTimes()
						mck.LinodeClient.EXPECT().
							ListInstanceConfigs(ctx, 123, gomock.Any()).
							After(getAddrs).
							Return(nil, &linodego.Error{Code: http.StatusTooManyRequests})
						res, err := reconciler.reconcile(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerLinodeErrorRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("Failed to get default instance configuration"))
					})),
				),
			),
			Path(
				Call("machine is created", func(ctx context.Context, mck Mock) {
					linodeMachine.Spec.Configuration = nil
				}),
				OneOf(
					Path(Result("creates a worker machine without disks", func(ctx context.Context, mck Mock) {
						listInst := mck.LinodeClient.EXPECT().
							ListInstances(ctx, gomock.Any()).
							Return([]linodego.Instance{}, nil)
						getRegion := mck.LinodeClient.EXPECT().
							GetRegion(ctx, gomock.Any()).
							After(listInst).
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
							}, nil).AnyTimes()
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
						Expect(*linodeMachine.Spec.InstanceID).To(Equal(123))
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
	instanceID := 12345
	linodeMachine := &infrav1alpha2.LinodeMachine{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeMachineSpec{
			InstanceID: &instanceID,
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
	}

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
		mScope.LinodeMachine = linodeMachine
		machinePatchHelper, err := patch.NewHelper(linodeMachine, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		mScope.PatchHelper = machinePatchHelper
		mScope.LinodeCluster = linodeCluster
		mScope.LinodeClient = mck.LinodeClient
		reconciler.Client = mck.K8sClient
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("machine is not deleted because there was an error deleting instance", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().DeleteInstance(gomock.Any(), gomock.Any()).
						Return(errors.New("failed to delete instance"))
				}),
				OneOf(
					Path(Result("delete requeues", func(ctx context.Context, mck Mock) {
						res, err := reconciler.reconcileDelete(ctx, mck.Logger(), mScope)
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rutil.DefaultMachineControllerRetryDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing Linode instance deletion"))
					})),
					Path(Result("create machine error - timeout error", func(ctx context.Context, mck Mock) {
						tempTimeout := reconciler.ReconcileTimeout
						reconciler.ReconcileTimeout = time.Nanosecond
						_, err := reconciler.reconcileDelete(ctx, mck.Logger(), mScope)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("failed to delete instance"))
						reconciler.ReconcileTimeout = tempTimeout
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
	var reconciler *LinodeMachineReconciler
	var lpgReconciler *LinodePlacementGroupReconciler
	var linodePlacementGroup infrav1alpha2.LinodePlacementGroup

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

		linodeMachine = infrav1alpha2.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: defaultNamespace,
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeMachineSpec{
				InstanceID: ptr.To(0),
				Type:       "g6-nanode-1",
				Image:      rutil.DefaultMachineControllerLinodeImage,
				PlacementGroupRef: &corev1.ObjectReference{
					Namespace: defaultNamespace,
					Name:      "test-pg",
				},
			},
		}

		lpgReconciler = &LinodePlacementGroupReconciler{
			Recorder: recorder,
			Client:   k8sClient,
		}

		reconciler = &LinodeMachineReconciler{
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

	It("creates a instance in a PlacementGroup", func(ctx SpecContext) {
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
			Client:              k8sClient,
			LinodeClient:        mockLinodeClient,
			LinodeDomainsClient: mockLinodeClient,
			Cluster:             &cluster,
			Machine:             &machine,
			LinodeCluster:       &linodeCluster,
			LinodeMachine:       &linodeMachine,
		}

		createOpts, err := reconciler.newCreateConfig(ctx, &mScope, []string{}, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(createOpts).NotTo(BeNil())
		Expect(createOpts.PlacementGroup.ID).To(Equal(1))
	})

})
