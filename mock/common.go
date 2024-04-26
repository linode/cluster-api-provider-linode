package mock

import (
	gomock "go.uber.org/mock/gomock"
)

// MockClient is a common interface for generated mock clients.
// Each implementation is not generated and must be updated manually.
type MockClient interface {
	mock()
}

func (MockLinodeMachineClient) mock()       {}
func (MockLinodeVPCClient) mock()           {}
func (MockLinodeNodeBalancerClient) mock()  {}
func (MockLinodeObjectStorageClient) mock() {}
func (MockK8sClient) mock()                 {}

// MockClients holds mock clients that may be instantiated.
type MockClients struct {
	MachineClient       *MockLinodeMachineClient
	VPCClient           *MockLinodeVPCClient
	NodeBalancerClient  *MockLinodeNodeBalancerClient
	ObjectStorageClient *MockLinodeObjectStorageClient
	K8sClient           *MockK8sClient
}

func (mc *MockClients) Build(client MockClient, ctrl *gomock.Controller) {
	switch client.(type) {
	case MockLinodeMachineClient, *MockLinodeMachineClient:
		mc.MachineClient = NewMockLinodeMachineClient(ctrl)
	case MockLinodeVPCClient, *MockLinodeVPCClient:
		mc.VPCClient = NewMockLinodeVPCClient(ctrl)
	case MockLinodeNodeBalancerClient, *MockLinodeNodeBalancerClient:
		mc.NodeBalancerClient = NewMockLinodeNodeBalancerClient(ctrl)
	case MockLinodeObjectStorageClient, *MockLinodeObjectStorageClient:
		mc.ObjectStorageClient = NewMockLinodeObjectStorageClient(ctrl)
	case MockK8sClient, *MockK8sClient:
		mc.K8sClient = NewMockK8sClient(ctrl)
	}
}
