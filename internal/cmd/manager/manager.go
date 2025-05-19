/*
Copyright 2020 Opstree Solutions.

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

package manager

import (
	"flag"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/scheme"
	rediscontroller "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/redis"
	redisclustercontroller "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/rediscluster"
	redisreplicationcontroller "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/redisreplication"
	redissentinelcontroller "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/redissentinel"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	internalenv "github.com/OT-CONTAINER-KIT/redis-operator/internal/env"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/monitoring"
	coreWebhook "github.com/OT-CONTAINER-KIT/redis-operator/internal/webhook"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var setupLog = ctrl.Log.WithName("setup")

// managerOptions contains all options needed for the manager
type managerOptions struct {
	metricsAddr             string
	probeAddr               string
	enableLeaderElection    bool
	enableWebhooks          bool
	maxConcurrentReconciles int
	featureGatesString      string
	zapOptions              zap.Options
}

// CreateCommand creates a cobra command for running the Redis operator manager
func CreateCommand() *cobra.Command {
	opts := &managerOptions{
		zapOptions: zap.Options{
			Development: false,
		},
	}

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Start the Redis operator manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runManager(opts)
		},
	}

	addFlags(cmd, opts)

	return cmd
}

// addFlags adds command line flags
func addFlags(cmd *cobra.Command, opts *managerOptions) {
	cmd.Flags().StringVar(&opts.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	cmd.Flags().StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	cmd.Flags().BoolVar(&opts.enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().BoolVar(&opts.enableWebhooks, "enable-webhooks", internalenv.IsWebhookEnabled(), "Enable webhooks")
	cmd.Flags().IntVar(&opts.maxConcurrentReconciles, "max-concurrent-reconciles", 1, "Max concurrent reconciles")
	cmd.Flags().StringVar(&opts.featureGatesString, "feature-gates", internalenv.GetFeatureGates(), "A set of key=value pairs that describe feature gates for alpha/experimental features. "+
		"Options are:\n  GenerateConfigInInitContainer=true|false: enables using init container for config generation")

	zapFlagSet := flag.NewFlagSet("zap", flag.ExitOnError)
	opts.zapOptions.BindFlags(zapFlagSet)
	zapFlagSet.VisitAll(func(f *flag.Flag) {
		cmd.Flags().AddGoFlag(f)
	})
}

// runManager executes the main logic of the manager
func runManager(opts *managerOptions) error {
	// Setup logging
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts.zapOptions)))

	monitoring.RegisterRedisReplicationMetrics()

	setupLog.Info("setting up v1beta2 scheme")
	scheme.SetupV1beta2Scheme()

	if err := setupFeatureGates(opts.featureGatesString); err != nil {
		return err
	}
	ctrlOptions := createControllerOptions(opts)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrlOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}
	k8sClient, dk8sClient, err := createK8sClients()
	if err != nil {
		return err
	}
	if err := setupControllers(mgr, k8sClient, dk8sClient, opts.maxConcurrentReconciles); err != nil {
		return err
	}
	if opts.enableWebhooks {
		if err := setupWebhooks(mgr); err != nil {
			return err
		}
	}
	if err := setupHealthChecks(mgr); err != nil {
		return err
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}

// setupFeatureGates sets up feature gates
func setupFeatureGates(featureGatesString string) error {
	if len(featureGatesString) > 0 {
		if err := features.MutableFeatureGate.Set(featureGatesString); err != nil {
			setupLog.Error(err, "unable to set feature gates")
			return err
		}
	}
	return nil
}

// createControllerOptions creates configuration options for the manager
func createControllerOptions(opts *managerOptions) ctrl.Options {
	options := ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress: opts.metricsAddr,
		},
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: 9443,
			},
		},
		HealthProbeBindAddress: opts.probeAddr,
		LeaderElection:         opts.enableLeaderElection,
		LeaderElectionID:       "6cab913b.redis.opstreelabs.in",
	}

	watchNamespaces := internalenv.GetWatchNamespaces()
	if len(watchNamespaces) > 0 {
		options.Cache.DefaultNamespaces = map[string]cache.Config{}
		for _, ns := range watchNamespaces {
			options.Cache.DefaultNamespaces[ns] = cache.Config{}
		}
	}

	return options
}

// createK8sClients creates Kubernetes clients
func createK8sClients() (kubernetes.Interface, dynamic.Interface, error) {
	k8sConfig := k8sutils.GenerateK8sConfig()

	k8sClient, err := k8sutils.GenerateK8sClient(k8sConfig)
	if err != nil {
		setupLog.Error(err, "unable to create k8s client")
		return nil, nil, err
	}

	dk8sClient, err := k8sutils.GenerateK8sDynamicClient(k8sConfig)
	if err != nil {
		setupLog.Error(err, "unable to create k8s dynamic client")
		return nil, nil, err
	}

	return k8sClient, dk8sClient, nil
}

// setupControllers sets up all controllers
func setupControllers(mgr ctrl.Manager, k8sClient kubernetes.Interface, dk8sClient dynamic.Interface, maxConcurrentReconciles int) error {
	// Get max concurrent reconciles from environment
	maxConcurrentReconciles = internalenv.GetMaxConcurrentReconciles(maxConcurrentReconciles)

	if err := (&rediscontroller.Reconciler{
		Client:    mgr.GetClient(),
		K8sClient: k8sClient,
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Redis")
		return err
	}
	if err := (&redisclustercontroller.Reconciler{
		Client:      mgr.GetClient(),
		K8sClient:   k8sClient,
		Dk8sClient:  dk8sClient,
		Recorder:    mgr.GetEventRecorderFor("rediscluster-controller"),
		StatefulSet: k8sutils.NewStatefulSetService(k8sClient),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RedisCluster")
		return err
	}
	if err := (&redisreplicationcontroller.Reconciler{
		Client:      mgr.GetClient(),
		K8sClient:   k8sClient,
		Dk8sClient:  dk8sClient,
		Pod:         k8sutils.NewPodService(k8sClient),
		StatefulSet: k8sutils.NewStatefulSetService(k8sClient),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RedisReplication")
		return err
	}
	if err := (&redissentinelcontroller.RedisSentinelReconciler{
		Client:             mgr.GetClient(),
		K8sClient:          k8sClient,
		Dk8sClient:         dk8sClient,
		ReplicationWatcher: intctrlutil.NewResourceWatcher(),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RedisSentinel")
		return err
	}

	return nil
}

// setupWebhooks sets up all webhooks
func setupWebhooks(mgr ctrl.Manager) error {
	if err := (&rvb2.Redis{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Redis")
		return err
	}
	if err := (&rcvb2.RedisCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "RedisCluster")
		return err
	}
	if err := (&rrvb2.RedisReplication{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "RedisReplication")
		return err
	}
	if err := (&rsvb2.RedisSentinel{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "RedisSentinel")
		return err
	}

	wblog := ctrl.Log.WithName("webhook").WithName("PodAffiniytMutate")
	mgr.GetWebhookServer().Register("/mutate-core-v1-pod", &webhook.Admission{
		Handler: coreWebhook.NewPodAffiniytMutate(mgr.GetClient(), admission.NewDecoder(scheme.Scheme), wblog),
	})

	return nil
}

// setupHealthChecks sets up health and readiness checks
func setupHealthChecks(mgr ctrl.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	return nil
}
