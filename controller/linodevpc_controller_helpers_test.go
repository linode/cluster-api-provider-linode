package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

func TestLinodeVPCSpecToCreateVPCConfig(t *testing.T) {
	t.Parallel()

	vpcSpec := infrav1alpha2.LinodeVPCSpec{
		Description: "description",
		Region:      "region",
		Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
			{
				Label: "subnet",
				IPv4:  "ipv4",
			},
		},
	}

	createConfig := linodeVPCSpecToVPCCreateConfig(vpcSpec)
	assert.NotNil(t, createConfig, "Failed to convert LinodeVPCSpec to VPCCreateOptions")
}
