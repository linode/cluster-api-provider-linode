package mock

import (
	gomock "go.uber.org/mock/gomock"
)

// MockClient is a common interface for generated mock clients.
// Each implementation is not generated and must be updated manually.
type MockClient interface {
	mock()
}

func (MockLinodeClient) mock() {}
func (MockK8sClient) mock()    {}

// MockClients holds mock clients that may be instantiated.
type MockClients struct {
	LinodeClient *MockLinodeClient
	K8sClient    *MockK8sClient
}

func (mc *MockClients) Build(client MockClient, ctrl *gomock.Controller) {
	switch client.(type) {
	case MockLinodeClient, *MockLinodeClient:
		mc.LinodeClient = NewMockLinodeClient(ctrl)
	case MockK8sClient, *MockK8sClient:
		mc.K8sClient = NewMockK8sClient(ctrl)
	}
}
