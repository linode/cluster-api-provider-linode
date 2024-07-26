package controller

import (
	"context"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"testing"

	. "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)

func TestSetupManagers(t *testing.T) {
	t.Parallel()
	cmdConfig := Config{
		LinodeToken:                          "TEST_TOKEN",
		LinodeDNSToken:                       "",
		MachineWatchFilter:                   "",
		ClusterWatchFilter:                   "",
		ObjectStorageBucketWatchFilter:       "",
		MetricsAddr:                          "",
		EnableLeaderElection:                 false,
		ProbeAddr:                            "",
		RestConfigQPS:                        0,
		RestConfigBurst:                      0,
		LinodeClusterConcurrency:             0,
		LinodeMachineConcurrency:             0,
		LinodeObjectStorageBucketConcurrency: 0,
		LinodeVPCConcurrency:                 0,
		LinodePlacementGroupConcurrency:      0,
	}
	var mgr manager.Manager
	NewSuite(t, mock.MockLinodeClient{}).Run(
		OneOf(
			Path(
				Call("Call Setup Managers", func(ctx context.Context, mck Mock) {
					mgr, err = SetupManagers(cmdConfig)
				}),
				Result("setup succeeded", func(ctx context.Context, mck Mock) {
					assert.Nil(t, err, "setup managers failed")
					assert.NotNil(t, mgr)
				}),
			),
		),
	)
}
