package rediscluster

import (
	"context"
	"os"
	"path/filepath"
	"time"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	controllercommon "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis Cluster Controller", func() {
	Context("When deploying Redis Cluster from testdata", func() {
		var (
			redisCluster *rcvb2.RedisCluster
			testFile     string
		)

		BeforeEach(func() {
			testFile = filepath.Join("testdata", "full.yaml")
			redisCluster = &rcvb2.RedisCluster{}

			yamlFile, err := os.ReadFile(testFile)
			Expect(err).NotTo(HaveOccurred())

			err = yaml.Unmarshal(yamlFile, redisCluster)
			Expect(err).NotTo(HaveOccurred())

			redisCluster.Namespace = ns

			Expect(k8sClient.Create(context.Background(), redisCluster)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), redisCluster)).Should(Succeed())
		})

		It("should create all required resources", func() {
			By("verifying the Redis Cluster StatefulSet is created")
			leaderSts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisCluster.Name + "-leader",
					Namespace: ns,
				}, leaderSts)
			}, timeout, interval).Should(Succeed())

			By("verifying the Redis Cluster Leader Service is created")
			leaderSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisCluster.Name + "-leader",
					Namespace: ns,
				}, leaderSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying the Redis Cluster headless Service is created")
			headlessSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      redisCluster.Name + "-leader-headless",
					Namespace: ns,
				}, headlessSvc)
			}, timeout, interval).Should(Succeed())

			By("verifying owner references")
			for _, obj := range []client.Object{leaderSts, leaderSvc, headlessSvc} {
				ownerRefs := obj.GetOwnerReferences()
				Expect(ownerRefs).To(HaveLen(1))
				Expect(ownerRefs[0].Name).To(Equal(redisCluster.Name))
			}

			By("verifying StatefulSet specifications")
			Expect(leaderSts.Spec.Template.Spec.SecurityContext).To(Equal(redisCluster.Spec.PodSecurityContext))
			Expect(leaderSts.Spec.Template.Spec.Containers[0].Image).To(Equal(redisCluster.Spec.KubernetesConfig.Image))
			Expect(leaderSts.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(redisCluster.Spec.KubernetesConfig.ImagePullPolicy))
			Expect(leaderSts.Spec.Template.Spec.Containers[0].Resources).To(Equal(*redisCluster.Spec.GetRedisLeaderResources()))

			By("verifying Service specifications")
			expectedLabels := map[string]string{
				"app":              redisCluster.Name + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}
			Expect(leaderSvc.Labels).To(Equal(expectedLabels))

			expectedHeadlessLabels := map[string]string{
				"app":              redisCluster.Name + "-leader",
				"redis_setup_type": "cluster",
				"role":             "leader",
			}
			Expect(headlessSvc.Labels).To(Equal(expectedHeadlessLabels))

			By("verifying cluster configuration")
			Expect(leaderSts.Spec.Replicas).NotTo(BeNil())
			expectedReplicas := int32(3)
			Expect(*leaderSts.Spec.Replicas).To(Equal(expectedReplicas))

			By("verifying Redis Cluster configuration")
			Expect(leaderSts.Spec.ServiceName).To(Equal(redisCluster.Name + "-leader-headless"))

			By("verifying resource requirements") // when set resources in redisLeader, it should be used instead of kubernetesConfig.resources
			container := leaderSts.Spec.Template.Spec.Containers[0]
			Expect(container.Resources.Limits).To(Equal(redisCluster.Spec.RedisLeader.Resources.Limits))
			Expect(container.Resources.Requests).To(Equal(redisCluster.Spec.RedisLeader.Resources.Requests))

			By("verifying Redis Exporter configuration")
			var exporterContainer *corev1.Container
			for _, c := range leaderSts.Spec.Template.Spec.Containers {
				if c.Name == "redis-exporter" {
					exporterContainer = &c //nolint:copyloopvar
					break
				}
			}
			Expect(exporterContainer).NotTo(BeNil(), "Redis Exporter container should exist")
			Expect(exporterContainer.Image).To(Equal(redisCluster.Spec.RedisExporter.Image))
			Expect(exporterContainer.ImagePullPolicy).To(Equal(redisCluster.Spec.RedisExporter.ImagePullPolicy))
			Expect(exporterContainer.Resources).To(Equal(*redisCluster.Spec.RedisExporter.Resources))
		})
	})

	Context("When testing skip-reconcile annotation behavior", func() {
		It("should trigger reconcile when skip-reconcile annotation changes from true to false", func() {
			testutil.RunSkipReconcileTest(k8sClient, testutil.SkipReconcileTestConfig{
				Object: &rcvb2.RedisCluster{
					ObjectMeta: testutil.CreateTestObject("redis-cluster-skip-test", ns, nil),
					Spec: rcvb2.RedisClusterSpec{
						ClusterSize: ptr.To(int32(3)),
						KubernetesConfig: common.KubernetesConfig{
							Image: testutil.DefaultRedisImage,
						},
					},
				},
				SkipAnnotationKey: controllercommon.RedisClusterSkipReconcileAnnotation,
				StatefulSetName:   "redis-cluster-skip-test-leader",
				Namespace:         ns,
				Timeout:           timeout,
				Interval:          interval,
			})
		})
	})

	Context("When pods become not ready after the cluster has been Ready", func() {
		const degradedName = "redis-cluster-degraded-test"

		// setStatefulSetReadyReplicas simulates pod readiness changes by updating the
		// StatefulSet status, since no StatefulSet controller runs in envtest.
		setStatefulSetReadyReplicas := func(name string, readyReplicas int32) {
			Eventually(func() error {
				sts := &appsv1.StatefulSet{}
				if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, sts); err != nil {
					return err
				}
				sts.Status.Replicas = *sts.Spec.Replicas
				sts.Status.ReadyReplicas = readyReplicas
				sts.Status.AvailableReplicas = readyReplicas
				sts.Status.CurrentReplicas = *sts.Spec.Replicas
				sts.Status.UpdatedReplicas = *sts.Spec.Replicas
				sts.Status.ObservedGeneration = sts.Generation
				return k8sClient.Status().Update(context.Background(), sts)
			}, timeout, interval).Should(Succeed())
		}

		getClusterStatus := func() (rcvb2.RedisClusterStatus, error) {
			rc := &rcvb2.RedisCluster{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: degradedName, Namespace: ns}, rc)
			return rc.Status, err
		}

		It("should leave the Ready state and decrease the ready replica counts", func() {
			redisCluster := &rcvb2.RedisCluster{
				ObjectMeta: testutil.CreateTestObject(degradedName, ns, nil),
				Spec: rcvb2.RedisClusterSpec{
					ClusterSize: ptr.To(int32(3)),
					KubernetesConfig: common.KubernetesConfig{
						Image: testutil.DefaultRedisImage,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), redisCluster)).Should(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.Background(), redisCluster)).Should(Succeed())
			})

			By("marking the leader and follower StatefulSets as ready")
			setStatefulSetReadyReplicas(degradedName+"-leader", 3)
			setStatefulSetReadyReplicas(degradedName+"-follower", 3)

			By("waiting for the operator to report all replicas as ready")
			Eventually(getClusterStatus, timeout, interval).Should(Equal(rcvb2.RedisClusterStatus{
				State:                 rcvb2.RedisClusterBootstrap,
				Reason:                rcvb2.BootstrapClusterReason,
				ReadyLeaderReplicas:   3,
				ReadyFollowerReplicas: 3,
			}))

			By("simulating a cluster that has reached the Ready state")
			Eventually(func() error {
				rc := &rcvb2.RedisCluster{}
				if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: degradedName, Namespace: ns}, rc); err != nil {
					return err
				}
				rc.Status = rcvb2.RedisClusterStatus{
					State:                 rcvb2.RedisClusterReady,
					Reason:                rcvb2.ReadyClusterReason,
					ReadyLeaderReplicas:   3,
					ReadyFollowerReplicas: 3,
				}
				return k8sClient.Status().Update(context.Background(), rc)
			}, timeout, interval).Should(Succeed())

			By("verifying the status stays Ready while all pods are ready")
			Consistently(func() (rcvb2.RedisClusterState, error) {
				status, err := getClusterStatus()
				return status.State, err
			}, time.Second*2, interval).Should(Equal(rcvb2.RedisClusterReady))

			By("dropping one follower pod from ready")
			setStatefulSetReadyReplicas(degradedName+"-follower", 2)

			By("verifying the status leaves Ready and the follower count drops")
			Eventually(getClusterStatus, timeout, interval).Should(Equal(rcvb2.RedisClusterStatus{
				State:                 rcvb2.RedisClusterInitializing,
				Reason:                rcvb2.InitializingClusterFollowerReason,
				ReadyLeaderReplicas:   3,
				ReadyFollowerReplicas: 2,
			}))

			By("dropping one leader pod from ready")
			setStatefulSetReadyReplicas(degradedName+"-leader", 2)

			By("verifying the leader count drops as well")
			Eventually(getClusterStatus, timeout, interval).Should(Equal(rcvb2.RedisClusterStatus{
				State:                 rcvb2.RedisClusterInitializing,
				Reason:                rcvb2.InitializingClusterLeaderReason,
				ReadyLeaderReplicas:   2,
				ReadyFollowerReplicas: 2,
			}))

			By("recovering all pods")
			setStatefulSetReadyReplicas(degradedName+"-leader", 3)
			setStatefulSetReadyReplicas(degradedName+"-follower", 3)

			By("verifying the status reports all replicas as ready again")
			Eventually(getClusterStatus, timeout, interval).Should(Equal(rcvb2.RedisClusterStatus{
				State:                 rcvb2.RedisClusterBootstrap,
				Reason:                rcvb2.BootstrapClusterReason,
				ReadyLeaderReplicas:   3,
				ReadyFollowerReplicas: 3,
			}))
		})
	})
})
