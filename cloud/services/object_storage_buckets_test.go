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
	"go.uber.org/mock/gomock"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
)

func TestGetObjectStorageKeys(t *testing.T) {
	tests := []struct {
		name    string
		bScope  *scope.ObjectStorageBucketScope
		expects func(*mock.MockLinodeObjectStorageClient)
		want    [2]linodego.ObjectStorageKey
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
			expects: func(mc *mock.MockLinodeObjectStorageClient) {
				mc.EXPECT().
					GetObjectStorageKey(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, keyID int) (*linodego.ObjectStorageKey, error) {
						return &linodego.ObjectStorageKey{ID: keyID}, nil
					}).
					Times(2)
			},
			want: [2]linodego.ObjectStorageKey{
				{ID: 0},
				{ID: 1},
			},
		},
		{
			name: "no key refs in status",
			bScope: &scope.ObjectStorageBucketScope{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
			},
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
			expects: func(mc *mock.MockLinodeObjectStorageClient) {
				mc.EXPECT().
					GetObjectStorageKey(gomock.Any(), 0).
					DoAndReturn(func(_ any, keyID int) (*linodego.ObjectStorageKey, error) {
						return &linodego.ObjectStorageKey{ID: keyID}, nil
					})
				mc.EXPECT().
					GetObjectStorageKey(gomock.Any(), 1).
					DoAndReturn(func(_ any, keyID int) (*linodego.ObjectStorageKey, error) {
						return nil, errors.New("some error")
					})
			},
			want: [2]linodego.ObjectStorageKey{
				{ID: 0},
				{},
			},
			wantErr: "some error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockClient *mock.MockLinodeObjectStorageClient
			if tt.expects != nil {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockClient = mock.NewMockLinodeObjectStorageClient(ctrl)
				tt.expects(mockClient)
				tt.bScope.LinodeClient = mockClient
			}

			got, err := GetObjectStorageKeys(context.TODO(), tt.bScope)
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("GetObjectStorageKeys() error = %v, should contain %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetObjectStorageKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}
