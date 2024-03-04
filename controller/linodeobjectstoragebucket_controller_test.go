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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"

	infrastructurev1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/scope/mock"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/linodego"
)

var _ = Describe("LinodeObjectStorageBucket controller", func() {
	ctx := context.Background()

	obj := &infrastructurev1alpha1.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "default",
		},
		Spec: infrastructurev1alpha1.LinodeObjectStorageBucketSpec{
			Cluster: "cluster",
		},
	}

	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("should reconcile an object", func() {
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		mockClient.EXPECT().
			ListObjectStorageBucketsInCluster(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]linodego.ObjectStorageBucket{}, nil)

		mockClient.EXPECT().
			CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{
				Label:    obj.Name,
				Cluster:  obj.Spec.Cluster,
				Created:  util.Pointer(time.Now()),
				Hostname: "hostname",
			}, nil)

		for idx, permission := range []string{"rw", "ro"} {
			mockClient.EXPECT().
				CreateObjectStorageKey(
					gomock.Any(),
					gomock.Cond(func(opt any) bool {
						createOpt, ok := opt.(linodego.ObjectStorageKeyCreateOptions)
						if !ok {
							return false
						}

						return createOpt.Label == fmt.Sprintf("%s-%s", obj.Name, permission)
					}),
				).
				Return(&linodego.ObjectStorageKey{ID: idx}, nil)
		}

		controllerReconciler := &LinodeObjectStorageBucketReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Logger:   ctrl.Log.WithName("LinodeObjectStorageBucketReconciler"),
			Recorder: record.NewFakeRecorder(3),
			LinodeClientFactory: func(apiKey string) scope.LinodeObjectStorageClient {
				return mockClient
			},
		}

		objectKey := client.ObjectKeyFromObject(obj)
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("updating its status fields")
		Expect(k8sClient.Get(ctx, objectKey, obj)).To(Succeed())
		Expect(*obj.Status.Hostname).To(Equal("hostname"))
		secretName := fmt.Sprintf(scope.AccessKeyNameTemplate, obj.Name)
		Expect(*obj.Status.KeySecretName).To(Equal(secretName))
		Expect(*obj.Status.LastKeyGeneration).To(Equal(*obj.Spec.KeyGeneration))
		Expect(*obj.Status.LastKeyGeneration).To(Equal(0))
		Expect(obj.Status.Ready).To(BeTrue())

		By("creating a Secret with access keys")
		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, client.ObjectKey{
			Name:      secretName,
			Namespace: obj.Namespace,
		}, &secret)).To(Succeed())
		Expect(secret.Data["read_write"]).To(Not(BeNil()))
		Expect(secret.Data["read_only"]).To(Not(BeNil()))
	})
})
