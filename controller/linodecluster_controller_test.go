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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cluster-lifecycle", Ordered, Label("cluster", "cluster-lifecycle"), func() {
	nodebalancerID := 1
	controlPlaneEndpointHost := "10.0.0.1"
	controlPlaneEndpointPort := 6443
	clusterName := "cluster-lifecycle"
	clusterNameSpace := "default"
	ownerRef := metav1.OwnerReference{
		Name:       clusterName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            clusterName,
		Namespace:       clusterNameSpace,
		OwnerReferences: ownerRefs,
	}

	linodeCluster := infrav1.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1.LinodeClusterSpec{
			Region: "us-ord",
		},
	}

	ctlrSuite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})
	reconciler := LinodeClusterReconciler{}

	cScope := &scope.ClusterScope{
		LinodeCluster: &linodeCluster,
	}

	BeforeAll(func(ctx SpecContext) {
		cScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
	})

	ctlrSuite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
		clusterKey := client.ObjectKey{Name: "cluster-lifecycle", Namespace: "default"}
		Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&linodeCluster, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		cScope.PatchHelper = patchHelper
	})

	ctlrSuite.Run(
		OneOf(
			Path(
				Call("cluster is created", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					getNB := mck.LinodeClient.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return(nil, nil)
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						After(getNB).
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
			),
			Path(
				Call("cluster is not created because there was an error creating nb", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					getNB := mck.LinodeClient.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return(nil, nil)
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						After(getNB).
						Return(nil, errors.New("create NB error"))
				}),
				Result("create nb error", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err.Error()).To(ContainSubstring("create NB error"))
				}),
			),
			Path(
				Call("cluster is not created because nb was nil", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					getNB := mck.LinodeClient.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return(nil, nil)
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						After(getNB).
						Return(nil, nil)
				}),
				Result("created nb is nil", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err.Error()).To(ContainSubstring("nodeBalancer created was nil"))
				}),
			),
			Path(
				Call("cluster is not created because nb config was nil", func(ctx context.Context, mck Mock) {
					cScope.LinodeClient = mck.LinodeClient
					getNB := mck.LinodeClient.EXPECT().ListNodeBalancers(gomock.Any(), gomock.Any()).Return(nil, nil)
					mck.LinodeClient.EXPECT().CreateNodeBalancer(gomock.Any(), gomock.Any()).
						After(getNB).
						Return(&linodego.NodeBalancer{
							ID:   nodebalancerID,
							IPv4: &controlPlaneEndpointHost,
						}, nil)
					mck.LinodeClient.EXPECT().CreateNodeBalancerConfig(gomock.Any(), gomock.Any(), gomock.Any()).
						After(getNB).
						Return(nil, errors.New("nodeBalancer config created was nil"))
				}),
				Result("created nb config is nil", func(ctx context.Context, mck Mock) {
					_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
					Expect(err.Error()).To(ContainSubstring("nodeBalancer config created was nil"))
				}),
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
		),
		Result("resource status is updated and NB is created", func(ctx context.Context, mck Mock) {
			_, err := reconciler.reconcile(ctx, cScope, logr.Logger{})
			Expect(err).NotTo(HaveOccurred())

			By("checking ready conditions")
			clusterKey := client.ObjectKeyFromObject(&linodeCluster)
			Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
			Expect(linodeCluster.Status.Ready).To(BeTrue())
			Expect(linodeCluster.Status.Conditions).To(HaveLen(1))
			Expect(linodeCluster.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))

			By("checking NB id")
			Expect(linodeCluster.Spec.Network.NodeBalancerID).To(Equal(&nodebalancerID))

			By("checking controlPlaneEndpoint/NB host and port")
			Expect(linodeCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(controlPlaneEndpointHost))
			Expect(linodeCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(controlPlaneEndpointPort)))
		}),
	)
})

var _ = Describe("cluster-delete", Ordered, Label("cluster", "cluster-delete"), func() {
	nodebalancerID := 1
	clusterName := "cluster-delete"
	clusterNameSpace := "default"
	ownerRef := metav1.OwnerReference{
		Name:       clusterName,
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		UID:        "00000000-000-0000-0000-000000000000",
	}
	ownerRefs := []metav1.OwnerReference{ownerRef}
	metadata := metav1.ObjectMeta{
		Name:            clusterName,
		Namespace:       clusterNameSpace,
		OwnerReferences: ownerRefs,
	}

	linodeCluster := infrav1.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1.NetworkSpec{
				NodeBalancerID: &nodebalancerID,
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
					mck.LinodeClient.EXPECT().DeleteNodeBalancer(gomock.Any(), gomock.Any()).Return(nil)
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
					mck.LinodeClient.EXPECT().DeleteNodeBalancer(gomock.Any(), gomock.Any()).Return(errors.New("delete NB error"))
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
