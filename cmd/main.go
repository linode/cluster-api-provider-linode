/*
Copyright 2023-2024 Akamai Technologies, Inc.

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

package main

import (
	"context"
	"flag"
	"fmt"
	infrastructurev1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/controller"
	"github.com/linode/cluster-api-provider-linode/version"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	_ "go.uber.org/automaxprocs"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	// +kubebuilder:scaffold:imports
)

const (
	controllerName                  = "cluster-api-provider-linode.linode.com"
	envK8sNodeName                  = "K8S_NODE_NAME"
	envK8sPodName                   = "K8S_POD_NAME"
	concurrencyDefault              = 10
	linodeMachineConcurrencyDefault = 1
	qpsDefault                      = 20
	burstDefault                    = 30
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(controller.Scheme))
	utilruntime.Must(capi.AddToScheme(controller.Scheme))
	utilruntime.Must(infrastructurev1alpha1.AddToScheme(controller.Scheme))
	utilruntime.Must(infrastructurev1alpha2.AddToScheme(controller.Scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	CmdConfig := controller.Config{
		LinodeToken:    os.Getenv("LINODE_TOKEN"),
		LinodeDNSToken: os.Getenv("LINODE_DNS_TOKEN"),
	}
	flag.StringVar(&CmdConfig.MachineWatchFilter, "machine-watch-filter", "", "The machines to watch by label.")
	flag.StringVar(&CmdConfig.ClusterWatchFilter, "cluster-watch-filter", "", "The clusters to watch by label.")
	flag.StringVar(&CmdConfig.ObjectStorageBucketWatchFilter, "object-storage-bucket-watch-filter", "", "The object bucket storages to watch by label.")
	flag.StringVar(&CmdConfig.MetricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&CmdConfig.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&CmdConfig.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&CmdConfig.RestConfigQPS, "kube-api-qps", qpsDefault,
		"Maximum queries per second from the controller client to the Kubernetes API server. Defaults to 20")
	flag.IntVar(&CmdConfig.RestConfigBurst, "kube-api-burst", burstDefault,
		"Maximum number of queries that should be allowed in one burst from the controller client to the Kubernetes API server. Default 30")
	flag.IntVar(&CmdConfig.LinodeClusterConcurrency, "linodecluster-concurrency", concurrencyDefault,
		"Number of LinodeClusters to process simultaneously. Default 10")
	flag.IntVar(&CmdConfig.LinodeMachineConcurrency, "linodemachine-concurrency", linodeMachineConcurrencyDefault,
		"Number of LinodeMachines to process simultaneously. Default 10")
	flag.IntVar(&CmdConfig.LinodeObjectStorageBucketConcurrency, "linodeobjectstoragebucket-concurrency", concurrencyDefault,
		"Number of linodeObjectStorageBuckets to process simultaneously. Default 10")
	flag.IntVar(&CmdConfig.LinodeVPCConcurrency, "linodevpc-concurrency", concurrencyDefault,
		"Number of LinodeVPCs to process simultaneously. Default 10")
	flag.IntVar(&CmdConfig.LinodePlacementGroupConcurrency, "linodeplacementgroup-concurrency", concurrencyDefault,
		"Number of Linode Placement Groups to process simultaneously. Default 10")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	controller.SetupLog.Info(fmt.Sprintf("CAPL version: %s", version.GetVersion()))

	mgr, err := controller.SetupManagers(CmdConfig)
	if err != nil {
		controller.SetupLog.Error(err, "setup-managers")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	// closure for mgr.Start, so we defers are running
	run := func(ctx context.Context) error {
		o11yShutdown := controller.SetupObservability(ctx)
		defer o11yShutdown()

		controller.SetupLog.Info("starting manager")
		return mgr.Start(ctx)
	}

	if err := run(ctx); err != nil {
		controller.SetupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
