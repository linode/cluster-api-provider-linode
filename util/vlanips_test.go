/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"context"
	"reflect"
	"testing"

	"github.com/linode/cluster-api-provider-linode/mock"
	"go.uber.org/mock/gomock"
)

func TestGetNextVlanIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		clusterName      string
		clusterNamespace string
		want             string
		expects          func(mock *mock.MockK8sClient)
	}{
		{
			name:             "provide key which exists in map",
			clusterName:      "test",
			clusterNamespace: "testna",
			want:             "10.0.0.3",
			expects: func(mock *mock.MockK8sClient) {
			},
		},
		{
			name:             "provide key which doesn't exist",
			clusterName:      "test",
			clusterNamespace: "testnonexistent",
			want:             "10.0.0.1",
			expects: func(mock *mock.MockK8sClient) {
				mock.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).MinTimes(1)
			},
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockK8sClient := mock.NewMockK8sClient(ctrl)

	for _, tt := range tests {
		vlanIPsMap["testna.test"] = &ClusterIPs{
			ips: []string{"10.0.0.1", "10.0.0.2"},
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.expects(mockK8sClient)
			got, err := GetNextVlanIP(context.Background(), tt.clusterName, tt.clusterNamespace, mockK8sClient)
			if err != nil {
				t.Error("error")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNextVlanIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
