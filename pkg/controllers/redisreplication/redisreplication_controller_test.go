package redisreplication

import (
	"context"
	"fmt"
	"math/rand"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	factories "github.com/OT-CONTAINER-KIT/redis-operator/pkg/testutil/factories/redisreplication"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis test", func() {
	Describe("When creating a redis without custom fields", func() {
		var (
			cr     *redisv1beta2.RedisReplication
			crName string
		)
		BeforeEach(func() {
			crName = fmt.Sprintf("redis-%d", rand.Int31()) //nolint:gosec
			cr = factories.New(crName)
			Expect(k8sClient.Create(context.TODO(), cr)).Should(Succeed())
		})

		DescribeTable("the reconciler",
			func(nameFmt string, obj client.Object) {
				key := types.NamespacedName{
					Name:      fmt.Sprintf(nameFmt, crName),
					Namespace: ns,
				}

				By("creating the resource when the cluster is created")
				Eventually(func() error { return k8sClient.Get(context.TODO(), key, obj) }, timeout).Should(Succeed())
			},
			Entry("reconciles the leader statefulset", "%s", &appsv1.StatefulSet{}),
			Entry("reconciles the leader headless service", "%s-headless", &corev1.Service{}),
			Entry("reconciles the leader additional service", "%s-additional", &corev1.Service{}),
		)

		Context("then deleting the redis CR", func() {
			It("should delete the statefulset", func() {
				cr := &redisv1beta2.RedisReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      crName,
						Namespace: ns,
					},
				}
				Expect(k8sClient.Delete(context.TODO(), cr)).To(BeNil())

				Eventually(func() bool {
					sts := &appsv1.StatefulSet{}
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Name:      crName,
						Namespace: ns,
					}, sts)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})
		})
	})

	Describe("When creating a redis, ignore annotations", func() {
		var (
			cr     *redisv1beta2.RedisReplication
			crName string
		)
		BeforeEach(func() {
			crName = fmt.Sprintf("redis-%d", rand.Int31()) //nolint:gosec
			cr = factories.New(
				crName,
				factories.WithAnnotations(map[string]string{
					"key1": "value1",
					"key2": "value2",
				}),
				factories.WithIgnoredKeys([]string{"key1"}),
			)
			Expect(k8sClient.Create(context.TODO(), cr)).Should(Succeed())
		})
		Describe("the reconciler", func() {
			It("should ignore key in statefulset", func() {
				stsLeader := &appsv1.StatefulSet{}
				stsLeaderNN := types.NamespacedName{
					Name:      crName,
					Namespace: ns,
				}
				Eventually(func() error { return k8sClient.Get(context.TODO(), stsLeaderNN, stsLeader) }, timeout, interval).Should(BeNil())
				Expect(stsLeader.Annotations).To(HaveKey("key2"))
				Expect(stsLeader.Annotations).NotTo(HaveKey("key1"))
			})
		})
	})
})
