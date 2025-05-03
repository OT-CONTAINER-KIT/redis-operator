package rediscluster

import (
	"context"
	"os"
	"path/filepath"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis Cluster Controller", func() {
	Context("When deploying Redis Cluster from testdata", func() {
		var (
			redisCluster *redisv1beta2.RedisCluster
			testFile     string
		)

		BeforeEach(func() {
			testFile = filepath.Join("testdata", "full.yaml")
			redisCluster = &redisv1beta2.RedisCluster{}

			yamlFile, err := os.ReadFile(testFile)
			Expect(err).NotTo(HaveOccurred())

			err = yaml.Unmarshal(yamlFile, redisCluster)
			Expect(err).NotTo(HaveOccurred())

			redisCluster.Namespace = ns

			Expect(k8sClient.Create(context.Background(), redisCluster)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), redisCluster)).Should(Succeed())
		})

		It("should create all required resources", func() {
			By("verifying the Redis Cluster StatefulSet is created")
			leaderSts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisCluster.Name + "-leader",
					Namespace: ns,
				}, leaderSts)
			}, timeout, interval).Should(Succeed())

			By("verifying the Redis Cluster Leader Service is created")
			leaderSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisCluster.Name + "-leader",
					Namespace: ns,
				}, leaderSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying the Redis Cluster headless Service is created")
			headlessSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisCluster.Name + "-leader-headless",
					Namespace: ns,
				}, headlessSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying owner references")
			for _, obj := range []client.Object{leaderSts, leaderSvc, headlessSvc} {
				ownerRefs := obj.GetOwnerReferences()
				Expect(ownerRefs).To(HaveLen(1))
				Expect(ownerRefs[0].Name).To(Equal(redisCluster.Name))
			}

			By("verifying StatefulSet specifications")
			Expect(leaderSts.Spec.Template.Spec.SecurityContext).To(Equal(redisCluster.Spec.PodSecurityContext))
			Expect(leaderSts.Spec.Template.Spec.Containers[0].Image).To(Equal(redisCluster.Spec.KubernetesConfig.Image))
			Expect(leaderSts.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(redisCluster.Spec.KubernetesConfig.ImagePullPolicy))
			Expect(leaderSts.Spec.Template.Spec.Containers[0].Resources).To(Equal(*redisCluster.Spec.GetRedisLeaderResources()))

			By("verifying Service specifications")
			expectedLabels := map[string]string{
				"app":              redisCluster.Name + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}
			Expect(leaderSvc.Labels).To(Equal(expectedLabels))

			expectedHeadlessLabels := map[string]string{
				"app":              redisCluster.Name + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}
			Expect(headlessSvc.Labels).To(Equal(expectedHeadlessLabels))

			By("verifying cluster configuration")
			Expect(leaderSts.Spec.Replicas).NotTo(BeNil())
			expectedReplicas := int32(3)
			Expect(*leaderSts.Spec.Replicas).To(Equal(expectedReplicas))

			By("verifying Redis Cluster configuration")
			Expect(leaderSts.Spec.ServiceName).To(Equal(redisCluster.Name + "-leader-headless"))

			By("verifying resource requirements") // when set resources in redisLeader, it should be used instead of kubernetesConfig.resources
			container := leaderSts.Spec.Template.Spec.Containers[0]
			Expect(container.Resources.Limits).To(Equal(redisCluster.Spec.RedisLeader.Resources.Limits))
			Expect(container.Resources.Requests).To(Equal(redisCluster.Spec.RedisLeader.Resources.Requests))

			By("verifying Redis Exporter configuration")
			var exporterContainer *corev1.Container
			for _, c := range leaderSts.Spec.Template.Spec.Containers {
				if c.Name == "redis-exporter" {
					exporterContainer = &c //nolint:exportloopref
					break
				}
			}
			Expect(exporterContainer).NotTo(BeNil(), "Redis Exporter container should exist")
			Expect(exporterContainer.Image).To(Equal(redisCluster.Spec.RedisExporter.Image))
			Expect(exporterContainer.ImagePullPolicy).To(Equal(redisCluster.Spec.RedisExporter.ImagePullPolicy))
			Expect(exporterContainer.Resources).To(Equal(*redisCluster.Spec.RedisExporter.Resources))
		})
	})
})
