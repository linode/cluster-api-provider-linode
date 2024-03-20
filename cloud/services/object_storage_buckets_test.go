package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

func TestRotateObjectStorageKeysRevocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		bucket  *infrav1alpha1.LinodeObjectStorageBucket
		expects func(*mock.MockLinodeObjectStorageClient)
	}{
		{
			name: "should revoke existing keys",
			bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bucket",
				},
				Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
					KeyGeneration: ptr.To(1),
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					LastKeyGeneration: ptr.To(0),
					AccessKeyRefs:     []int{0, 1},
				},
			},
			expects: func(mockClient *mock.MockLinodeObjectStorageClient) {
				for keyID := range 2 {
					mockClient.EXPECT().
						DeleteObjectStorageKey(gomock.Any(), keyID).
						Return(nil).
						Times(1)
				}
			},
		},
		{
			name: "shouldInitKeys",
			bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bucket",
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					LastKeyGeneration: nil,
				},
			},
		},
		{
			name: "not shouldRotateKeys",
			bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bucket",
				},
				Spec: infrav1alpha1.LinodeObjectStorageBucketSpec{
					KeyGeneration: ptr.To(1),
				},
				Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
					LastKeyGeneration: ptr.To(1),
				},
			},
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockLinodeObjectStorageClient(ctrl)
			mockClient.EXPECT().
				CreateObjectStorageKey(gomock.Any(), gomock.Any()).
				Return(&linodego.ObjectStorageKey{ID: 3}, nil).
				Times(2)
			if testcase.expects != nil {
				testcase.expects(mockClient)
			}

			bScope := &scope.ObjectStorageBucketScope{
				LinodeClient: mockClient,
				Bucket:       testcase.bucket,
			}

			_, err := RotateObjectStorageKeys(context.TODO(), bScope)
			require.NoError(t, err)
		})
	}
}

func TestGetObjectStorageKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		bScope  *scope.ObjectStorageBucketScope
		expects func(*mock.MockLinodeObjectStorageClient)
		want    [2]*linodego.ObjectStorageKey
		wantErr string
	}{
		{
			name: "happy path",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
						AccessKeyRefs: []int{0, 1},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeObjectStorageClient) {
				mockClient.EXPECT().
					GetObjectStorageKey(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, keyID int) (*linodego.ObjectStorageKey, error) {
						return &linodego.ObjectStorageKey{ID: keyID}, nil
					}).
					Times(2)
			},
			want: [2]*linodego.ObjectStorageKey{
				{ID: 0},
				{ID: 1},
			},
		},
		{
			name: "not two key refs in status",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
			},
			wantErr: "expected two object storage key IDs in .status.accessKeyRefs",
		},
		{
			name: "one client error",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
					Status: infrav1alpha1.LinodeObjectStorageBucketStatus{
						AccessKeyRefs: []int{0, 1},
					},
				},
			},
			expects: func(mockClient *mock.MockLinodeObjectStorageClient) {
				mockClient.EXPECT().
					GetObjectStorageKey(gomock.Any(), 0).
					DoAndReturn(func(_ any, keyID int) (*linodego.ObjectStorageKey, error) {
						return &linodego.ObjectStorageKey{ID: keyID}, nil
					})
				mockClient.EXPECT().
					GetObjectStorageKey(gomock.Any(), 1).
					DoAndReturn(func(_ any, keyID int) (*linodego.ObjectStorageKey, error) {
						return nil, errors.New("some error")
					})
			},
			want: [2]*linodego.ObjectStorageKey{
				{ID: 0},
				nil,
			},
			wantErr: "some error",
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			var mockClient *mock.MockLinodeObjectStorageClient
			if testcase.expects != nil {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockClient = mock.NewMockLinodeObjectStorageClient(ctrl)
				testcase.expects(mockClient)
				testcase.bScope.LinodeClient = mockClient
			}

			got, err := GetObjectStorageKeys(context.TODO(), testcase.bScope)
			if testcase.wantErr != "" && (err == nil || !strings.Contains(err.Error(), testcase.wantErr)) {
				t.Errorf("GetObjectStorageKeys() error = %v, should contain %v", err, testcase.wantErr)
			}
			if !reflect.DeepEqual(got, testcase.want) {
				t.Errorf("GetObjectStorageKeys() = %v, want %v", got, testcase.want)
			}
		})
	}
}
