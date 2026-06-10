package main

import (
	"context"
	"flag"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/controllers"
	"github.com/terminus-io/a2a-registry/internal/healthcheck"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(a2aiov1.AddToScheme(scheme))
}


func main() {
	var metricsAddr string
	var probeAddr string
	var registryAPIAddr string
	var enableLeaderElection bool
	var leaderElectionID string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoints bind to.")
	flag.StringVar(&registryAPIAddr, "registry-api-bind-address", ":8082", "The address the registry API binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "a2a-registry", "Leader election id.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Shared dependencies
	resolver := registry.NewAgentCardResolver(30 * time.Second)
	checker := healthcheck.NewChecker(resolver)

	// Registry API server (runs as a Runnable within the manager)
	apiServer := registry.NewServer(mgr.GetClient(), registryAPIAddr)
	if err := mgr.Add(apiServer); err != nil {
		setupLog.Error(err, "unable to add registry API server")
		os.Exit(1)
	}

	// A2AAgent controller
	if err = (&controllers.A2AAgentReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		AgentCardResolver: resolver,
		HealthChecker:     checker,
		Recorder:          mgr.GetEventRecorderFor("a2aagent-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "A2AAgent")
		os.Exit(1)
	}

	// A2ARegistry controller
	if err = (&controllers.A2ARegistryReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		APIServer: apiServer,
		Recorder:  mgr.GetEventRecorderFor("a2aregistry-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "A2ARegistry")
		os.Exit(1)
	}

	// Webhooks (if enabled)
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = a2aiov1.SetupA2AAgentWebhook(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "A2AAgent")
			os.Exit(1)
		}
		if err = a2aiov1.SetupA2ARegistryWebhook(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "A2ARegistry")
			os.Exit(1)
		}
	}

	// Health probes
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Ensure the default agent namespace exists
	if err := ensureNamespace(mgr.GetClient(), "outbound-agent"); err != nil {
		setupLog.Error(err, "unable to ensure default namespace")
		// non-fatal: operator continues even if namespace creation fails
	}


	setupLog.Info("starting A2A Registry manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// ensureNamespace creates the namespace if it does not already exist.
func ensureNamespace(c client.Client, name string) error {
	ctx := context.Background()
	ns := &corev1.Namespace{}
	if err := c.Get(ctx, client.ObjectKey{Name: name}, ns); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if err := c.Create(ctx, ns); err != nil {
			return err
		}
		setupLog.Info("created default agent namespace", "namespace", name)
	}
	return nil
}