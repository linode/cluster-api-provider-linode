/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusteraddonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("lifecycle", Ordered, Label("key", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	key := infrav1.LinodeObjectStorageKey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: "default",
		},
		Spec: infrav1.LinodeObjectStorageKeySpec{
			BucketAccess: []infrav1.BucketAccessRef{
				{
					BucketName:  "mybucket",
					Permissions: "read_only",
					Region:      "us-ord",
				},
			},
		},
	}

	keyScope := scope.ObjectStorageKeyScope{
		Key: &key,
	}

	reconciler := LinodeObjectStorageKeyReconciler{}

	BeforeAll(func(ctx SpecContext) {
		keyScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &key)).To(Succeed())
	})

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
		keyScope.Logger = mck.Logger()

		objectKey := client.ObjectKeyFromObject(&key)
		Expect(k8sClient.Get(ctx, objectKey, &key)).To(Succeed())

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&key, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		keyScope.PatchHelper = patchHelper
	})

	suite.Run(
		OneOf(
			Path(
				Call("key is not created", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("create key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err.Error()).To(ContainSubstring("create key error"))
				}),
			),
			Path(
				Call("key is created", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().CreateObjectStorageKey(ctx, gomock.Any()).
						Return(&linodego.ObjectStorageKey{
							ID:        1,
							AccessKey: "access-key-1",
							SecretKey: "secret-key-1",
						}, nil)
				}),
				Result("resources are updated", func(ctx context.Context, mck Mock) {
					objectKey := client.ObjectKeyFromObject(&key)
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err).NotTo(HaveOccurred())

					By("status")
					Expect(k8sClient.Get(ctx, objectKey, &key)).To(Succeed())
					Expect(key.Status.Ready).To(BeTrue())
					Expect(key.Status.Conditions).To(HaveLen(1))
					Expect(key.Status.Conditions[0].Type).To(Equal(clusterv1.ReadyCondition))
					Expect(key.Status.CreationTime).NotTo(BeNil())
					Expect(*key.Status.LastKeyGeneration).To(Equal(key.Spec.KeyGeneration))
					Expect(*key.Status.LastKeyGeneration).To(Equal(0))
					Expect(*key.Status.AccessKeyRef).To(Equal(1))

					By("secret")
					var secret corev1.Secret
					secretKey := client.ObjectKey{Namespace: "default", Name: *key.Status.SecretName}
					Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
					Expect(secret.Data).To(HaveLen(2))
					Expect(string(secret.Data["access_key"])).To(Equal("access-key-1"))
					Expect(string(secret.Data["secret_key"])).To(Equal("secret-key-1"))

					events := mck.Events()
					Expect(events).To(ContainSubstring("Object storage key assigned"))
					Expect(events).To(ContainSubstring("Object storage key stored in secret"))
					Expect(events).To(ContainSubstring("Object storage key synced"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
					Expect(logOutput).To(ContainSubstring("Secret %s was created with access key", *key.Status.SecretName))
				}),
			),
		),
		Call("keyGeneration is modified", func(ctx context.Context, _ Mock) {
			key.Spec.KeyGeneration = 1
			Expect(k8sClient.Update(ctx, &key)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("key is not rotated", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("rotate key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err.Error()).To(ContainSubstring("rotate key error"))
				}),
			),
			Path(
				Call("key is rotated", func(ctx context.Context, mck Mock) {
					createCall := mck.LinodeClient.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).
						Return(&linodego.ObjectStorageKey{
							ID:        2,
							AccessKey: "access-key-2",
							SecretKey: "secret-key-2",
						}, nil)
					mck.LinodeClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), 1).After(createCall).Return(nil)
				}),
				Result("resources are updated", func(ctx context.Context, mck Mock) {
					objectKey := client.ObjectKeyFromObject(&key)
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err).NotTo(HaveOccurred())

					By("status")
					Expect(k8sClient.Get(ctx, objectKey, &key)).To(Succeed())
					Expect(*key.Status.LastKeyGeneration).To(Equal(1))
					Expect(*key.Status.AccessKeyRef).To(Equal(2))

					By("secret")
					var secret corev1.Secret
					secretKey := client.ObjectKey{Namespace: "default", Name: *key.Status.SecretName}
					Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
					Expect(secret.Data).To(HaveLen(2))
					Expect(string(secret.Data["access_key"])).To(Equal("access-key-2"))
					Expect(string(secret.Data["secret_key"])).To(Equal("secret-key-2"))

					events := mck.Events()
					Expect(events).To(ContainSubstring("Object storage key assigned"))
					Expect(events).To(ContainSubstring("Object storage key stored in secret"))
					Expect(events).To(ContainSubstring("Object storage key synced"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
					Expect(logOutput).To(ContainSubstring("Secret %s was updated with access key", *key.Status.SecretName))
				}),
			),
		),
		Once("secret is deleted", func(ctx context.Context, _ Mock) {
			var secret corev1.Secret
			secretKey := client.ObjectKey{Namespace: "default", Name: *key.Status.SecretName}
			Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("(secret is deleted) > key is not retrieved", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageKey(gomock.Any(), 2).Return(nil, errors.New("get key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err.Error()).To(ContainSubstring("get key error"))
				}),
			),
			Path(
				Call("(secret is deleted) > key is retrieved", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageKey(gomock.Any(), 2).
						Return(&linodego.ObjectStorageKey{
							ID:        2,
							AccessKey: "access-key-2",
							SecretKey: "secret-key-2",
						}, nil)
				}),
				Result("secret is recreated", func(ctx context.Context, mck Mock) {
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err).NotTo(HaveOccurred())

					var secret corev1.Secret
					secretKey := client.ObjectKey{Namespace: "default", Name: *key.Status.SecretName}
					Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
					Expect(secret.Data).To(HaveLen(2))
					Expect(string(secret.Data["access_key"])).To(Equal("access-key-2"))
					Expect(string(secret.Data["secret_key"])).To(Equal("secret-key-2"))

					events := mck.Events()
					Expect(events).To(ContainSubstring("Object storage key retrieved"))
					Expect(events).To(ContainSubstring("Object storage key stored in secret"))
					Expect(events).To(ContainSubstring("Object storage key synced"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
					Expect(logOutput).To(ContainSubstring("Secret %s was created with access key", *key.Status.SecretName))
				}),
			),
		),
		Once("secretType set to cluster resource set", func(ctx context.Context, _ Mock) {
			key.Spec.SecretType = clusteraddonsv1.ClusterResourceSetSecretType
			Expect(k8sClient.Update(ctx, &key)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("(secretType set to cluster resource set) > key is not retrieved", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageKey(gomock.Any(), 2).Return(nil, errors.New("get key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err.Error()).To(ContainSubstring("get key error"))
				}),
			),
			Path(
				Call("(secretType set to cluster resource set) > key is retrieved", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageKey(gomock.Any(), 2).
						Return(&linodego.ObjectStorageKey{
							ID:        2,
							AccessKey: "access-key-2",
							SecretKey: "secret-key-2",
						}, nil)
				}),
				OneOf(
					Path(
						Call("bucket is not retrieved", func(ctx context.Context, mck Mock) {
							mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), "us-ord", "mybucket").Return(nil, errors.New("get bucket error"))
						}),
						Result("error", func(ctx context.Context, mck Mock) {
							keyScope.LinodeClient = mck.LinodeClient
							_, err := reconciler.reconcile(ctx, &keyScope)
							Expect(err.Error()).To(ContainSubstring("get bucket error"))
						}),
					),
					Path(
						Call("bucket is retrieved", func(ctx context.Context, mck Mock) {
							mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), "us-ord", "mybucket").Return(&linodego.ObjectStorageBucket{
								Label:    "mybucket",
								Region:   "us-ord",
								Hostname: "mybucket.us-ord-1.linodeobjects.com",
							}, nil)
						}),
						Result("secret is recreated as cluster resource set type", func(ctx context.Context, mck Mock) {
							keyScope.LinodeClient = mck.LinodeClient
							_, err := reconciler.reconcile(ctx, &keyScope)
							Expect(err).NotTo(HaveOccurred())

							var secret corev1.Secret
							secretKey := client.ObjectKey{Namespace: "default", Name: *key.Status.SecretName}
							Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
							Expect(secret.Data).To(HaveLen(1))
							Expect(string(secret.Data[scope.ClusterResourceSetSecretFilename])).To(Equal(fmt.Sprintf(scope.BucketKeySecret,
								*key.Status.SecretName,
								"mybucket",
								"us-ord",
								"mybucket.us-ord-1.linodeobjects.com",
								"access-key-2",
								"secret-key-2",
							)))

							events := mck.Events()
							Expect(events).To(ContainSubstring("Object storage key retrieved"))
							Expect(events).To(ContainSubstring("Object storage key stored in secret"))
							Expect(events).To(ContainSubstring("Object storage key synced"))

							logOutput := mck.Logs()
							Expect(logOutput).To(ContainSubstring("Reconciling apply"))
							Expect(logOutput).To(ContainSubstring("Secret %s was created with access key", *key.Status.SecretName))
						}),
					),
				),
			),
		),
		Once("resource is deleted", func(ctx context.Context, _ Mock) {
			// nb: client.Delete does not set DeletionTimestamp on the object, so re-fetch from the apiserver.
			objectKey := client.ObjectKeyFromObject(&key)
			Expect(k8sClient.Delete(ctx, &key)).To(Succeed())
			Expect(k8sClient.Get(ctx, objectKey, &key)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("(resource is deleted) > key is not revoked", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), 2).Return(errors.New("revoke key error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err.Error()).To(ContainSubstring("revoke key error"))
				}),
			),
			Path(
				Call("(resource is deleted) > key is revoked", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().DeleteObjectStorageKey(gomock.Any(), 2).Return(nil)
				}),
				Result("finalizer is removed, resource is not found", func(ctx context.Context, mck Mock) {
					objectKey := client.ObjectKeyFromObject(&key)
					k8sClient.Get(ctx, objectKey, &key)
					keyScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &keyScope)
					Expect(err).NotTo(HaveOccurred())
					Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &key))).To(BeTrue())

					events := mck.Events()
					Expect(events).To(ContainSubstring("Object storage key revoked"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling delete"))
				}),
			),
		),
	)
})
