package k8sutils

import (
	redisv1beta1 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
)

// RedisClusterSTS is a interface to call Redis Statefulset function
type RedisClusterSTS struct {
	RedisStateFulType             string
	ExternalConfig                *string
	Affinity                      *corev1.Affinity `json:"affinity,omitempty"`
	TerminationGracePeriodSeconds *int64           `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	ReadinessProbe                *redisv1beta1.Probe
	LivenessProbe                 *redisv1beta1.Probe
	NodeSelector                  map[string]string
	Tolerations                   *[]corev1.Toleration
}

// RedisClusterService is a interface to call Redis Service function
type RedisClusterService struct {
	RedisServiceRole string
}

// generateRedisClusterParams generates Redis cluster information
func generateRedisClusterParams(cr *redisv1beta1.RedisCluster, replicas int32, externalConfig *string, params RedisClusterSTS) statefulSetParameters {
	res := statefulSetParameters{
		Metadata:                      cr.ObjectMeta,
		Replicas:                      &replicas,
		NodeSelector:                  params.NodeSelector,
		SecurityContext:               cr.Spec.SecurityContext,
		PriorityClassName:             cr.Spec.PriorityClassName,
		Affinity:                      params.Affinity,
		TerminationGracePeriodSeconds: params.TerminationGracePeriodSeconds,
		Tolerations:                   params.Tolerations,
		ServiceAccountName:            cr.Spec.ServiceAccountName,
		UpdateStrategy:                cr.Spec.KubernetesConfig.UpdateStrategy,
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
	if _, found := cr.ObjectMeta.GetAnnotations()["redis.opstreelabs.in/recreate-statefulset"]; found {
		res.RecreateStatefulSet = true
	}
	return res
}

func generateRedisClusterInitContainerParams(cr *redisv1beta1.RedisCluster) initContainerParameters {
	trueProperty := true
	initcontainerProp := initContainerParameters{}

	if cr.Spec.InitContainer != nil {
		initContainer := cr.Spec.InitContainer

		initcontainerProp = initContainerParameters{
			Enabled:               initContainer.Enabled,
			Role:                  "cluster",
			Image:                 initContainer.Image,
			ImagePullPolicy:       initContainer.ImagePullPolicy,
			Resources:             initContainer.Resources,
			AdditionalEnvVariable: initContainer.EnvVars,
			Command:               initContainer.Command,
			Arguments:             initContainer.Args,
		}

		if cr.Spec.Storage != nil {
			initcontainerProp.AdditionalVolume = cr.Spec.Storage.VolumeMount.Volume
			initcontainerProp.AdditionalMountPath = cr.Spec.Storage.VolumeMount.MountPath
		}
		if cr.Spec.Storage != nil {
			initcontainerProp.PersistenceEnabled = &trueProperty
		}

	}

	return initcontainerProp
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
	if cr.Spec.Storage != nil && cr.Spec.PersistenceEnabled != nil && *cr.Spec.PersistenceEnabled {
		containerProp.PersistenceEnabled = &trueProperty
	} else {
		containerProp.PersistenceEnabled = &falseProperty
	}
	if cr.Spec.TLS != nil {
		containerProp.TLSConfig = cr.Spec.TLS
	}
	if cr.Spec.ACL != nil {
		containerProp.ACLConfig = cr.Spec.ACL
	}

	return containerProp
}

// CreateRedisLeader will create a leader redis setup
func CreateRedisLeader(cr *redisv1beta1.RedisCluster) error {
	prop := RedisClusterSTS{
		RedisStateFulType:             "leader",
		Affinity:                      cr.Spec.RedisLeader.Affinity,
		TerminationGracePeriodSeconds: cr.Spec.RedisLeader.TerminationGracePeriodSeconds,
		NodeSelector:                  cr.Spec.RedisLeader.NodeSelector,
		Tolerations:                   cr.Spec.RedisLeader.Tolerations,
		ReadinessProbe:                cr.Spec.RedisLeader.ReadinessProbe,
		LivenessProbe:                 cr.Spec.RedisLeader.LivenessProbe,
	}
	if cr.Spec.RedisLeader.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisClusterSetup(cr)
}

// CreateRedisFollower will create a follower redis setup
func CreateRedisFollower(cr *redisv1beta1.RedisCluster) error {
	prop := RedisClusterSTS{
		RedisStateFulType:             "follower",
		Affinity:                      cr.Spec.RedisFollower.Affinity,
		TerminationGracePeriodSeconds: cr.Spec.RedisFollower.TerminationGracePeriodSeconds,
		NodeSelector:                  cr.Spec.RedisFollower.NodeSelector,
		Tolerations:                   cr.Spec.RedisFollower.Tolerations,
		ReadinessProbe:                cr.Spec.RedisFollower.ReadinessProbe,
		LivenessProbe:                 cr.Spec.RedisFollower.LivenessProbe,
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

func CreateRedisNetworkPolicy(cr *redisv1beta1.RedisCluster) error {
	networkPolicyMeta := generateObjectMetaInformation(cr.ObjectMeta.Name+"-nodes", cr.ObjectMeta.Namespace, cr.ObjectMeta.Labels, cr.ObjectMeta.Annotations)
	return CreateOrUpdateNetworkPolicy(cr.Namespace, networkPolicyMeta, redisClusterAsOwner(cr), cr.Spec.NetworkPolicy)
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
		generateRedisClusterParams(cr, service.getReplicaCount(cr), service.ExternalConfig, service),
		redisClusterAsOwner(cr),
		generateRedisClusterInitContainerParams(cr),
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
	} else {
		enableMetrics = false
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
