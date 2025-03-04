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
	"strconv"
	"strings"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	rediscontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/redis"
	redisclustercontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/rediscluster"
	redisreplicationcontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/redisreplication"
	redissentinelcontroller "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllers/redissentinel"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/pkg/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/k8sutils"
	coreWebhook "github.com/OT-CONTAINER-KIT/redis-operator/pkg/webhook"
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
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enableWebhooks bool
	var maxConcurrentReconciles int

	flag.BoolVar(&enableWebhooks, "enable-webhooks", os.Getenv("ENABLE_WEBHOOKS") != "false", "Enable webhooks")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1, "Max concurrent reconciles")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

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

	if envMaxConcurrentReconciles, exists := os.LookupEnv("MAX_CONCURRENT_RECONCILES"); exists {
		if val, err := strconv.Atoi(envMaxConcurrentReconciles); err == nil {
			maxConcurrentReconciles = val
		}
	}

	if namespaces := strings.TrimSpace(os.Getenv("WATCH_NAMESPACE")); namespaces != "" {
		options.Cache.DefaultNamespaces = map[string]cache.Config{}
		for _, ns := range strings.Split(namespaces, ",") {
			if ns = strings.TrimSpace(ns); ns != "" {
				options.Cache.DefaultNamespaces[ns] = cache.Config{}
			}
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	k8sclient, err := k8sutils.GenerateK8sClient(k8sutils.GenerateK8sConfig())
	if err != nil {
		setupLog.Error(err, "unable to create k8s client")
		os.Exit(1)
	}

	dk8sClient, err := k8sutils.GenerateK8sDynamicClient(k8sutils.GenerateK8sConfig())
	if err != nil {
		setupLog.Error(err, "unable to create k8s dynamic client")
		os.Exit(1)
	}

	if err = (&rediscontroller.Reconciler{
		Client:    mgr.GetClient(),
		K8sClient: k8sclient,
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Redis")
		os.Exit(1)
	}
	if err = (&redisclustercontroller.Reconciler{
		Client:      mgr.GetClient(),
		K8sClient:   k8sclient,
		Dk8sClient:  dk8sClient,
		Recorder:    mgr.GetEventRecorderFor("rediscluster-controller"),
		StatefulSet: k8sutils.NewStatefulSetService(k8sclient),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RedisCluster")
		os.Exit(1)
	}
	if err = (&redisreplicationcontroller.Reconciler{
		Client:      mgr.GetClient(),
		K8sClient:   k8sclient,
		Dk8sClient:  dk8sClient,
		Pod:         k8sutils.NewPodService(k8sclient),
		StatefulSet: k8sutils.NewStatefulSetService(k8sclient),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RedisReplication")
		os.Exit(1)
	}
	if err = (&redissentinelcontroller.RedisSentinelReconciler{
		Client:             mgr.GetClient(),
		K8sClient:          k8sclient,
		Dk8sClient:         dk8sClient,
		ReplicationWatcher: intctrlutil.NewResourceWatcher(),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RedisSentinel")
		os.Exit(1)
	}

	if enableWebhooks {
		if err = (&redisv1beta2.Redis{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Redis")
			os.Exit(1)
		}
		if err = (&redisv1beta2.RedisCluster{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "RedisCluster")
			os.Exit(1)
		}
		if err = (&redisv1beta2.RedisReplication{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "RedisReplication")
			os.Exit(1)
		}
		if err = (&redisv1beta2.RedisSentinel{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "RedisSentinel")
			os.Exit(1)
		}

		wblog := ctrl.Log.WithName("webhook").WithName("PodAffiniytMutate")
		mgr.GetWebhookServer().Register("/mutate-core-v1-pod", &webhook.Admission{
			Handler: coreWebhook.NewPodAffiniytMutate(mgr.GetClient(), admission.NewDecoder(scheme), wblog),
		})
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

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
