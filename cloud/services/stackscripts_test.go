package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestEnsureStackscripts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		want          int
		expectedError error
		expects       func(client *mock.MockLinodeMachineClient)
	}{
		{
			name:         "Success - Successfully get existing StackScript",
			machineScope: &scope.MachineScope{},
			want:         1234,
			expects: func(mockClient *mock.MockLinodeMachineClient) {
				mockClient.EXPECT().ListStackscripts(gomock.Any(), &linodego.ListOptions{Filter: "{\"label\":\"CAPL-dev\"}"}).Return([]linodego.Stackscript{{
					Label: "CAPI Test 1",
					ID:    1234,
				}}, nil)
			},
		},
		{
			name:         "Error - failed get existing StackScript",
			machineScope: &scope.MachineScope{},
			expects: func(mockClient *mock.MockLinodeMachineClient) {
				mockClient.EXPECT().ListStackscripts(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed to get StackScript"))
			},
			expectedError: fmt.Errorf("failed to get StackScript"),
		},
		{
			name:         "Success - Successfully created StackScript",
			machineScope: &scope.MachineScope{},
			want:         56345,
			expects: func(mockClient *mock.MockLinodeMachineClient) {
				mockClient.EXPECT().ListStackscripts(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockClient.EXPECT().CreateStackscript(gomock.Any(), linodego.StackscriptCreateOptions{
					Label:       "CAPL-dev",
					Description: "Stackscript for creating CAPL clusters with CAPL controller version dev",
					Script: `#!/bin/sh
# <UDF name="instancedata" label="instance-data contents(base64 encoded" />
# <UDF name="userdata" label="user-data file contents (base64 encoded)" />

cat > /etc/cloud/cloud.cfg.d/100_none.cfg <<EOF
datasource_list: [ "None"]
datasource:
  None:
    metadata:
      id: $LINODE_ID
$(echo "${INSTANCEDATA}" | base64 -d | sed "s/^/      /")
    userdata_raw: |
$(echo "${USERDATA}" | base64 -d | sed "s/^/      /")

EOF

cloud-init clean
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg init --local
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg init
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg modules --mode=config
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg modules --mode=final
`,
					Images: []string{"any/all"},
				}).Return(&linodego.Stackscript{
					Label: "CAPI Test 1",
					ID:    56345,
				}, nil)
			},
		},
		{
			name:         "Error - failed create StackScript",
			machineScope: &scope.MachineScope{},
			expects: func(mockClient *mock.MockLinodeMachineClient) {
				mockClient.EXPECT().ListStackscripts(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockClient.EXPECT().CreateStackscript(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed to create StackScript"))
			},
			expectedError: fmt.Errorf("failed to create StackScript"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockLinodeMachineClient(ctrl)

			testcase.machineScope.LinodeClient = mockClient

			testcase.expects(mockClient)

			got, err := EnsureStackscript(context.Background(), testcase.machineScope)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.Equal(t, testcase.want, got)
			}
		})
	}
}
