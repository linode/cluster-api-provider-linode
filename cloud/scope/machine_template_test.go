/*
Copyright 2025 Akamai Technologies, Inc.

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

package scope

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestValidateMachineTemplateScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		LinodeMachineTemplate *infrav1alpha2.LinodeMachineTemplate
		expErr                string
	}{
		{
			name: "Success - valid LinodeMachineTemplate",
			LinodeMachineTemplate: &infrav1alpha2.LinodeMachineTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-lmt",
				},
			},
			expErr: "",
		},
		{
			name:                  "Failure - nil LinodeMachineTemplate",
			LinodeMachineTemplate: nil,
			expErr:                "LinodeMachineTemplate is required when creating a MachineTemplateScope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateMachineTemplateScope(MachineTemplateScopeParams{
				LinodeMachineTemplate: tt.LinodeMachineTemplate,
			})

			if tt.expErr != "" {
				assert.ErrorContains(t, err, tt.expErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewMachineTemplateScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		LinodeMachineTemplate *infrav1alpha2.LinodeMachineTemplate
		expErr                string
		expects               func(mock *mock.MockK8sClient)
	}{
		{
			name: "Success - able to create a new MachineTemplateScope",
			LinodeMachineTemplate: &infrav1alpha2.LinodeMachineTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-lmt",
				},
			},
			expects: func(mock *mock.MockK8sClient) {
				scheme := runtime.NewScheme()
				infrav1alpha2.AddToScheme(scheme)
				mock.EXPECT().Scheme().Return(scheme)
			},
		},
		{
			name:                  "Failure - nil LinodeMachineTemplate",
			LinodeMachineTemplate: nil,
			expErr:                "LinodeMachineTemplate is required",
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)

			if tt.expects != nil {
				tt.expects(mockK8sClient)
			}

			lmtScope, err := NewMachineTemplateScope(
				t.Context(),
				MachineTemplateScopeParams{
					Client:                mockK8sClient,
					LinodeMachineTemplate: testcase.LinodeMachineTemplate,
				},
			)

			if tt.expErr != "" {
				require.ErrorContains(t, err, tt.expErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, lmtScope)
				require.Equal(t, lmtScope.LinodeMachineTemplate, testcase.LinodeMachineTemplate)
			}
		})
	}
}
