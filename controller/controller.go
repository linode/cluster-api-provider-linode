package controller

import (
	"context"
	"errors"
	"fmt"
	"github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/observability/tracing"
	"github.com/linode/cluster-api-provider-linode/observability/wrappers/reconciler"
	"github.com/linode/cluster-api-provider-linode/version"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/semconv/v1.25.0"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"strings"
	"sync"
	"time"
)

var (
	Scheme   = runtime.NewScheme()
	SetupLog = controllerruntime.Log.WithName("setup")
)

const (
	Name           = "cluster-api-provider-linode.linode.com"
	EnvK8sNodeName = "K8S_NODE_NAME"
	EnvK8sPodName  = "K8S_POD_NAME"
)

type Config struct {
	LinodeToken    string
	LinodeDNSToken string

	MachineWatchFilter             string
	ClusterWatchFilter             string
	ObjectStorageBucketWatchFilter string
	MetricsAddr                    string
	EnableLeaderElection           bool
	ProbeAddr                      string

	RestConfigQPS                        int
	RestConfigBurst                      int
	LinodeClusterConcurrency             int
	LinodeMachineConcurrency             int
	LinodeObjectStorageBucketConcurrency int
	LinodeVPCConcurrency                 int
	LinodePlacementGroupConcurrency      int
}

func SetupManagers(CmdConfig Config) (manager.Manager, error) {
	// Check environment variables
	if CmdConfig.LinodeToken == "" {
		return nil, errors.New("failed to get LINODE_TOKEN environment variable")
	}
	if CmdConfig.LinodeDNSToken == "" {
		CmdConfig.LinodeDNSToken = CmdConfig.LinodeToken
	}

	restConfig := controllerruntime.GetConfigOrDie()
	restConfig.QPS = float32(CmdConfig.RestConfigQPS)
	restConfig.Burst = CmdConfig.RestConfigBurst
	restConfig.UserAgent = fmt.Sprintf("CAPL/%s", version.GetVersion())

	mgr, err := controllerruntime.NewManager(restConfig, controllerruntime.Options{
		Scheme:                 Scheme,
		Metrics:                server.Options{BindAddress: CmdConfig.MetricsAddr},
		HealthProbeBindAddress: CmdConfig.ProbeAddr,
		LeaderElection:         CmdConfig.EnableLeaderElection,
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
		return nil, err
	}

	if err = reconciler.NewReconcilerWithTracing(
		&LinodeClusterReconciler{
			Client:           mgr.GetClient(),
			Recorder:         mgr.GetEventRecorderFor("LinodeClusterReconciler"),
			WatchFilterValue: CmdConfig.ClusterWatchFilter,
			LinodeApiKey:     CmdConfig.LinodeToken,
		},
	).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: CmdConfig.LinodeClusterConcurrency}); err != nil {
		return nil, err
	}

	if err = reconciler.NewReconcilerWithTracing(
		&LinodeMachineReconciler{
			Client:           mgr.GetClient(),
			Recorder:         mgr.GetEventRecorderFor("LinodeMachineReconciler"),
			WatchFilterValue: CmdConfig.MachineWatchFilter,
			LinodeApiKey:     CmdConfig.LinodeToken,
			LinodeDNSAPIKey:  CmdConfig.LinodeDNSToken,
		},
	).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: CmdConfig.LinodeMachineConcurrency}); err != nil {
		return nil, err
	}

	if err = reconciler.NewReconcilerWithTracing(
		&LinodeVPCReconciler{
			Client:           mgr.GetClient(),
			Recorder:         mgr.GetEventRecorderFor("LinodeVPCReconciler"),
			WatchFilterValue: CmdConfig.ClusterWatchFilter,
			LinodeApiKey:     CmdConfig.LinodeToken,
		},
	).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: CmdConfig.LinodeVPCConcurrency}); err != nil {
		return nil, err
	}

	if err = reconciler.NewReconcilerWithTracing(
		&LinodeObjectStorageBucketReconciler{
			Client:           mgr.GetClient(),
			Logger:           controllerruntime.Log.WithName("LinodeObjectStorageBucketReconciler"),
			Recorder:         mgr.GetEventRecorderFor("LinodeObjectStorageBucketReconciler"),
			WatchFilterValue: CmdConfig.ObjectStorageBucketWatchFilter,
			LinodeApiKey:     CmdConfig.LinodeToken,
		},
	).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: CmdConfig.LinodeObjectStorageBucketConcurrency}); err != nil {
		return nil, err
	}

	if err = reconciler.NewReconcilerWithTracing(
		&LinodePlacementGroupReconciler{
			Client:           mgr.GetClient(),
			Recorder:         mgr.GetEventRecorderFor("LinodePlacementGroupReconciler"),
			WatchFilterValue: CmdConfig.ClusterWatchFilter,
			LinodeApiKey:     CmdConfig.LinodeToken,
		},
	).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: CmdConfig.LinodePlacementGroupConcurrency}); err != nil {
		return nil, err
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		SetupWebhooks(mgr)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, err
	}
	return mgr, nil
}

func SetupWebhooks(mgr manager.Manager) {
	var err error
	if err = (&v1alpha1.LinodeCluster{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeCluster")
		os.Exit(1)
	}
	if err = (&v1alpha2.LinodeCluster{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeCluster")
		os.Exit(1)
	}
	if err = (&v1alpha2.LinodeClusterTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeCluster")
		os.Exit(1)
	}
	if err = (&v1alpha1.LinodeMachine{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeMachine")
		os.Exit(1)
	}
	if err = (&v1alpha2.LinodeMachine{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeMachine")
		os.Exit(1)
	}
	if err = (&v1alpha2.LinodeMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeCluster")
		os.Exit(1)
	}
	if err = (&v1alpha1.LinodeVPC{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeVPC")
		os.Exit(1)
	}
	if err = (&v1alpha2.LinodeVPC{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeVPC")
		os.Exit(1)
	}
	if err = (&v1alpha1.LinodeObjectStorageBucket{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodeObjectStorageBucket")
		os.Exit(1)
	}
	if err = (&v1alpha2.LinodePlacementGroup{}).SetupWebhookWithManager(mgr); err != nil {
		SetupLog.Error(err, "unable to create webhook", "webhook", "LinodePlacementGroup")
		os.Exit(1)
	}
}

func SetupObservability(ctx context.Context) func() {
	node := os.Getenv(EnvK8sNodeName)
	pod := os.Getenv(EnvK8sPodName)

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(Name),
		semconv.ServiceVersion(version.GetVersion()),
		semconv.K8SPodName(pod),
		semconv.K8SNodeName(node),
	)

	tracingShutdown, err := tracing.Setup(ctx, res)
	if err != nil {
		SetupLog.Error(err, "failed to setup tracing")
	}

	attrs := []any{}

	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(k, "OTEL_") {
			attrs = append(attrs, k, v)
		}
	}

	SetupLog.Info("opentelemetry configuration applied",
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
					SetupLog.Error(err, "failed to shutdown tracing")
				}
			}()
		}

		wg.Wait()
	}
}
