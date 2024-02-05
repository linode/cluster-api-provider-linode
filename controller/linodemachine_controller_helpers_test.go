package controller

import (
	"bytes"
	"encoding/gob"
	"testing"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinodeMachineSpecToCreateInstanceConfig(t *testing.T) {
	t.Parallel()

	subnetID := 1

	machineSpec := infrav1.LinodeMachineSpec{
		Region:          "region",
		Type:            "type",
		Label:           "label",
		Group:           "group",
		RootPass:        "rootPass",
		AuthorizedKeys:  []string{"key"},
		AuthorizedUsers: []string{"user"},
		StackScriptID:   1,
		StackScriptData: map[string]string{"script": "data"},
		BackupID:        1,
		Image:           "image",
		Interfaces: []infrav1.InstanceConfigInterfaceCreateOptions{
			{
				IPAMAddress: "address",
				Label:       "label",
				Purpose:     linodego.InterfacePurposePublic,
				Primary:     true,
				SubnetID:    &subnetID,
				IPv4: &infrav1.VPCIPv4{
					VPC:     "vpc",
					NAT1To1: "nat11",
				},
				IPRanges: []string{"ip"},
			},
		},
		BackupsEnabled: true,
		PrivateIP:      true,
		Tags:           []string{"tag"},
		Metadata: &infrav1.InstanceMetadataOptions{
			UserData: "userdata",
		},
		FirewallID: 1,
	}

	createConfig := linodeMachineSpecToInstanceCreateConfig(machineSpec)
	assert.NotNil(t, createConfig, "Failed to convert LinodeMachineSpec to InstanceCreateOptions")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(createConfig)
	require.NoError(t, err, "Failed to encode InstanceCreateOptions")

	var actualMachineSpec infrav1.LinodeMachineSpec
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&actualMachineSpec)
	require.NoError(t, err, "Failed to decode LinodeMachineSpec")

	assert.Equal(t, machineSpec, actualMachineSpec)
}
