package controller

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/infrastructure/v1alpha1"
)

func TestLinodeVPCSpecToCreateVPCConfig(t *testing.T) {
	t.Parallel()

	vpcSpec := infrav1alpha1.LinodeVPCSpec{
		Description: "description",
		Region:      "region",
		Subnets: []infrav1alpha1.VPCSubnetCreateOptions{
			{
				Label: "subnet",
				IPv4:  "ipv4",
			},
		},
	}

	createConfig := linodeVPCSpecToVPCCreateConfig(vpcSpec)
	assert.NotNil(t, createConfig, "Failed to convert LinodeVPCSpec to VPCCreateOptions")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(createConfig)
	require.NoError(t, err, "Failed to encode VPCCreateOptions")

	var actualVPCSpec infrav1alpha1.LinodeVPCSpec
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&actualVPCSpec)
	require.NoError(t, err, "Failed to decode LinodeVPCSpec")

	assert.Equal(t, vpcSpec, actualVPCSpec)
}
