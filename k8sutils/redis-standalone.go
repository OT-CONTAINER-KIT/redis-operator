package k8sutils

import (
	redisv1beta1 "redis-operator/api/v1beta1"
)

var (
	k8sServiceType string
	enableMetrics  bool
)

// CreateStandAloneService method will create standalone service for Redis
func CreateStandAloneService(cr *redisv1beta1.Redis) error {
	logger := serviceLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getRedisLabels(cr.ObjectMeta.Name, "standalone", "standalone")
	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		enableMetrics = true
	}
	k8sServiceType = cr.Spec.KubernetesConfig.ServiceType
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, generateServiceAnots())
	headlessObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-headless", cr.Namespace, labels, generateServiceAnots())
	err := CreateOrUpdateHeadlessService(cr.Namespace, headlessObjectMetaInfo, labels, redisAsOwner(cr))
	if err != nil {
		logger.Error(err, "Cannot create standalone headless service for Redis")
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, labels, redisAsOwner(cr), k8sServiceType, enableMetrics)
	if err != nil {
		logger.Error(err, "Cannot create standalone service for Redis")
		return err
	}
	return nil
}

// CreateStandAloneRedis will create a standalone redis setup
func CreateStandAloneRedis(cr *redisv1beta1.Redis) error {
	logger := stateFulSetLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getRedisLabels(cr.ObjectMeta.Name, "standalone", "standalone")
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, generateStatefulSetsAnots())
	err := CreateOrUpdateStateFul(cr.Namespace,
		objectMetaInfo,
		labels,
		generateRedisStandaloneParams(cr),
		redisAsOwner(cr),
		generateRedisStandaloneContainerParams(cr),
	)
	if err != nil {
		logger.Error(err, "Cannot create standalone statefulset for Redis")
		return err
	}
	return nil
}

// generateRedisStandalone generates Redis standalone information
func generateRedisStandaloneParams(cr *redisv1beta1.Redis) statefulSetParameters {
	replicas := int32(1)
	res := statefulSetParameters{
		Replicas:          &replicas,
		NodeSelector:      cr.Spec.NodeSelector,
		SecurityContext:   cr.Spec.SecurityContext,
		PriorityClassName: cr.Spec.PriorityClassName,
		Affinity:          cr.Spec.Affinity,
		Tolerations:       cr.Spec.Tolerations,
		EnableMetrics:     cr.Spec.RedisExporter.Enabled,
	}
	if cr.Spec.KubernetesConfig.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.KubernetesConfig.ImagePullSecrets
	}
	if cr.Spec.Storage != nil {
		res.PersistentVolumeClaim = cr.Spec.Storage.VolumeClaimTemplate
	}
	return res
}

// generateRedisStandaloneContainerParams generates Redis container information
func generateRedisStandaloneContainerParams(cr *redisv1beta1.Redis) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:                         "standalone",
		Image:                        cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy:              cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:                    cr.Spec.KubernetesConfig.Resources,
		RedisExporterImage:           cr.Spec.RedisExporter.Image,
		RedisExporterImagePullPolicy: cr.Spec.RedisExporter.ImagePullPolicy,
		RedisExporterResources:       cr.Spec.RedisExporter.Resources,
	}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		containerProp.EnabledPassword = &trueProperty
		containerProp.SecretName = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name
		containerProp.SecretKey = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key
	} else {
		containerProp.EnabledPassword = &falseProperty
	}
	if cr.Spec.RedisExporter.EnvVars != nil {
		containerProp.RedisExporterEnv = cr.Spec.RedisExporter.EnvVars
	}
	if cr.Spec.Storage != nil {
		containerProp.PersistenceEnabled = &trueProperty
	}
	return containerProp
}
