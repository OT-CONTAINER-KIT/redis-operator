package k8sutils

import (
	"context"
	"encoding/json"

	redisv1beta1 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type RedisReplicationObject struct {
	RedisReplication *redisv1beta1.RedisReplication
}

// Redis Sentinel Create the Redis Sentinel Setup
func CreateRedisSentinel(cr *redisv1beta1.RedisSentinel) error {
	prop := RedisSentinelSTS{
		RedisStateFulType: "sentinel",
		Affinity:          cr.Spec.Affinity,
		ReadinessProbe:    cr.Spec.ReadinessProbe,
		LivenessProbe:     cr.Spec.LivenessProbe,
	}

	if cr.Spec.RedisSentinelConfig.AdditionalSentinelConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisSentinelConfig.AdditionalSentinelConfig
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

	stateFulName := cr.ObjectMeta.Name
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

	if cr.Spec.KubernetesConfig.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.KubernetesConfig.ImagePullSecrets
	}
	if externalConfig != nil {
		res.ExternalConfig = externalConfig
	}
	if cr.Spec.RedisExporter != nil {
		res.EnableMetrics = cr.Spec.RedisExporter.Enabled

	}
	return res
}

// Create Redis Sentinel Statefulset Container Params
func generateRedisSentinelContainerParams(cr *redisv1beta1.RedisSentinel, readinessProbeDef *redisv1beta1.Probe, livenessProbeDef *redisv1beta1.Probe) containerParameters {

	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:                  "sentinel",
		Image:                 cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy:       cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:             cr.Spec.KubernetesConfig.Resources,
		AdditionalEnvVariable: getSentinelEnvVariable(cr),
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

func getSentinelEnvVariable(cr *redisv1beta1.RedisSentinel) *[]corev1.EnvVar {

	envVar := &[]corev1.EnvVar{
		{
			Name:  "MASTER_GROUP_NAME",
			Value: cr.Spec.RedisSentinelConfig.MasterGroupName,
		},
		{
			Name:  "IP",
			Value: getRedisReplicationMasterIP(cr),
		},
		{
			Name:  "PORT",
			Value: cr.Spec.RedisSentinelConfig.RedisPort,
		},
		{
			Name:  "QUORUM",
			Value: cr.Spec.RedisSentinelConfig.Quorum,
		},
		{
			Name:  "DOWN_AFTER_MILLISECONDS",
			Value: cr.Spec.RedisSentinelConfig.DownAfterMilliseconds,
		},
		{
			Name:  "PARALLEL_SYNCS",
			Value: cr.Spec.RedisSentinelConfig.ParallelSyncs,
		},
		{
			Name:  "FAILOVER_TIMEOUT",
			Value: cr.Spec.RedisSentinelConfig.FailoverTimeout,
		},
	}

	return envVar

}

func getRedisReplicationMasterIP(cr *redisv1beta1.RedisSentinel) string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)

	replicationName := cr.Spec.RedisSentinelConfig.RedisReplicationName
	replicationNamespace := cr.Namespace

	var replicationInstance redisv1beta1.RedisReplication

	// Get Request on Dynamic Client
	customObject, err := generateK8sDynamicClient().Resource(schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta1",
		Resource: "redisreplications",
	}).Namespace(replicationNamespace).Get(context.TODO(), replicationName, v1.GetOptions{})

	if err != nil {
		logger.Error(err, "Failed to Execute Get Request", "replication name", replicationName, "namespace", replicationNamespace)
		return ""
	} else {
		logger.Info("Successfully Execute the Get Request", "replication name", replicationName, "namespace", replicationNamespace)
	}

	// Marshal CustomObject to JSON
	replicationJSON, err := customObject.MarshalJSON()
	if err != nil {
		logger.Error(err, "Failed To Load JSON")
		return ""
	}

	// Unmarshal The JSON on Object
	if err := json.Unmarshal(replicationJSON, &replicationInstance); err != nil {
		logger.Error(err, "Failed To Unmarshal JSON over the Object")
		return ""
	}

	masterPods := GetRedisNodesByRole(&replicationInstance, "master")
	realMasterPod := checkAttachedSlave(&replicationInstance, masterPods)
	realMasterInfo := RedisDetails{
		PodName:   realMasterPod,
		Namespace: replicationNamespace,
	}

	realMasterPodIP := getRedisServerIP(realMasterInfo)
	return realMasterPodIP

}
