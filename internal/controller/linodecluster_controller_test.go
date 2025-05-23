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

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
	rec "github.com/linode/cluster-api-provider-linode/util/reconciler"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cluster-lifecycle", Ordered, Label("cluster", "cluster-lifecycle"), func() {
	nodebalancerID := 1
	nbConfigID := util.Pointer(3)
	controlPlaneEndpointHost := "10.0.0.1"
	controlPlaneEndpointPort := 6443
	clusterName := "cluster-lifecycle"
	ownerRef := metav1.OwnerReference{
		Name:       clusterName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            clusterName,
		Namespace:       defaultNamespace,
		OwnerReferences: ownerRefs,
	}
	linodeCluster := infrav1alpha2.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			VPCRef: &corev1.ObjectReference{Name: "vpctest", Namespace: defaultNamespace},
		},
	}
	linodeVPC := infrav1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vpctest",
			Namespace: defaultNamespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: linodeCluster.Name,
			},
		},
		Spec: infrav1alpha2.LinodeVPCSpec{
			Region: "us-ord",
			Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
				{Label: "subnet1", IPv4: "10.0.0.0/8"},
			},
		},
	}
	linodeFirewall := infrav1alpha2.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewalltest",
			Namespace: defaultNamespace,
		},
		Spec: infrav1alpha2.LinodeFirewallSpec{
			FirewallID: util.Pointer(123), // Test firewall ID
		},
	}

	ctlrSuite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})
	reconciler := LinodeClusterReconciler{}
	cScope := &scope.ClusterScope{}
	clusterKey := client.ObjectKeyFromObject(&linodeCluster)

	BeforeAll(func(ctx SpecContext) {
		cScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()

		Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
		cScope.LinodeCluster = &linodeCluster

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&linodeCluster, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		cScope.PatchHelper = patchHelper
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("vpc present but not ready", func(ctx context.Context, mck Mock) {
					Expect(k8sClient.Create(ctx, &linodeVPC)).To(Succeed())
					linodeVPC.Status.Ready = false
					k8sClient.Status().Update(ctx, &linodeVPC)
				}),
				OneOf(
					Path(Result("", func(ctx context.Context, mck Mock) {
						reconciler.Client = k8sClient
						// first for pause reconciliation
						_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						// second for real
						_, err = reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						Expect(rec.ConditionTrue(&linodeCluster, ConditionPreflightLinodeVPCReady)).To(BeFalse())
					})),
				),
			),
			Path(
				Call("firewall doesn't exist", func(ctx context.Context, mck Mock) {
					cScope.LinodeCluster.Spec.NodeBalancerFirewallRef = &corev1.ObjectReference{Name: "firewalltest"}
				}),
				Result("", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					// first reconcile is for pause
					_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
					Expect(err).NotTo(HaveOccurred())

					// second reconcile is for real
					_, err = reconciler.reconcile(ctx, cScope, mck.Logger())
					Expect(err).NotTo(HaveOccurred())
					Expect(rec.ConditionTrue(&linodeCluster, ConditionPreflightLinodeNBFirewallReady)).To(BeFalse())
				}),
			),
			Path(
				Call("firewall present but not ready", func(ctx context.Context, mck Mock) {
					cScope.LinodeCluster.Spec.NodeBalancerFirewallRef = &corev1.ObjectReference{Name: "firewalltest"}
					Expect(k8sClient.Create(ctx, &linodeFirewall)).To(Succeed())
					linodeFirewall.Spec.FirewallID = nil
					k8sClient.Update(ctx, &linodeFirewall)
				}),
				Result("", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
					Expect(err).NotTo(HaveOccurred())
					Expect(rec.ConditionTrue(&linodeCluster, ConditionPreflightLinodeNBFirewallReady)).To(BeFalse())
				}),
			),
			Path(
				Call("vpc doesn't exist", func(ctx context.Context, mck Mock) {
				}),
				OneOf(
					Path(Result("", func(ctx context.Context, mck Mock) {
						reconciler.Client = k8sClient
						_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						Expect(rec.ConditionTrue(&linodeCluster, ConditionPreflightLinodeVPCReady)).To(BeFalse())
					})),
				),
			),
			Path(
				Call("cluster is not created because there was an error creating nb", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					// Set VPC as ready
					linodeVPC.Status.Ready = true
					Expect(k8sClient.Status().Update(ctx, &linodeVPC)).To(Succeed())

					// Create and mark firewall as ready if using firewall ref
					if cScope.LinodeCluster.Spec.NodeBalancerFirewallRef != nil {
						linodeFirewall.Spec.FirewallID = util.Pointer(123)
						k8sClient.Update(ctx, &linodeFirewall)
					}

					// If using direct firewall ID
					if cScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID != nil {
						// Mock GetFirewall call for direct ID reference
						mck.LinodeClient.EXPECT().GetFirewall(gomock.Any(), *cScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID).
							Return(&linodego.Firewall{ID: *cScope.LinodeCluster.Spec.Network.NodeBalancerFirewallID}, nil)
					}

					// Mock the NodeBalancer creation failure
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to ensure nodebalancer"))
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						reconciler.Client = k8sClient
						res, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultClusterControllerReconcileDelay))
						Expect(mck.Logs()).To(Or(
							ContainSubstring("re-queuing cluster/load-balancer creation"),
							ContainSubstring("failed to ensure nodebalancer"),
						))
					})),
				),
			),
			Path(
				Call("cluster is not created because nb was nil", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient

					// Mock CreateNodeBalancer to return nil
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						Return(nil, nil)
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						reconciler.Client = k8sClient
						res, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultClusterControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing cluster/load-balancer creation"))
					})),
				),
			),
			Path(
				Call("cluster is not created because nb config was nil", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient

					// Mock CreateNodeBalancerConfig to return nil
					mck.LinodeClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("nodeBalancer config created was nil"))
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
							Return(&linodego.NodeBalancer{
								ID:   nodebalancerID,
								IPv4: &controlPlaneEndpointHost,
							}, nil)
						reconciler.Client = k8sClient
						res, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultClusterControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing cluster/load-balancer creation"))
					})),
				),
			),
			Path(
				Call("cluster is not created because there was an error getting nb config", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					reconciler.Client = k8sClient
					cScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID = nbConfigID
					mck.LinodeClient.EXPECT().GetNodeBalancer(gomock.Any(), gomock.Any()).
						Return(&linodego.NodeBalancer{
							ID:   nodebalancerID,
							IPv4: &controlPlaneEndpointHost,
						}, nil)
					mck.LinodeClient.EXPECT().GetNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to get nodebalancer config"))
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						reconciler.Client = k8sClient
						res, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultClusterControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing cluster/load-balancer creation"))
					})),
				),
			),
			Path(
				Call("cluster is not created because there is no capl cluster", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
				}),
				Result("no capl cluster error", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(cScope.LinodeCluster),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(linodeCluster.Status.Ready).To(BeFalseBecause("failed to get Cluster/no-capl-cluster: clusters.cluster.x-k8s.io \"no-capl-cluster\" not found"))
				}),
			),
			Path(
				Call("cluster is created", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					cScope.LinodeCluster.Spec.Network.ApiserverNodeBalancerConfigID = nil
					mck.LinodeClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.NodeBalancerNode{}, nil).AnyTimes()
					getNB := mck.LinodeClient.EXPECT().GetNodeBalancer(gomock.Any(), gomock.Any()).
						Return(&linodego.NodeBalancer{
							ID:   nodebalancerID,
							IPv4: &controlPlaneEndpointHost,
						}, nil)
					mck.LinodeClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).After(getNB).Return(&linodego.NodeBalancerConfig{
						Port:           controlPlaneEndpointPort,
						Protocol:       linodego.ProtocolTCP,
						Algorithm:      linodego.AlgorithmRoundRobin,
						Check:          linodego.CheckConnection,
						NodeBalancerID: nodebalancerID,
					}, nil)
				}),
				Result("cluster created", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err).NotTo(HaveOccurred())

					By("checking ready conditions")
					clusterKey := client.ObjectKeyFromObject(&linodeCluster)
					Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
					Expect(linodeCluster.Status.Ready).To(BeTrue())
					Expect(linodeCluster.Status.Conditions).To(HaveLen(3))
					Expect(conditions.Get(&linodeCluster, string(clusterv1.ReadyCondition)).Status).To(Equal(metav1.ConditionTrue))
					Expect(conditions.Get(&linodeCluster, ConditionPreflightLinodeNBFirewallReady)).NotTo(BeNil())
					Expect(conditions.Get(&linodeCluster, ConditionPreflightLinodeVPCReady)).NotTo(BeNil())
					By("checking NB id")
					Expect(linodeCluster.Spec.Network.NodeBalancerID).To(Equal(&nodebalancerID))

					By("checking controlPlaneEndpoint/NB host and port")
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(controlPlaneEndpointHost))
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(controlPlaneEndpointPort)))
				}),
			),
		),
	)
})

var _ = Describe("cluster-lifecycle-dns", Ordered, Label("cluster", "cluster-lifecycle-dns"), func() {
	controlPlaneEndpointHost := "cluster-lifecycle-dns-abc123.lkedevs.net"
	controlPlaneEndpointPort := 1000
	clusterName := "cluster-lifecycle-dns"
	ownerRef := metav1.OwnerReference{
		Name:       clusterName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            clusterName,
		Namespace:       defaultNamespace,
		OwnerReferences: ownerRefs,
	}

	linodeMachine := infrav1alpha2.LinodeMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName + "-control-plane",
			Namespace: defaultNamespace,
			UID:       "12345",
		},
		Spec: infrav1alpha2.LinodeMachineSpec{
			Type:           "g6-nanode-1",
			Image:          rec.DefaultMachineControllerLinodeImage,
			DiskEncryption: string(linodego.InstanceDiskEncryptionEnabled),
		},
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType:          "dns",
				DNSRootDomain:             "lkedevs.net",
				DNSUniqueIdentifier:       "abc123",
				DNSTTLSec:                 30,
				ApiserverLoadBalancerPort: controlPlaneEndpointPort,
			},
		},
	}

	ctlrSuite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})
	reconciler := LinodeClusterReconciler{}
	cScope := &scope.ClusterScope{}
	clusterKey := client.ObjectKeyFromObject(&linodeCluster)

	BeforeAll(func(ctx SpecContext) {
		cScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
		Expect(k8sClient.Create(ctx, &linodeMachine)).To(Succeed())
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()

		Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
		cScope.LinodeCluster = &linodeCluster

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&linodeCluster, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		cScope.PatchHelper = patchHelper
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("cluster with dns loadbalancing is created", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					cScope.LinodeDomainsClient = mck.LinodeClient
					mck.LinodeClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
						{
							ID:     1,
							Domain: "lkedevs.net",
						},
					}, nil).AnyTimes()
					mck.LinodeClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{
						{
							ID:     1234,
							Type:   "A",
							Name:   "test-cluster",
							TTLSec: 30,
						},
					}, nil).AnyTimes()
					mck.LinodeClient.EXPECT().DeleteDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				}),
				Result("cluster created", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err).NotTo(HaveOccurred())

					// Once more for pause
					_, err = reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err).NotTo(HaveOccurred())
					By("checking ready conditions")
					clusterKey := client.ObjectKeyFromObject(&linodeCluster)
					Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
					Expect(linodeCluster.Status.Ready).To(BeTrue())
					Expect(linodeCluster.Status.Conditions).To(HaveLen(1))
					readyCond := conditions.Get(&linodeCluster, string(clusterv1.ReadyCondition))
					Expect(readyCond).NotTo(BeNil())

					By("checking controlPlaneEndpoint/NB host and port")
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(controlPlaneEndpointHost))
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(controlPlaneEndpointPort)))
				}),
			),
		),
	)
})

var _ = Describe("cluster-delete", Ordered, Label("cluster", "cluster-delete"), func() {
	nodebalancerID := 1
	clusterName := "cluster-delete"
	ownerRef := metav1.OwnerReference{
		Name:       clusterName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            clusterName,
		Namespace:       defaultNamespace,
		OwnerReferences: ownerRefs,
	}

	linodeCluster := infrav1alpha2.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				LoadBalancerType: "NodeBalancer",
				NodeBalancerID:   &nodebalancerID,
			},
		},
	}

	linodeMachine := infrav1alpha2.LinodeMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine",
			Namespace: defaultNamespace,
		},
		Spec: infrav1alpha2.LinodeMachineSpec{
			ProviderID: ptr.To("linode://123"),
		},
		Status: infrav1alpha2.LinodeMachineStatus{
			Addresses: []clusterv1.MachineAddress{},
		},
	}

	ctlrSuite := NewControllerSuite(
		GinkgoT(),
		mock.MockLinodeClient{},
		mock.MockK8sClient{},
	)
	reconciler := LinodeClusterReconciler{}

	cScope := &scope.ClusterScope{
		LinodeCluster: &linodeCluster,
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metadata,
		},
	}

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("cluster with vlan is deleted", func(ctx context.Context, mck Mock) {
					cScope.LinodeCluster.Spec.Network.UseVlan = true
					cScope.LinodeClient = mck.LinodeClient
					cScope.Client = mck.K8sClient
					mck.LinodeClient.EXPECT().DeleteNodeBalancer(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				}),
				Result("cluster with vlan deleted", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					err := reconciler.reconcileDelete(ctx, logr.Logger{}, cScope)
					Expect(err).NotTo(HaveOccurred())
				}),
			),
			Path(
				Call("cluster is deleted", func(ctx context.Context, mck Mock) {
					cScope.LinodeCluster.Spec.Network.UseVlan = false
					cScope.LinodeClient = mck.LinodeClient
					cScope.Client = mck.K8sClient
					mck.LinodeClient.EXPECT().DeleteNodeBalancer(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				}),
				Result("cluster deleted", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					err := reconciler.reconcileDelete(ctx, logr.Logger{}, cScope)
					Expect(err).NotTo(HaveOccurred())
				}),
			),
			Path(
				Call("nothing to do because NB ID is nil", func(ctx context.Context, mck Mock) {
					cScope.Client = mck.K8sClient
					cScope.LinodeClient = mck.LinodeClient
					cScope.LinodeCluster.Spec.Network.NodeBalancerID = nil
				}),
				Result("nothing to do because NB ID is nil", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					err := reconciler.reconcileDelete(ctx, logr.Logger{}, cScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(mck.Events()).To(ContainSubstring("Warning NodeBalancerIDMissing NodeBalancer already removed, nothing to do"))
				}),
			),
			Path(
				Call("cluster not deleted because the nb can't be deleted", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					cScope.Client = mck.K8sClient
					cScope.LinodeCluster.Spec.Network.NodeBalancerID = &nodebalancerID
					mck.LinodeClient.EXPECT().DeleteNodeBalancer(gomock.Any(), gomock.Any()).Return(errors.New("delete NB error")).AnyTimes()
				}),
				Result("cluster not deleted because the nb can't be deleted", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					err := reconciler.reconcileDelete(ctx, logr.Logger{}, cScope)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("delete NB error"))
				}),
			),
			Path(
				Call("cluster not deleted because some LinodeMachines are yet to be deleted and NB present", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					cScope.Client = mck.K8sClient
					cScope.LinodeCluster.Spec.Network.NodeBalancerID = &nodebalancerID
					mck.LinodeClient.EXPECT().DeleteNodeBalancer(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
					cScope.LinodeMachines = infrav1alpha2.LinodeMachineList{
						Items: []infrav1alpha2.LinodeMachine{linodeMachine},
					}
				}),
				Result("cluster not deleted because some LinodeMachines are yet to be deleted and NB present", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					err := reconciler.reconcileDelete(ctx, logr.Logger{}, cScope)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("waiting for associated LinodeMachine objects to be deleted"))
				}),
			),
			Path(
				Call("cluster not deleted because some LinodeMachines are yet to be deleted and NB nil", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					cScope.Client = mck.K8sClient
					cScope.LinodeCluster.Spec.Network.NodeBalancerID = nil
					cScope.LinodeMachines = infrav1alpha2.LinodeMachineList{
						Items: []infrav1alpha2.LinodeMachine{linodeMachine},
					}
				}),
				Result("cluster not deleted because some LinodeMachines are yet to be deleted and NB nil", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					err := reconciler.reconcileDelete(ctx, logr.Logger{}, cScope)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("waiting for associated LinodeMachine objects to be deleted"))
				}),
			),
		),
	)
})

var _ = Describe("dns-override-endpoint", Ordered, Label("cluster", "dns-override-endpoint"), func() {
	subDomainOverRide := "dns-override-endpoint"
	controlPlaneEndpointHost := "dns-override-endpoint.lkedevs.net"
	controlPlaneEndpointPort := 1000
	clusterName := "dns-override-endpoint"
	ownerRef := metav1.OwnerReference{
		Name:       clusterName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            clusterName,
		Namespace:       defaultNamespace,
		OwnerReferences: ownerRefs,
	}
	cluster := clusterv1.Cluster{
		ObjectMeta: metadata,
	}
	linodeCluster := infrav1alpha2.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1alpha2.NetworkSpec{
				ApiserverLoadBalancerPort: controlPlaneEndpointPort,
				LoadBalancerType:          "dns",
				DNSSubDomainOverride:      subDomainOverRide,
				DNSRootDomain:             "lkedevs.net",
			},
		},
	}
	linodeMachine := infrav1alpha2.LinodeMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine",
			Namespace: defaultNamespace,
		},
		Spec: infrav1alpha2.LinodeMachineSpec{
			ProviderID: ptr.To("linode://123"),
		},
		Status: infrav1alpha2.LinodeMachineStatus{
			Addresses: []clusterv1.MachineAddress{},
		},
	}

	ctlrSuite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})
	reconciler := LinodeClusterReconciler{}
	cScope := &scope.ClusterScope{}
	clusterKey := client.ObjectKeyFromObject(&linodeCluster)

	BeforeAll(func(ctx SpecContext) {
		cScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &cluster)).To(Succeed())
		Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()

		Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
		cScope.Cluster = &cluster
		cScope.LinodeCluster = &linodeCluster

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&linodeCluster, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		cScope.PatchHelper = patchHelper
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("cluster with dns loadbalancing is created", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					cScope.LinodeDomainsClient = mck.LinodeClient
					cScope.AkamaiDomainsClient = mck.AkamEdgeDNSClient
					linodeMachines := infrav1alpha2.LinodeMachineList{
						Items: []infrav1alpha2.LinodeMachine{linodeMachine},
					}
					Expect(k8sClient.Create(ctx, &linodeMachine)).To(Succeed())
					cScope.LinodeMachines = linodeMachines
					mck.LinodeClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return([]linodego.Domain{
						{
							ID:     1,
							Domain: "lkedevs.net",
						},
					}, nil).AnyTimes()
					mck.LinodeClient.EXPECT().ListDomainRecords(gomock.Any(), gomock.Any(), gomock.Any()).Return([]linodego.DomainRecord{
						{
							ID:     1234,
							Type:   "A",
							Name:   "test-cluster",
							TTLSec: 30,
						},
					}, nil).AnyTimes()
					mck.LinodeClient.EXPECT().DeleteDomainRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				}),
				Result("cluster created", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err).NotTo(HaveOccurred())

					// once more for pause
					_, err = reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err).NotTo(HaveOccurred())

					By("checking ready conditions")
					clusterKey := client.ObjectKeyFromObject(&linodeCluster)
					Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
					Expect(linodeCluster.Status.Ready).To(BeTrue())
					Expect(linodeCluster.Status.Conditions).To(HaveLen(1))
					cond := conditions.Get(&linodeCluster, string(clusterv1.ReadyCondition))
					Expect(cond).NotTo(BeNil())

					By("checking controlPlaneEndpoint/NB host and port")
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(controlPlaneEndpointHost))
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(controlPlaneEndpointPort)))
				}),
			),
		),
	)
})

var _ = Describe("cluster-with-direct-vpcid", Ordered, Label("cluster", "direct-vpcid"), func() {
	clusterName := "cluster-with-direct-vpcid"
	ownerRef := metav1.OwnerReference{
		Name:       clusterName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		UID:        "00000000-000-0000-0000-000000000001",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            clusterName,
		Namespace:       defaultNamespace,
		OwnerReferences: ownerRefs,
	}
	linodeCluster := infrav1alpha2.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1alpha2.LinodeClusterSpec{
			Region: "us-ord",
			VPCID:  ptr.To(12345),
		},
	}

	ctlrSuite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})
	reconciler := LinodeClusterReconciler{}
	cScope := &scope.ClusterScope{}
	clusterKey := client.ObjectKeyFromObject(&linodeCluster)

	BeforeAll(func(ctx SpecContext) {
		cScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()

		Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
		cScope.LinodeCluster = &linodeCluster

		// Create patch helper with latest state of resource.
		patchHelper, err := patch.NewHelper(&linodeCluster, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		cScope.PatchHelper = patchHelper
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("direct VPCID exists and has subnets", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient

					// Mock GetVPC call to return a VPC with subnets
					mck.LinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(12345)).
						Return(&linodego.VPC{
							ID:     12345,
							Label:  "test-vpc",
							Region: "us-ord",
							Subnets: []linodego.VPCSubnet{
								{
									ID:    1001,
									Label: "subnet-1",
								},
							},
						}, nil)

					// Mock the CreateNodeBalancer call to avoid unexpected call error
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						Return(&linodego.NodeBalancer{
							ID:     12345,
							Label:  ptr.To("test-nodebalancer"),
							Region: "us-ord",
							IPv4:   ptr.To("192.168.1.2"),
						}, nil).
						AnyTimes()

					// Mock the GetNodeBalancer call
					mck.LinodeClient.EXPECT().GetNodeBalancer(gomock.Any(), gomock.Any()).
						Return(&linodego.NodeBalancer{
							ID:     12345,
							Label:  ptr.To("test-nodebalancer"),
							Region: "us-ord",
							IPv4:   ptr.To("192.168.1.2"),
						}, nil).
						AnyTimes()

					// Mock the CreateNodeBalancerConfig call
					mck.LinodeClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&linodego.NodeBalancerConfig{
							ID:           123,
							Port:         6443,
							Protocol:     "tcp",
							Algorithm:    "roundrobin",
							Stickiness:   "none",
							Check:        "connection",
							CheckPassive: true,
						}, nil).
						AnyTimes()

					// Mock the GetNodeBalancerConfig call
					mck.LinodeClient.EXPECT().GetNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&linodego.NodeBalancerConfig{
							ID:           123,
							Port:         6443,
							Protocol:     "tcp",
							Algorithm:    "roundrobin",
							Stickiness:   "none",
							Check:        "connection",
							CheckPassive: true,
						}, nil).
						AnyTimes()

					// Mock the CreateNodeBalancerNode call
					mck.LinodeClient.EXPECT().CreateNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&linodego.NodeBalancerNode{
							ID:      456,
							Label:   "test-node",
							Address: "192.168.1.2:6443",
							Status:  "UP",
							Weight:  100,
						}, nil).
						AnyTimes()

					// Mock the ListNodeBalancerNodes call
					mck.LinodeClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return([]linodego.NodeBalancerNode{
							{
								ID:      456,
								Label:   "test-node",
								Address: "192.168.1.2:6443",
								Status:  "UP",
								Weight:  100,
							},
						}, nil).
						AnyTimes()

					// Mock the DeleteNodeBalancerNode call
					mck.LinodeClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil).
						AnyTimes()
				}),
				Result("VPC preflight check passes", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
					Expect(err).NotTo(HaveOccurred())
					Expect(rec.ConditionTrue(&linodeCluster, ConditionPreflightLinodeVPCReady)).To(BeTrue())
				}),
			),
			Path(
				Call("direct VPCID exists but has no subnets", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient

					// Set the condition to false initially
					conditions.Set(cScope.LinodeCluster, metav1.Condition{
						Type:   ConditionPreflightLinodeVPCReady,
						Status: metav1.ConditionFalse,
						Reason: "InitialState",
					})

					// Mock GetVPC call to return a VPC with no subnets
					mck.LinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(12345)).
						Return(&linodego.VPC{
							ID:      12345,
							Label:   "test-vpc",
							Region:  "us-ord",
							Subnets: []linodego.VPCSubnet{},
						}, nil)

					// Mock the ListNodeBalancerNodes call
					mck.LinodeClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return([]linodego.NodeBalancerNode{
							{
								ID:      456,
								Label:   "test-node",
								Address: "192.168.1.2:6443",
								Status:  "UP",
								Weight:  100,
							},
						}, nil).
						AnyTimes()

					// Mock the DeleteNodeBalancerNode call
					mck.LinodeClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil).
						AnyTimes()
				}),
				Result("VPC preflight check fails", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
					Expect(err).NotTo(HaveOccurred())
					Expect(rec.ConditionTrue(&linodeCluster, ConditionPreflightLinodeVPCReady)).To(BeFalse())
					condition := conditions.Get(&linodeCluster, ConditionPreflightLinodeVPCReady)
					Expect(condition).NotTo(BeNil())
					Expect(condition.Message).To(ContainSubstring("VPC with ID 12345 has no subnets"))
				}),
			),
			Path(
				Call("direct VPCID does not exist", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient

					// Mock GetVPC call to return an error
					mck.LinodeClient.EXPECT().GetVPC(gomock.Any(), gomock.Eq(12345)).
						Return(nil, errors.New("VPC not found"))

					// Mock the ListNodeBalancerNodes call
					mck.LinodeClient.EXPECT().ListNodeBalancerNodes(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return([]linodego.NodeBalancerNode{
							{
								ID:      456,
								Label:   "test-node",
								Address: "192.168.1.2:6443",
								Status:  "UP",
								Weight:  100,
							},
						}, nil).
						AnyTimes()

					// Mock the DeleteNodeBalancerNode call
					mck.LinodeClient.EXPECT().DeleteNodeBalancerNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil).
						AnyTimes()
				}),
				Result("VPC preflight check fails", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
					Expect(err).NotTo(HaveOccurred())
					Expect(rec.ConditionTrue(&linodeCluster, ConditionPreflightLinodeVPCReady)).To(BeFalse())
					condition := conditions.Get(&linodeCluster, ConditionPreflightLinodeVPCReady)
					Expect(condition).NotTo(BeNil())
					Expect(condition.Message).To(ContainSubstring("VPC not found"))
				}),
			),
		),
	)
})
