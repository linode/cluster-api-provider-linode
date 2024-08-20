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

	"github.com/go-logr/logr"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
	rec "github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/linodego"

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
				Call("cluster is not created because there was an error creating nb", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to ensure nodebalancer"))
				}),
				OneOf(
					Path(Result("create requeues", func(ctx context.Context, mck Mock) {
						reconciler.Client = k8sClient
						res, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).NotTo(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(rec.DefaultClusterControllerReconcileDelay))
						Expect(mck.Logs()).To(ContainSubstring("re-queuing cluster/load-balancer creation"))
					})),
					Path(Result("create nb error - timeout error", func(ctx context.Context, mck Mock) {
						tempTimeout := reconciler.ReconcileTimeout
						reconciler.ReconcileTimeout = time.Nanosecond
						reconciler.Client = k8sClient
						_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("failed to ensure nodebalancer"))
						reconciler.ReconcileTimeout = tempTimeout
					})),
				),
			),
			Path(
				Call("cluster is not created because nb was nil", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
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
					Path(Result("create nb error - timeout error", func(ctx context.Context, mck Mock) {
						tempTimeout := reconciler.ReconcileTimeout
						reconciler.ReconcileTimeout = time.Nanosecond
						reconciler.Client = k8sClient
						_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("nodeBalancer created was nil"))
						reconciler.ReconcileTimeout = tempTimeout
					})),
				),
			),
			Path(
				Call("cluster is not created because nb config was nil", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
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
					Path(Result("create nb error - timeout error", func(ctx context.Context, mck Mock) {
						mck.LinodeClient.EXPECT().GetNodeBalancer(gomock.Any(), gomock.Any()).
							Return(&linodego.NodeBalancer{
								ID:   nodebalancerID,
								IPv4: &controlPlaneEndpointHost,
							}, nil)

						tempTimeout := reconciler.ReconcileTimeout
						reconciler.Client = k8sClient
						reconciler.ReconcileTimeout = time.Nanosecond
						_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("nodeBalancer config created was nil"))
						reconciler.ReconcileTimeout = tempTimeout
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
					Path(Result("create nb error - timeout error", func(ctx context.Context, mck Mock) {
						tempTimeout := reconciler.ReconcileTimeout
						reconciler.ReconcileTimeout = time.Nanosecond
						reconciler.Client = k8sClient
						_, err := reconciler.reconcile(ctx, cScope, mck.Logger())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("failed to get nodebalancer config"))
						reconciler.ReconcileTimeout = tempTimeout
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
					Expect(linodeCluster.Status.Conditions).To(HaveLen(2))
					Expect(linodeCluster.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))

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
				}),
				Result("cluster created", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err).NotTo(HaveOccurred())

					By("checking ready conditions")
					clusterKey := client.ObjectKeyFromObject(&linodeCluster)
					Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
					Expect(linodeCluster.Status.Ready).To(BeTrue())
					Expect(linodeCluster.Status.Conditions).To(HaveLen(2))
					Expect(linodeCluster.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))

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

	ctlrSuite := NewControllerSuite(
		GinkgoT(),
		mock.MockLinodeClient{},
		mock.MockK8sClient{},
	)
	reconciler := LinodeClusterReconciler{}

	cScope := &scope.ClusterScope{
		LinodeCluster: &linodeCluster,
	}

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("cluster is deleted", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					cScope.Client = mck.K8sClient
					mck.LinodeClient.EXPECT().DeleteNodeBalancer(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
					Expect(mck.Events()).To(ContainSubstring("Warning NodeBalancerIDMissing NodeBalancer ID is missing, nothing to do"))
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
		),
		Result("cluster deleted", func(ctx context.Context, mck Mock) {
			reconciler.Client = mck.K8sClient
			err := reconciler.reconcileDelete(ctx, logr.Logger{}, cScope)
			Expect(err).NotTo(HaveOccurred())
		}),
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
				}),
				Result("cluster created", func(ctx context.Context, mck Mock) {
					reconciler.Client = k8sClient
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err).NotTo(HaveOccurred())

					By("checking ready conditions")
					clusterKey := client.ObjectKeyFromObject(&linodeCluster)
					Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
					Expect(linodeCluster.Status.Ready).To(BeTrue())
					Expect(linodeCluster.Status.Conditions).To(HaveLen(2))
					Expect(linodeCluster.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))

					By("checking controlPlaneEndpoint/NB host and port")
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(controlPlaneEndpointHost))
					Expect(linodeCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(controlPlaneEndpointPort)))
				}),
			),
		),
	)
})
