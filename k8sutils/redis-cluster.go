package k8sutils

import (
	corev1 "k8s.io/api/core/v1"
	redisv1beta1 "redis-operator/api/v1beta1"
)

// RedisClusterSTS is a interface to call Redis Statefulset function
type RedisClusterSTS struct {
	RedisStateFulType string
	ExternalConfig    *string
	Affinity          *corev1.Affinity `json:"affinity,omitempty"`
}

// RedisClusterService is a interface to call Redis Service function
type RedisClusterService struct {
	RedisServiceRole string
}

// generateRedisStandalone generates Redis standalone information
func generateRedisClusterParams(cr *redisv1beta1.RedisCluster, replicas *int32, externalConfig *string, affinity *corev1.Affinity) statefulSetParameters {
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
func generateRedisClusterContainerParams(cr *redisv1beta1.RedisCluster) containerParameters {
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
	if cr.Spec.Storage != nil {
		containerProp.PersistenceEnabled = &trueProperty
	}
	return containerProp
}

// CreateRedisLeader will create a leader redis setup
func CreateRedisLeader(cr *redisv1beta1.RedisCluster) error {
	prop := RedisClusterSTS{
		RedisStateFulType: "leader",
		Affinity:          cr.Spec.RedisLeader.Affinity,
	}
	if cr.Spec.RedisLeader.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisClusterSetup(cr)
}

// CreateRedisFollower will create a follower redis setup
func CreateRedisFollower(cr *redisv1beta1.RedisCluster) error {
	prop := RedisClusterSTS{
		RedisStateFulType: "follower",
		Affinity:          cr.Spec.RedisFollower.Affinity,
	}
	if cr.Spec.RedisFollower.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisFollower.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisClusterSetup(cr)
}

// CreateRedisLeaderService method will create service for Redis Leader
func CreateRedisLeaderService(cr *redisv1beta1.RedisCluster) error {
	prop := RedisClusterService{
		RedisServiceRole: "leader",
	}
	return prop.CreateRedisClusterService(cr)
}

// CreateRedisFollowerService method will create service for Redis Follower
func CreateRedisFollowerService(cr *redisv1beta1.RedisCluster) error {
	prop := RedisClusterService{
		RedisServiceRole: "follower",
	}
	return prop.CreateRedisClusterService(cr)
}

func (service RedisClusterSTS) getReplicaCount(cr *redisv1beta1.RedisCluster) *int32 {
	var replicas *int32
	if service.RedisStateFulType == "leader" {
		replicas = cr.Spec.RedisLeader.Replicas
	} else {
		replicas = cr.Spec.RedisFollower.Replicas
	}

	// We fall back to the overall/default size if we don't have a specific one.
	if replicas == nil {
		replicas = cr.Spec.Size
	}
	return replicas
}

// CreateRedisClusterSetup will create Redis Setup for leader and follower
func (service RedisClusterSTS) CreateRedisClusterSetup(cr *redisv1beta1.RedisCluster) error {
	stateFulName := cr.ObjectMeta.Name + "-" + service.RedisStateFulType
	logger := stateFulSetLogger(cr.Namespace, stateFulName)
	labels := getRedisLabels(stateFulName, "cluster", service.RedisStateFulType)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, generateStatefulSetsAnots())
	err := CreateOrUpdateStateFul(
		cr.Namespace,
		objectMetaInfo,
		labels,
		generateRedisClusterParams(cr, service.getReplicaCount(cr), service.ExternalConfig, service.Affinity),
		redisClusterAsOwner(cr),
		generateRedisClusterContainerParams(cr),
	)
	if err != nil {
		logger.Error(err, "Cannot create statefulset for Redis", "Setup.Type", service.RedisStateFulType)
		return err
	}
	return nil
}

// CreateRedisClusterService method will create service for Redis
func (service RedisClusterService) CreateRedisClusterService(cr *redisv1beta1.RedisCluster) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole
	logger := serviceLogger(cr.Namespace, serviceName)
	labels := getRedisLabels(serviceName, "cluster", service.RedisServiceRole)
	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		enableMetrics = true
	}
	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, generateServiceAnots())
	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, generateServiceAnots())
	err := CreateOrUpdateHeadlessService(cr.Namespace, headlessObjectMetaInfo, labels, redisClusterAsOwner(cr))
	if err != nil {
		logger.Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, labels, redisClusterAsOwner(cr), enableMetrics)
	if err != nil {
		logger.Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}

func getRedisLabels(name, setupType, role string) map[string]string {
	return map[string]string{
		"app":              name,
		"redis_setup_type": setupType,
		"role":             role,
	}
}
