package redisreplication

import (
	"context"
	"os"
	"path/filepath"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	controllercommon "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis Replication Controller", func() {
	Context("When deploying Redis Replication from testdata", func() {
		var (
			redisReplication *rrvb2.RedisReplication
			testFile         string
		)

		BeforeEach(func() {
			testFile = filepath.Join("testdata", "full.yaml")
			redisReplication = &rrvb2.RedisReplication{}

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

	Context("When deploying Redis Replication with the embedded Sentinel", func() {
		newSentinelReplication := func(name string, tls *common.TLSConfig) *rrvb2.RedisReplication {
			rr := &rrvb2.RedisReplication{
				ObjectMeta: testutil.CreateTestObject(name, ns, nil),
				Spec: rrvb2.RedisReplicationSpec{
					Size:             ptr.To(int32(3)),
					KubernetesConfig: common.KubernetesConfig{Image: testutil.DefaultRedisImage},
					TLS:              tls,
					Sentinel:         &rrvb2.Sentinel{Size: 3},
				},
			}
			rr.Spec.Sentinel.Image = "quay.io/opstree/redis-sentinel:latest"
			return rr
		}
		getSentinelStatefulSet := func(rr *rrvb2.RedisReplication) *appsv1.StatefulSet {
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: rr.SentinelStatefulSet(), Namespace: ns}, sts)
			}, timeout, interval).Should(Succeed())
			return sts
		}

		It("should mount the TLS secret and the TLS sentinel config when TLS is enabled", func() {
			rr := newSentinelReplication("redis-replication-sentinel-tls", &common.TLSConfig{
				Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"},
			})
			Expect(k8sClient.Create(context.Background(), rr)).Should(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(context.Background(), rr)).Should(Succeed()) })

			By("verifying the owned TLS sentinel ConfigMap")
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: rr.SentinelStatefulSet() + "-tls", Namespace: ns}, cm)
			}, timeout, interval).Should(Succeed())
			Expect(cm.Data).To(HaveKeyWithValue("redis-sentinel-additional.conf", "tls-replication yes\ntls-auth-clients yes\n"))
			Expect(cm.GetOwnerReferences()).To(HaveLen(1))
			Expect(cm.GetOwnerReferences()[0].Name).To(Equal(rr.Name))

			By("verifying the sentinel pod template")
			podSpec := getSentinelStatefulSet(rr).Spec.Template.Spec
			Expect(podSpec.Volumes).To(ContainElements(
				HaveField("Secret.SecretName", "redis-tls-cert"),
				HaveField("ConfigMap.Name", cm.Name),
			))
			Expect(podSpec.Containers[0].VolumeMounts).To(ContainElements(
				HaveField("MountPath", "/tls"),
				HaveField("MountPath", "/etc/redis/external.conf.d"),
			))
			Expect(podSpec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "TLS_MODE", Value: "true"}))
		})

		It("should not render TLS for the sentinel when TLS is disabled", func() {
			rr := newSentinelReplication("redis-replication-sentinel-plain", nil)
			Expect(k8sClient.Create(context.Background(), rr)).Should(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(context.Background(), rr)).Should(Succeed()) })

			// The ConfigMap would be reconciled before the StatefulSet.
			podSpec := getSentinelStatefulSet(rr).Spec.Template.Spec
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: rr.SentinelStatefulSet() + "-tls", Namespace: ns}, &corev1.ConfigMap{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "unexpected TLS sentinel ConfigMap: %v", err)
			Expect(podSpec.Volumes).To(BeEmpty())
			Expect(podSpec.Containers[0].VolumeMounts).To(BeEmpty())
		})
	})

	Context("When testing skip-reconcile annotation behavior", func() {
		It("should trigger reconcile when skip-reconcile annotation changes from true to false", func() {
			testutil.RunSkipReconcileTest(k8sClient, testutil.SkipReconcileTestConfig{
				Object: &rrvb2.RedisReplication{
					ObjectMeta: testutil.CreateTestObject("redis-replication-skip-test", ns, nil),
					Spec: rrvb2.RedisReplicationSpec{
						Size: ptr.To(int32(3)),
						KubernetesConfig: common.KubernetesConfig{
							Image: testutil.DefaultRedisImage,
						},
					},
				},
				SkipAnnotationKey: controllercommon.RedisReplicationSkipReconcileAnnotation,
				StatefulSetName:   "redis-replication-skip-test",
				Namespace:         ns,
				Timeout:           timeout,
				Interval:          interval,
			})
		})
	})
})
