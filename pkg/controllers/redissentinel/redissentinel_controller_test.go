package redissentinel

import (
	"context"
	"fmt"
	"math/rand"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Redis sentinel test", func() {
	var (
		redisSentinelCR     redisv1beta2.RedisSentinel
		redisSentinelCRName string
	)

	BeforeEach(func() {
		redisSentinelCRName = fmt.Sprintf("redis-sentinel-%d", rand.Int31()) //nolint:gosec
		size := int32(3)
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
		err := k8sClient.Create(context.TODO(), &redisSentinelCR)
		Expect(err).Should(Succeed())
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
	})
})
