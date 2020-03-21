package otmachinery

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	redisv1alpha1 "redis-operator/redis-operator/pkg/apis/redis/v1alpha1"
)

const (
	constRedisExpoterName = "redis-exporter"
)

// GenerateStateFulSetsDef generates the statefulsets definition
func GenerateStateFulSetsDef(cr *redisv1alpha1.Redis, labels map[string]string, role string, replicas *int32) *appsv1.StatefulSet{
	statefulset := &appsv1.StatefulSet{
		TypeMeta: GenerateMetaInformation("StatefulSet", "apps/v1"),
		ObjectMeta: GenerateObjectMetaInformation(cr.ObjectMeta.Name + "-" + role, cr.Namespace, labels, GenerateStatefulSetsAnots()),
		Spec: appsv1.StatefulSetSpec{
			Selector:    LabelSelectors(labels),
			ServiceName: cr.ObjectMeta.Name + "-" + role,
			Replicas:    replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: FinalContainerDef(cr, role),
				},
			},
		},
	}
	AddOwnerRefToObject(statefulset, AsOwner(cr))
	return statefulset
}

// GenerateContainerDef generates container definition
func GenerateContainerDef(cr *redisv1alpha1.Redis, role string) []corev1.Container{
	var containerDefinition []corev1.Container
	if cr.Spec.RedisPassword != nil {
		containerDefinition = append(containerDefinition, corev1.Container{
			Name:            cr.ObjectMeta.Name + "-" + role,
			Image:           cr.Spec.ImageName,
			ImagePullPolicy: cr.Spec.ImagePullPolicy,
			Env: []corev1.EnvVar{
				{
					Name: "REDIS_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cr.ObjectMeta.Name + "-" + "generic",
							},
							Key: "password",
						},
					},
				},{
					Name: "SERVER_MODE",
					Value: role,
				},
				{
					Name: "SETUP_MODE",
					Value: "cluster",
				},
			},
		})
	} else {
		containerDefinition = append(containerDefinition, corev1.Container{
			Name:            cr.ObjectMeta.Name + "-" + role,
			Image:           cr.Spec.ImageName,
			ImagePullPolicy: cr.Spec.ImagePullPolicy,
			Env: []corev1.EnvVar{
				{
					Name: "SERVER_MODE",
					Value: role,
				},
				{
					Name: "SETUP_MODE",
					Value: "cluster",
				},
			},
		})
	}
	return containerDefinition
}

// FinalContainerDef will generate the final statefulset definition
func FinalContainerDef(cr *redisv1alpha1.Redis, role string) []corev1.Container{
	var containerDefinition []corev1.Container

	containerDefinition = GenerateContainerDef(cr, role)

	if cr.Spec.RedisExporter != true {
		containerDefinition = GenerateContainerDef(cr, role)
	} else {
		containerDefinition = append(containerDefinition, corev1.Container{
			Name:            constRedisExpoterName,
			Image:           cr.Spec.RedisExporterImage,
			ImagePullPolicy: cr.Spec.ImagePullPolicy,
			Env: []corev1.EnvVar{
				{
					Name: "REDIS_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cr.ObjectMeta.Name + "-" + "generic",
							},
							Key: "password",
						},
					},
				},{
					Name: "REDIS_ADDR",
					Value: "redis://localhost:6379",
				},
			},
		})
	}
	return containerDefinition
}

// CreateRedisMaster will create a Redis Master
func CreateRedisMaster(cr *redisv1alpha1.Redis) *appsv1.StatefulSet{
	var tempReplicas int32
	var replicas *int32

	for count, _ := range cr.Spec.Master {
		tempReplicas += int32(count)
		replicas = &tempReplicas
	}

	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-" + "master",
		"role": "master",
	}
	return GenerateStateFulSetsDef(cr, labels, "master", replicas)
}

// CreateRedisSlave will create a Redis Slave
func CreateRedisSlave(cr *redisv1alpha1.Redis) *appsv1.StatefulSet{
	var tempReplicas int32
	var replicas *int32

	for count, _ := range cr.Spec.Master {
		tempReplicas += int32(count)
		replicas = &tempReplicas
	}

	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-" + "slave",
		"role": "slave",
	}
	return GenerateStateFulSetsDef(cr, labels, "slave", replicas)
}
