package rediscluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	Context("When deploying a NodePort Redis Cluster", func() {
		var redisCluster *rcvb2.RedisCluster

		// announceEnv collects the cluster announce variables of the pod template.
		announceEnv := func(sts *appsv1.StatefulSet) map[string]string {
			env := map[string]string{}
			for _, container := range sts.Spec.Template.Spec.Containers {
				for _, e := range container.Env {
					if strings.HasPrefix(e.Name, "announce_port_") || strings.HasPrefix(e.Name, "announce_bus_port_") {
						env[e.Name] = e.Value
					}
				}
			}
			return env
		}

		// expectAnnounceEnvMatchesServices asserts that every announce variable of the
		// pod template carries the node port Kubernetes actually allocated to the
		// matching per-pod Service.
		expectAnnounceEnvMatchesServices := func(sts *appsv1.StatefulSet, role string) {
			env := announceEnv(sts)
			ExpectWithOffset(1, env).To(HaveLen(6), "every ordinal must contribute a client and a bus announce variable")
			for i := 0; i < 3; i++ {
				serviceName := fmt.Sprintf("%s-%s-%d", redisCluster.Name, role, i)
				svc := &corev1.Service{}
				ExpectWithOffset(1, k8sClient.Get(context.Background(), types.NamespacedName{Name: serviceName, Namespace: ns}, svc)).To(Succeed())

				ports := map[string]int32{}
				for _, port := range svc.Spec.Ports {
					ports[port.Name] = port.NodePort
				}
				ExpectWithOffset(1, ports["redis-client"]).NotTo(BeZero(), "Kubernetes must have allocated a client node port")
				ExpectWithOffset(1, ports["redis-bus"]).NotTo(BeZero(), "Kubernetes must have allocated a bus node port")

				prefix := strings.ReplaceAll(serviceName, "-", "_")
				ExpectWithOffset(1, env["announce_port_"+prefix]).To(Equal(strconv.Itoa(int(ports["redis-client"]))))
				ExpectWithOffset(1, env["announce_bus_port_"+prefix]).To(Equal(strconv.Itoa(int(ports["redis-bus"]))))
			}
		}

		getStatefulSet := func(name string) (*appsv1.StatefulSet, error) {
			sts := &appsv1.StatefulSet{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, sts)
			return sts, err
		}

		BeforeEach(func() {
			redisCluster = &rcvb2.RedisCluster{}
			yamlFile, err := os.ReadFile(filepath.Join("testdata", "nodeport.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(yaml.Unmarshal(yamlFile, redisCluster)).To(Succeed())
			redisCluster.Namespace = ns

			Expect(k8sClient.Create(context.Background(), redisCluster)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.Background(), redisCluster)).Should(Succeed())
		})

		It("should announce the allocated node ports from the first StatefulSet revision", func() {
			leaderName := redisCluster.Name + "-leader"
			followerName := redisCluster.Name + "-follower"

			By("waiting for the leader StatefulSet")
			var leaderSts *appsv1.StatefulSet
			Eventually(func() error {
				var err error
				leaderSts, err = getStatefulSet(leaderName)
				return err
			}, timeout, interval).Should(Succeed())

			By("verifying the leader announce variables match the node ports Kubernetes allocated")
			expectAnnounceEnvMatchesServices(leaderSts, "leader")

			By("verifying the leader pod template was not corrected by a later reconciliation")
			// The generation only stays at 1 if the first rendered spec already carried
			// the announce variables. When the per-pod Services were created after the
			// StatefulSet, the first revision was missing them and a second update
			// bumped the generation.
			Expect(leaderSts.Generation).To(BeEquivalentTo(1))
			Consistently(func() (int64, error) {
				sts, err := getStatefulSet(leaderName)
				return sts.Generation, err
			}, time.Second*2, interval).Should(BeEquivalentTo(1))

			By("marking the leader StatefulSet ready so the follower is reconciled")
			Eventually(func() error {
				sts, err := getStatefulSet(leaderName)
				if err != nil {
					return err
				}
				sts.Status.Replicas = *sts.Spec.Replicas
				sts.Status.ReadyReplicas = *sts.Spec.Replicas
				sts.Status.AvailableReplicas = *sts.Spec.Replicas
				sts.Status.CurrentReplicas = *sts.Spec.Replicas
				sts.Status.UpdatedReplicas = *sts.Spec.Replicas
				sts.Status.ObservedGeneration = sts.Generation
				return k8sClient.Status().Update(context.Background(), sts)
			}, timeout, interval).Should(Succeed())

			By("waiting for the follower StatefulSet")
			var followerSts *appsv1.StatefulSet
			Eventually(func() error {
				var err error
				followerSts, err = getStatefulSet(followerName)
				return err
			}, timeout, interval).Should(Succeed())

			By("verifying the follower announces its own node ports from its first revision")
			expectAnnounceEnvMatchesServices(followerSts, "follower")
			Expect(followerSts.Generation).To(BeEquivalentTo(1))

			By("verifying the leader announce variables did not leak into the follower template")
			for name := range announceEnv(followerSts) {
				Expect(name).NotTo(ContainSubstring("_leader_"))
			}
		})
	})
})
