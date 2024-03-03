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

var _ = Describe("Redis standalone test", func() {
	var (
		redisCR     redisv1beta2.Redis
		redisCRName string
		// Used to create unique name for each test
		testCount int
	)

	JustBeforeEach(func() {
		redisCR = redisv1beta2.Redis{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "redisv1beta2/apiVersion",
				Kind:       "Redis",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      redisCRName,
				Namespace: ns,
			},
		}
		Expect(k8sClient.Create(context.TODO(), &redisCR)).Should(Succeed())
		testCount++
	})

	BeforeEach(func() {
		redisCRName = fmt.Sprintf("redis-%d", testCount)
	})

	Context("When creating a redis standalone CR", func() {
		It("should create a statefulset, service", func() {
			sts := &appsv1.StatefulSet{}
			svc := &corev1.Service{}

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisCRName,
					Namespace: ns,
				}, sts)
			}, timeout, interval).Should(BeNil())

			Expect(sts.Labels).To(Equal(map[string]string{
				"app":              redisCRName,
				"redis_setup_type": "standalone",
				"role":             "standalone",
			}))

			Expect(*sts.Spec.Replicas).To(BeEquivalentTo(1))
			Expect(sts.Spec.ServiceName).To(Equal(redisCRName + "-headless"))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisCR.Name,
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisCRName,
				"redis_setup_type": "standalone",
				"role":             "standalone",
			}))

			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisCR.Name + "-headless",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisCRName,
				"redis_setup_type": "standalone",
				"role":             "standalone",
			}))
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), types.NamespacedName{
					Name:      redisCR.Name + "-additional",
					Namespace: ns,
				}, svc)
			}, timeout, interval).Should(BeNil())

			Expect(svc.Labels).To(Equal(map[string]string{
				"app":              redisCRName,
				"redis_setup_type": "standalone",
				"role":             "standalone",
			}))
		})

		Context("then deleting the redis standalone CR", func() {
			It("should delete the statefulset", func() {
				redisCR := &redisv1beta2.Redis{
					ObjectMeta: metav1.ObjectMeta{
						Name:      redisCRName,
						Namespace: ns,
					},
				}
				Expect(k8sClient.Delete(context.TODO(), redisCR)).To(BeNil())

				Eventually(func() bool {
					sts := &appsv1.StatefulSet{}
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Name:      redisCRName,
						Namespace: ns,
					}, sts)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
})
