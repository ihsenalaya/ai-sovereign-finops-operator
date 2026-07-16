/*
Copyright 2026.

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

// Command govar-manager is the standalone controller manager of the
// ai-govar-operator: it reconciles AIWorkloadBinding and hosts the
// AIChangeRequest approval stamping/validation webhooks that the GOV-AR
// admission chain requires (authenticated requester/reviewer evidence,
// fail closed). The runtime admission itself is served by the separate
// gov-ar-admission binary; calibration evidence is produced by the
// separate gov-ar-calibration-producer binary.
package main

import (
	"crypto/tls"
	"flag"
	"os"
	"path/filepath"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/controller"
	"github.com/imperium/ai-sovereign-finops-operator/internal/webhook/bootstrap"
	"github.com/imperium/ai-sovereign-finops-operator/internal/webhook/changeapproval"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(aiopsv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Disable http/2 by default (Rapid Reset / Stream Cancellation CVEs).
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}
	webhookCertDir := filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs")
	cfg := ctrl.GetConfigOrDie()

	webhookServer := webhook.NewServer(webhook.Options{
		CertDir: webhookCertDir,
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "govar.2d92db7d.imperium.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	bootstrapClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create bootstrap client")
		os.Exit(1)
	}

	if err = (&controller.AIWorkloadBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AIWorkloadBinding")
		os.Exit(1)
	}

	// The GOV-AR approval chain is fail-closed: these webhooks stamp and
	// validate authenticated requester/reviewer identity on AIChangeRequest
	// objects. Both paths receive only aichangerequests requests (the
	// bootstrap below registers ScopeChangeRequests rules).
	mgr.GetWebhookServer().Register("/mutate-aiops-changeapproval", &admission.Webhook{
		Handler: changeapproval.NewMutation(mgr.GetScheme()),
	})
	mgr.GetWebhookServer().Register("/validate-aiops-changeapproval", &admission.Webhook{
		Handler: changeapproval.NewValidation(mgr.GetScheme()),
	})

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	ctx := ctrl.SetupSignalHandler()
	if err := bootstrap.Ensure(ctx, bootstrap.Options{
		Client:           bootstrapClient,
		Name:             "aiops-govar-change-approval",
		ServiceName:      os.Getenv("WEBHOOK_SERVICE_NAME"),
		ServiceNamespace: os.Getenv("POD_NAMESPACE"),
		Path:             "/mutate-aiops-changeapproval",
		CertDir:          webhookCertDir,
		Scope:            bootstrap.ScopeChangeRequests,
	}); err != nil {
		setupLog.Error(err, "unable to bootstrap mutating webhook")
		os.Exit(1)
	}
	if err := bootstrap.EnsureValidation(ctx, bootstrap.Options{
		Client:           bootstrapClient,
		Name:             "aiops-govar-change-approval-validator",
		ServiceName:      os.Getenv("WEBHOOK_SERVICE_NAME"),
		ServiceNamespace: os.Getenv("POD_NAMESPACE"),
		Path:             "/validate-aiops-changeapproval",
		CertDir:          webhookCertDir,
		Scope:            bootstrap.ScopeChangeRequests,
	}); err != nil {
		setupLog.Error(err, "unable to bootstrap validating webhook")
		os.Exit(1)
	}

	setupLog.Info("starting govar manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
