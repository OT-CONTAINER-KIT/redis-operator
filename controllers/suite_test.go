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

package controllers

import (
	"path/filepath"
	"testing"
	"time"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/k8sutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
)

const (
	ns = "default"

	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	// err = redisv1beta1.AddToScheme(scheme.Scheme)
	// Expect(err).ToNot(HaveOccurred())

	err = redisv1beta2.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	k8sClient, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	dk8sClient, err := dynamic.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	err = (&RedisReconciler{
		Client:     k8sManager.GetClient(),
		K8sClient:  k8sClient,
		Dk8sClient: dk8sClient,
		Scheme:     k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	rcLog := ctrl.Log.WithName("controllers").WithName("RedisCluster")
	err = (&RedisClusterReconciler{
		Client:      k8sManager.GetClient(),
		K8sClient:   k8sClient,
		Dk8sClient:  dk8sClient,
		Scheme:      k8sManager.GetScheme(),
		StatefulSet: k8sutils.NewStatefulSetService(k8sClient, rcLog),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	rrLog := ctrl.Log.WithName("controllers").WithName("RedisReplication")
	err = (&RedisReplicationReconciler{
		Client:      k8sManager.GetClient(),
		K8sClient:   k8sClient,
		Dk8sClient:  dk8sClient,
		Scheme:      k8sManager.GetScheme(),
		Pod:         k8sutils.NewPodService(k8sClient, rrLog),
		StatefulSet: k8sutils.NewStatefulSetService(k8sClient, rrLog),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&RedisSentinelReconciler{
		Client:     k8sManager.GetClient(),
		K8sClient:  k8sClient,
		Dk8sClient: dk8sClient,
		Scheme:     k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		gexec.KillAndWait(4 * time.Second)

		// Teardown the test environment once controller is fnished.
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	}()
})
