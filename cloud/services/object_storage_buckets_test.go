package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
						Region: "test-region",
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
