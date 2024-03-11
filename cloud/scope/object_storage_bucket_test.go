package scope

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_validateObjectStorageBucketScopeParams(t *testing.T) {
	type args struct {
		params ObjectStorageBucketScopeParams
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
			if err := validateObjectStorageBucketScopeParams(tt.args.params); (err != nil) != tt.wantErr {
				t.Errorf("validateObjectStorageBucketScopeParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewObjectStorageBucketScope(t *testing.T) {
	type args struct {
		ctx    context.Context
		apiKey string
		params ObjectStorageBucketScopeParams
	}
	tests := []struct {
		name    string
		args    args
		want    *ObjectStorageBucketScope
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewObjectStorageBucketScope(tt.args.ctx, tt.args.apiKey, tt.args.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewObjectStorageBucketScope() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewObjectStorageBucketScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectStorageBucketScope_AddFinalizer(t *testing.T) {
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			if err := s.AddFinalizer(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("ObjectStorageBucketScope.AddFinalizer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestObjectStorageBucketScope_ApplyAccessKeySecret(t *testing.T) {
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	type args struct {
		ctx        context.Context
		keys       [NumAccessKeys]linodego.ObjectStorageKey
		secretName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			if err := s.ApplyAccessKeySecret(tt.args.ctx, tt.args.keys, tt.args.secretName); (err != nil) != tt.wantErr {
				t.Errorf("ObjectStorageBucketScope.ApplyAccessKeySecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestObjectStorageBucketScope_GetAccessKeySecret(t *testing.T) {
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev1.Secret
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			got, err := s.GetAccessKeySecret(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeySecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeySecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectStorageBucketScope_GetAccessKeysFromSecret(t *testing.T) {
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	type args struct {
		ctx    context.Context
		secret *corev1.Secret
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    [NumAccessKeys]int
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			got, err := s.GetAccessKeysFromSecret(tt.args.ctx, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeysFromSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeysFromSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectStorageBucketScope_ShouldRotateKeys(t *testing.T) {
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			if got := s.ShouldRotateKeys(); got != tt.want {
				t.Errorf("ObjectStorageBucketScope.ShouldRotateKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}
