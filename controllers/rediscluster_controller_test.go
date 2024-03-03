package controllers

import (
	"context"
	"fmt"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Redis cluster test", func() {
	var (
		redisClusterCR     redisv1beta2.RedisCluster
		redisClusterCRName string
		size               int32
		// Used to create unique name for each test
		testCount int
	)

	JustBeforeEach(func() {
		size = 3
		redisClusterCR = redisv1beta2.RedisCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "redis.redis.opstreelabs.in/v1beta2",
				Kind:       "RedisCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      redisClusterCRName,
				Namespace: ns,
			},
			Spec: redisv1beta2.RedisClusterSpec{
				Size:    &size,
				Storage: &redisv1beta2.ClusterStorage{},
			},
		}
		Expect(k8sClient.Create(context.TODO(), &redisClusterCR)).Should(Succeed())
		testCount++
	})

	BeforeEach(func() {
		redisClusterCRName = fmt.Sprintf("redis-cluster-%d", testCount)
	})

	Context("When creating a redis cluster CR", func() {
		It("should create a statefulset, service", func() {
			sts := &appsv1.StatefulSet{}
			svc := &corev1.Service{}

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisClusterCRName + "-leader",
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(BeNil())

			Expect(sts.Labels).To(Equal(map[string]string{
				"app":              redisClusterCRName + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}))

			Expect(*sts.Spec.Replicas).To(BeEquivalentTo(3))
			Expect(sts.Spec.ServiceName).To(Equal(redisClusterCRName + "-leader-headless"))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisClusterCRName + "-leader",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisClusterCRName + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisClusterCRName + "-leader-headless",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisClusterCRName + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}))
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisClusterCRName + "-leader-additional",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisClusterCRName + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}))
		})

		Context("then deleting the redis cluster CR", func() {
			It("should delete the statefulset", func() {
				redisClusterCR := &redisv1beta2.RedisCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      redisClusterCRName,
						Namespace: ns,
					},
				}
				Expect(k8sClient.Delete(context.TODO(), redisClusterCR)).To(BeNil())

				Eventually(func() bool {
					sts := &appsv1.StatefulSet{}
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Name:      redisClusterCRName,
						Namespace: ns,
					}, sts)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
})
