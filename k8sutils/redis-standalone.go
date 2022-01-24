package k8sutils

import (
	redisv1beta1 "redis-operator/api/v1beta1"
)

var (
	enableMetrics bool
)

// CreateStandaloneService method will create standalone service for Redis
func CreateStandaloneService(cr *redisv1beta1.Redis) error {
	logger := serviceLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getRedisLabels(cr.ObjectMeta.Name, "standalone", "standalone", cr.ObjectMeta.Labels)
	annotations := generateServiceAnots(cr.ObjectMeta)
	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		enableMetrics = true
	}
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-headless", cr.Namespace, labels, annotations)
	err := CreateOrUpdateService(cr.Namespace, headlessObjectMetaInfo, redisAsOwner(cr), false, true)
	if err != nil {
		logger.Error(err, "Cannot create standalone headless service for Redis")
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisAsOwner(cr), enableMetrics, false)
	if err != nil {
		logger.Error(err, "Cannot create standalone service for Redis")
		return err
	}
	return nil
}

// CreateStandaloneRedis will create a standalone redis setup
func CreateStandaloneRedis(cr *redisv1beta1.Redis) error {
	logger := statefulSetLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getRedisLabels(cr.ObjectMeta.Name, "standalone", "standalone", cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta)
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(cr.Namespace,
		objectMetaInfo,
		generateRedisStandaloneParams(cr),
		redisAsOwner(cr),
		generateRedisStandaloneContainerParams(cr),
		cr.Spec.Sidecars,
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
	}
	if cr.Spec.KubernetesConfig.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.KubernetesConfig.ImagePullSecrets
	}
	if cr.Spec.Storage != nil {
		res.PersistentVolumeClaim = cr.Spec.Storage.VolumeClaimTemplate
	}
	if cr.Spec.RedisConfig != nil {
		res.ExternalConfig = cr.Spec.RedisConfig.AdditionalRedisConfig
	}
	if cr.Spec.RedisExporter != nil {
		res.EnableMetrics = cr.Spec.RedisExporter.Enabled

	}
	return res
}

// generateRedisStandaloneContainerParams generates Redis container information
func generateRedisStandaloneContainerParams(cr *redisv1beta1.Redis) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "standalone",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resources,
	}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		containerProp.EnabledPassword = &trueProperty
		containerProp.SecretName = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name
		containerProp.SecretKey = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key
	} else {
		containerProp.EnabledPassword = &falseProperty
	}
	if cr.Spec.RedisExporter != nil {
		containerProp.RedisExporterImage = cr.Spec.RedisExporter.Image
		containerProp.RedisExporterImagePullPolicy = cr.Spec.RedisExporter.ImagePullPolicy

		if cr.Spec.RedisExporter.Resources != nil {
			containerProp.RedisExporterResources = cr.Spec.RedisExporter.Resources
		}

		if cr.Spec.RedisExporter.EnvVars != nil {
			containerProp.RedisExporterEnv = cr.Spec.RedisExporter.EnvVars
		}

	}
	if cr.Spec.ReadinessProbe != nil {
		containerProp.ReadinessProbe = cr.Spec.ReadinessProbe
	}
	if cr.Spec.LivenessProbe != nil {
		containerProp.LivenessProbe = cr.Spec.LivenessProbe
	}
	if cr.Spec.Storage != nil {
		containerProp.PersistenceEnabled = &trueProperty
	}
	return containerProp
}
