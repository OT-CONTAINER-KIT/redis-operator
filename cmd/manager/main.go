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

package main

import (
	"flag"
	"os"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	internalenv "github.com/OT-CONTAINER-KIT/redis-operator/internal/env"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/agent/bootstrap"
	rediscontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/redis"
	redisclustercontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/rediscluster"
	redisreplicationcontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/redisreplication"
	redissentinelcontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/redissentinel"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/features"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/monitoring"
	coreWebhook "github.com/OT-CONTAINER-KIT/redis-operator/pkg/webhook"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

//nolint:gochecknoinits
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(redisv1beta2.AddToScheme(scheme))
	monitoring.RegisterRedisReplicationMetrics()
	//+kubebuilder:scaffold:scheme
}

func createManagerCommand() *cobra.Command {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enableWebhooks bool
	var maxConcurrentReconciles int
	var featureGatesString string

	// Create zap options
	zapOptions := zap.Options{
		Development: false,
	}

	// Create the standard golang flag set for zap flags
	zapFlagSet := flag.NewFlagSet("zap", flag.ExitOnError)
	zapOptions.BindFlags(zapFlagSet)

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Start the Redis operator manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set up logging
			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOptions)))

			// Parse feature gates from flags
			if len(featureGatesString) > 0 {
				if err := features.MutableFeatureGate.Set(featureGatesString); err != nil {
					setupLog.Error(err, "unable to set feature gates")
					return err
				}
			}
			// Log enabled feature gates
			setupLog.Info("Feature gates enabled", "GenerateConfigInInitContainer", features.Enabled(features.GenerateConfigInInitContainer))

			options := ctrl.Options{
				Scheme: scheme,
				Metrics: metricsserver.Options{
					BindAddress: metricsAddr,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
					},
				},
				HealthProbeBindAddress: probeAddr,
				LeaderElection:         enableLeaderElection,
				LeaderElectionID:       "6cab913b.redis.opstreelabs.in",
			}

			// Use env package to get max concurrent reconciles
			maxConcurrentReconciles = internalenv.GetMaxConcurrentReconciles(maxConcurrentReconciles)

			// Use env package to get watch namespaces
			watchNamespaces := internalenv.GetWatchNamespaces()
			if len(watchNamespaces) > 0 {
				options.Cache.DefaultNamespaces = map[string]cache.Config{}
				for _, ns := range watchNamespaces {
					options.Cache.DefaultNamespaces[ns] = cache.Config{}
				}
			}

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
			if err != nil {
				setupLog.Error(err, "unable to start manager")
				return err
			}

			k8sclient, err := k8sutils.GenerateK8sClient(k8sutils.GenerateK8sConfig())
			if err != nil {
				setupLog.Error(err, "unable to create k8s client")
				return err
			}

			dk8sClient, err := k8sutils.GenerateK8sDynamicClient(k8sutils.GenerateK8sConfig())
			if err != nil {
				setupLog.Error(err, "unable to create k8s dynamic client")
				return err
			}

			if err = (&rediscontroller.Reconciler{
				Client:    mgr.GetClient(),
				K8sClient: k8sclient,
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "Redis")
				return err
			}
			if err = (&redisclustercontroller.Reconciler{
				Client:      mgr.GetClient(),
				K8sClient:   k8sclient,
				Dk8sClient:  dk8sClient,
				Recorder:    mgr.GetEventRecorderFor("rediscluster-controller"),
				StatefulSet: k8sutils.NewStatefulSetService(k8sclient),
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "RedisCluster")
				return err
			}
			if err = (&redisreplicationcontroller.Reconciler{
				Client:      mgr.GetClient(),
				K8sClient:   k8sclient,
				Dk8sClient:  dk8sClient,
				Pod:         k8sutils.NewPodService(k8sclient),
				StatefulSet: k8sutils.NewStatefulSetService(k8sclient),
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "RedisReplication")
				return err
			}
			if err = (&redissentinelcontroller.RedisSentinelReconciler{
				Client:             mgr.GetClient(),
				K8sClient:          k8sclient,
				Dk8sClient:         dk8sClient,
				ReplicationWatcher: intctrlutil.NewResourceWatcher(),
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "RedisSentinel")
				return err
			}

			if enableWebhooks {
				if err = (&redisv1beta2.Redis{}).SetupWebhookWithManager(mgr); err != nil {
					setupLog.Error(err, "unable to create webhook", "webhook", "Redis")
					return err
				}
				if err = (&redisv1beta2.RedisCluster{}).SetupWebhookWithManager(mgr); err != nil {
					setupLog.Error(err, "unable to create webhook", "webhook", "RedisCluster")
					return err
				}
				if err = (&redisv1beta2.RedisReplication{}).SetupWebhookWithManager(mgr); err != nil {
					setupLog.Error(err, "unable to create webhook", "webhook", "RedisReplication")
					return err
				}
				if err = (&redisv1beta2.RedisSentinel{}).SetupWebhookWithManager(mgr); err != nil {
					setupLog.Error(err, "unable to create webhook", "webhook", "RedisSentinel")
					return err
				}

				wblog := ctrl.Log.WithName("webhook").WithName("PodAffiniytMutate")
				mgr.GetWebhookServer().Register("/mutate-core-v1-pod", &webhook.Admission{
					Handler: coreWebhook.NewPodAffiniytMutate(mgr.GetClient(), admission.NewDecoder(scheme), wblog),
				})
			}
			// +kubebuilder:scaffold:builder

			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up health check")
				return err
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up ready check")
				return err
			}

			setupLog.Info("starting manager")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				setupLog.Error(err, "problem running manager")
				return err
			}
			return nil
		},
	}

	// Define flags for the manager command
	cmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	cmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	cmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().BoolVar(&enableWebhooks, "enable-webhooks", internalenv.IsWebhookEnabled(), "Enable webhooks")
	cmd.Flags().IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1, "Max concurrent reconciles")
	cmd.Flags().StringVar(&featureGatesString, "feature-gates", internalenv.GetFeatureGates(), "A set of key=value pairs that describe feature gates for alpha/experimental features. "+
		"Options are:\n  GenerateConfigInInitContainer=true|false: enables using init container for config generation")

	// Add the zap flags from the flag set to the command's flags
	zapFlagSet.VisitAll(func(f *flag.Flag) {
		cmd.Flags().AddGoFlag(f)
	})

	return cmd
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "operator",
		Short: "Redis Operator for Kubernetes",
	}

	// Add manager subcommand
	rootCmd.AddCommand(createManagerCommand())

	// Add agent subcommand
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent is a tool which run as a init/sidecar container along with redis/sentinel",
	}
	agentCmd.AddCommand(bootstrap.BootstrapCmd)
	rootCmd.AddCommand(agentCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
