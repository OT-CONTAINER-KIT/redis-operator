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

var _ = Describe("Redis sentinel test", func() {
	var (
		redisSentinelCR     redisv1beta2.RedisSentinel
		redisSentinelCRName string
		size                int32
		// Used to create unique name for each test
		testCount int
	)

	JustBeforeEach(func() {
		size = 3
		redisSentinelCR = redisv1beta2.RedisSentinel{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "redis.redis.opstreelabs.in/v1beta2",
				Kind:       "RedisReplication",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      redisSentinelCRName,
				Namespace: ns,
			},
			Spec: redisv1beta2.RedisSentinelSpec{
				Size: &size,
			},
		}
		Expect(k8sClient.Create(context.TODO(), &redisSentinelCR)).Should(Succeed())
		testCount++
	})

	BeforeEach(func() {
		redisSentinelCRName = fmt.Sprintf("redis-sentinel-%d", testCount)
	})

	Context("When creating a redis sentinel CR", func() {
		It("should create a statefulset, service", func() {
			sts := &appsv1.StatefulSet{}
			svc := &corev1.Service{}

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisSentinelCRName + "-sentinel",
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(BeNil())

			Expect(*sts.Spec.Replicas).To(BeEquivalentTo(3))
			Expect(sts.Spec.ServiceName).To(Equal(redisSentinelCRName + "-sentinel-headless"))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisSentinelCRName + "-sentinel",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisSentinelCRName + "-sentinel",
				"redis_setup_type": "sentinel",
				"role":             "sentinel",
			}))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisSentinelCRName + "-sentinel-headless",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisSentinelCRName + "-sentinel",
				"redis_setup_type": "sentinel",
				"role":             "sentinel",
			}))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisSentinelCRName + "-sentinel-additional",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisSentinelCRName + "-sentinel",
				"redis_setup_type": "sentinel",
				"role":             "sentinel",
			}))
		})

		Context("then deleting the redis sentinel CR", func() {
			It("should delete the statefulset", func() {
				redisSentinelCR := &redisv1beta2.RedisSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      redisSentinelCRName,
						Namespace: ns,
					},
				}
				Expect(k8sClient.Delete(context.TODO(), redisSentinelCR)).To(BeNil())

				Eventually(func() bool {
					sts := &appsv1.StatefulSet{}
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Name:      redisSentinelCRName + "-sentinel",
						Namespace: ns,
					}, sts)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
})
