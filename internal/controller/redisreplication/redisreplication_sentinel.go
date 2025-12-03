package redisreplication

import (
	"fmt"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func newSentinelService(rr *rrvb2.RedisReplication) corev1.Service {
	labels := common.GetRedisLabels(
		rr.SentinelStatefulSet(),
		common.SetupTypeSentinel,
		"sentinel",
		rr.GetLabels(),
	)

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rr.SentinelHLService(),
			Namespace: rr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "sentinel",
					Port:       26379,
					TargetPort: intstr.FromInt(26379),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

func newSentinelStatefulSet(rr *rrvb2.RedisReplication, svcName string) appsv1.StatefulSet {
	labels := common.GetRedisLabels(
		rr.SentinelStatefulSet(),
		common.SetupTypeSentinel,
		"sentinel",
		rr.GetLabels(),
	)
	return statefulset.New(statefulset.Params{
		Name:            rr.SentinelStatefulSet(),
		Namespace:       rr.Namespace,
		Replicas:        rr.Spec.Sentinel.Size,
		ServiceName:     svcName,
		PodTemplateSpec: buildSentinelPodTemplate(rr, labels),
	})
}

func buildSentinelPodTemplate(rr *rrvb2.RedisReplication, labels map[string]string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				buildSentinelContainer(rr),
			},
		},
	}
}

func buildSentinelContainer(rr *rrvb2.RedisReplication) corev1.Container {
	container := corev1.Container{
		Name:            "sentinel",
		Image:           rr.Spec.Sentinel.Image,
		ImagePullPolicy: rr.Spec.Sentinel.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "sentinel",
				ContainerPort: 26379,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: buildSentinelEnv(rr),
	}
	if rr.Spec.Sentinel.Resources != nil {
		container.Resources = *rr.Spec.Sentinel.Resources
	}
	return container
}

func buildSentinelEnv(rr *rrvb2.RedisReplication) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{Name: "QUORUM", Value: fmt.Sprintf("%d", rr.Spec.Sentinel.Size/2+1)},
	}
	if rr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		envs = append(envs, corev1.EnvVar{
			Name: "MASTER_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *rr.Spec.KubernetesConfig.ExistingPasswordSecret.Name,
					},
					Key: *rr.Spec.KubernetesConfig.ExistingPasswordSecret.Key,
				},
			},
		})
	}

	return envs
}
