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
	"errors"
	"fmt"
	"time"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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

var _ = Describe("lifecycle", func() {
	obj := infrav1.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: "default",
		},
		Spec: infrav1.LinodeObjectStorageBucketSpec{
			Cluster: "cluster",
		},
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(scope.AccessKeyNameTemplate, obj.Name),
			Namespace: "default",
		},
	}

	// Create a recorder with a buffered channel for consuming event strings.
	recorder := record.NewFakeRecorder(10)

	var mockCtrl *gomock.Controller

	BeforeEach(func() {
		// Create a new gomock controller for each test run
		mockCtrl = gomock.NewController(GinkgoT())
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

	It("should provision a bucket and keys", func(ctx SpecContext) {
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
				Return(&linodego.ObjectStorageKey{
					ID:        idx,
					AccessKey: fmt.Sprintf("key-%d", idx),
				}, nil).
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

		objectKey := client.ObjectKeyFromObject(&obj)
		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("updating the bucket resource's status fields")
		Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
		Expect(*obj.Status.Hostname).To(Equal("hostname"))
		Expect(*obj.Status.KeySecretName).To(Equal(secret.Name))
		Expect(*obj.Status.LastKeyGeneration).To(Equal(*obj.Spec.KeyGeneration))
		Expect(*obj.Status.LastKeyGeneration).To(Equal(0))
		Expect(obj.Status.Ready).To(BeTrue())

		By("creating a secret with access keys")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
		Expect(secret.Data).To(HaveLen(2))
		Expect(string(secret.Data["read_write"])).To(Equal(string("key-0")))
		Expect(string(secret.Data["read_only"])).To(Equal(string("key-1")))

		By("recording the expected events")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys assigned"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))
	})

	It("should ensure the bucket's secret exists", func(ctx SpecContext) {
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		getCall := mockClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
			Return(&linodego.ObjectStorageBucket{
				Label:    obj.Name,
				Cluster:  obj.Spec.Cluster,
				Created:  util.Pointer(time.Now()),
				Hostname: "hostname",
			}, nil).
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

		objectKey := client.ObjectKeyFromObject(&obj)
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("re-creating it when it is deleted")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
		Expect(secret.Data).To(HaveLen(2))
		Expect(string(secret.Data["read_write"])).To(Equal("key-0"))
		Expect(string(secret.Data["read_only"])).To(Equal("key-1"))

		By("recording the expected events")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys retrieved"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))
	})

	It("should revoke the bucket's keys", func(ctx SpecContext) {
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

		objectKey := client.ObjectKeyFromObject(&obj)
		Expect(k8sClient.Delete(ctx, &obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("removing the bucket's finalizer so it is deleted")
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &obj))).To(BeTrue())

		By("recording the expected event")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys revoked"))
	})
})

var _ = Describe("cases", func() {
	var obj infrav1.LinodeObjectStorageBucket
	var secret corev1.Secret
	var mockCtrl *gomock.Controller
	var reconciler *LinodeObjectStorageBucketReconciler
	var testLogs *bytes.Buffer

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func() {
		obj = infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "case-",
				Namespace:    "default",
			},
			Spec: infrav1.LinodeObjectStorageBucketSpec{
				Cluster: "cluster",
			},
		}
		mockCtrl = gomock.NewController(GinkgoT())
		testLogs = &bytes.Buffer{}
		reconciler = &LinodeObjectStorageBucketReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Logger:   ctrl.Log.WithName("LinodeObjectStorageBucketReconciler"),
			Recorder: recorder,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
		for len(recorder.Events) > 0 {
			<-recorder.Events
		}
	})

	It("returns a nil error when the resource does not exist", func(ctx SpecContext) {
		obj.Name = "empty"
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&obj),
		})
		Expect(err).To(BeNil())
	})

	It("fails when the resource cannot be fetched", func(ctx SpecContext) {
		reconciler.Logger = zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		reconciler.Client = mockK8sClient
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("non-404 error")).
			Times(1)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&obj),
		})
		Expect(err.Error()).To(ContainSubstring("non-404 error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to fetch LinodeObjectStorageBucket"))
	})

	It("fails when a scope cannot be created", func(ctx SpecContext) {
		reconciler.Logger = zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&obj),
		})
		Expect(err.Error()).To(ContainSubstring("failed to create object storage bucket scope"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to create object storage bucket scope"))
	})

	It("fails when it can't ensure an OBJ bucket exists", func(ctx SpecContext) {
		reconciler.Logger = zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		reconciler.LinodeClientBuilder = mockClientBuilder(mockClient)
		mockClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("non-404 error")).
			Times(1)

		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&obj),
		})
		Expect(err.Error()).To(ContainSubstring("non-404 error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to ensure bucket exists"))
	})

	It("fails when it can't provision new access keys", func(ctx SpecContext) {
		reconciler.Logger = zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		reconciler.LinodeClientBuilder = mockClientBuilder(mockClient)
		mockClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{
				Created: ptr.To(time.Now()),
			}, nil).
			Times(1)
		mockClient.EXPECT().
			CreateObjectStorageKey(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("api error")).
			Times(1)

		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&obj),
		})
		Expect(err.Error()).To(ContainSubstring("api error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to provision new access keys"))
	})

	It("fails when it can't evaluate whether to restore a key secret", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil).
			Times(1)

		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		mockK8sClient.EXPECT().
			Scheme().
			Return(scheme.Scheme).
			Times(2)
		mockK8sClient.EXPECT().
			Patch(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("api error")).
			Times(1)

		obj = infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
			},
			Spec: infrav1.LinodeObjectStorageBucketSpec{
				KeyGeneration: ptr.To(1),
			},
			Status: infrav1.LinodeObjectStorageBucketStatus{
				AccessKeyRefs:     []int{0, 1},
				LastKeyGeneration: ptr.To(1),
				KeySecretName:     ptr.To("mock-access-keys"),
			},
		}

		testLogger := zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		bScope, err := scope.NewObjectStorageBucketScope(ctx, "", scope.ObjectStorageBucketScopeParams{
			Client:              mockK8sClient,
			LinodeClientBuilder: mockClientBuilder(mockLinodeClient),
			Bucket:              &obj,
			Logger:              &testLogger,
		})
		Expect(err).NotTo(HaveOccurred())

		err = reconciler.reconcileApply(ctx, bScope)
		Expect(err.Error()).To(ContainSubstring("api error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to ensure access key secret exists"))
	})

	It("fails when it can't retrieve access keys for a deleted secret", func(ctx SpecContext) {
		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		mockK8sClient.EXPECT().
			Scheme().
			Return(scheme.Scheme).
			Times(2)
		mockK8sClient.EXPECT().
			Patch(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-access-keys")).
			Times(1)

		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil).
			Times(1)
		mockLinodeClient.EXPECT().
			GetObjectStorageKey(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("key creation error")).
			Times(2)

		obj = infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
			},
			Spec: infrav1.LinodeObjectStorageBucketSpec{
				KeyGeneration: ptr.To(1),
			},
			Status: infrav1.LinodeObjectStorageBucketStatus{
				AccessKeyRefs:     []int{0, 1},
				LastKeyGeneration: ptr.To(1),
				KeySecretName:     ptr.To("mock-access-keys"),
			},
		}

		testLogger := zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		bScope, err := scope.NewObjectStorageBucketScope(ctx, "", scope.ObjectStorageBucketScopeParams{
			Client:              mockK8sClient,
			LinodeClientBuilder: mockClientBuilder(mockLinodeClient),
			Bucket:              &obj,
			Logger:              &testLogger,
		})
		Expect(err).NotTo(HaveOccurred())

		err = reconciler.reconcileApply(ctx, bScope)
		Expect(err.Error()).To(ContainSubstring("key creation error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to restore access keys for deleted secret"))
	})

	It("fails when it can't generate a secret", func(ctx SpecContext) {
		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		schemeCalls := mockK8sClient.EXPECT().
			Scheme().
			Return(scheme.Scheme).
			Times(2)
		mockK8sClient.EXPECT().
			Scheme().
			After(schemeCalls).
			Return(runtime.NewScheme()).
			Times(1)
		mockK8sClient.EXPECT().
			Patch(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-access-keys")).
			Times(1)

		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil).
			Times(1)
		for idx := range 2 {
			mockLinodeClient.EXPECT().
				GetObjectStorageKey(gomock.Any(), idx).
				Return(&linodego.ObjectStorageKey{ID: idx}, nil).
				Times(1)
		}

		obj = infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
				UID:       "12345",
			},
			Spec: infrav1.LinodeObjectStorageBucketSpec{
				KeyGeneration: ptr.To(1),
			},
			Status: infrav1.LinodeObjectStorageBucketStatus{
				AccessKeyRefs:     []int{0, 1},
				LastKeyGeneration: ptr.To(1),
				KeySecretName:     ptr.To("mock-access-keys"),
			},
		}

		testLogger := zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		bScope, err := scope.NewObjectStorageBucketScope(ctx, "", scope.ObjectStorageBucketScopeParams{
			Client:              mockK8sClient,
			LinodeClientBuilder: mockClientBuilder(mockLinodeClient),
			Bucket:              &obj,
			Logger:              &testLogger,
		})
		Expect(err).NotTo(HaveOccurred())

		err = reconciler.reconcileApply(ctx, bScope)
		Expect(err.Error()).To(ContainSubstring("no kind is registered"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to generate key secret"))
	})

	It("fails when it can't restore a deleted secret", func(ctx SpecContext) {
		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		mockK8sClient.EXPECT().
			Scheme().
			Return(scheme.Scheme).
			Times(3)
		mockK8sClient.EXPECT().
			Patch(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-access-keys")).
			Times(2)
		mockK8sClient.EXPECT().
			Create(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("secret creation error"))

		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil).
			Times(1)
		for idx := range 2 {
			mockLinodeClient.EXPECT().
				GetObjectStorageKey(gomock.Any(), idx).
				Return(&linodego.ObjectStorageKey{ID: idx}, nil).
				Times(1)
		}

		obj = infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
				UID:       "12345",
			},
			Spec: infrav1.LinodeObjectStorageBucketSpec{
				KeyGeneration: ptr.To(1),
			},
			Status: infrav1.LinodeObjectStorageBucketStatus{
				AccessKeyRefs:     []int{0, 1},
				LastKeyGeneration: ptr.To(1),
				KeySecretName:     ptr.To("mock-access-keys"),
			},
		}

		testLogger := zap.New(zap.WriteTo(testLogs), zap.UseDevMode(true))
		bScope, err := scope.NewObjectStorageBucketScope(ctx, "", scope.ObjectStorageBucketScopeParams{
			Client:              mockK8sClient,
			LinodeClientBuilder: mockClientBuilder(mockLinodeClient),
			Bucket:              &obj,
			Logger:              &testLogger,
		})
		Expect(err).NotTo(HaveOccurred())

		err = reconciler.reconcileApply(ctx, bScope)
		Expect(err.Error()).To(ContainSubstring("secret creation error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to apply key secret"))
	})

	It("BLANK TODO", func(ctx SpecContext) {
		mockClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		reconciler.LinodeClientBuilder = mockClientBuilder(mockClient)

		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(scope.AccessKeyNameTemplate, obj.Name),
				Namespace: "default",
			},
		}
		Expect(secret.Name).To(Not(BeEmpty()))
	})
})
