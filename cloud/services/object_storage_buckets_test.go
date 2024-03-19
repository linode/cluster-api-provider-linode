package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
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
		expects       func(mock *mock.MockLinodeClient)
	}{
		{
			name: "Success - Successfully get the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
						Cluster: "test-cluster",
					},
				},
			},
			want: &linodego.ObjectStorageBucket{
				Label: "test-bucket",
			},
			expects: func(c *mock.MockLinodeClient) {
				c.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageBucket{
					Label: "test-bucket",
				}, nil)
			},
			expectedError: nil,
		},
		{
			name: "Error - Unable to get the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
						Cluster: "test-cluster",
					},
				},
			},
			want: nil,
			expects: func(c *mock.MockLinodeClient) {
				c.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error in getting object storage bucket"))
			},
			expectedError: fmt.Errorf("failed to get bucket from cluster"),
		},
		{
			name: "Success - Successfully create the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
						Cluster: "test-cluster",
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
			expectedError: nil,
		},
		{
			name: "Error - unable to create the OBJ bucket",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
						Cluster: "test-cluster",
					},
				},
			},
			want: nil,
			expects: func(c *mock.MockLinodeClient) {
				c.EXPECT().GetObjectStorageBucket(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				c.EXPECT().CreateObjectStorageBucket(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error in creating object storage bucket"))
			},
			expectedError: fmt.Errorf("failed to create bucket:"),
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

			got, err := EnsureObjectStorageBucket(context.Background(), testcase.bScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.Equal(t, testcase.want, got)
			}
		})
	}
}

// TestRotateObjectStorageKeys tests the RotateObjectStorageKeys function along
// with createObjectStorageKey(), RevokeObjectStorageKeys(), and revokeObjectStorageKey()
func TestRotateObjectStorageKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		bScope        *scope.ObjectStorageBucketScope
		want          [scope.NumAccessKeys]linodego.ObjectStorageKey
		expectedError error
		expects       func(c *mock.MockLinodeClient)
	}{
		{
			name: "Success - Create new access keys and revoke old access keys",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
						Cluster:       "test-cluster",
						KeyGeneration: ptr.To(1),
					},
					Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
						LastKeyGeneration: ptr.To(0),
						AccessKeyRefs: []int{
							11,
							22,
						},
					},
				},
			},
			want: [scope.NumAccessKeys]linodego.ObjectStorageKey{
				{
					ID:    1234,
					Label: "test-bucket-rw",
				},
				{
					ID:    5678,
					Label: "test-bucket-ro",
				},
			},
			expectedError: nil,
			expects: func(mock *mock.MockLinodeClient) {
				mock.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageKey{
					Label: "test-bucket-rw",
					ID:    1234,
				}, nil)
				mock.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageKey{
					Label: "test-bucket-ro",
					ID:    5678,
				}, nil)
				mock.EXPECT().DeleteObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil)
				mock.EXPECT().DeleteObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "Success - Create new access keys but unable to revoke old access keys",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
						Cluster:       "test-cluster",
						KeyGeneration: ptr.To(1),
					},
					Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
						LastKeyGeneration: ptr.To(0),
						AccessKeyRefs: []int{
							11,
							22,
						},
					},
				},
			},
			want: [scope.NumAccessKeys]linodego.ObjectStorageKey{
				{
					ID:    1234,
					Label: "test-bucket-rw",
				},
				{
					ID:    5678,
					Label: "test-bucket-ro",
				},
			},
			expectedError: nil,
			expects: func(mock *mock.MockLinodeClient) {
				mock.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageKey{
					Label: "test-bucket-rw",
					ID:    1234,
				}, nil)
				mock.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(&linodego.ObjectStorageKey{
					Label: "test-bucket-ro",
					ID:    5678,
				}, nil)
				mock.EXPECT().DeleteObjectStorageKey(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error in deleting access key")).Times(2)
			},
		},
		{
			name: "Error - Rotated OBJ keys",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-bucket",
					},
					Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
						Cluster: "test-cluster",
					},
				},
			},
			want:          [scope.NumAccessKeys]linodego.ObjectStorageKey{},
			expectedError: fmt.Errorf("failed to create access key:"),
			expects: func(c *mock.MockLinodeClient) {
				c.EXPECT().CreateObjectStorageKey(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error in creating access key"))
			},
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

			got, err := RotateObjectStorageKeys(context.Background(), testcase.bScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.Equal(t, testcase.want, got)
			}
		})
	}
}
