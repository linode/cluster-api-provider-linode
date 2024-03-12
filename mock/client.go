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
	meta "k8s.io/apimachinery/pkg/api/meta"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	patch "sigs.k8s.io/cluster-api/util/patch"
	client "sigs.k8s.io/controller-runtime/pkg/client"
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

// GetObjectStorageBucket mocks base method.
func (m *MockLinodeObjectStorageClient) GetObjectStorageBucket(ctx context.Context, cluster, label string) (*linodego.ObjectStorageBucket, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetObjectStorageBucket", ctx, cluster, label)
	ret0, _ := ret[0].(*linodego.ObjectStorageBucket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetObjectStorageBucket indicates an expected call of GetObjectStorageBucket.
func (mr *MockLinodeObjectStorageClientMockRecorder) GetObjectStorageBucket(ctx, cluster, label any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetObjectStorageBucket", reflect.TypeOf((*MockLinodeObjectStorageClient)(nil).GetObjectStorageBucket), ctx, cluster, label)
}

// Mockk8sClient is a mock of k8sClient interface.
type Mockk8sClient struct {
	ctrl     *gomock.Controller
	recorder *Mockk8sClientMockRecorder
}

// Mockk8sClientMockRecorder is the mock recorder for Mockk8sClient.
type Mockk8sClientMockRecorder struct {
	mock *Mockk8sClient
}

// NewMockk8sClient creates a new mock instance.
func NewMockk8sClient(ctrl *gomock.Controller) *Mockk8sClient {
	mock := &Mockk8sClient{ctrl: ctrl}
	mock.recorder = &Mockk8sClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockk8sClient) EXPECT() *Mockk8sClientMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *Mockk8sClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *Mockk8sClientMockRecorder) Create(ctx, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*Mockk8sClient)(nil).Create), varargs...)
}

// Delete mocks base method.
func (m *Mockk8sClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Delete", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *Mockk8sClientMockRecorder) Delete(ctx, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*Mockk8sClient)(nil).Delete), varargs...)
}

// DeleteAllOf mocks base method.
func (m *Mockk8sClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteAllOf", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAllOf indicates an expected call of DeleteAllOf.
func (mr *Mockk8sClientMockRecorder) DeleteAllOf(ctx, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAllOf", reflect.TypeOf((*Mockk8sClient)(nil).DeleteAllOf), varargs...)
}

// Get mocks base method.
func (m *Mockk8sClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, key, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Get", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Get indicates an expected call of Get.
func (mr *Mockk8sClientMockRecorder) Get(ctx, key, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, key, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*Mockk8sClient)(nil).Get), varargs...)
}

// GroupVersionKindFor mocks base method.
func (m *Mockk8sClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GroupVersionKindFor", obj)
	ret0, _ := ret[0].(schema.GroupVersionKind)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GroupVersionKindFor indicates an expected call of GroupVersionKindFor.
func (mr *Mockk8sClientMockRecorder) GroupVersionKindFor(obj any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GroupVersionKindFor", reflect.TypeOf((*Mockk8sClient)(nil).GroupVersionKindFor), obj)
}

// IsObjectNamespaced mocks base method.
func (m *Mockk8sClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsObjectNamespaced", obj)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsObjectNamespaced indicates an expected call of IsObjectNamespaced.
func (mr *Mockk8sClientMockRecorder) IsObjectNamespaced(obj any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsObjectNamespaced", reflect.TypeOf((*Mockk8sClient)(nil).IsObjectNamespaced), obj)
}

// List mocks base method.
func (m *Mockk8sClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, list}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "List", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// List indicates an expected call of List.
func (mr *Mockk8sClientMockRecorder) List(ctx, list any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, list}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*Mockk8sClient)(nil).List), varargs...)
}

// Patch mocks base method.
func (m *Mockk8sClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, obj, patch}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Patch", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Patch indicates an expected call of Patch.
func (mr *Mockk8sClientMockRecorder) Patch(ctx, obj, patch any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, obj, patch}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Patch", reflect.TypeOf((*Mockk8sClient)(nil).Patch), varargs...)
}

// RESTMapper mocks base method.
func (m *Mockk8sClient) RESTMapper() meta.RESTMapper {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RESTMapper")
	ret0, _ := ret[0].(meta.RESTMapper)
	return ret0
}

// RESTMapper indicates an expected call of RESTMapper.
func (mr *Mockk8sClientMockRecorder) RESTMapper() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RESTMapper", reflect.TypeOf((*Mockk8sClient)(nil).RESTMapper))
}

// Scheme mocks base method.
func (m *Mockk8sClient) Scheme() *runtime.Scheme {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Scheme")
	ret0, _ := ret[0].(*runtime.Scheme)
	return ret0
}

// Scheme indicates an expected call of Scheme.
func (mr *Mockk8sClientMockRecorder) Scheme() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Scheme", reflect.TypeOf((*Mockk8sClient)(nil).Scheme))
}

// Status mocks base method.
func (m *Mockk8sClient) Status() client.SubResourceWriter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(client.SubResourceWriter)
	return ret0
}

// Status indicates an expected call of Status.
func (mr *Mockk8sClientMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*Mockk8sClient)(nil).Status))
}

// SubResource mocks base method.
func (m *Mockk8sClient) SubResource(subResource string) client.SubResourceClient {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubResource", subResource)
	ret0, _ := ret[0].(client.SubResourceClient)
	return ret0
}

// SubResource indicates an expected call of SubResource.
func (mr *Mockk8sClientMockRecorder) SubResource(subResource any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubResource", reflect.TypeOf((*Mockk8sClient)(nil).SubResource), subResource)
}

// Update mocks base method.
func (m *Mockk8sClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *Mockk8sClientMockRecorder) Update(ctx, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*Mockk8sClient)(nil).Update), varargs...)
}

// MockPatchHelper is a mock of PatchHelper interface.
type MockPatchHelper struct {
	ctrl     *gomock.Controller
	recorder *MockPatchHelperMockRecorder
}

// MockPatchHelperMockRecorder is the mock recorder for MockPatchHelper.
type MockPatchHelperMockRecorder struct {
	mock *MockPatchHelper
}

// NewMockPatchHelper creates a new mock instance.
func NewMockPatchHelper(ctrl *gomock.Controller) *MockPatchHelper {
	mock := &MockPatchHelper{ctrl: ctrl}
	mock.recorder = &MockPatchHelperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPatchHelper) EXPECT() *MockPatchHelperMockRecorder {
	return m.recorder
}

// Patch mocks base method.
func (m *MockPatchHelper) Patch(ctx context.Context, obj client.Object, opts ...patch.Option) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, obj}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Patch", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Patch indicates an expected call of Patch.
func (mr *MockPatchHelperMockRecorder) Patch(ctx, obj any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, obj}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Patch", reflect.TypeOf((*MockPatchHelper)(nil).Patch), varargs...)
}
