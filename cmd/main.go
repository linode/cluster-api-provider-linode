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
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrastructurev1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	infrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/internal/controller"
	webhookinfrastructurev1alpha2 "github.com/linode/cluster-api-provider-linode/internal/webhook/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/observability/tracing"
	"github.com/linode/cluster-api-provider-linode/version"

	_ "go.uber.org/automaxprocs"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	controllerName     = "cluster-api-provider-linode.linode.com"
	envK8sNodeName     = "K8S_NODE_NAME"
	envK8sPodName      = "K8S_POD_NAME"
	concurrencyDefault = 10
	qpsDefault         = 20
	burstDefault       = 30
)

type flagVars struct {
	machineWatchFilter                   string
	clusterWatchFilter                   string
	objectStorageBucketWatchFilter       string
	objectStorageKeyWatchFilter          string
	metricsAddr                          string
	secureMetrics                        bool
	enableLeaderElection                 bool
	probeAddr                            string
	restConfigQPS                        int
	restConfigBurst                      int
	linodeClusterConcurrency             int
	linodeMachineConcurrency             int
	linodeObjectStorageBucketConcurrency int
	linodeVPCConcurrency                 int
	linodePlacementGroupConcurrency      int
	linodeFirewallConcurrency            int
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(capi.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1alpha1.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1alpha2.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	flags, opts := parseFlags()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info(fmt.Sprintf("CAPL version: %s", version.GetVersion()))

	linodeClientConfig, dnsClientConfig := validateEnvironment()
	mgr := setupManager(flags, linodeClientConfig, dnsClientConfig)

	ctx := ctrl.SetupSignalHandler()
	run := func(ctx context.Context) error {
		o11yShutdown := setupObservability(ctx)
		defer o11yShutdown()

		setupLog.Info("starting manager")
		return mgr.Start(ctx)
	}

	if err := run(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// parseFlags initializes command-line flags and returns the parsed flags and zap options.
// It sets up various configuration options for the application, including filters, metrics,
// health probe addresses, leader election, and concurrency settings.
func parseFlags() (flags flagVars, opts zap.Options) {
	flags = flagVars{}
	flag.StringVar(&flags.machineWatchFilter, "machine-watch-filter", "", "The machines to watch by label.")
	flag.StringVar(&flags.clusterWatchFilter, "cluster-watch-filter", "", "The clusters to watch by label.")
	flag.StringVar(&flags.objectStorageBucketWatchFilter, "object-storage-bucket-watch-filter", "", "The object bucket storages to watch by label.")
	flag.StringVar(&flags.metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")

	// Mitigate CVE-2023-44487 by disabling HTTP2 by default until the Go
	// standard library and golang.org/x/net are fully fixed.
	// Right now, it is possible for authenticated and unauthenticated users to
	// hold open HTTP2 connections and consume huge amounts of memory.
	// See:
	// * https://github.com/kubernetes/kubernetes/pull/121120
	// * https://github.com/kubernetes/kubernetes/issues/121197
	// * https://github.com/golang/go/issues/63417#issuecomment-1758858612
	flag.BoolVar(&flags.secureMetrics, "metrics-secure", false, "If set, the metrics endpoint is served securely via HTTPS.")
	flag.StringVar(&flags.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&flags.enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.IntVar(&flags.restConfigQPS, "kube-api-qps", qpsDefault, "Maximum queries per second from the controller client")
	flag.IntVar(&flags.restConfigBurst, "kube-api-burst", burstDefault, "Maximum number of queries in one burst")
	flag.IntVar(&flags.linodeClusterConcurrency, "linodecluster-concurrency", concurrencyDefault, "Number of LinodeClusters to process simultaneously")
	flag.IntVar(&flags.linodeMachineConcurrency, "linodemachine-concurrency", concurrencyDefault, "Number of LinodeMachines to process simultaneously")
	flag.IntVar(&flags.linodeObjectStorageBucketConcurrency, "linodeobjectstoragebucket-concurrency", concurrencyDefault, "Number of linodeObjectStorageBuckets to process simultaneously")
	flag.IntVar(&flags.linodeVPCConcurrency, "linodevpc-concurrency", concurrencyDefault, "Number of LinodeVPCs to process simultaneously")
	flag.IntVar(&flags.linodePlacementGroupConcurrency, "linodeplacementgroup-concurrency", concurrencyDefault, "Number of Linode Placement Groups to process simultaneously")
	flag.IntVar(&flags.linodeFirewallConcurrency, "linodefirewall-concurrency", concurrencyDefault, "Number of Linode Firewall to process simultaneously")

	opts = zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	return flags, opts
}

// validateEnvironment checks for required environment variables and returns the configuration for Linode and DNS clients.
// It ensures that LINODE_TOKEN is set and defaults LINODE_DNS_TOKEN to LINODE_TOKEN if not provided.
func validateEnvironment() (linodeConfig, dnsConfig scope.ClientConfig) {
	linodeToken := os.Getenv("LINODE_TOKEN")
	linodeDNSToken := os.Getenv("LINODE_DNS_TOKEN")
	linodeDNSURL := os.Getenv("LINODE_DNS_URL")
	linodeDNSCA := os.Getenv("LINODE_DNS_CA")

	if linodeToken == "" {
		setupLog.Error(errors.New("failed to get LINODE_TOKEN environment variable"), "unable to start operator")
		os.Exit(1)
	}
	if linodeDNSToken == "" {
		setupLog.Info("LINODE_DNS_TOKEN not provided, defaulting to the value of LINODE_TOKEN")
		linodeDNSToken = linodeToken
	}

	return scope.ClientConfig{Token: linodeToken},
		scope.ClientConfig{Token: linodeDNSToken, BaseUrl: linodeDNSURL, RootCertificatePath: linodeDNSCA}
}

// setupManager initializes and returns a new manager instance with the provided configurations.
// It sets up the REST configuration, metrics server options, and registers controllers and webhooks.
func setupManager(flags flagVars, linodeConfig, dnsConfig scope.ClientConfig) manager.Manager {
	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = float32(flags.restConfigQPS)
	restConfig.Burst = flags.restConfigBurst
	restConfig.UserAgent = fmt.Sprintf("CAPL/%s", version.GetVersion())

	var tlsOpts []func(*tls.Config)
	if !flags.secureMetrics {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.2"}
		})
	}

	metricsServerOptions := metricsserver.Options{
		BindAddress:   flags.metricsAddr,
		SecureServing: flags.secureMetrics,
		TLSOpts:       tlsOpts,
	}
	if flags.secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: flags.probeAddr,
		LeaderElection:         flags.enableLeaderElection,
		LeaderElectionID:       "3cfd31c3.cluster.x-k8s.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// Ensure mgr is not nil before proceeding
	if mgr == nil {
		setupLog.Error(errors.New("manager creation failed"), "manager is nil")
		os.Exit(1)
	}

	setupControllers(mgr, flags, linodeConfig, dnsConfig)

	// Setup webhooks if enabled (defaults to true)
	webhooksEnabled := true // default to enabled
	if webhooksEnv := os.Getenv("ENABLE_WEBHOOKS"); webhooksEnv != "" {
		var err error
		webhooksEnabled, err = strconv.ParseBool(webhooksEnv)
		if err != nil {
			setupLog.Error(err, "invalid ENABLE_WEBHOOKS value, defaulting to true")
		}
	}
	if webhooksEnabled {
		setupWebhooks(mgr)
	}

	setupHealthChecks(mgr)

	return mgr
}

// setupHealthChecks adds health and readiness checks to the manager.
// It registers a health check at the "healthz" endpoint and a readiness check at the "readyz" endpoint.
func setupHealthChecks(mgr manager.Manager) {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
}

// setupControllers initializes and registers various controllers with the manager.
// It sets up controllers for Linode resources, configuring each with the appropriate client and options.
func setupControllers(mgr manager.Manager, flags flagVars, linodeClientConfig, dnsConfig scope.ClientConfig) {
	// LinodeCluster Controller
	if err := (&controller.LinodeClusterReconciler{
		Client:             mgr.GetClient(),
		Recorder:           mgr.GetEventRecorderFor("LinodeClusterReconciler"),
		WatchFilterValue:   flags.clusterWatchFilter,
		LinodeClientConfig: linodeClientConfig,
		DnsClientConfig:    dnsConfig,
	}).SetupWithManager(mgr, crcontroller.Options{MaxConcurrentReconciles: flags.linodeClusterConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeCluster")
		os.Exit(1)
	}

	useGzip, err := strconv.ParseBool(os.Getenv("GZIP_COMPRESSION_ENABLED"))
	if err != nil {
		setupLog.Error(err, "proceeding without gzip compression for cloud-init data")
	}

	// LinodeMachine Controller
	if err := (&controller.LinodeMachineReconciler{
		Client:                 mgr.GetClient(),
		Recorder:               mgr.GetEventRecorderFor("LinodeMachineReconciler"),
		WatchFilterValue:       flags.machineWatchFilter,
		LinodeClientConfig:     linodeClientConfig,
		GzipCompressionEnabled: useGzip,
	}).SetupWithManager(mgr, crcontroller.Options{MaxConcurrentReconciles: flags.linodeMachineConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeMachine")
		os.Exit(1)
	}

	// LinodeVPC Controller
	if err := (&controller.LinodeVPCReconciler{
		Client:             mgr.GetClient(),
		Recorder:           mgr.GetEventRecorderFor("LinodeVPCReconciler"),
		WatchFilterValue:   flags.clusterWatchFilter,
		LinodeClientConfig: linodeClientConfig,
	}).SetupWithManager(mgr, crcontroller.Options{MaxConcurrentReconciles: flags.linodeVPCConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeVPC")
		os.Exit(1)
	}

	// LinodeObjectStorageBucket Controller
	if err := (&controller.LinodeObjectStorageBucketReconciler{
		Client:             mgr.GetClient(),
		Logger:             ctrl.Log.WithName("LinodeObjectStorageBucketReconciler"),
		Recorder:           mgr.GetEventRecorderFor("LinodeObjectStorageBucketReconciler"),
		WatchFilterValue:   flags.objectStorageBucketWatchFilter,
		LinodeClientConfig: linodeClientConfig,
	}).SetupWithManager(mgr, crcontroller.Options{MaxConcurrentReconciles: flags.linodeObjectStorageBucketConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeObjectStorageBucket")
		os.Exit(1)
	}

	// LinodePlacementGroup Controller
	if err := (&controller.LinodePlacementGroupReconciler{
		Client:             mgr.GetClient(),
		Recorder:           mgr.GetEventRecorderFor("LinodePlacementGroupReconciler"),
		WatchFilterValue:   flags.clusterWatchFilter,
		LinodeClientConfig: linodeClientConfig,
	}).SetupWithManager(mgr, crcontroller.Options{MaxConcurrentReconciles: flags.linodePlacementGroupConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodePlacementGroup")
		os.Exit(1)
	}

	// LinodeObjectStorageKey Controller
	if err := (&controller.LinodeObjectStorageKeyReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Logger:             ctrl.Log.WithName("LinodeObjectStorageKeyReconciler"),
		Recorder:           mgr.GetEventRecorderFor("LinodeObjectStorageKeyReconciler"),
		WatchFilterValue:   flags.objectStorageKeyWatchFilter,
		LinodeClientConfig: linodeClientConfig,
	}).SetupWithManager(mgr, crcontroller.Options{MaxConcurrentReconciles: flags.linodeObjectStorageBucketConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeObjectStorageKey")
		os.Exit(1)
	}

	// LinodeFirewall Controller
	if err := (&controller.LinodeFirewallReconciler{
		Client:             mgr.GetClient(),
		Recorder:           mgr.GetEventRecorderFor("LinodeFirewallReconciler"),
		WatchFilterValue:   flags.clusterWatchFilter,
		LinodeClientConfig: linodeClientConfig,
	}).SetupWithManager(mgr, crcontroller.Options{MaxConcurrentReconciles: flags.linodeFirewallConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinodeFirewall")
		os.Exit(1)
	}
}

// setupWebhooks initializes webhooks for the specified resources in the manager.
// It sets up webhooks for various Linode resources to handle admission control and validation.
func setupWebhooks(mgr manager.Manager) {
	var err error
	if err = webhookinfrastructurev1alpha2.SetupLinodeClusterWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeCluster")
		os.Exit(1)
	}
	if err = webhookinfrastructurev1alpha2.SetupLinodeClusterTemplateWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeClusterTemplate")
		os.Exit(1)
	}
	if err = webhookinfrastructurev1alpha2.SetupLinodeMachineWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeMachine")
		os.Exit(1)
	}
	if err = webhookinfrastructurev1alpha2.SetupLinodeMachineTemplateWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeMachineTemplate")
		os.Exit(1)
	}
	if err = webhookinfrastructurev1alpha2.SetupLinodeVPCWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeVPC")
		os.Exit(1)
	}
	if err = webhookinfrastructurev1alpha2.SetupLinodeObjectStorageBucketWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeObjectStorageBucket")
		os.Exit(1)
	}
	if err = webhookinfrastructurev1alpha2.SetupLinodePlacementGroupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodePlacementGroup")
		os.Exit(1)
	}
	if err = webhookinfrastructurev1alpha2.SetupLinodeObjectStorageKeyWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinodeObjectStorageKey")
		os.Exit(1)
	}
}

// setup configures observability features and returns a cleanup function.
// It sets up OpenTelemetry tracing and logs the configuration applied, returning a function to clean up resources.
func setupObservability(ctx context.Context) func() {
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
