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
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrastructurev1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	controller2 "github.com/linode/cluster-api-provider-linode/controller"
	"github.com/linode/cluster-api-provider-linode/observability/tracing"
	"github.com/linode/cluster-api-provider-linode/observability/wrappers/reconciler"
	"github.com/linode/cluster-api-provider-linode/version"

	_ "go.uber.org/automaxprocs"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	controllerName = "cluster-api-provider-linode.linode.com"
	gracePeriod    = 5 * time.Second
	envK8sNodeName = "K8S_NODE_NAME"
	envK8sPodName  = "K8S_POD_NAME"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(capi.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1alpha1.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1alpha2.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		// Environment variables
		linodeToken    = os.Getenv("LINODE_TOKEN")
		linodeDNSToken = os.Getenv("LINODE_DNS_TOKEN")

		machineWatchFilter             string
		clusterWatchFilter             string
		objectStorageBucketWatchFilter string
		metricsAddr                    string
		enableLeaderElection           bool
		probeAddr                      string
	)
	flag.StringVar(&machineWatchFilter, "machine-watch-filter", "", "The machines to watch by label.")
	flag.StringVar(&clusterWatchFilter, "cluster-watch-filter", "", "The clusters to watch by label.")
	flag.StringVar(&objectStorageBucketWatchFilter, "object-storage-bucket-watch-filter", "", "The object bucket storages to watch by label.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog.Info(fmt.Sprintf("CAPL version: %s", version.GetVersion()))
	// Check environment variables
	if linodeToken == "" {
		setupLog.Error(errors.New("failed to get LINODE_TOKEN environment variable"), "unable to start operator")
		os.Exit(1)
	}
	if linodeDNSToken == "" {
		linodeDNSToken = linodeToken
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "3cfd31c3.cluster.x-k8s.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if mgr == nil || err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = reconciler.NewReconcilerWithTracing(
		&controller2.LinodeClusterReconciler{
			Client:           mgr.GetClient(),
			Recorder:         mgr.GetEventRecorderFor("LinodeClusterReconciler"),
			WatchFilterValue: clusterWatchFilter,
			LinodeApiKey:     linodeToken,
		},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeCluster")
		os.Exit(1)
	}

	if err = reconciler.NewReconcilerWithTracing(
		&controller2.LinodeMachineReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			Recorder:         mgr.GetEventRecorderFor("LinodeMachineReconciler"),
			WatchFilterValue: machineWatchFilter,
			LinodeApiKey:     linodeToken,
			LinodeDNSAPIKey:  linodeDNSToken,
		},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeMachine")
		os.Exit(1)
	}

	if err = reconciler.NewReconcilerWithTracing(
		&controller2.LinodeVPCReconciler{
			Client:           mgr.GetClient(),
			Recorder:         mgr.GetEventRecorderFor("LinodeVPCReconciler"),
			WatchFilterValue: clusterWatchFilter,
			LinodeApiKey:     linodeToken,
		},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeVPC")
		os.Exit(1)
	}

	if err = reconciler.NewReconcilerWithTracing(
		&controller2.LinodeObjectStorageBucketReconciler{
			Client:           mgr.GetClient(),
			Logger:           ctrl.Log.WithName("LinodeObjectStorageBucketReconciler"),
			Recorder:         mgr.GetEventRecorderFor("LinodeObjectStorageBucketReconciler"),
			WatchFilterValue: objectStorageBucketWatchFilter,
			LinodeApiKey:     linodeToken,
		},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeObjectStorageBucket")
		os.Exit(1)
	}
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		setupWebhooks(mgr)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	// closure for mgr.Start, so we defers are running
	run := func(ctx context.Context) error {
		o11yShutdown := setupObservabillity(ctx)
		defer o11yShutdown()

		setupLog.Info("starting manager")
		return mgr.Start(ctx)
	}

	if err := run(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupWebhooks(mgr manager.Manager) {
	var err error
	if err = (&infrastructurev1alpha1.LinodeCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeCluster")
		os.Exit(1)
	}
	if err = (&infrastructurev1alpha2.LinodeCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeCluster")
		os.Exit(1)
	}
	if err = (&infrastructurev1alpha1.LinodeMachine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeMachine")
		os.Exit(1)
	}
	if err = (&infrastructurev1alpha1.LinodeVPC{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeVPC")
		os.Exit(1)
	}
	if err = (&infrastructurev1alpha1.LinodeObjectStorageBucket{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeObjectStorageBucket")
		os.Exit(1)
	}
}

func setupObservabillity(ctx context.Context) func() {
	node := os.Getenv(envK8sNodeName)
	pod := os.Getenv(envK8sPodName)

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(controllerName),
		semconv.ServiceVersion(version.GetVersion()),
		semconv.K8SPodName(pod),
		semconv.K8SNodeName(node),
	)

	tracingShutdown, err := tracing.Setup(ctx, res)
	if err != nil {
		setupLog.Error(err, "failed to setup tracing")
	}

	attrs := []any{}

	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(k, "OTEL_") {
			attrs = append(attrs, k, v)
		}
	}

	setupLog.Info("opentelemetry configuration applied",
		attrs...,
	)

	return func() {
		timeout := 25 * time.Second //nolint:mnd // 2.5x default OTLP timeout

		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()

		wg := &sync.WaitGroup{}

		if tracingShutdown != nil {
			wg.Add(1)

			go func() {
				defer wg.Done()

				if err := tracingShutdown(ctx); err != nil {
					setupLog.Error(err, "failed to shutdown tracing")
				}
			}()
		}

		wg.Wait()
	}
}
