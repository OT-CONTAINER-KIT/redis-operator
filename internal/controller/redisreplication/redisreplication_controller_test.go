package redisreplication

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

var _ = Describe("Redis Replication Controller", func() {
	Context("When deploying Redis Replication from testdata", func() {
		var (
			redisReplication *redisv1beta2.RedisReplication
			testFile         string
		)

		BeforeEach(func() {
			testFile = filepath.Join("testdata", "full.yaml")
			redisReplication = &redisv1beta2.RedisReplication{}

			yamlFile, err := os.ReadFile(testFile)
			Expect(err).NotTo(HaveOccurred())

			err = yaml.Unmarshal(yamlFile, redisReplication)
			Expect(err).NotTo(HaveOccurred())

			redisReplication.Namespace = ns

			Expect(k8sClient.Create(context.Background(), redisReplication)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), redisReplication)).Should(Succeed())
		})

		It("should create all required resources", func() {
			By("verifying the StatefulSet is created")
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisReplication.Name,
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(Succeed())

			By("verifying the headless Service is created")
			headlessSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisReplication.Name + "-headless",
					Namespace: ns,
				}, headlessSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying the additional Service is created")
			additionalSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisReplication.Name + "-additional",
					Namespace: ns,
				}, additionalSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying owner references")
			for _, obj := range []client.Object{sts, headlessSvc, additionalSvc} {
				ownerRefs := obj.GetOwnerReferences()
				Expect(ownerRefs).To(HaveLen(1))
				Expect(ownerRefs[0].Name).To(Equal(redisReplication.Name))
			}

			By("verifying StatefulSet specifications")
			Expect(sts.Spec.Template.Spec.SecurityContext).To(Equal(redisReplication.Spec.PodSecurityContext))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal(redisReplication.Spec.KubernetesConfig.Image))

			By("verifying PVC specifications")
			Expect(sts.Spec.VolumeClaimTemplates).To(HaveLen(1))
			Expect(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage()).To(Equal(
				redisReplication.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests.Storage()))

			By("verifying replication configuration")
			Expect(sts.Spec.Replicas).NotTo(BeNil())
			expectedReplicas := int32(3)
			Expect(*sts.Spec.Replicas).To(Equal(expectedReplicas))

			By("verifying Redis Exporter configuration")
			By("verifying Redis Exporter container")
			var exporterContainer *corev1.Container
			for i := range sts.Spec.Template.Spec.Containers {
				if sts.Spec.Template.Spec.Containers[i].Name == "redis-exporter" {
					exporterContainer = &sts.Spec.Template.Spec.Containers[i]
					break
				}
			}
			Expect(exporterContainer).NotTo(BeNil())
			Expect(exporterContainer.Image).To(Equal(redisReplication.Spec.RedisExporter.Image))
			Expect(exporterContainer.ImagePullPolicy).To(Equal(redisReplication.Spec.RedisExporter.ImagePullPolicy))
		})
	})
})
