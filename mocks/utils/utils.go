package utils

import (
	"context"
	"fmt"
	"strconv"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func CreateFakeClientWithPodIPs_LeaderPods(cr *redisv1beta2.RedisCluster) *fake.Clientset {
	replicas := cr.Spec.GetReplicaCounts("leader")
	pods := make([]runtime.Object, replicas)

	for i := 0; i < int(replicas); i++ {
		podName := cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(i)
		pods[i] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: cr.Namespace,
			},
			Status: corev1.PodStatus{
				PodIP: fmt.Sprintf("192.168.1.%d", i+1),
			},
		}
	}
	return fake.NewSimpleClientset(pods...)
}

func CreateFakeObjectWithPodIPs(cr *redisv1beta2.RedisCluster) []runtime.Object {
	leaderReplicas := cr.Spec.GetReplicaCounts("leader")
	followerReplicas := cr.Spec.GetReplicaCounts("follower")
	pods := make([]runtime.Object, leaderReplicas+followerReplicas)

	for i := 0; i < int(leaderReplicas); i++ {
		podName := cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(i)
		pods[i] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: cr.Namespace,
			},
			Status: corev1.PodStatus{
				PodIP: fmt.Sprintf("192.168.1.%d", i+1),
			},
		}
	}
	for i := 0; i < int(followerReplicas); i++ {
		podName := cr.ObjectMeta.Name + "-follower-" + strconv.Itoa(i)
		pods[i+int(leaderReplicas)] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: cr.Namespace,
			},
			Status: corev1.PodStatus{
				PodIP: fmt.Sprintf("192.168.2.%d", i+1),
			},
		}
	}

	return pods
}

func CreateFakeObjectWithSecret(name, namespace, key string) []runtime.Object {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key: []byte("password"),
		},
	}
	return []runtime.Object{secret}
}

func CreateFakeClientWithSecrets(ctx context.Context, cr *redisv1beta2.RedisCluster, secretName, secretKey, secretValue string) *fake.Clientset {
	leaderReplicas := cr.Spec.GetReplicaCounts("leader")
	followerReplicas := cr.Spec.GetReplicaCounts("follower")
	pods := make([]runtime.Object, 0)

	for i := 0; i < int(leaderReplicas); i++ {
		podName := cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(i)
		pods[i] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: cr.Namespace,
			},
			Status: corev1.PodStatus{
				PodIP: fmt.Sprintf("192.168.1.%d", i+1),
			},
		}
	}
	for i := 0; i < int(followerReplicas); i++ {
		podName := cr.ObjectMeta.Name + "-follower-" + strconv.Itoa(i)
		pods[i+int(leaderReplicas)] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: cr.Namespace,
			},
			Status: corev1.PodStatus{
				PodIP: fmt.Sprintf("192.168.2.%d", i+1),
			},
		}
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cr.Namespace,
		},
		Data: map[string][]byte{
			secretKey: []byte(secretValue),
		},
	}

	return fake.NewSimpleClientset(append(pods, secret)...)
}
