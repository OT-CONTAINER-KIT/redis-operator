package redis

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

var _ = Describe("Redis Controller", func() {
	Context("When deploying Redis from testdata", func() {
		var (
			redis    *redisv1beta2.Redis
			testFile string
		)

		BeforeEach(func() {
			testFile = filepath.Join("testdata", "full.yaml")
			redis = &redisv1beta2.Redis{}

			yamlFile, err := os.ReadFile(testFile)
			Expect(err).NotTo(HaveOccurred())

			err = yaml.Unmarshal(yamlFile, redis)
			Expect(err).NotTo(HaveOccurred())

			redis.Namespace = ns

			Expect(k8sClient.Create(context.Background(), redis)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), redis)).Should(Succeed())
		})

		It("should create all required resources", func() {
			By("verifying the StatefulSet is created")
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name,
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(Succeed())

			By("verifying the headless Service is created")
			headlessSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name + "-headless",
					Namespace: ns,
				}, headlessSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying the additional Service is created")
			additionalSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name + "-additional",
					Namespace: ns,
				}, additionalSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying owner references")
			for _, obj := range []client.Object{sts, headlessSvc, additionalSvc} {
				ownerRefs := obj.GetOwnerReferences()
				Expect(ownerRefs).To(HaveLen(1))
				Expect(ownerRefs[0].Name).To(Equal(redis.Name))
			}

			By("verifying StatefulSet specifications")
			Expect(sts.Spec.Template.Spec.SecurityContext).To(Equal(redis.Spec.PodSecurityContext))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal(redis.Spec.KubernetesConfig.Image))

			By("verifying PVC specifications")
			Expect(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage()).To(Equal(
				redis.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests.Storage()))

			By("verifying Redis Exporter configuration")
			var exporterContainer *corev1.Container
			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == "redis-exporter" {
					exporterContainer = &container //nolint:exportloopref
					break
				}
			}
			Expect(exporterContainer).NotTo(BeNil(), "Redis Exporter container should exist")
			Expect(exporterContainer.Image).To(Equal(redis.Spec.RedisExporter.Image))
			Expect(exporterContainer.ImagePullPolicy).To(Equal(redis.Spec.RedisExporter.ImagePullPolicy))
			Expect(exporterContainer.Resources).To(Equal(*redis.Spec.RedisExporter.Resources))
		})
	})
})
