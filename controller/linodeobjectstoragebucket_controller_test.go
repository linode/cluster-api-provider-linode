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
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type accessKeySecret struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	StringData struct {
		BucketName     string `json:"bucket_name"`
		BucketRegion   string `json:"bucket_region"`
		BucketEndpoint string `json:"bucket_endpoint"`
		AccessKeyRW    string `json:"access_key_rw"`
		SecretKeyRW    string `json:"secret_key_rw"`
		AccessKeyRO    string `json:"access_key_ro"`
		SecretKeyRO    string `json:"secret_key_ro"`
	} `json:"stringData"`
}

var _ = Describe("lifecycle", Ordered, Label("bucket", "lifecycle"), func() {
	obj := infrav1.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: "default",
		},
		Spec: infrav1.LinodeObjectStorageBucketSpec{
			Cluster: "cluster",
		},
	}

	ctlrSuite := NewControllerTestSuite(mock.MockLinodeObjectStorageClient{})
	reconciler := LinodeObjectStorageBucketReconciler{
		Recorder: ctlrSuite.Recorder(),
	}
	bScope := scope.ObjectStorageBucketScope{
		Bucket: &obj,
		Logger: ctlrSuite.Logger(),
	}

	BeforeAll(func(ctx SpecContext) {
		bScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
	})

	BeforeEach(func(ctx SpecContext) {
		objectKey := client.ObjectKey{Name: "lifecycle", Namespace: "default"}
		Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&obj, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		bScope.PatchHelper = patchHelper
	})

	ctlrSuite.Run(Paths(
		Either(
			Call("bucket is created", func(ctx context.Context, mck Mock) {
				getBucket := mck.ObjectStorageClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).Return(nil, nil)
				mck.ObjectStorageClient.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
					After(getBucket).
					Return(&linodego.ObjectStorageBucket{
						Label:    obj.Name,
						Cluster:  obj.Spec.Cluster,
						Created:  util.Pointer(time.Now()),
						Hostname: "hostname",
					}, nil)
			}),
			Case(
				Call("bucket is not created", func(ctx context.Context, mck Mock) {
					getBucket := mck.ObjectStorageClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).Return(nil, nil)
					mck.ObjectStorageClient.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).After(getBucket).Return(nil, errors.New("create bucket error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("create bucket error"))
				}),
			),
		),
		Either(
			Call("keys are created", func(ctx context.Context, mck Mock) {
				for idx := range 2 {
					mck.ObjectStorageClient.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).
						Return(&linodego.ObjectStorageKey{
							ID:        idx,
							AccessKey: fmt.Sprintf("access-key-%d", idx),
							SecretKey: fmt.Sprintf("secret-key-%d", idx),
						}, nil)
				}
			}),
			Case(
				Call("keys are not created", func(ctx context.Context, mck Mock) {
					mck.ObjectStorageClient.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("create key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("create key error"))
				}),
			),
		),
		Result("resource status is updated and key secret is created", func(ctx context.Context, mck Mock) {
			objectKey := client.ObjectKeyFromObject(&obj)
			bScope.LinodeClient = mck.ObjectStorageClient
			_, err := reconciler.reconcile(ctx, &bScope)
			Expect(err).NotTo(HaveOccurred())

			By("status")
			Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
			Expect(obj.Status.Ready).To(BeTrue())
			Expect(obj.Status.Conditions).To(HaveLen(1))
			Expect(obj.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))
			Expect(*obj.Status.Hostname).To(Equal("hostname"))
			Expect(obj.Status.CreationTime).NotTo(BeNil())
			Expect(*obj.Status.LastKeyGeneration).To(Equal(*obj.Spec.KeyGeneration))
			Expect(*obj.Status.LastKeyGeneration).To(Equal(0))
			Expect(*obj.Status.KeySecretName).To(Equal(fmt.Sprintf(scope.AccessKeyNameTemplate, "lifecycle")))
			Expect(obj.Status.AccessKeyRefs).To(HaveLen(scope.NumAccessKeys))

			By("secret")
			var secret corev1.Secret
			secretKey := client.ObjectKey{Namespace: "default", Name: *obj.Status.KeySecretName}
			Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
			Expect(secret.Data).To(HaveLen(1))

			var key accessKeySecret
			Expect(yaml.Unmarshal(secret.Data["bucket-details-secret.yaml"], &key)).NotTo(HaveOccurred())
			Expect(key.StringData.BucketName).To(Equal("lifecycle"))
			Expect(key.StringData.BucketRegion).To(Equal("cluster"))
			Expect(key.StringData.BucketEndpoint).To(Equal("hostname"))
			Expect(key.StringData.AccessKeyRW).To(Equal("access-key-0"))
			Expect(key.StringData.SecretKeyRW).To(Equal("secret-key-0"))
			Expect(key.StringData.AccessKeyRO).To(Equal("access-key-1"))
			Expect(key.StringData.SecretKeyRO).To(Equal("secret-key-1"))

			Expect(<-mck.Events()).To(ContainSubstring("Object storage keys assigned"))
			Expect(<-mck.Events()).To(ContainSubstring("Object storage keys stored in secret"))
			Expect(<-mck.Events()).To(ContainSubstring("Object storage bucket synced"))

			logOutput := mck.Logs()
			Expect(logOutput).To(ContainSubstring("Reconciling apply"))
			Expect(logOutput).To(ContainSubstring("Secret lifecycle-bucket-details was applied with new access keys"))
		}),
		Either(
			Call("bucket is retrieved on update", func(ctx context.Context, mck Mock) {
				mck.ObjectStorageClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
					Return(&linodego.ObjectStorageBucket{
						Label:    obj.Name,
						Cluster:  obj.Spec.Cluster,
						Created:  util.Pointer(time.Now()),
						Hostname: "hostname",
					}, nil)
			}),
			Case(
				Call("bucket is not retrieved on update", func(ctx context.Context, mck Mock) {
					mck.ObjectStorageClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).Return(nil, errors.New("get bucket error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("get bucket error"))
				}),
			),
		),
		Once("resource keyGeneration is modified", func(ctx context.Context) {
			objectKey := client.ObjectKeyFromObject(&obj)
			Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
			obj.Spec.KeyGeneration = ptr.To(1)
			Expect(k8sClient.Update(ctx, &obj)).To(Succeed())
		}),
		Either(
			// nb: Order matters for paths of the same length. The leftmost path is evaluated first.
			// If we evaluate the happy path first, the bucket resource is mutated so the error path won't occur.
			Case(
				Call("keys are not rotated", func(ctx context.Context, mck Mock) {
					mck.ObjectStorageClient.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("create key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("create key error"))
				}),
			),
			Case(
				Call("keys are rotated", func(ctx context.Context, mck Mock) {
					for idx := range 2 {
						createCall := mck.ObjectStorageClient.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).
							Return(&linodego.ObjectStorageKey{
								ID:        idx + 2,
								AccessKey: fmt.Sprintf("access-key-%d", idx+2),
								SecretKey: fmt.Sprintf("secret-key-%d", idx+2),
							}, nil)
						mck.ObjectStorageClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), idx).After(createCall).Return(nil)
					}
				}),
				Result("resource lastKeyGeneration is updated", func(ctx context.Context, mck Mock) {
					objectKey := client.ObjectKeyFromObject(&obj)
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
					Expect(*obj.Status.LastKeyGeneration).To(Equal(1))

					Expect(<-mck.Events()).To(ContainSubstring("Object storage keys assigned"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
					Expect(logOutput).To(ContainSubstring("Secret lifecycle-bucket-details was applied with new access keys"))
				}),
			),
			Once("secret is deleted", func(ctx context.Context) {
				var secret corev1.Secret
				secretKey := client.ObjectKey{Namespace: "default", Name: *obj.Status.KeySecretName}
				Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
				Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
			}),
		),
		Either(
			Case(
				Call("keys are not retrieved", func(ctx context.Context, mck Mock) {
					mck.ObjectStorageClient.EXPECT().GetObjectStorageKey(gomock.Any(), gomock.Any()).Times(2).Return(nil, errors.New("get key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("get key error"))
				}),
			),
			Case(
				Call("keys are retrieved", func(ctx context.Context, mck Mock) {
					for idx := range 2 {
						mck.ObjectStorageClient.EXPECT().GetObjectStorageKey(gomock.Any(), idx+2).
							Return(&linodego.ObjectStorageKey{
								ID:        idx + 2,
								AccessKey: fmt.Sprintf("access-key-%d", idx+2),
								SecretKey: fmt.Sprintf("secret-key-%d", idx+2),
							}, nil)
					}
				}),
				Result("secret is restored", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err).NotTo(HaveOccurred())

					var secret corev1.Secret
					secretKey := client.ObjectKey{Namespace: "default", Name: *obj.Status.KeySecretName}
					Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
					Expect(secret.Data).To(HaveLen(1))

					var key accessKeySecret
					Expect(yaml.Unmarshal(secret.Data["bucket-details-secret.yaml"], &key)).NotTo(HaveOccurred())
					Expect(key.StringData.BucketName).To(Equal("lifecycle"))
					Expect(key.StringData.BucketRegion).To(Equal("cluster"))
					Expect(key.StringData.BucketEndpoint).To(Equal("hostname"))
					Expect(key.StringData.AccessKeyRW).To(Equal("access-key-2"))
					Expect(key.StringData.SecretKeyRW).To(Equal("secret-key-2"))
					Expect(key.StringData.AccessKeyRO).To(Equal("access-key-3"))
					Expect(key.StringData.SecretKeyRO).To(Equal("secret-key-3"))

					Expect(<-mck.Events()).To(ContainSubstring("Object storage keys retrieved"))
					Expect(<-mck.Events()).To(ContainSubstring("Object storage keys stored in secret"))
					Expect(<-mck.Events()).To(ContainSubstring("Object storage bucket synced"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
					Expect(logOutput).To(ContainSubstring("Secret lifecycle-bucket-details was applied with new access keys"))
				}),
			),
		),
		Once("resource is deleted", func(ctx context.Context) {
			// nb: client.Delete does not set DeletionTimestamp on the object, so re-fetch from the apiserver.
			objectKey := client.ObjectKeyFromObject(&obj)
			Expect(k8sClient.Delete(ctx, &obj)).To(Succeed())
			Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
		}),
		Either(
			Case(
				Call("keys are not revoked", func(ctx context.Context, mck Mock) {
					mck.ObjectStorageClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), gomock.Any()).Times(2).Return(errors.New("revoke error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("revoke error"))
				}),
			),
			Case(
				Call("keys are revoked", func(ctx context.Context, mck Mock) {
					mck.ObjectStorageClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), 2).Return(nil)
					mck.ObjectStorageClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), 3).Return(nil)
				}),
				Result("finalizer is removed", func(ctx context.Context, mck Mock) {
					objectKey := client.ObjectKeyFromObject(&obj)
					k8sClient.Get(ctx, objectKey, &obj)
					bScope.LinodeClient = mck.ObjectStorageClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &obj))).To(BeTrue())

					Expect(<-mck.Events()).To(ContainSubstring("Object storage keys revoked"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling delete"))
				}),
			),
		),
	))
})

var _ = Describe("errors", Label("bucket", "errors"), func() {
	ctlrSuite := NewControllerTestSuite(
		mock.MockLinodeObjectStorageClient{},
		mock.MockK8sClient{},
	)

	reconciler := LinodeObjectStorageBucketReconciler{Recorder: ctlrSuite.Recorder()}
	bScope := scope.ObjectStorageBucketScope{Logger: ctlrSuite.Logger()}

	BeforeEach(func() {
		// Reset obj to base state to be modified in each test path.
		// We can use a consistent name since these tests are stateless.
		bScope.Bucket = &infrav1.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
				UID:       "12345",
			},
			Spec: infrav1.LinodeObjectStorageBucketSpec{
				Cluster: "cluster",
			},
		}
	})

	ctlrSuite.Run(Paths(
		Either(
			Call("resource can be fetched", func(ctx context.Context, mck Mock) {
				mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}),
			Case(
				Call("resource is not found", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{}, "mock"))
				}),
				Result("no error", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(bScope.Bucket),
					})
					Expect(err).NotTo(HaveOccurred())
				}),
			),
			Case(
				Call("resource can't be fetched", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("non-404 error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					reconciler.Client = mck.K8sClient
					reconciler.Logger = bScope.Logger
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(bScope.Bucket),
					})
					Expect(err.Error()).To(ContainSubstring("non-404 error"))
					Expect(mck.Logs()).To(ContainSubstring("Failed to fetch LinodeObjectStorageBucket"))
				}),
			),
		),
		Result("scope params is missing args", func(ctx context.Context, mck Mock) {
			reconciler.Client = mck.K8sClient
			reconciler.Logger = bScope.Logger
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(bScope.Bucket),
			})
			Expect(err.Error()).To(ContainSubstring("failed to create object storage bucket scope"))
			Expect(mck.Logs()).To(ContainSubstring("Failed to create object storage bucket scope"))
		}),
		Call("scheme with no infrav1alpha1", func(ctx context.Context, mck Mock) {
			prev := mck.K8sClient.EXPECT().Scheme().Return(scheme.Scheme)
			mck.K8sClient.EXPECT().Scheme().After(prev).Return(runtime.NewScheme()).Times(2)
		}),
		Result("error", func(ctx context.Context, mck Mock) {
			bScope.Client = mck.K8sClient

			patchHelper, err := patch.NewHelper(bScope.Bucket, mck.K8sClient)
			Expect(err).NotTo(HaveOccurred())
			bScope.PatchHelper = patchHelper

			_, err = reconciler.reconcile(ctx, &bScope)
			Expect(err.Error()).To(ContainSubstring("no kind is registered"))
		}),
		Call("get bucket", func(ctx context.Context, mck Mock) {
			mck.ObjectStorageClient.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil)
		}),
		Either(
			Case(
				Call("failed check for deleted secret", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("api error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.Bucket.Spec.KeyGeneration = ptr.To(1)
					bScope.Bucket.Status.LastKeyGeneration = bScope.Bucket.Spec.KeyGeneration
					bScope.Bucket.Status.KeySecretName = ptr.To("mock-bucket-details")
					bScope.Bucket.Status.AccessKeyRefs = []int{0, 1}

					bScope.LinodeClient = mck.ObjectStorageClient
					bScope.Client = mck.K8sClient
					err := reconciler.reconcileApply(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("api error"))
					Expect(<-mck.Events()).To(ContainSubstring("api error"))
					Expect(mck.Logs()).To(ContainSubstring("Failed to ensure access key secret exists"))
				}),
			),
			Call("secret deleted", func(ctx context.Context, mck Mock) {
				mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-bucket-details"))
			}),
		),
		Call("get keys", func(ctx context.Context, mck Mock) {
			for idx := range 2 {
				mck.ObjectStorageClient.EXPECT().GetObjectStorageKey(gomock.Any(), idx).Return(&linodego.ObjectStorageKey{ID: idx}, nil)
			}
		}),
		Either(
			Case(
				Call("secret resource creation fails", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Scheme().Return(scheme.Scheme).AnyTimes()
					mck.K8sClient.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("secret creation error"))
				}),
				Result("creation error", func(ctx context.Context, mck Mock) {
					bScope.Bucket.Spec.KeyGeneration = ptr.To(1)
					bScope.Bucket.Status.LastKeyGeneration = bScope.Bucket.Spec.KeyGeneration
					bScope.Bucket.Status.KeySecretName = ptr.To("mock-bucket-details")
					bScope.Bucket.Status.AccessKeyRefs = []int{0, 1}

					bScope.LinodeClient = mck.ObjectStorageClient
					bScope.Client = mck.K8sClient
					err := reconciler.reconcileApply(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("secret creation error"))
					Expect(<-mck.Events()).To(ContainSubstring("keys retrieved"))
					Expect(<-mck.Events()).To(ContainSubstring("secret creation error"))
					Expect(mck.Logs()).To(ContainSubstring("Failed to apply key secret"))
				}),
			),
			Case(
				Call("secret generation fails", func(ctx context.Context, mck Mock) {
					mck.K8sClient.EXPECT().Scheme().Return(runtime.NewScheme())
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.Bucket.Spec.KeyGeneration = ptr.To(1)
					bScope.Bucket.Status.LastKeyGeneration = bScope.Bucket.Spec.KeyGeneration
					bScope.Bucket.Status.KeySecretName = ptr.To("mock-bucket-details")
					bScope.Bucket.Status.AccessKeyRefs = []int{0, 1}

					bScope.LinodeClient = mck.ObjectStorageClient
					bScope.Client = mck.K8sClient
					err := reconciler.reconcileApply(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("no kind is registered"))
					Expect(<-mck.Events()).To(ContainSubstring("keys retrieved"))
					Expect(<-mck.Events()).To(ContainSubstring("no kind is registered"))
					Expect(mck.Logs()).To(ContainSubstring("Failed to generate key secret"))
				}),
			),
		),
		Once("finalizer is missing", func(ctx context.Context) {
			bScope.Bucket.Status.AccessKeyRefs = []int{0, 1}
			bScope.Bucket.ObjectMeta.Finalizers = []string{}
		}),
		Call("revoke keys", func(ctx context.Context, mck Mock) {
			mck.ObjectStorageClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), gomock.Any()).Times(2).Return(nil)
		}),
		Result("error", func(ctx context.Context, mck Mock) {
			bScope.LinodeClient = mck.ObjectStorageClient
			bScope.Client = mck.K8sClient
			err := reconciler.reconcileDelete(ctx, &bScope)
			Expect(err.Error()).To(ContainSubstring("failed to remove finalizer from bucket"))
			Expect(<-mck.Events()).To(ContainSubstring("failed to remove finalizer from bucket"))
		}),
	))
})
