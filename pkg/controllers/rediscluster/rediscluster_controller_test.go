package rediscluster

import (
	"context"
	"fmt"
	"math/rand"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	factories "github.com/OT-CONTAINER-KIT/redis-operator/pkg/testutil/factories/rediscluster"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis cluster test", func() {
	Describe("When creating a redis cluster without custom fields", func() {
		var (
			redisClusterCR     *redisv1beta2.RedisCluster
			redisClusterCRName string
		)
		BeforeEach(func() {
			redisClusterCRName = fmt.Sprintf("redis-cluster-%d", rand.Int31()) //nolint:gosec
			redisClusterCR = factories.New(redisClusterCRName)
			Expect(k8sClient.Create(context.TODO(), redisClusterCR)).Should(Succeed())
		})

		DescribeTable("the reconciler",
			func(nameFmt string, obj client.Object) {
				key := types.NamespacedName{
					Name:      fmt.Sprintf(nameFmt, redisClusterCRName),
					Namespace: ns,
				}

				By("creating the resource when the cluster is created")
				Eventually(func() error { return k8sClient.Get(context.TODO(), key, obj) }, timeout).Should(Succeed())

				By("setting the owner reference")
				ownerRefs := obj.GetOwnerReferences()
				Expect(ownerRefs).To(HaveLen(1))
				Expect(ownerRefs[0].Name).To(Equal(redisClusterCRName))
			},
			Entry("reconciles the leader statefulset", "%s-leader", &appsv1.StatefulSet{}),
			Entry("reconciles the leader service", "%s-leader", &corev1.Service{}),
			Entry("reconciles the leader headless service", "%s-leader-headless", &corev1.Service{}),
			Entry("reconciles the leader additional service", "%s-leader-additional", &corev1.Service{}),
		)
	})

	Describe("When creating a redis cluster with DisablePersistence", func() {
		var (
			redisClusterCR     *redisv1beta2.RedisCluster
			redisClusterCRName string
		)
		BeforeEach(func() {
			redisClusterCRName = fmt.Sprintf("redis-cluster-%d", rand.Int31()) //nolint:gosec
			redisClusterCR = factories.New(redisClusterCRName, factories.DisablePersistence())
			Expect(k8sClient.Create(context.TODO(), redisClusterCR)).Should(Succeed())
		})

		It("should create leader statefulset without persistence volume", func() {
			stsLeader := &appsv1.StatefulSet{}
			stsLeaderNN := types.NamespacedName{
				Name:      redisClusterCRName + "-leader",
				Namespace: ns,
			}
			Eventually(func() error { return k8sClient.Get(context.TODO(), stsLeaderNN, stsLeader) }, timeout, interval).Should(BeNil())
			Expect(stsLeader.Spec.VolumeClaimTemplates).To(HaveLen(0))
		})
	})

	Describe("When creating a redis cluster, ignore annotations", func() {
		var (
			redisClusterCR     *redisv1beta2.RedisCluster
			redisClusterCRName string
		)
		BeforeEach(func() {
			redisClusterCRName = fmt.Sprintf("redis-cluster-%d", rand.Int31()) //nolint:gosec
			redisClusterCR = factories.New(
				redisClusterCRName,
				factories.WithAnnotations(map[string]string{
					"key1": "value1",
					"key2": "value2",
				}),
				factories.WithIgnoredKeys([]string{"key1"}),
			)
			Expect(k8sClient.Create(context.TODO(), redisClusterCR)).Should(Succeed())
		})
		Describe("the reconciler", func() {
			It("should ignore key in leader statefulset", func() {
				stsLeader := &appsv1.StatefulSet{}
				stsLeaderNN := types.NamespacedName{
					Name:      redisClusterCRName + "-leader",
					Namespace: ns,
				}
				Eventually(func() error { return k8sClient.Get(context.TODO(), stsLeaderNN, stsLeader) }, timeout, interval).Should(BeNil())
				Expect(stsLeader.Annotations).To(HaveKey("key2"))
				Expect(stsLeader.Annotations).NotTo(HaveKey("key1"))
			})
		})
	})
})
