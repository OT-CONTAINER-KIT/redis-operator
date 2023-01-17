package k8sutils

import (
	redisv1beta1 "redis-operator/api/v1beta1"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
)

// RedisClusterSTS is a interface to call Redis Statefulset function
type RedisClusterSTS struct {
	RedisStateFulType string
	ExternalConfig    *string
	Affinity          *corev1.Affinity `json:"affinity,omitempty"`
	ReadinessProbe    *redisv1beta1.Probe
	LivenessProbe     *redisv1beta1.Probe
}

// RedisClusterService is a interface to call Redis Service function
type RedisClusterService struct {
	RedisServiceRole string
}

// generateRedisClusterParams generates Redis cluster information
func generateRedisClusterParams(cr *redisv1beta1.RedisCluster, replicas int32, externalConfig *string, affinity *corev1.Affinity) statefulSetParameters {
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

// generateRedisClusterContainerParams generates Redis container information
func generateRedisClusterContainerParams(cr *redisv1beta1.RedisCluster, readinessProbeDef *redisv1beta1.Probe, livenessProbeDef *redisv1beta1.Probe) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "cluster",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resources,
	}
	if cr.Spec.Storage != nil {
		containerProp.AdditionalVolume = cr.Spec.Storage.VolumeMount.Volume
		containerProp.AdditionalMountPath = cr.Spec.Storage.VolumeMount.MountPath
	}
	switch true {
	case cr.Spec.KubernetesConfig.ExistOrGenerateSecret.ExistingPasswordSecret != nil:
		containerProp.EnabledPassword = &trueProperty
		containerProp.SecretName = cr.Spec.KubernetesConfig.ExistOrGenerateSecret.ExistingPasswordSecret.Name
		containerProp.SecretKey = cr.Spec.KubernetesConfig.ExistOrGenerateSecret.ExistingPasswordSecret.Key

	case cr.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret != nil:
		containerProp.EnabledPassword = &trueProperty
		containerProp.SecretName = cr.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.Name
		containerProp.SecretKey = cr.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.Key

	default:
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
	if cr.Spec.Storage != nil && cr.Spec.PersistenceEnabled != nil && *cr.Spec.PersistenceEnabled {
		containerProp.PersistenceEnabled = &trueProperty
	} else {
		containerProp.PersistenceEnabled = &falseProperty
	}
	if cr.Spec.TLS != nil {
		containerProp.TLSConfig = cr.Spec.TLS
	}

	return containerProp
}

// CreateRedisLeader will create a leader redis setup
func CreateRedisLeader(cr *redisv1beta1.RedisCluster) error {
	prop := RedisClusterSTS{
		RedisStateFulType: "leader",
		Affinity:          cr.Spec.RedisLeader.Affinity,
		ReadinessProbe:    cr.Spec.RedisLeader.ReadinessProbe,
		LivenessProbe:     cr.Spec.RedisLeader.LivenessProbe,
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
		ReadinessProbe:    cr.Spec.RedisFollower.ReadinessProbe,
		LivenessProbe:     cr.Spec.RedisFollower.LivenessProbe,
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

func (service RedisClusterSTS) getReplicaCount(cr *redisv1beta1.RedisCluster) int32 {
	return cr.Spec.GetReplicaCounts(service.RedisStateFulType)
}

// CreateRedisClusterSetup will create Redis Setup for leader and follower
func (service RedisClusterSTS) CreateRedisClusterSetup(cr *redisv1beta1.RedisCluster) error {
	stateFulName := cr.ObjectMeta.Name + "-" + service.RedisStateFulType
	logger := statefulSetLogger(cr.Namespace, stateFulName)
	labels := getRedisLabels(stateFulName, "cluster", service.RedisStateFulType, cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(
		cr.Namespace,
		objectMetaInfo,
		generateRedisClusterParams(cr, service.getReplicaCount(cr), service.ExternalConfig, service.Affinity),
		redisClusterAsOwner(cr),
		generateRedisClusterContainerParams(cr, service.ReadinessProbe, service.LivenessProbe),
		cr.Spec.Sidecars,
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
	err := CreateOrUpdateService(cr.Namespace, headlessObjectMetaInfo, redisClusterAsOwner(cr), false, true, "ClusterIP")
	if err != nil {
		logger.Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisClusterAsOwner(cr), enableMetrics, false, "ClusterIP")
	if err != nil {
		logger.Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	additionalServiceType := "ClusterIP"
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceType = cr.Spec.KubernetesConfig.Service.ServiceType
	}
	err = CreateOrUpdateService(cr.Namespace, additionalObjectMetaInfo, redisClusterAsOwner(cr), false, false, additionalServiceType)
	if err != nil {
		logger.Error(err, "Cannot create additional service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}

func CreateRedisClusterSecrets(cr *redisv1beta1.RedisCluster) error {

	var name = *cr.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.Name
	var namespacelist = cr.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.NameSpace
	var key = cr.Spec.KubernetesConfig.ExistOrGenerateSecret.GeneratePasswordSecret.Key
	ownerRef := redisClusterAsOwner(cr)

	genLogger := log.WithValues()

	// If key is empty add the default value
	if key == nil {
		*key = "key"
	}
	genLogger.Info("The key is set to ", "key", *key)

	// If no namespacelist is defined default would be added automatically
	if namespacelist == nil {
		namespacelist = append(namespacelist, "default")
	}
	genLogger.Info("Namespaces passed to generate secrets are", "namespaces", namespacelist)

	rndID, err := uuid.NewRandom()
	if err != nil {
		genLogger.Error(err, "Unable to generate the UUID")
	}
	value, err := rndID.MarshalBinary()
	if err != nil {
		genLogger.Error(err, "Failed to create password")
	}
	genLogger.Info("Secrets would be generated in ", "namespace", namespacelist)

	for _, namespace := range namespacelist {
		err := createSecretIfNotExist(name, namespace, key, value, ownerRef)
		if err != nil {

			return err
		}
	}

	return nil

}
