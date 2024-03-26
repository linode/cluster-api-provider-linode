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
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	rutil "github.com/linode/cluster-api-provider-linode/util/reconciler"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("create", Label("machine", "create"), func() {
	var linodeMachine infrav1alpha1.LinodeMachine
	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer
	var logger logr.Logger

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock",
			Namespace: "default",
		},
	}

	machine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.MachineControlPlaneLabel: "true",
			},
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: ptr.To("bootstrap-secret"),
			},
		},
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("userdata"),
		},
	}

	linodeCluster := infrav1alpha1.LinodeCluster{
		Spec: infrav1alpha1.LinodeClusterSpec{
			Network: infrav1alpha1.NetworkSpec{
				NodeBalancerID:       ptr.To(1),
				NodeBalancerConfigID: ptr.To(2),
			},
		},
	}

	recorder := record.NewFakeRecorder(10)
	reconciler := &LinodeMachineReconciler{
		Recorder: recorder,
	}

	BeforeEach(func() {
		linodeMachine = infrav1alpha1.LinodeMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
				UID:       "12345",
			},
			Spec: infrav1alpha1.LinodeMachineSpec{
				InstanceID: ptr.To(0),
				Type:       "g6-nanode-1",
				Image:      rutil.DefaultMachineControllerLinodeImage,
			},
		}
		mockCtrl = gomock.NewController(GinkgoT())
		testLogs = &bytes.Buffer{}
		logger = zap.New(
			zap.WriteTo(GinkgoWriter),
			zap.WriteTo(testLogs),
			zap.UseDevMode(true),
		)
	})

	AfterEach(func() {
		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("creates an instance", func(ctx SpecContext) {
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		mockLinodeClient := mock.NewMockLinodeMachineClient(mockCtrl)
		listInst := mockLinodeClient.EXPECT().
			ListInstances(ctx, gomock.Any()).
			Return([]linodego.Instance{}, nil).
			Times(1)
		createInst := mockLinodeClient.EXPECT().
			CreateInstance(ctx, gomock.Cond(func(opts any) bool {
				createOpts, ok := opts.(linodego.InstanceCreateOptions)
				if !ok {
					return false
				}
				return createOpts.Image == ""
			})).
			After(listInst).
			Return(&linodego.Instance{
				ID:   123,
				IPv4: []*net.IP{ptr.To(net.IPv4(192, 168, 0, 2))},
			}, nil).
			Times(1)
		getType := mockLinodeClient.EXPECT().
			GetType(ctx, linodeMachine.Spec.Type).
			After(createInst).
			Return(&linodego.LinodeType{Disk: 25600}, nil).
			Times(1)
		createRootDisk := mockLinodeClient.EXPECT().
			CreateInstanceDisk(ctx, 123, gomock.Cond(func(opts any) bool {
				createOpts, ok := opts.(linodego.InstanceDiskCreateOptions)
				if !ok {
					return false
				}
				return createOpts.Label == "root" && createOpts.Size == 25600-defaultEtcdDiskSize
			})).
			After(getType).
			Return(&linodego.InstanceDisk{ID: 100}, nil).
			Times(1)
		createEtcdDisk := mockLinodeClient.EXPECT().
			CreateInstanceDisk(ctx, 123, gomock.Cond(func(opts any) bool {
				createOpts, ok := opts.(linodego.InstanceDiskCreateOptions)
				if !ok {
					return false
				}
				return createOpts.Label == "etcd-data"
			})).
			After(createRootDisk).
			Return(&linodego.InstanceDisk{ID: 101}, nil).
			Times(1)
		createInstConf := mockLinodeClient.EXPECT().
			CreateInstanceConfig(ctx, 123, linodego.InstanceConfigCreateOptions{
				Label: fmt.Sprintf("%s disk profile", linodeMachine.Spec.Image),
				Devices: linodego.InstanceConfigDeviceMap{
					SDA: &linodego.InstanceConfigDevice{DiskID: 100},
					SDB: &linodego.InstanceConfigDevice{DiskID: 101},
				},
				Helpers: &linodego.InstanceConfigHelpers{
					UpdateDBDisabled:  true,
					Distro:            true,
					ModulesDep:        true,
					Network:           true,
					DevTmpFsAutomount: true,
				},
				Kernel: "linode/grub2",
			}).
			After(createEtcdDisk).
			Return(nil, nil).
			Times(1)
		bootInst := mockLinodeClient.EXPECT().
			BootInstance(ctx, 123, 0).
			After(createInstConf).
			Return(nil).
			Times(1)
		getAddrs := mockLinodeClient.EXPECT().
			GetInstanceIPAddresses(ctx, 123).
			After(bootInst).
			Return(&linodego.InstanceIPAddressResponse{
				IPv4: &linodego.InstanceIPv4Response{
					Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
				},
			}, nil).
			Times(1)
		mockLinodeClient.EXPECT().
			CreateNodeBalancerNode(ctx, 1, 2, linodego.NodeBalancerNodeCreateOptions{
				Label:   "mock",
				Address: "192.168.0.2:6443",
				Mode:    linodego.ModeAccept,
			}).
			After(getAddrs).
			Return(nil, nil).
			Times(1)

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

		Expect(linodeMachine.Status.Ready).To(BeTrue())
		Expect(*linodeMachine.Spec.InstanceID).To(Equal(123))
		Expect(*linodeMachine.Spec.ProviderID).To(Equal("linode://123"))
		Expect(linodeMachine.Status.Addresses).To(Equal([]clusterv1.MachineAddress{{
			Type:    clusterv1.MachineInternalIP,
			Address: "192.168.0.2",
		}}))

		Expect(testLogs.String()).To(ContainSubstring("creating machine"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to list Linode machine instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Linode instance already exists"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine InstanceCreateOptions"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to create Linode machine instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to boot instance"))
		Expect(testLogs.String()).NotTo(ContainSubstring("multiple instances found"))
		Expect(testLogs.String()).NotTo(ContainSubstring("Failed to add instance to Node Balancer backend"))
	})
})
