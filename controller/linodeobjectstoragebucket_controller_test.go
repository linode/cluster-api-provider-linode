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
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"

	// "sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"

	. "github.com/linode/cluster-api-provider-linode/util/testmock"
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

func mockLinodeClientBuilder(m *mock.MockLinodeObjectStorageClient) scope.LinodeObjectStorageClientBuilder {
	return func(_ string) (scope.LinodeObjectStorageClient, error) {
		return m, nil
	}
}

var _ = Describe("lifecycle", Ordered, Label("bucket", "lifecycle"), func() {
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

	for _, path := range Paths(
		Once("resource is created", func(ctx context.Context) {
			Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
		}),
		Either(
			Mock("bucket is created", func(c *mock.MockLinodeObjectStorageClient) {
				getBucket := c.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).Return(nil, nil)
				c.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
					After(getBucket).
					Return(&linodego.ObjectStorageBucket{
						Label:    obj.Name,
						Cluster:  obj.Spec.Cluster,
						Created:  util.Pointer(time.Now()),
						Hostname: "hostname",
					}, nil)
			}),
			Case(
				Mock("bucket is not created", func(c *mock.MockLinodeObjectStorageClient) {
					getBucket := c.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).Return(nil, nil)
					c.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).After(getBucket).Return(nil, errors.New("create bucket error"))
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(c)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&obj),
					})
					Expect(err.Error()).To(ContainSubstring("create bucket error"))
				}),
			),
		),
		Either(
			Mock("keys are created", func(c *mock.MockLinodeObjectStorageClient) {
				for idx := range 2 {
					c.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).
						Return(&linodego.ObjectStorageKey{
							ID:        idx,
							AccessKey: fmt.Sprintf("access-key-%d", idx),
							SecretKey: fmt.Sprintf("secret-key-%d", idx),
						}, nil)
				}
			}),
			Case(
				Mock("keys are not created", func(c *mock.MockLinodeObjectStorageClient) {
					c.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("create key error"))
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(c)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&obj),
					})
					Expect(err.Error()).To(ContainSubstring("create key error"))
				}),
			),
		),
		Result("resource status is updated and key secret is created", func(ctx context.Context, mockLinodeClient *mock.MockLinodeObjectStorageClient) {
			objectKey := client.ObjectKeyFromObject(&obj)
			reconciler.LinodeClientBuilder = mockLinodeClientBuilder(mockLinodeClient)
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: objectKey,
			})
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
			Expect(*obj.Status.KeySecretName).To(Equal(secret.Name))
			Expect(obj.Status.AccessKeyRefs).To(HaveLen(scope.NumAccessKeys))

			By("secret")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
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

			Expect(<-recorder.Events).To(ContainSubstring("Object storage keys assigned"))
			Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
			Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))

			logOutput := testLogs.String()
			Expect(logOutput).To(ContainSubstring("Reconciling apply"))
			Expect(logOutput).To(ContainSubstring("Secret lifecycle-bucket-details was applied with new access keys"))
		}),
		Either(
			Mock("bucket is retrieved on update", func(c *mock.MockLinodeObjectStorageClient) {
				c.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).
					Return(&linodego.ObjectStorageBucket{
						Label:    obj.Name,
						Cluster:  obj.Spec.Cluster,
						Created:  util.Pointer(time.Now()),
						Hostname: "hostname",
					}, nil)
			}),
			Case(
				Mock("bucket is not retrieved on update", func(c *mock.MockLinodeObjectStorageClient) {
					c.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Cluster, gomock.Any()).Return(nil, errors.New("get bucket error"))
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(c)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&obj),
					})
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
				Mock("keys are not rotated", func(c *mock.MockLinodeObjectStorageClient) {
					c.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("create key error"))
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(c)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&obj),
					})
					Expect(err.Error()).To(ContainSubstring("create key error"))
				}),
			),
			Case(
				Mock("keys are rotated", func(c *mock.MockLinodeObjectStorageClient) {
					for idx := range 2 {
						createCall := c.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).
							Return(&linodego.ObjectStorageKey{
								ID:        idx + 2,
								AccessKey: fmt.Sprintf("access-key-%d", idx+2),
								SecretKey: fmt.Sprintf("secret-key-%d", idx+2),
							}, nil)
						c.EXPECT().DeleteObjectStorageKey(gomock.Any(), idx).After(createCall).Return(nil)
					}
				}),
				Result("resource lastKeyGeneration is updated", func(ctx context.Context, mockLinodeClient *mock.MockLinodeObjectStorageClient) {
					objectKey := client.ObjectKeyFromObject(&obj)
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(mockLinodeClient)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: objectKey,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
					Expect(*obj.Status.LastKeyGeneration).To(Equal(1))

					Expect(<-recorder.Events).To(ContainSubstring("Object storage keys assigned"))

					logOutput := testLogs.String()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
					Expect(logOutput).To(ContainSubstring("Secret lifecycle-bucket-details was applied with new access keys"))
				}),
			),
			Once("secret is deleted", func(ctx context.Context) {
				Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
			}),
		),
		Either(
			Case(
				Mock("keys are not retrieved", func(c *mock.MockLinodeObjectStorageClient) {
					c.EXPECT().GetObjectStorageKey(gomock.Any(), gomock.Any()).Times(2).Return(nil, errors.New("get key error"))
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(c)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&obj),
					})
					Expect(err.Error()).To(ContainSubstring("get key error"))
				}),
			),
			Case(
				Mock("keys are retrieved", func(c *mock.MockLinodeObjectStorageClient) {
					for idx := range 2 {
						c.EXPECT().GetObjectStorageKey(gomock.Any(), idx+2).
							Return(&linodego.ObjectStorageKey{
								ID:        idx + 2,
								AccessKey: fmt.Sprintf("access-key-%d", idx+2),
								SecretKey: fmt.Sprintf("secret-key-%d", idx+2),
							}, nil)
					}
				}),
				Result("secret is restored", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(c)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&obj),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
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

					Expect(<-recorder.Events).To(ContainSubstring("Object storage keys retrieved"))
					Expect(<-recorder.Events).To(ContainSubstring("Object storage keys stored in secret"))
					Expect(<-recorder.Events).To(ContainSubstring("Object storage bucket synced"))

					logOutput := testLogs.String()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
					Expect(logOutput).To(ContainSubstring("Secret lifecycle-bucket-details was applied with new access keys"))
				}),
			),
		),
		Once("resource is deleted", func(ctx context.Context) {
			Expect(k8sClient.Delete(ctx, &obj)).To(Succeed())
		}),
		Either(
			Case(
				Mock("keys are not revoked", func(c *mock.MockLinodeObjectStorageClient) {
					c.EXPECT().DeleteObjectStorageKey(gomock.Any(), gomock.Any()).Times(2).Return(errors.New("revoke error"))
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(c)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&obj),
					})
					Expect(err.Error()).To(ContainSubstring("revoke error"))
				}),
			),
			Case(
				Mock("keys are revoked", func(c *mock.MockLinodeObjectStorageClient) {
					c.EXPECT().DeleteObjectStorageKey(gomock.Any(), 2).Return(nil)
					c.EXPECT().DeleteObjectStorageKey(gomock.Any(), 3).Return(nil)
				}),
				Result("finalizer is removed", func(ctx context.Context, mockLinodeClient *mock.MockLinodeObjectStorageClient) {
					reconciler.LinodeClientBuilder = mockLinodeClientBuilder(mockLinodeClient)
					objectKey := client.ObjectKeyFromObject(&obj)
					_, err := reconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: objectKey,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &obj))).To(BeTrue())

					Expect(<-recorder.Events).To(ContainSubstring("Object storage keys revoked"))

					logOutput := testLogs.String()
					Expect(logOutput).To(ContainSubstring("Reconciling delete"))
				}),
			),
		),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			Run(path, GinkgoT(), ctx, mock.NewMockLinodeObjectStorageClient(mockCtrl))
		})
	}
})

var _ = Describe("pre-reconcile", Label("bucket", "pre-reconcile"), func() {
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
		mockK8sClient := mock.NewMockK8sClient(mockCtrl)
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

var _ = Describe("corners", Label("bucket", "corners"), func() {
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

	for _, path := range Paths(
		Mock("scheme with no infrav1alpha1", func(c *mock.MockK8sClient) {
			prev := c.EXPECT().Scheme().Return(scheme.Scheme)
			c.EXPECT().Scheme().After(prev).Return(runtime.NewScheme()).Times(2)
		}),
		Result("error", func(ctx context.Context, c *mock.MockK8sClient) {
			patchHelper, err := patch.NewHelper(&obj, c)
			Expect(err).NotTo(HaveOccurred())

			// Create a scope directly since only a subset of fields are needed.
			bScope := scope.ObjectStorageBucketScope{
				Client:      c,
				Bucket:      &obj,
				PatchHelper: patchHelper,
			}

			_, err = reconciler.reconcile(ctx, &bScope)
			Expect(err.Error()).To(ContainSubstring("no kind is registered"))
		}),
		Mock("get bucket", func(c *mock.MockLinodeObjectStorageClient) {
			c.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{Created: ptr.To(time.Now())}, nil)
		}),
		Either(
			Case(
				Mock("failed check for deleted secret", func(c *mock.MockK8sClient) {
					c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("api error"))
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient, kc *mock.MockK8sClient) {
					obj.Spec.KeyGeneration = ptr.To(1)
					obj.Status.LastKeyGeneration = obj.Spec.KeyGeneration
					obj.Status.KeySecretName = ptr.To("mock-bucket-details")
					obj.Status.AccessKeyRefs = []int{0, 1}

					bScope := scope.ObjectStorageBucketScope{
						Client:       kc,
						LinodeClient: c,
						Bucket:       &obj,
						Logger:       reconciler.Logger,
					}

					err := reconciler.reconcileApply(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("api error"))
					Expect(<-recorder.Events).To(ContainSubstring("api error"))
					Expect(testLogs.String()).To(ContainSubstring("Failed to ensure access key secret exists"))
				}),
			),
			Mock("secret deleted", func(c *mock.MockK8sClient) {
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, "mock-bucket-details"))
			}),
		),
		Mock("get keys", func(c *mock.MockLinodeObjectStorageClient) {
			for idx := range 2 {
				c.EXPECT().GetObjectStorageKey(gomock.Any(), idx).Return(&linodego.ObjectStorageKey{ID: idx}, nil)
			}
		}),
		Either(
			Case(
				Mock("secret resource creation fails", func(c *mock.MockK8sClient) {
					c.EXPECT().Scheme().Return(scheme.Scheme).AnyTimes()
					c.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("secret creation error"))
				}),
				Result("creation error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient, kc *mock.MockK8sClient) {
					obj.Spec.KeyGeneration = ptr.To(1)
					obj.Status.LastKeyGeneration = obj.Spec.KeyGeneration
					obj.Status.KeySecretName = ptr.To("mock-bucket-details")
					obj.Status.AccessKeyRefs = []int{0, 1}

					bScope := scope.ObjectStorageBucketScope{
						Client:       kc,
						LinodeClient: c,
						Bucket:       &obj,
						Logger:       reconciler.Logger,
					}

					err := reconciler.reconcileApply(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("secret creation error"))
					Expect(<-recorder.Events).To(ContainSubstring("keys retrieved"))
					Expect(<-recorder.Events).To(ContainSubstring("secret creation error"))
					Expect(testLogs.String()).To(ContainSubstring("Failed to apply key secret"))
				}),
			),
			Case(
				Mock("secret generation fails", func(c *mock.MockK8sClient) {
					c.EXPECT().Scheme().Return(runtime.NewScheme())
				}),
				Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient, kc *mock.MockK8sClient) {
					obj.Spec.KeyGeneration = ptr.To(1)
					obj.Status.LastKeyGeneration = obj.Spec.KeyGeneration
					obj.Status.KeySecretName = ptr.To("mock-bucket-details")
					obj.Status.AccessKeyRefs = []int{0, 1}

					bScope := scope.ObjectStorageBucketScope{
						Client:       kc,
						LinodeClient: c,
						Bucket:       &obj,
						Logger:       reconciler.Logger,
					}

					err := reconciler.reconcileApply(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("no kind is registered"))
					Expect(<-recorder.Events).To(ContainSubstring("keys retrieved"))
					Expect(<-recorder.Events).To(ContainSubstring("no kind is registered"))
					Expect(testLogs.String()).To(ContainSubstring("Failed to generate key secret"))
				}),
			),
		),
		Once("finalizer is missing", func(ctx context.Context) {
			obj.Status.AccessKeyRefs = []int{0, 1}
			obj.ObjectMeta.Finalizers = []string{}
		}),
		Mock("revoke keys", func(c *mock.MockLinodeObjectStorageClient) {
			c.EXPECT().DeleteObjectStorageKey(gomock.Any(), gomock.Any()).Times(2).Return(nil)
		}),
		Result("error", func(ctx context.Context, c *mock.MockLinodeObjectStorageClient, kc *mock.MockK8sClient) {
			bScope := scope.ObjectStorageBucketScope{
				Client:       kc,
				LinodeClient: c,
				Bucket:       &obj,
				Logger:       reconciler.Logger,
			}

			err := reconciler.reconcileDelete(ctx, &bScope)
			Expect(err.Error()).To(ContainSubstring("failed to remove finalizer from bucket"))
			Expect(<-recorder.Events).To(ContainSubstring("failed to remove finalizer from bucket"))
		}),
	) {
		It(path.Describe(), func(ctx SpecContext) {
			Run(path, GinkgoT(), ctx,
				mock.NewMockLinodeObjectStorageClient(mockCtrl),
				mock.NewMockK8sClient(mockCtrl),
			)
		})
	}
})
