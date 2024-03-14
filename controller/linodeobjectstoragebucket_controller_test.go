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

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func mockClientBuilder(m *mock.MockLinodeObjectStorageClient) scope.LinodeObjectStorageClientBuilder {
	return func(_ string) (scope.LinodeObjectStorageClient, error) {
		return m, nil
	}
}

var _ = Describe("LinodeObjectStorageBucket controller", func() {
	ctx := context.Background()

	obj := &infrav1.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-bucket",
			Namespace: "default",
		},
		Spec: infrav1.LinodeObjectStorageBucketSpec{
			Cluster: "cluster",
		},
	}

	recorder := record.NewFakeRecorder(3)

	secretName := fmt.Sprintf(scope.AccessKeyNameTemplate, obj.Name)

	var secret corev1.Secret
	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		// Create a new gomock controller for each test run
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		// At the end of each test run, tell the gomock controller it's done
		// so it can check configured expectations and validate the methods called
		mockCtrl.Finish()
	})

	It("should provision a bucket and keys", func() {
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		getCall := mockClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
			Return(nil, nil).
			Times(1)

		createBucketCall := mockClient.EXPECT().
			CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{
				Label:    obj.Name,
				Cluster:  obj.Spec.Cluster,
				Created:  util.Pointer(time.Now()),
				Hostname: "hostname",
			}, nil).
			Times(1).
			After(getCall)

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
				Return(
					&linodego.ObjectStorageKey{
						ID:        idx,
						AccessKey: fmt.Sprintf("key-%d", idx),
					},
					nil,
				).
				Times(1).
				After(createBucketCall)
		}

		reconciler := &LinodeObjectStorageBucketReconciler{
			Client:              k8sClient,
			Scheme:              k8sClient.Scheme(),
			Logger:              ctrl.Log.WithName("LinodeObjectStorageBucketReconciler"),
			Recorder:            recorder,
			LinodeClientBuilder: mockClientBuilder(mockClient),
		}

		objectKey := client.ObjectKeyFromObject(obj)
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("updating the bucket resource's status fields")
		Expect(k8sClient.Get(ctx, objectKey, obj)).To(Succeed())
		Expect(*obj.Status.Hostname).To(Equal("hostname"))
		Expect(*obj.Status.KeySecretName).To(Equal(secretName))
		Expect(*obj.Status.LastKeyGeneration).To(Equal(*obj.Spec.KeyGeneration))
		Expect(*obj.Status.LastKeyGeneration).To(Equal(0))
		Expect(obj.Status.Ready).To(BeTrue())

		By("creating a secret with access keys")
		Expect(k8sClient.Get(ctx, client.ObjectKey{
			Name:      secretName,
			Namespace: obj.Namespace,
		}, &secret)).To(Succeed())
		Expect(secret.Data).To(HaveLen(2))
		Expect(string(secret.Data["read_write"])).To(Equal(string("key-0")))
		Expect(string(secret.Data["read_only"])).To(Equal(string("key-1")))

		By("recording the expected events")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys assigned"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))
	})

	It("should ensure the bucket's secret exists", func() {
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		getCall := mockClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
			Return(
				&linodego.ObjectStorageBucket{
					Label:    obj.Name,
					Cluster:  obj.Spec.Cluster,
					Created:  util.Pointer(time.Now()),
					Hostname: "hostname",
				},
				nil,
			).
			Times(1)

		for idx := range 2 {
			mockClient.EXPECT().
				GetObjectStorageKey(gomock.Any(), idx).
				Return(&linodego.ObjectStorageKey{
					ID:        idx,
					AccessKey: fmt.Sprintf("key-%d", idx),
				}, nil).
				Times(1).
				After(getCall)
		}

		reconciler := &LinodeObjectStorageBucketReconciler{
			Client:              k8sClient,
			Scheme:              k8sClient.Scheme(),
			Logger:              ctrl.Log.WithName("LinodeObjectStorageBucketReconciler"),
			Recorder:            recorder,
			LinodeClientBuilder: mockClientBuilder(mockClient),
		}

		objectKey := client.ObjectKeyFromObject(obj)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: obj.Namespace,
			},
		}
		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("re-creating it when it is deleted")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
		Expect(secret.Data).To(HaveLen(2))
		Expect(string(secret.Data["read_write"])).To(Equal("key-0"))
		Expect(string(secret.Data["read_only"])).To(Equal("key-1"))

		By("recording the expected events")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys retrieved"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))
	})

	It("should revoke the bucket's keys", func() {
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		for i := range 2 {
			mockClient.EXPECT().
				DeleteObjectStorageKey(gomock.Any(), i).
				Return(nil).
				Times(1)
		}

		reconciler := &LinodeObjectStorageBucketReconciler{
			Client:              k8sClient,
			Scheme:              k8sClient.Scheme(),
			Logger:              ctrl.Log.WithName("LinodeObjectStorageBucketReconciler"),
			Recorder:            recorder,
			LinodeClientBuilder: mockClientBuilder(mockClient),
		}

		objectKey := client.ObjectKeyFromObject(obj)
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("removing the bucket's finalizer so it is deleted")
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, obj))).To(BeTrue())

		By("recording the expected event")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys revoked"))
	})
})
