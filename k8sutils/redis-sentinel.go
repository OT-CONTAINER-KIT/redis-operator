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
	ReadinessProbe    *corev1.Probe
	LivenessProbe     *corev1.Probe
}

// RedisSentinelService is a interface to call Redis Service function
type RedisSentinelService struct {
	RedisServiceRole string
}

// generateRedisStandalone generates Redis standalone information
func generateRedisSentinelParams(cr *redisv1beta1.RedisSentinel, replicas *int32, externalConfig *string, affinity *corev1.Affinity) statefulSetParameters {
	res := statefulSetParameters{
		Replicas:          replicas,
		NodeSelector:      cr.Spec.NodeSelector,
		SecurityContext:   cr.Spec.SecurityContext,
		PriorityClassName: cr.Spec.PriorityClassName,
		Affinity:          affinity,
		Tolerations:       cr.Spec.Tolerations,
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

// generateRedisStandaloneContainerParams generates Redis container information
func generateRedisSentinelContainerParams(cr *redisv1beta1.RedisSentinel, readinessProbeDef *corev1.Probe, livenessProbeDef *corev1.Probe) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "cluster",
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
	if readinessProbeDef != nil {
		containerProp.ReadinessProbe = readinessProbeDef
	}
	if livenessProbeDef != nil {
		containerProp.LivenessProbe = livenessProbeDef
	}
	if cr.Spec.Storage != nil {
		containerProp.PersistenceEnabled = &trueProperty
	}
	return containerProp
}

// CreateRedisReplica will create a redis setup
func CreateRedisReplica(cr *redisv1beta1.RedisSentinel) error {
	prop := RedisSentinelSTS{
		RedisStateFulType: "replica",
		Affinity:          cr.Spec.RedisReplica.Affinity,
		ReadinessProbe:    cr.Spec.RedisReplica.ReadinessProbe,
		LivenessProbe:     cr.Spec.RedisReplica.LivenessProbe,
	}
	if cr.Spec.RedisReplica.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisReplica.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisSentinelSetup(cr)
}

// CreateRedisFollower will create a follower redis setup
func CreateRedisSentinel(cr *redisv1beta1.RedisSentinel) error {
	prop := RedisSentinelSTS{
		RedisStateFulType: "sentinel",
		Affinity:          cr.Spec.RedisSentinel.Affinity,
		ReadinessProbe:    cr.Spec.RedisSentinel.ReadinessProbe,
		LivenessProbe:     cr.Spec.RedisSentinel.LivenessProbe,
	}
	// if cr.Spec.RedisSentinel.RedisConfig != nil {
	// 	prop.ExternalConfig = cr.Spec.RedisSentinel.RedisConfig.AdditionalRedisConfig
	// }
	return prop.CreateRedisSentinelSetup(cr)
}

// CreateRedisSentinelReplicaService method will create service for Redis Replica
func CreateRedisSentinelReplicaService(cr *redisv1beta1.RedisSentinel) error {
	prop := RedisSentinelService{
		RedisServiceRole: "replica",
	}
	return prop.CreateRedisSentinelRepliaService(cr)
}

// CreateRedisFollowerService method will create service for Redis Follower
func CreateRedisSentinelService(cr *redisv1beta1.RedisSentinel) error {
	prop := RedisSentinelService{
		RedisServiceRole: "sentinel",
	}
	return prop.CreateRedisSentinelService(cr)
}

func (service RedisSentinelSTS) getReplicaCount(cr *redisv1beta1.RedisSentinel) *int32 {
	var replicas *int32
	if service.RedisStateFulType == "replica" {
		replicas = cr.Spec.RedisReplica.Replicas
	} else {
		replicas = cr.Spec.RedisSentinel.Replicas
	}

	// We fall back to the overall/default size if we don't have a specific one.
	if replicas == nil {
		replicas = cr.Spec.Size
	}
	return replicas
}

// CreateRedisSentinelSetup will create Redis Setup for Replica and Sentinels
func (service RedisSentinelSTS) CreateRedisSentinelSetup(cr *redisv1beta1.RedisSentinel) error {
	stateFulName := cr.ObjectMeta.Name + "-" + service.RedisStateFulType
	logger := stateFulSetLogger(cr.Namespace, stateFulName)
	labels := getRedisLabels(stateFulName, "sentinel", service.RedisStateFulType)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, generateStatefulSetsAnots())
	err := CreateOrUpdateStateFul(
		cr.Namespace,
		objectMetaInfo,
		labels,
		generateRedisSentinelParams(cr, service.getReplicaCount(cr), service.ExternalConfig, service.Affinity),
		RedisSentinelAsOwner(cr),
		generateRedisSentinelContainerParams(cr, service.ReadinessProbe, service.LivenessProbe),
		cr.Spec.Sidecars,
	)
	if err != nil {
		logger.Error(err, "Cannot create statefulset for Redis", "Setup.Type", service.RedisStateFulType)
		return err
	}
	return nil
}

// CreateRedisSentinelService method will create service for Redis
func (service RedisSentinelService) CreateRedisSentinelService(cr *redisv1beta1.RedisSentinel) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole
	logger := serviceLogger(cr.Namespace, serviceName)
	labels := getRedisLabels(serviceName, "sentinel", service.RedisServiceRole)

	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		enableMetrics = true
	}

	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, generateServiceAnots())
	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, generateServiceAnots())

	err := CreateOrUpdateHeadlessService(cr.Namespace, headlessObjectMetaInfo, labels, RedisSentinelAsOwner(cr))
	if err != nil {
		logger.Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}

	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, labels, RedisSentinelAsOwner(cr), enableMetrics)
	if err != nil {
		logger.Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}

// CreateRedisSentinelService method will create service for Redis
func (service RedisSentinelService) CreateRedisSentinelRepliaService(cr *redisv1beta1.RedisSentinel) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole
	logger := serviceLogger(cr.Namespace, serviceName)
	labels := getRedisLabels(serviceName, "sentinel", service.RedisServiceRole)

	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		enableMetrics = true
	}

	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, generateServiceAnots())

	err := CreateOrUpdateHeadlessService(cr.Namespace, objectMetaInfo, labels, RedisSentinelAsOwner(cr))
	if err != nil {
		logger.Error(err, "Cannot create service for Redis Replica", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}
