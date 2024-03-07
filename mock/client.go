// Code generated by MockGen. DO NOT EDIT.
// Source: ./cloud/scope/client.go
//
// Generated by this command:
//
//	mockgen -source=./cloud/scope/client.go -destination ./mock/client.go -package mock
//

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	linodego "github.com/linode/linodego"
	gomock "go.uber.org/mock/gomock"
)

// MockLinodeObjectStorageClient is a mock of LinodeObjectStorageClient interface.
type MockLinodeObjectStorageClient struct {
	ctrl     *gomock.Controller
	recorder *MockLinodeObjectStorageClientMockRecorder
}

// MockLinodeObjectStorageClientMockRecorder is the mock recorder for MockLinodeObjectStorageClient.
type MockLinodeObjectStorageClientMockRecorder struct {
	mock *MockLinodeObjectStorageClient
}

// NewMockLinodeObjectStorageClient creates a new mock instance.
func NewMockLinodeObjectStorageClient(ctrl *gomock.Controller) *MockLinodeObjectStorageClient {
	mock := &MockLinodeObjectStorageClient{ctrl: ctrl}
	mock.recorder = &MockLinodeObjectStorageClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLinodeObjectStorageClient) EXPECT() *MockLinodeObjectStorageClientMockRecorder {
	return m.recorder
}

// CreateObjectStorageBucket mocks base method.
func (m *MockLinodeObjectStorageClient) CreateObjectStorageBucket(ctx context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateObjectStorageBucket", ctx, opts)
	ret0, _ := ret[0].(*linodego.ObjectStorageBucket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateObjectStorageBucket indicates an expected call of CreateObjectStorageBucket.
func (mr *MockLinodeObjectStorageClientMockRecorder) CreateObjectStorageBucket(ctx, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateObjectStorageBucket", reflect.TypeOf((*MockLinodeObjectStorageClient)(nil).CreateObjectStorageBucket), ctx, opts)
}

// CreateObjectStorageKey mocks base method.
func (m *MockLinodeObjectStorageClient) CreateObjectStorageKey(ctx context.Context, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateObjectStorageKey", ctx, opts)
	ret0, _ := ret[0].(*linodego.ObjectStorageKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateObjectStorageKey indicates an expected call of CreateObjectStorageKey.
func (mr *MockLinodeObjectStorageClientMockRecorder) CreateObjectStorageKey(ctx, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateObjectStorageKey", reflect.TypeOf((*MockLinodeObjectStorageClient)(nil).CreateObjectStorageKey), ctx, opts)
}

// DeleteObjectStorageKey mocks base method.
func (m *MockLinodeObjectStorageClient) DeleteObjectStorageKey(ctx context.Context, keyID int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteObjectStorageKey", ctx, keyID)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteObjectStorageKey indicates an expected call of DeleteObjectStorageKey.
func (mr *MockLinodeObjectStorageClientMockRecorder) DeleteObjectStorageKey(ctx, keyID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteObjectStorageKey", reflect.TypeOf((*MockLinodeObjectStorageClient)(nil).DeleteObjectStorageKey), ctx, keyID)
}

// ListObjectStorageBucketsInCluster mocks base method.
func (m *MockLinodeObjectStorageClient) ListObjectStorageBucketsInCluster(ctx context.Context, opts *linodego.ListOptions, cluster string) ([]linodego.ObjectStorageBucket, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListObjectStorageBucketsInCluster", ctx, opts, cluster)
	ret0, _ := ret[0].([]linodego.ObjectStorageBucket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListObjectStorageBucketsInCluster indicates an expected call of ListObjectStorageBucketsInCluster.
func (mr *MockLinodeObjectStorageClientMockRecorder) ListObjectStorageBucketsInCluster(ctx, opts, cluster any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListObjectStorageBucketsInCluster", reflect.TypeOf((*MockLinodeObjectStorageClient)(nil).ListObjectStorageBucketsInCluster), ctx, opts, cluster)
}