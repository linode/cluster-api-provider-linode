package services

import (
	"context"
	"reflect"
	"testing"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnsureObjectStorageBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		bScope *scope.ObjectStorageBucketScope
		want    *linodego.ObjectStorageBucket
		expectedError error
		expects func(mock *mock.MockLinodeClient)
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

func TestRotateObjectStorageKeys(t *testing.T) {
	type args struct {
		ctx    context.Context
		bScope *scope.ObjectStorageBucketScope
	}
	tests := []struct {
		name    string
		args    args
		want    [scope.NumAccessKeys]linodego.ObjectStorageKey
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RotateObjectStorageKeys(tt.args.ctx, tt.args.bScope)
			if (err != nil) != tt.wantErr {
				t.Errorf("RotateObjectStorageKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RotateObjectStorageKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_createObjectStorageKey(t *testing.T) {
	type args struct {
		ctx        context.Context
		bScope     *scope.ObjectStorageBucketScope
		label      string
		permission string
	}
	tests := []struct {
		name    string
		args    args
		want    *linodego.ObjectStorageKey
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createObjectStorageKey(tt.args.ctx, tt.args.bScope, tt.args.label, tt.args.permission)
			if (err != nil) != tt.wantErr {
				t.Errorf("createObjectStorageKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createObjectStorageKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRevokeObjectStorageKeys(t *testing.T) {
	type args struct {
		ctx    context.Context
		bScope *scope.ObjectStorageBucketScope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RevokeObjectStorageKeys(tt.args.ctx, tt.args.bScope); (err != nil) != tt.wantErr {
				t.Errorf("RevokeObjectStorageKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_revokeObjectStorageKey(t *testing.T) {
	type args struct {
		ctx    context.Context
		bScope *scope.ObjectStorageBucketScope
		keyID  int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := revokeObjectStorageKey(tt.args.ctx, tt.args.bScope, tt.args.keyID); (err != nil) != tt.wantErr {
				t.Errorf("revokeObjectStorageKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
