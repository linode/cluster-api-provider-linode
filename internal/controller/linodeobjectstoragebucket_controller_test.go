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
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"time"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("lifecycle", Ordered, Label("bucket", "lifecycle"), func() {
	suite := NewControllerSuite(GinkgoT(), mock.MockLinodeClient{})

	obj := infrav1alpha2.LinodeObjectStorageBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle",
			Namespace: "default",
		},
		Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
			Region: "region",
		},
	}

	bScope := scope.ObjectStorageBucketScope{
		Bucket: &obj,
	}

	reconciler := LinodeObjectStorageBucketReconciler{}

	BeforeAll(func(ctx SpecContext) {
		bScope.Client = k8sClient
		Expect(k8sClient.Create(ctx, &obj)).To(Succeed())
	})

	suite.BeforeEach(func(ctx context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
		bScope.Logger = mck.Logger()

		objectKey := client.ObjectKey{Name: "lifecycle", Namespace: "default"}
		Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())

		// Create patch helper with latest state of resource.
		// This is only needed when relying on envtest's k8sClient.
		patchHelper, err := patch.NewHelper(&obj, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		bScope.PatchHelper = patchHelper
	})

	suite.Run(
		OneOf(
			Path(
				Call("bucket is created", func(ctx context.Context, mck Mock) {
					getBucket := mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Region, gomock.Any()).Return(nil, nil)
					mck.LinodeClient.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
						After(getBucket).
						Return(&linodego.ObjectStorageBucket{
							Label:    "bucket",
							Region:   obj.Spec.Region,
							Created:  util.Pointer(time.Now()),
							Hostname: "hostname",
						}, nil)
				}),
				Result("resource status is updated", func(ctx context.Context, mck Mock) {
					objectKey := client.ObjectKeyFromObject(&obj)
					bScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err).NotTo(HaveOccurred())
					// second one is the real thing
					_, err = reconciler.reconcile(ctx, &bScope)
					Expect(err).NotTo(HaveOccurred())

					By("status")
					Expect(k8sClient.Get(ctx, objectKey, &obj)).To(Succeed())
					Expect(obj.Status.Ready).To(BeTrue())
					Expect(obj.Status.FailureMessage).To(BeNil())
					Expect(obj.Status.Conditions).To(HaveLen(2))
					readyCond := conditions.Get(&obj, string(clusterv1.ReadyCondition))
					Expect(readyCond).NotTo(BeNil())
					Expect(*obj.Status.Hostname).To(Equal("hostname"))
					Expect(obj.Status.CreationTime).NotTo(BeNil())

					events := mck.Events()
					Expect(events).To(ContainSubstring("Object storage bucket synced"))

					logOutput := mck.Logs()
					Expect(logOutput).To(ContainSubstring("Reconciling apply"))
				}),
			),
			Path(
				Call("bucket is not created", func(ctx context.Context, mck Mock) {
					getBucket := mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Region, gomock.Any()).Return(nil, nil)
					mck.LinodeClient.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).After(getBucket).Return(nil, errors.New("create bucket error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("create bucket error"))
				}),
			),
		),
		Once("resource ACL is modified", func(ctx context.Context, _ Mock) {
			obj.Spec.ACL = infrav1alpha2.ACLPublicRead
			Expect(k8sClient.Update(ctx, &obj)).To(Succeed())
		}),
		OneOf(
			Path(
				Call("bucket is not retrieved on update", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Region, gomock.Any()).Return(nil, errors.New("get bucket error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err).NotTo(BeNil())
					// pause is done, now retry
					_, err = reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("get bucket error"))
				}),
			),
			Path(
				Call("bucket access options are not retrieved on update", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Region, gomock.Any()).
						Return(&linodego.ObjectStorageBucket{
							Label:    "bucket",
							Region:   obj.Spec.Region,
							Hostname: "hostname",
							Created:  util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().GetObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("bucket access options fetch error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("failed to get bucket access details"))
				}),
			),
			Path(
				Call("bucket access options are not successfully updated", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Region, gomock.Any()).
						Return(&linodego.ObjectStorageBucket{
							Label:    "bucket",
							Region:   obj.Spec.Region,
							Hostname: "hostname",
							Created:  util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().GetObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&linodego.ObjectStorageBucketAccess{
							ACL:         linodego.ACLPrivate,
							CorsEnabled: true,
						}, nil)
					mck.LinodeClient.EXPECT().UpdateObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(errors.New("bucket access options update error"))
				}),
				Result("error", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err.Error()).To(ContainSubstring("failed to update the bucket access options"))
				}),
			),
			Path(
				Call("bucket is retrieved and updated", func(ctx context.Context, mck Mock) {
					mck.LinodeClient.EXPECT().GetObjectStorageBucket(gomock.Any(), obj.Spec.Region, gomock.Any()).
						Return(&linodego.ObjectStorageBucket{
							Label:    "bucket",
							Region:   obj.Spec.Region,
							Hostname: "hostname",
							Created:  util.Pointer(time.Now()),
						}, nil)
					mck.LinodeClient.EXPECT().GetObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&linodego.ObjectStorageBucketAccess{
							ACL:         linodego.ACLPrivate,
							CorsEnabled: true,
						}, nil)
					mck.LinodeClient.EXPECT().UpdateObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil)
				}),
				Result("success", func(ctx context.Context, mck Mock) {
					bScope.LinodeClient = mck.LinodeClient
					_, err := reconciler.reconcile(ctx, &bScope)
					Expect(err).ToNot(HaveOccurred())
				}),
			),
		),
		Call("resource is deleted", func(ctx context.Context, _ Mock) {
			Expect(k8sClient.Delete(ctx, &obj)).To(Succeed())
		}),
		Result("success", func(ctx context.Context, mck Mock) {
			objectKey := client.ObjectKeyFromObject(&obj)
			k8sClient.Get(ctx, objectKey, &obj)
			bScope.LinodeClient = mck.LinodeClient
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, objectKey, &obj))).To(BeTrue())
		}),
	)
})

var _ = Describe("errors", Label("bucket", "errors"), func() {
	suite := NewControllerSuite(
		GinkgoT(),
		mock.MockLinodeClient{},
		mock.MockK8sClient{},
	)

	reconciler := LinodeObjectStorageBucketReconciler{}
	bScope := scope.ObjectStorageBucketScope{}

	suite.BeforeEach(func(_ context.Context, mck Mock) {
		reconciler.Recorder = mck.Recorder()
		bScope.Logger = mck.Logger()

		// Reset obj to base state to be modified in each test path.
		// We can use a consistent name since these tests are stateless.
		bScope.Bucket = &infrav1alpha2.LinodeObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock",
				Namespace: "default",
				UID:       "12345",
			},
			Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
				Region: "region",
			},
		}
	})

	suite.Run(
		OneOf(
			Path(Call("resource can be fetched", func(ctx context.Context, mck Mock) {
				mck.K8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			})),
			Path(
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
			Path(
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
	)
})
