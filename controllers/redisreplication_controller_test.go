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

var _ = Describe("Redis replication test", func() {
	var (
		redisReplicationCR     redisv1beta2.RedisReplication
		redisReplicationCRName string
		size                   int32
		// Used to create unique name for each test
		testCount int
	)

	JustBeforeEach(func() {
		size = 3
		redisReplicationCR = redisv1beta2.RedisReplication{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "redis.redis.opstreelabs.in/v1beta2",
				Kind:       "RedisReplication",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      redisReplicationCRName,
				Namespace: ns,
			},
			Spec: redisv1beta2.RedisReplicationSpec{
				Size:    &size,
				Storage: &redisv1beta2.Storage{},
			},
		}
		Expect(k8sClient.Create(context.TODO(), &redisReplicationCR)).Should(Succeed())
		testCount++
	})

	BeforeEach(func() {
		redisReplicationCRName = fmt.Sprintf("redis-replication-%d", testCount)
	})

	Context("When creating a redis replication CR", func() {
		It("should create a statefulset, service", func() {
			svc := &corev1.Service{}
			sts := &appsv1.StatefulSet{}

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisReplicationCRName,
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(BeNil())

			Expect(*sts.Spec.Replicas).To(BeEquivalentTo(3))
			Expect(sts.Spec.ServiceName).To(Equal(redisReplicationCRName + "-headless"))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisReplicationCRName,
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisReplicationCRName,
				"redis_setup_type": "replication",
				"role":             "replication",
			}))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisReplicationCRName + "-headless",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisReplicationCRName,
				"redis_setup_type": "replication",
				"role":             "replication",
			}))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisReplicationCRName + "-additional",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisReplicationCRName,
				"redis_setup_type": "replication",
				"role":             "replication",
			}))
		})

		Context("then deleting the redis replication CR", func() {
			It("should delete the statefulset", func() {
				redisReplicationCR := &redisv1beta2.RedisReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      redisReplicationCRName,
						Namespace: ns,
					},
				}
				Expect(k8sClient.Delete(context.TODO(), redisReplicationCR)).To(BeNil())

				Eventually(func() bool {
					sts := &appsv1.StatefulSet{}
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Name:      redisReplicationCRName,
						Namespace: ns,
					}, sts)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
})
