package mock

import "github.com/linode/cluster-api-provider-linode/cloud/scope"

func (m *MockLinodeObjectStorageClient) Builder(_ string) scope.LinodeObjectStorageClient {
	return m
}
