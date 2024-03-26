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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type AccessKeySecret struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	StringData struct {
		Bucket_Name   string `yaml:"bucket_name"`
		Access_Key_RW string `yaml:"access_key_rw"`
		Secret_Key_RW string `yaml:"secret_key_rw"`
		Access_Key_RO string `yaml:"access_key_ro"`
		Secret_Key_RO string `yaml:"secret_key_ro"`
	} `yaml:"stringData"`
}

func mockLinodeClientBuilder(m *mock.MockLinodeObjectStorageClient) scope.LinodeObjectStorageClientBuilder {
	return func(_ string) (scope.LinodeObjectStorageClient, error) {
		return m, nil
	}
}

var _ = Describe("lifecycle", Label("lifecycle"), func() {
	var mockCtrl *gomock.Controller
	var reconciler *LinodeObjectStorageBucketReconciler
	var testLogs *bytes.Buffer

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

	BeforeEach(func() {
		// Create a new gomock controller for each test run
		mockCtrl = gomock.NewController(GinkgoT())
		// Inject io.Writer as log sink for consuming logs
		testLogs = &bytes.Buffer{}
		reconciler = &LinodeObjectStorageBucketReconciler{
			Client:   k8sClient,
			Recorder: recorder,
			Logger: zap.New(
				zap.WriteTo(GinkgoWriter),
				zap.WriteTo(testLogs),
				zap.UseDevMode(true),
			),
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

	It("should provision a bucket and keys", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		getCall := mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
			Return(nil, nil).
			Times(1)

		createBucketCall := mockLinodeClient.EXPECT().
			CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{
				Label:    obj.Name,
				Cluster:  obj.Spec.Cluster,
				Created:  util.Pointer(time.Now()),
				Hostname: "hostname",
			}, nil).
			Times(1).
			After(getCall)

		for idx := range 2 {
			mockLinodeClient.EXPECT().
				CreateObjectStorageKey(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(_ any, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error) {
						return &linodego.ObjectStorageKey{ID: idx, AccessKey: fmt.Sprintf("key-%d", idx)}, nil
					}).
				Times(1).
				After(createBucketCall)
		}

		objectKey := client.ObjectKeyFromObject(&obj)
		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())

		reconciler.LinodeClientBuilder = mockLinodeClientBuilder(mockLinodeClient)
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("updating the bucket resource's status fields")
		Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
		Expect(obj.Status.Ready).To(BeTrue())
		Expect(obj.Status.Conditions).To(HaveLen(1))
		Expect(obj.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))
		Expect(*obj.Status.Hostname).To(Equal("hostname"))
		Expect(obj.Status.CreationTime).NotTo(BeNil())
		Expect(*obj.Status.LastKeyGeneration).To(Equal(*obj.Spec.KeyGeneration))
		Expect(*obj.Status.LastKeyGeneration).To(Equal(0))
		Expect(*obj.Status.KeySecretName).To(Equal(secret.Name))
		Expect(obj.Status.AccessKeyRefs).To(HaveLen(scope.NumAccessKeys))

		By("creating a secret with access keys")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
		Expect(secret.Data).To(HaveLen(1))
		var key AccessKeySecret
		unMarshallingErr := yaml.Unmarshal(secret.Data["access-keys-secret.yaml"], &key)
		Expect(unMarshallingErr).NotTo(HaveOccurred())
		Expect(key.StringData.Bucket_Name).To(Equal("lifecycle"))
		Expect(key.StringData.Access_Key_RW).To(Equal("key-0"))
		Expect(key.StringData.Secret_Key_RW).To(Equal(""))
		Expect(key.StringData.Access_Key_RO).To(Equal("key-1"))
		Expect(key.StringData.Secret_Key_RO).To(Equal(""))

		By("recording the expected events")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys assigned"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))

		By("logging the expected messages")
		logOutput := testLogs.String()
		Expect(logOutput).To(ContainSubstring("Reconciling apply"))
		Expect(logOutput).To(ContainSubstring("Secret lifecycle-access-keys was applied with new access keys"))
	})

	It("should ensure the bucket's secret exists", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		getCall := mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
			Return(&linodego.ObjectStorageBucket{
				Label:    obj.Name,
				Cluster:  obj.Spec.Cluster,
				Created:  util.Pointer(time.Now()),
				Hostname: "hostname",
			}, nil).
			Times(1)

		for idx := range 2 {
			mockLinodeClient.EXPECT().
				GetObjectStorageKey(gomock.Any(), idx).
				Return(&linodego.ObjectStorageKey{
					ID:        idx,
					AccessKey: fmt.Sprintf("key-%d", idx),
				}, nil).
				Times(1).
				After(getCall)
		}

		objectKey := client.ObjectKeyFromObject(&obj)
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		reconciler.LinodeClientBuilder = mockLinodeClientBuilder(mockLinodeClient)
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("re-creating it when it is deleted")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
		Expect(secret.Data).To(HaveLen(1))
		var key AccessKeySecret
		unMarshallingErr := yaml.Unmarshal(secret.Data["access-keys-secret.yaml"], &key)
		Expect(unMarshallingErr).NotTo(HaveOccurred())
		Expect(key.StringData.Bucket_Name).To(Equal("lifecycle"))
		Expect(key.StringData.Access_Key_RW).To(Equal("key-0"))
		Expect(key.StringData.Secret_Key_RW).To(Equal(""))
		Expect(key.StringData.Access_Key_RO).To(Equal("key-1"))
		Expect(key.StringData.Secret_Key_RO).To(Equal(""))

		By("recording the expected events")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys retrieved"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
		Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))

		By("logging the expected messages")
		logOutput := testLogs.String()
		Expect(logOutput).To(ContainSubstring("Reconciling apply"))
		Expect(logOutput).To(ContainSubstring("Secret lifecycle-access-keys was applied with new access keys"))
	})

	It("should rotate the bucket's keys", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		getCall := mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
			Return(&linodego.ObjectStorageBucket{
				Label:    obj.Name,
				Cluster:  obj.Spec.Cluster,
				Created:  util.Pointer(time.Now()),
				Hostname: "hostname",
			}, nil).
			Times(1)

		for idx := range 2 {
			createCall := mockLinodeClient.EXPECT().
				CreateObjectStorageKey(gomock.Any(), gomock.Any()).
				After(getCall).
				DoAndReturn(
					func(_ any, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error) {
						return &linodego.ObjectStorageKey{ID: idx + 2, AccessKey: fmt.Sprintf("key-%d", idx+2)}, nil
					}).
				Times(1)
			mockLinodeClient.EXPECT().
				DeleteObjectStorageKey(gomock.Any(), idx).
				After(createCall).
				Return(nil).
				Times(1)
		}

		objectKey := client.ObjectKeyFromObject(&obj)
		Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
		obj.Spec.KeyGeneration = ptr.To(1)
		Expect(k8sClient.Update(ctx, &obj)).To(Succeed())

		reconciler.LinodeClientBuilder = mockLinodeClientBuilder(mockLinodeClient)
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("updating the bucket resource's status fields")
		Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
		Expect(*obj.Status.LastKeyGeneration).To(Equal(1))

		By("recording the expected event")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys assigned"))

		By("logging the expected messages")
		logOutput := testLogs.String()
		Expect(logOutput).To(ContainSubstring("Reconciling apply"))
		Expect(logOutput).To(ContainSubstring("Secret lifecycle-access-keys was applied with new access keys"))
	})

	It("should revoke the bucket's keys", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)

		for i := range 2 {
			mockLinodeClient.EXPECT().
				DeleteObjectStorageKey(gomock.Any(), i+2).
				Return(nil).
				Times(1)
		}

		objectKey := client.ObjectKeyFromObject(&obj)
		Expect(k8sClient.Delete(ctx, &obj)).To(Succeed())

		reconciler.LinodeClientBuilder = mockLinodeClientBuilder(mockLinodeClient)
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: objectKey,
		})
		Expect(err).NotTo(HaveOccurred())

		By("removing the bucket's finalizer so it is deleted")
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &obj))).To(BeTrue())

		By("recording the expected event")
		Expect(<-recorder.Events).To(ContainSubstring("Object storage keys revoked"))

		By("logging the expected messages")
		logOutput := testLogs.String()
		Expect(logOutput).To(ContainSubstring("Reconciling delete"))
	})
})

var _ = Describe("pre-reconcile", Label("pre-reconcile"), func() {
	var obj infrav1.LinodeObjectStorageBucket
	var mockCtrl *gomock.Controller
	var reconciler *LinodeObjectStorageBucketReconciler
	var testLogs *bytes.Buffer

	recorder := record.NewFakeRecorder(10)

	BeforeEach(func() {
		// Use a generated name to isolate objects per spec.
		obj = infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pre-reconcile-",
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
			Recorder: recorder,
			Logger: zap.New(
				zap.WriteTo(GinkgoWriter),
				zap.WriteTo(testLogs),
				zap.UseDevMode(true),
			),
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
		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("non-404 error")).
			Times(1)

		reconciler.Client = mockK8sClient
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&obj),
		})
		Expect(err.Error()).To(ContainSubstring("non-404 error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to fetch LinodeObjectStorageBucket"))
	})

	It("fails when a scope cannot be created due to missing arguments", func(ctx SpecContext) {
		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&obj),
		})
		Expect(err.Error()).To(ContainSubstring("failed to create object storage bucket scope"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to create object storage bucket scope"))
	})
})

var _ = Describe("apply", Label("apply"), func() {
	var obj infrav1.LinodeObjectStorageBucket
	var mockCtrl *gomock.Controller
	var testLogs *bytes.Buffer

	recorder := record.NewFakeRecorder(10)
	reconciler := &LinodeObjectStorageBucketReconciler{
		Logger: zap.New(
			zap.WriteTo(GinkgoWriter),
			zap.WriteTo(testLogs),
			zap.UseDevMode(true),
		),
		Recorder: recorder,
	}

	BeforeEach(func() {
		// We can use a consistent name since these tests are stateless.
		obj = infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
				UID:       "12345",
			},
			Spec: infrav1.LinodeObjectStorageBucketSpec{
				Cluster: "cluster",
			},
		}
		mockCtrl = gomock.NewController(GinkgoT())
		testLogs = &bytes.Buffer{}
		reconciler.Logger = zap.New(
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

	It("fails when a finalizer cannot be added", Label("current"), func(ctx SpecContext) {
		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		prev := mockK8sClient.EXPECT().
			Scheme().
			Return(scheme.Scheme).
			Times(1)
		mockK8sClient.EXPECT().
			Scheme().
			After(prev).
			Return(runtime.NewScheme()).
			Times(2)

		patchHelper, err := patch.NewHelper(&obj, mockK8sClient)
		Expect(err).NotTo(HaveOccurred())

		// Create a scope directly since only a subset of fields are needed.
		bScope := scope.ObjectStorageBucketScope{
			Client:      mockK8sClient,
			Bucket:      &obj,
			PatchHelper: patchHelper,
		}

		_, err = reconciler.reconcile(ctx, &bScope)
		Expect(err.Error()).To(ContainSubstring("no kind is registered"))
	})

	It("fails when it can't ensure a bucket exists", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("non-404 error")).
			Times(1)

		bScope := scope.ObjectStorageBucketScope{
			LinodeClient: mockLinodeClient,
			Bucket:       &obj,
			Logger:       reconciler.Logger,
		}

		err := reconciler.reconcileApply(ctx, &bScope)
		Expect(err.Error()).To(ContainSubstring("non-404 error"))
		Expect(<-recorder.Events).To(ContainSubstring("non-404 error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to ensure bucket exists"))
	})

	It("fails when it can't provision new access keys", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil).
			Times(1)
		mockLinodeClient.EXPECT().
			CreateObjectStorageKey(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("api error")).
			Times(1)

		bScope := scope.ObjectStorageBucketScope{
			LinodeClient: mockLinodeClient,
			Bucket:       &obj,
			Logger:       reconciler.Logger,
		}

		err := reconciler.reconcileApply(ctx, &bScope)
		Expect(err.Error()).To(ContainSubstring("api error"))
		Expect(<-recorder.Events).To(ContainSubstring("api error"))
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
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("api error")).
			Times(1)

		obj.Spec.KeyGeneration = ptr.To(1)
		obj.Status.LastKeyGeneration = obj.Spec.KeyGeneration
		obj.Status.KeySecretName = ptr.To("mock-access-keys")
		obj.Status.AccessKeyRefs = []int{0, 1}

		bScope := scope.ObjectStorageBucketScope{
			Client:       mockK8sClient,
			LinodeClient: mockLinodeClient,
			Bucket:       &obj,
			Logger:       reconciler.Logger,
		}

		err := reconciler.reconcileApply(ctx, &bScope)
		Expect(err.Error()).To(ContainSubstring("api error"))
		Expect(<-recorder.Events).To(ContainSubstring("api error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to ensure access key secret exists"))
	})

	It("fails when it can't retrieve access keys for a deleted secret", func(ctx SpecContext) {
		mockLinodeClient := mock.NewMockLinodeObjectStorageClient(mockCtrl)
		mockLinodeClient.EXPECT().
			GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil).
			Times(1)
		mockLinodeClient.EXPECT().
			GetObjectStorageKey(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("key creation error")).
			Times(2)

		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-access-keys")).
			Times(1)

		obj.Spec.KeyGeneration = ptr.To(1)
		obj.Status.LastKeyGeneration = obj.Spec.KeyGeneration
		obj.Status.KeySecretName = ptr.To("mock-access-keys")
		obj.Status.AccessKeyRefs = []int{0, 1}

		bScope := scope.ObjectStorageBucketScope{
			Client:       mockK8sClient,
			LinodeClient: mockLinodeClient,
			Bucket:       &obj,
			Logger:       reconciler.Logger,
		}

		err := reconciler.reconcileApply(ctx, &bScope)
		Expect(err.Error()).To(ContainSubstring("key creation error"))
		Expect(<-recorder.Events).To(ContainSubstring("key creation error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to restore access keys for deleted secret"))
	})

	It("fails when it can't generate a secret", func(ctx SpecContext) {
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

		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-access-keys")).
			Times(1)
		mockK8sClient.EXPECT().
			Scheme().
			Return(runtime.NewScheme()).
			Times(1)

		obj.Spec.KeyGeneration = ptr.To(1)
		obj.Status.LastKeyGeneration = obj.Spec.KeyGeneration
		obj.Status.KeySecretName = ptr.To("mock-access-keys")
		obj.Status.AccessKeyRefs = []int{0, 1}

		bScope := scope.ObjectStorageBucketScope{
			Client:       mockK8sClient,
			LinodeClient: mockLinodeClient,
			Bucket:       &obj,
			Logger:       reconciler.Logger,
		}

		err := reconciler.reconcileApply(ctx, &bScope)
		Expect(err.Error()).To(ContainSubstring("no kind is registered"))
		Expect(<-recorder.Events).To(ContainSubstring("keys retrieved"))
		Expect(<-recorder.Events).To(ContainSubstring("no kind is registered"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to generate key secret"))
	})

	It("fails when it can't restore a deleted secret", func(ctx SpecContext) {
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

		mockK8sClient := mock.NewMockk8sClient(mockCtrl)
		mockK8sClient.EXPECT().
			Scheme().
			Return(scheme.Scheme).
			Times(1)
		mockK8sClient.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-access-keys")).
			Times(1)
		mockK8sClient.EXPECT().
			Create(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("secret creation error"))

		obj.Spec.KeyGeneration = ptr.To(1)
		obj.Status.LastKeyGeneration = obj.Spec.KeyGeneration
		obj.Status.KeySecretName = ptr.To("mock-access-keys")
		obj.Status.AccessKeyRefs = []int{0, 1}

		bScope := scope.ObjectStorageBucketScope{
			Client:       mockK8sClient,
			LinodeClient: mockLinodeClient,
			Bucket:       &obj,
			Logger:       reconciler.Logger,
		}

		err := reconciler.reconcileApply(ctx, &bScope)
		Expect(err.Error()).To(ContainSubstring("secret creation error"))
		Expect(<-recorder.Events).To(ContainSubstring("keys retrieved"))
		Expect(<-recorder.Events).To(ContainSubstring("secret creation error"))
		Expect(testLogs.String()).To(ContainSubstring("Failed to apply key secret"))
	})
})
