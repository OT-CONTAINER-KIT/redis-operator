package redissentinel

import (
	"context"
	"os"
	"path/filepath"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis Sentinel Controller", func() {
	Context("When deploying Redis Sentinel from testdata", func() {
		var (
			redisSentinel *redisv1beta2.RedisSentinel
			testFile      string
		)

		BeforeEach(func() {
			testFile = filepath.Join("testdata", "full.yaml")
			redisSentinel = &redisv1beta2.RedisSentinel{}

			yamlFile, err := os.ReadFile(testFile)
			Expect(err).NotTo(HaveOccurred())

			err = yaml.Unmarshal(yamlFile, redisSentinel)
			Expect(err).NotTo(HaveOccurred())

			redisSentinel.Namespace = ns

			Expect(k8sClient.Create(context.Background(), redisSentinel)).Should(Succeed())
		})

		AfterEach(func() {
			// Clean up resources
			Expect(k8sClient.Delete(context.Background(), redisSentinel)).Should(Succeed())
		})

		It("should create all required resources", func() {
			By("verifying the Sentinel StatefulSet is created")
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisSentinel.Name + "-sentinel",
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(Succeed())

			By("verifying the Sentinel Service is created")
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisSentinel.Name + "-sentinel",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(Succeed())

			By("verifying the Sentinel headless Service is created")
			headlessSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisSentinel.Name + "-sentinel-headless",
					Namespace: ns,
				}, headlessSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying the Sentinel additional Service is created")
			additionalSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisSentinel.Name + "-sentinel-additional",
					Namespace: ns,
				}, additionalSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying owner references")
			for _, obj := range []client.Object{sts, svc, headlessSvc, additionalSvc} {
				ownerRefs := obj.GetOwnerReferences()
				Expect(ownerRefs).To(HaveLen(1))
				Expect(ownerRefs[0].Name).To(Equal(redisSentinel.Name))
			}

			By("verifying StatefulSet specifications")
			Expect(sts.Spec.Template.Spec.SecurityContext).To(Equal(redisSentinel.Spec.PodSecurityContext))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal(redisSentinel.Spec.KubernetesConfig.Image))
			Expect(sts.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(redisSentinel.Spec.KubernetesConfig.ImagePullPolicy))

			By("verifying Service specifications")
			expectedLabels := map[string]string{
				"app":              redisSentinel.Name + "-sentinel",
				"redis_setup_type": "sentinel",
				"role":             "sentinel",
			}
			Expect(svc.Labels).To(Equal(expectedLabels))
			Expect(headlessSvc.Labels).To(Equal(expectedLabels))
			Expect(additionalSvc.Labels).To(Equal(expectedLabels))

			By("verifying cluster configuration")
			Expect(sts.Spec.Replicas).NotTo(BeNil())
			expectedReplicas := int32(3)
			Expect(*sts.Spec.Replicas).To(Equal(expectedReplicas))

			By("verifying Redis Sentinel configuration")
			Expect(sts.Spec.ServiceName).To(Equal(redisSentinel.Name + "-sentinel-headless"))

			By("verifying resource requirements")
			container := sts.Spec.Template.Spec.Containers[0]
			Expect(container.Resources.Limits).To(Equal(redisSentinel.Spec.KubernetesConfig.Resources.Limits))
			Expect(container.Resources.Requests).To(Equal(redisSentinel.Spec.KubernetesConfig.Resources.Requests))

			By("verifying PodDisruptionBudget configuration")
			pdb := &policyv1.PodDisruptionBudget{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisSentinel.Name + "-sentinel",
					Namespace: ns,
				}, pdb)
			}, timeout, interval).Should(Succeed())

			minAvailable := intstr.FromInt(1)
			Expect(pdb.Spec.MinAvailable).To(Equal(&minAvailable))
		})
	})
})
