package services

import (
	"context"
	"reflect"
	"testing"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/linodego"
)

func TestEnsureObjectStorageBucket(t *testing.T) {
	type args struct {
		ctx    context.Context
		bScope *scope.ObjectStorageBucketScope
	}
	tests := []struct {
		name    string
		args    args
		want    *linodego.ObjectStorageBucket
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureObjectStorageBucket(tt.args.ctx, tt.args.bScope)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureObjectStorageBucket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EnsureObjectStorageBucket() = %v, want %v", got, tt.want)
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
