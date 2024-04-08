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
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("lifecycle", Ordered, Label("cluster", "lifecycle"), func() {
	var mockCtrl *gomock.Controller
	var reconciler *LinodeClusterReconciler
	nodebalancerID := 1
	clusterName := "lifecycle"
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

	caplCluster := clusterv1.Cluster{
		ObjectMeta: metadata,
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Kind:      "LinodeCluster",
				Name:      clusterName,
				Namespace: clusterNameSpace,
			},
			ControlPlaneRef: &corev1.ObjectReference{
				Kind:      "KubeadmControlPlane",
				Name:      "lifecycle-control-plane",
				Namespace: clusterNameSpace,
			},
		},
	}

	linodeCluster := infrav1.LinodeCluster{
		ObjectMeta: metadata,
		Spec: infrav1.LinodeClusterSpec{
			Region: "us-ord",
			Network: infrav1.NetworkSpec{
				NodeBalancerID: &nodebalancerID,
			},
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: "10.0.0.1",
			},
		},
	}

	// Create a recorder with a buffered channel for consuming event strings.
	recorder := record.NewFakeRecorder(10)

	BeforeEach(func() {
		// Create a new gomock controller for each test run
		mockCtrl = gomock.NewController(GinkgoT())
		reconciler = &LinodeClusterReconciler{
			Client:       k8sClient,
			Recorder:     recorder,
			LinodeApiKey: "test-key",
		}
	})

	AfterEach(func() {
		// At the end of each test run, tell the gomock controller it's done
		// so it can check configured expectations and validate the methods called
		mockCtrl.Finish()
		// Flush the channel if any events were not consumed.
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("should provision a control plane endpoint", func(ctx SpecContext) {
		clusterKey := client.ObjectKeyFromObject(&linodeCluster)
		Expect(k8sClient.Create(ctx, &caplCluster)).To(Succeed())
		Expect(k8sClient.Create(ctx, &linodeCluster)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: clusterKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("checking nb id")
		Expect(k8sClient.Get(ctx, clusterKey, &linodeCluster)).To(Succeed())
		Expect(linodeCluster.Status.Ready).To(BeTrue())
		Expect(linodeCluster.Status.Conditions).To(HaveLen(1))
		Expect(linodeCluster.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))
		Expect(linodeCluster.Spec.Network.NodeBalancerID).To(Equal(&nodebalancerID))
	})
})
