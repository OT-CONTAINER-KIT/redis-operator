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

type StatefulInterface struct {
	Existing *appsv1.StatefulSet
	Desired  *appsv1.StatefulSet
	Type     string
}

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
func GenerateContainerDef(cr *redisv1alpha1.Redis, role string) corev1.Container{
	var containerDefinition corev1.Container
	containerDefinition = corev1.Container{
		Name:            cr.ObjectMeta.Name + "-" + role,
		Image:           cr.Spec.ImageName,
		ImagePullPolicy: cr.Spec.ImagePullPolicy,
		Env: []corev1.EnvVar{
			{
				Name: "SERVER_MODE",
				Value: role,
			},
		},
	}
	if cr.Spec.RedisPassword != nil {
		containerDefinition.Env = append(containerDefinition.Env, corev1.EnvVar{
			Name: "REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cr.ObjectMeta.Name,
					},
					Key: "password",
				},
			},
		})
	}
	if cr.Spec.Mode != "cluster" {
		containerDefinition.Env = append(containerDefinition.Env, corev1.EnvVar{
			Name: "SETUP_MODE",
			Value: "standalone",
		})
	} else {
		containerDefinition.Env = append(containerDefinition.Env, corev1.EnvVar{
			Name: "SETUP_MODE",
			Value: "cluster",
		})
	}
	return containerDefinition
}

// FinalContainerDef will generate the final statefulset definition
func FinalContainerDef(cr *redisv1alpha1.Redis, role string) []corev1.Container{
	var containerDefinition []corev1.Container

	containerDefinition = append(containerDefinition, GenerateContainerDef(cr, role))

	if cr.Spec.RedisExporter != true {
		containerDefinition = append(containerDefinition, GenerateContainerDef(cr, role))
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
								Name: cr.ObjectMeta.Name,
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
func CreateRedisMaster(cr *redisv1alpha1.Redis) {

	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-master",
		"role": "master",
	}
	statefulDefinition := GenerateStateFulSetsDef(cr, labels, "master", cr.Spec.Size)
	statefulObject, err := GenerateK8sClient().AppsV1().StatefulSets(cr.Namespace).Get(cr.ObjectMeta.Name + "-master", metav1.GetOptions{})
	stateful := StatefulInterface{
		Existing: statefulObject,
		Desired:  statefulDefinition,
		Type:     "master",
	}
	CompareAndCreateStateful(cr, stateful, err)
}

// CreateRedisSlave will create a Redis Slave
func CreateRedisSlave(cr *redisv1alpha1.Redis) {
	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-slave",
		"role": "slave",
	}
	statefulDefinition := GenerateStateFulSetsDef(cr, labels, "slave", cr.Spec.Size)
	statefulObject, err := GenerateK8sClient().AppsV1().StatefulSets(cr.Namespace).Get(cr.ObjectMeta.Name + "-slave", metav1.GetOptions{})

	stateful := StatefulInterface{
		Existing: statefulObject,
		Desired:  statefulDefinition,
		Type:     "slave",
	}
	CompareAndCreateStateful(cr, stateful, err)
}

// CreateRedisStandalone will create a Redis Standalone server
func CreateRedisStandalone(cr *redisv1alpha1.Redis){
	var standaloneReplica int32 = 1

	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-" + "standalone",
		"role": "standalone",
	}
	statefulDefinition := GenerateStateFulSetsDef(cr, labels, "standalone", &standaloneReplica)
	statefulObject, err := GenerateK8sClient().AppsV1().StatefulSets(cr.Namespace).Get(cr.ObjectMeta.Name + "-standalone", metav1.GetOptions{})

	stateful := StatefulInterface{
		Existing: statefulObject,
		Desired:  statefulDefinition,
		Type:     "standalone",
	}
	CompareAndCreateStateful(cr, stateful, err)
}

// CompareAndCreateStateful will compare and create a statefulset pod
func CompareAndCreateStateful(cr *redisv1alpha1.Redis, clusterInfo StatefulInterface, err error) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)

	if err != nil {
		reqLogger.Info("Creating redis setup", "Redis.Name", cr.ObjectMeta.Name + "-" + clusterInfo.Type, "Setup.Type", clusterInfo.Type)
		GenerateK8sClient().AppsV1().StatefulSets(cr.Namespace).Create(clusterInfo.Desired)
	} else if clusterInfo.Existing != clusterInfo.Desired {
		reqLogger.Info("Reconciling redis setup", "Redis.Name", cr.ObjectMeta.Name + "-" + clusterInfo.Type, "Setup.Type", clusterInfo.Type)
		GenerateK8sClient().AppsV1().StatefulSets(cr.Namespace).Update(clusterInfo.Desired)	
	} else {
		reqLogger.Info("Redis setup is in sync", "Redis.Name", cr.ObjectMeta.Name + "-" + clusterInfo.Type, "Setup.Type", clusterInfo.Type)
	}
}
