package redis

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	controllercommon "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis Controller", func() {
	Context("When deploying Redis from testdata", func() {
		var (
			redis    *rvb2.Redis
			testFile string
		)

		BeforeEach(func() {
			testFile = filepath.Join("testdata", "full.yaml")
			redis = &rvb2.Redis{}

			yamlFile, err := os.ReadFile(testFile)
			Expect(err).NotTo(HaveOccurred())

			err = yaml.Unmarshal(yamlFile, redis)
			Expect(err).NotTo(HaveOccurred())

			redis.Namespace = ns

			Expect(k8sClient.Create(context.Background(), redis)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), redis)).Should(Succeed())
			// Wait until the finalizer is removed and the object is gone so the
			// next spec can recreate a Redis with the same name.
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name,
					Namespace: ns,
				}, &rvb2.Redis{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
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
					exporterContainer = &container //nolint:copyloopvar
					break
				}
			}
			Expect(exporterContainer).NotTo(BeNil(), "Redis Exporter container should exist")
			Expect(exporterContainer.Image).To(Equal(redis.Spec.RedisExporter.Image))
			Expect(exporterContainer.ImagePullPolicy).To(Equal(redis.Spec.RedisExporter.ImagePullPolicy))
			Expect(exporterContainer.Resources).To(Equal(*redis.Spec.RedisExporter.Resources))
		})

		It("should transition status from Initializing to Ready when the StatefulSet becomes ready", func() {
			By("verifying the status is Initializing")
			Eventually(func() (rvb2.RedisState, error) {
				current := &rvb2.Redis{}
				if err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name,
					Namespace: ns,
				}, current); err != nil {
					return "", err
				}
				return current.Status.State, nil
			}, timeout, interval).Should(Equal(rvb2.RedisInitializing))

			By("marking the StatefulSet as ready")
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name,
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(Succeed())

			replicas := int32(1)
			if sts.Spec.Replicas != nil {
				replicas = *sts.Spec.Replicas
			}
			sts.Status.Replicas = replicas
			sts.Status.ReadyReplicas = replicas
			sts.Status.CurrentReplicas = replicas
			sts.Status.UpdatedReplicas = replicas
			sts.Status.ObservedGeneration = sts.Generation
			Expect(k8sClient.Status().Update(context.Background(), sts)).Should(Succeed())

			By("verifying the status becomes Ready")
			Eventually(func() (rvb2.RedisState, error) {
				current := &rvb2.Redis{}
				if err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name,
					Namespace: ns,
				}, current); err != nil {
					return "", err
				}
				return current.Status.State, nil
			}, timeout, interval).Should(Equal(rvb2.RedisReady))

			current := &rvb2.Redis{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      redis.Name,
				Namespace: ns,
			}, current)).Should(Succeed())
			Expect(current.Status.Reason).To(Equal(rvb2.ReadyReason))
		})
	})

	Context("When Redis resource creation fails", func() {
		var redis *rvb2.Redis

		BeforeEach(func() {
			// The name is a valid CR and StatefulSet name, but the derived headless
			// Service name ("<name>-headless") exceeds the 63-character limit for
			// Service names, so the API server rejects the Service creation.
			name := "redis-status-failed-" + strings.Repeat("x", 40)
			redis = &rvb2.Redis{
				ObjectMeta: testutil.CreateTestObject(name, ns, nil),
				Spec: rvb2.RedisSpec{
					KubernetesConfig: common.KubernetesConfig{
						Image: testutil.DefaultRedisImage,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), redis)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), redis)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name,
					Namespace: ns,
				}, &rvb2.Redis{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should set the status to Failed when resource creation fails", func() {
			Eventually(func() (rvb2.RedisState, error) {
				current := &rvb2.Redis{}
				if err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redis.Name,
					Namespace: ns,
				}, current); err != nil {
					return "", err
				}
				return current.Status.State, nil
			}, timeout, interval).Should(Equal(rvb2.RedisFailed))

			current := &rvb2.Redis{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      redis.Name,
				Namespace: ns,
			}, current)).Should(Succeed())
			Expect(current.Status.Reason).To(Equal(rvb2.FailedReason))
		})
	})

	Context("When testing skip-reconcile annotation behavior", func() {
		It("should trigger reconcile when skip-reconcile annotation changes from true to false", func() {
			testutil.RunSkipReconcileTest(k8sClient, testutil.SkipReconcileTestConfig{
				Object: &rvb2.Redis{
					ObjectMeta: testutil.CreateTestObject("redis-skip-test", ns, nil),
					Spec: rvb2.RedisSpec{
						KubernetesConfig: common.KubernetesConfig{
							Image: testutil.DefaultRedisImage,
						},
					},
				},
				SkipAnnotationKey: controllercommon.RedisSkipReconcileAnnotation,
				StatefulSetName:   "redis-skip-test",
				Namespace:         ns,
				Timeout:           timeout,
				Interval:          interval,
			})
		})
	})
})
