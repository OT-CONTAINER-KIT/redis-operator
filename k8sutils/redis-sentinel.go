package k8sutils

import (
	redisv1beta1 "redis-operator/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
)

// RedisSentinelSTS is a interface to call Redis Statefulset function
type RedisSentinelSTS struct {
	RedisStateFulType string
	ExternalConfig    *string
	Affinity          *corev1.Affinity `json:"affinity,omitempty"`
	ReadinessProbe    *redisv1beta1.Probe
	LivenessProbe     *redisv1beta1.Probe
}

// RedisSentinelService is a interface to call Redis Service function
type RedisSentinelService struct {
	RedisServiceRole string
}

// Redis Sentinel Create the Redis Sentinel Setup
func CreateRedisSentinel(cr *redisv1beta1.RedisSentinel) error {
	prop := RedisSentinelSTS{
		RedisStateFulType: "sentinel",
		Affinity:          cr.Spec.RedisSnt.Affinity,
		ReadinessProbe:    cr.Spec.RedisSnt.ReadinessProbe,
		LivenessProbe:     cr.Spec.RedisSnt.LivenessProbe,
	}

	if cr.Spec.RedisSnt.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisSnt.RedisConfig.AdditionalRedisConfig
	}

	return prop.CreateRedisSentinelSetup(cr)

}

// Create RedisSentinel Service
func CreateRedisSentinelService(cr *redisv1beta1.RedisSentinel) error {

	prop := RedisSentinelService{
		RedisServiceRole: "sentinel",
	}
	return prop.CreateRedisSentinelService(cr)
}

// Create Redis Sentinel Cluster Setup
func (service RedisSentinelSTS) CreateRedisSentinelSetup(cr *redisv1beta1.RedisSentinel) error {

	stateFulName := cr.ObjectMeta.Name + "-" + service.RedisStateFulType
	logger := statefulSetLogger(cr.Namespace, stateFulName)
	labels := getRedisLabels(stateFulName, "cluster", service.RedisStateFulType, cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(
		cr.Namespace,
		objectMetaInfo,
		generateRedisSentinelParams(cr, service.getSentinelCount(cr), service.ExternalConfig, service.Affinity),
		redisSentinelAsOwner(cr),
		generateRedisSentinelContainerParams(cr, service.ReadinessProbe, service.LivenessProbe),
		cr.Spec.Sidecars,
	)

	if err != nil {
		logger.Error(err, "Cannot create Sentinel statefulset for Redis")
		return err
	}
	return nil
}

// Create Redis Sentile Params for the statefulset
func generateRedisSentinelParams(cr *redisv1beta1.RedisSentinel, replicas int32, externalConfig *string, affinity *corev1.Affinity) statefulSetParameters {

	res := statefulSetParameters{
		Metadata:           cr.ObjectMeta,
		Replicas:           &replicas,
		NodeSelector:       cr.Spec.NodeSelector,
		SecurityContext:    cr.Spec.SecurityContext,
		PriorityClassName:  cr.Spec.PriorityClassName,
		Affinity:           affinity,
		Tolerations:        cr.Spec.Tolerations,
		ServiceAccountName: cr.Spec.ServiceAccountName,
		UpdateStrategy:     cr.Spec.KubernetesConfig.UpdateStrategy,
	}

	if cr.Spec.RedisExporter != nil {
		res.EnableMetrics = cr.Spec.RedisExporter.Enabled
	}
	if cr.Spec.KubernetesConfig.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.KubernetesConfig.ImagePullSecrets
	}
	if cr.Spec.Storage != nil {
		res.PersistentVolumeClaim = cr.Spec.Storage.VolumeClaimTemplate
	}
	if externalConfig != nil {
		res.ExternalConfig = externalConfig
	}

	return res
}

// Create Redis Sentinel Statefulset Container Params
func generateRedisSentinelContainerParams(cr *redisv1beta1.RedisSentinel, readinessProbeDef *redisv1beta1.Probe, livenessProbeDef *redisv1beta1.Probe) containerParameters {

	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:                "sentinel",
		Image:               cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy:     cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:           cr.Spec.KubernetesConfig.Resources,
		AdditionalVolume:    cr.Spec.Storage.VolumeMount.Volume,
		AdditionalMountPath: cr.Spec.Storage.VolumeMount.MountPath,
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
	if readinessProbeDef != nil {
		containerProp.ReadinessProbe = readinessProbeDef
	}
	if livenessProbeDef != nil {
		containerProp.LivenessProbe = livenessProbeDef
	}
	if cr.Spec.Storage != nil {
		containerProp.PersistenceEnabled = &trueProperty
	} else {
		containerProp.PersistenceEnabled = &falseProperty
	}
	if cr.Spec.TLS != nil {
		containerProp.TLSConfig = cr.Spec.TLS
	}

	return containerProp

}

// Get the Count of the Sentinel
func (service RedisSentinelSTS) getSentinelCount(cr *redisv1beta1.RedisSentinel) int32 {
	return cr.Spec.GetSentinelCounts(service.RedisStateFulType)
}

// Create the Service for redis sentinel
func (service RedisSentinelService) CreateRedisSentinelService(cr *redisv1beta1.RedisSentinel) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole
	logger := serviceLogger(cr.Namespace, serviceName)
	labels := getRedisLabels(serviceName, "cluster", service.RedisServiceRole, cr.ObjectMeta.Labels)
	annotations := generateServiceAnots(cr.ObjectMeta, nil)

	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		enableMetrics = true
	}
	additionalServiceAnnotations := map[string]string{}
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceAnnotations = cr.Spec.KubernetesConfig.Service.ServiceAnnotations
	}

	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, annotations)
	additionalObjectMetaInfo := generateObjectMetaInformation(serviceName+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, additionalServiceAnnotations))

	err := CreateOrUpdateService(cr.Namespace, headlessObjectMetaInfo, redisSentinelAsOwner(cr), false, true, "ClusterIP")
	if err != nil {
		logger.Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisSentinelAsOwner(cr), enableMetrics, false, "ClusterIP")
	if err != nil {
		logger.Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}

	additionalServiceType := "ClusterIP"
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceType = cr.Spec.KubernetesConfig.Service.ServiceType
	}
	err = CreateOrUpdateService(cr.Namespace, additionalObjectMetaInfo, redisSentinelAsOwner(cr), false, false, additionalServiceType)
	if err != nil {
		logger.Error(err, "Cannot create additional service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil

}
