package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestEnsureObjectStorageBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		bScope        *scope.ObjectStorageBucketScope
		want          *linodego.ObjectStorageBucket
		expectedError error
		expects       func(*mock.MockLinodeClient)
	}{
		{
			name: "Success - Successfully get the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region:      "test-region",
						ACL:         infrav1alpha2.ACLPrivate,
						CorsEnabled: true,
					},
				},
			},
			want: &linodego.ObjectStorageBucket{
				Label: "test-bucket",
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{
					Label: "test-bucket",
				}, nil)
				mockClient.EXPECT().GetObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucketAccess{
					ACL:         linodego.ACLPrivate,
					CorsEnabled: true,
				}, nil)
			},
		},
		{
			name: "Error - Unable to get the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
					},
				},
			},
			expects: func(c *mock.MockLinodeClient) {
				c.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error in getting object storage bucket"))
			},
			expectedError: fmt.Errorf("failed to get bucket from region"),
		},
		{
			name: "Success - Successfully create the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
					},
				},
			},
			want: &linodego.ObjectStorageBucket{
				Label: "test-bucket",
			},
			expects: func(c *mock.MockLinodeClient) {
				c.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				c.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{
					Label: "test-bucket",
				}, nil)
			},
		},
		{
			name: "Error - unable to create the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
					},
				},
			},
			expects: func(c *mock.MockLinodeClient) {
				c.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				c.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error in creating object storage bucket"))
			},
			expectedError: fmt.Errorf("failed to create bucket:"),
		},
		{
			name: "Success - Successfully update the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region:      "test-region",
						ACL:         infrav1alpha2.ACLPublicRead,
						CorsEnabled: true,
					},
				},
			},
			want: &linodego.ObjectStorageBucket{
				Label: "test-bucket",
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{
					Label: "test-bucket",
				}, nil)
				mockClient.EXPECT().GetObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucketAccess{
					ACL:         linodego.ACLPrivate,
					CorsEnabled: true,
				}, nil)
				mockClient.EXPECT().UpdateObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "Error - unable to update the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{
					Label: "test-bucket",
				}, nil)
				mockClient.EXPECT().GetObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucketAccess{
					ACL:         linodego.ACLPrivate,
					CorsEnabled: true,
				}, nil)
				mockClient.EXPECT().UpdateObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("error in updating object storage bucket"))
			},
			expectedError: fmt.Errorf("failed to update the bucket access options"),
		},
		{
			name: "Error - unable to fetch the OBJ bucket Access",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{
					Label: "test-bucket",
				}, nil)
				mockClient.EXPECT().GetObjectStorageBucketAccess(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error in fetching object storage bucket access details"))
			},
			expectedError: fmt.Errorf("failed to get bucket access details"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockLinodeClient(ctrl)

			testcase.bScope.LinodeClient = mockClient

			testcase.expects(mockClient)

			got, err := EnsureAndUpdateObjectStorageBucket(t.Context(), testcase.bScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.Equal(t, testcase.want, got)
			}
		})
	}
}

func TestCreateS3ClientWithAccessKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		bScope        *scope.ObjectStorageBucketScope
		expectedError error
		expects       func(client *mock.MockK8sClient)
	}{
		{
			name: "Success - Successfully create client",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
						AccessKeyRef: &v1.ObjectReference{
							Name: "test",
						},
					},
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, name types.NamespacedName, obj *v1.Secret, opts ...client.GetOption) error {
					secret := v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-bucket-obj-key",
						},
						Data: map[string][]byte{
							"access": []byte("test-access-key"),
							"secret": []byte("test-secret-key"),
							"bucket": []byte("test-bucket"),
						},
					}
					*obj = secret
					return nil
				})
			},
		},
		{
			name: "Error - failed to get access key",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
						AccessKeyRef: &v1.ObjectReference{
							Name: "test",
						},
					},
				},
			},
			expects: func(k8s *mock.MockK8sClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewNotFound(schema.GroupResource{}, ""))
			},
			expectedError: fmt.Errorf("failed to get bucket secret"),
		},
		{
			name: "Error - access key is nil",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
					},
				},
			},
			expects:       func(k8s *mock.MockK8sClient) {},
			expectedError: fmt.Errorf("accessKeyRef is nil"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockK8sClient(ctrl)

			testcase.bScope.Client = mockClient

			testcase.expects(mockClient)

			s3Client, err := createS3ClientWithAccessKey(t.Context(), testcase.bScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.NotNil(t, s3Client)
			}
		})
	}
}

func TestDeleteBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		bScope        *scope.ObjectStorageBucketScope
		expectedError error
		expects       func(k8s *mock.MockK8sClient, lc *mock.MockLinodeClient)
	}{
		{
			name: "Error - failed to purge all objects",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
						AccessKeyRef: &v1.ObjectReference{
							Name: "test-bucket",
						},
					},
				},
			},
			expects: func(k8s *mock.MockK8sClient, lc *mock.MockLinodeClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, name types.NamespacedName, obj *v1.Secret, opts ...client.GetOption) error {
					secret := v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-bucket-obj-key",
						},
						Data: map[string][]byte{
							"access": []byte("test-access-key"),
							"secret": []byte("test-secret-key"),
							"bucket": []byte("test-bucket"),
						},
					}
					*obj = secret
					return nil
				})
			},
			expectedError: fmt.Errorf("failed to purge all objects"),
		},
		{
			name: "Error - failed to create S3 client",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha2.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha2.LinodeObjectStorageBucketSpec{
						Region: "test-region",
						AccessKeyRef: &v1.ObjectReference{
							Name: "test",
						},
					},
				},
			},
			expects: func(k8s *mock.MockK8sClient, lc *mock.MockLinodeClient) {
				k8s.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewNotFound(schema.GroupResource{}, ""))
			},
			expectedError: fmt.Errorf("failed to create S3 client"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8s := mock.NewMockK8sClient(ctrl)
			testcase.bScope.Client = mockK8s
			mockClient := mock.NewMockLinodeClient(ctrl)
			testcase.bScope.LinodeClient = mockClient

			testcase.expects(mockK8s, mockClient)

			err := DeleteBucket(t.Context(), testcase.bScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			}
		})
	}
}
