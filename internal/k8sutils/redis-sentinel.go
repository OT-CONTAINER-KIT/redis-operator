package k8sutils

import (
	"context"
	"errors"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RedisSentinelSTS is a interface to call Redis Statefulset function
type RedisSentinelSTS struct {
	RedisStateFulType             string
	ExternalConfig                *string
	Affinity                      *corev1.Affinity `json:"affinity,omitempty"`
	TerminationGracePeriodSeconds *int64           `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	ReadinessProbe                *corev1.Probe
	LivenessProbe                 *corev1.Probe
}

// RedisSentinelService is a interface to call Redis Service function
type RedisSentinelService struct {
	RedisServiceRole string
}

// Redis Sentinel Create the Redis Sentinel Setup
func CreateRedisSentinel(ctx context.Context, client kubernetes.Interface, cr *rsvb2.RedisSentinel, cl kubernetes.Interface, ctrlClient client.Client) error {
	prop := RedisSentinelSTS{
		RedisStateFulType:             "sentinel",
		Affinity:                      cr.Spec.Affinity,
		ReadinessProbe:                cr.Spec.ReadinessProbe,
		LivenessProbe:                 cr.Spec.LivenessProbe,
		TerminationGracePeriodSeconds: cr.Spec.TerminationGracePeriodSeconds,
	}

	if cr.Spec.RedisSentinelConfig != nil && cr.Spec.RedisSentinelConfig.AdditionalSentinelConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisSentinelConfig.AdditionalSentinelConfig
	}

	return prop.CreateRedisSentinelSetup(ctx, client, cr, cl, ctrlClient)
}

// Create RedisSentinel Service
func CreateRedisSentinelService(ctx context.Context, cr *rsvb2.RedisSentinel, cl kubernetes.Interface) error {
	prop := RedisSentinelService{
		RedisServiceRole: "sentinel",
	}
	return prop.CreateRedisSentinelService(ctx, cr, cl)
}

// Create Redis Sentinel Cluster Setup
func (service RedisSentinelSTS) CreateRedisSentinelSetup(ctx context.Context, client kubernetes.Interface, cr *rsvb2.RedisSentinel, cl kubernetes.Interface, ctrlClient client.Client) error {
	stateFulName := cr.Name + "-" + service.RedisStateFulType
	labels := getRedisLabels(stateFulName, sentinel, service.RedisStateFulType, cr.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	containerParams, err := generateRedisSentinelContainerParams(ctx, client, cr, service.ReadinessProbe, service.LivenessProbe, ctrlClient)
	if err != nil {
		return err
	}

	err = CreateOrUpdateStateFul(
		ctx,
		cl,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisSentinelParams(ctx, cr, service.getSentinelCount(cr), service.ExternalConfig, service.Affinity),
		redisSentinelAsOwner(cr),
		generateRedisSentinelInitContainerParams(cr),
		containerParams,
		cr.Spec.Sidecars,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create Sentinel statefulset for Redis")
		return err
	}
	return nil
}

// Create Redis Sentile Params for the statefulset
func generateRedisSentinelParams(ctx context.Context, cr *rsvb2.RedisSentinel, replicas int32, externalConfig *string, affinity *corev1.Affinity) statefulSetParameters {
	var minreadyseconds int32 = 0
	if cr.Spec.KubernetesConfig.MinReadySeconds != nil {
		minreadyseconds = *cr.Spec.KubernetesConfig.MinReadySeconds
	}
	res := statefulSetParameters{
		Replicas:                             &replicas,
		ClusterMode:                          false,
		NodeConfVolume:                       false,
		NodeSelector:                         cr.Spec.NodeSelector,
		PodSecurityContext:                   cr.Spec.PodSecurityContext,
		PriorityClassName:                    cr.Spec.PriorityClassName,
		Affinity:                             affinity,
		TerminationGracePeriodSeconds:        cr.Spec.TerminationGracePeriodSeconds,
		Tolerations:                          cr.Spec.Tolerations,
		TopologySpreadConstraints:            cr.Spec.TopologySpreadConstrains,
		ServiceAccountName:                   cr.Spec.ServiceAccountName,
		UpdateStrategy:                       cr.Spec.KubernetesConfig.UpdateStrategy,
		PersistentVolumeClaimRetentionPolicy: cr.Spec.KubernetesConfig.PersistentVolumeClaimRetentionPolicy,
		IgnoreAnnotations:                    cr.Spec.KubernetesConfig.IgnoreAnnotations,
		MinReadySeconds:                      minreadyseconds,
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
	if value, found := cr.GetAnnotations()[common.AnnotationKeyRecreateStatefulset]; found && value == "true" {
		res.RecreateStatefulSet = true
		res.RecreateStatefulsetStrategy = getDeletionPropagationStrategy(cr.GetAnnotations())
	}
	return res
}

// generateRedisSentinelInitContainerParams generates Redis sentinel initcontainer information
func generateRedisSentinelInitContainerParams(cr *rsvb2.RedisSentinel) initContainerParameters {
	initcontainerProp := initContainerParameters{}

	if cr.Spec.InitContainer != nil {
		initContainer := cr.Spec.InitContainer

		initcontainerProp = initContainerParameters{
			Enabled:               initContainer.Enabled,
			Role:                  "sentinel",
			Image:                 initContainer.Image,
			ImagePullPolicy:       initContainer.ImagePullPolicy,
			Resources:             initContainer.Resources,
			AdditionalEnvVariable: initContainer.EnvVars,
			Command:               initContainer.Command,
			Arguments:             initContainer.Args,
			SecurityContext:       initContainer.SecurityContext,
		}
		if cr.Spec.VolumeMount != nil {
			initcontainerProp.AdditionalVolume = cr.Spec.VolumeMount.Volume
			initcontainerProp.AdditionalMountPath = cr.Spec.VolumeMount.MountPath
		}
	}
	return initcontainerProp
}

// Create Redis Sentinel Statefulset Container Params
func generateRedisSentinelContainerParams(ctx context.Context, client kubernetes.Interface, cr *rsvb2.RedisSentinel, readinessProbeDef *corev1.Probe, livenessProbeDef *corev1.Probe, ctrlClient client.Client) (containerParameters, error) {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:                  "sentinel",
		Image:                 cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy:       cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:             cr.Spec.KubernetesConfig.Resources,
		SecurityContext:       cr.Spec.SecurityContext,
		Port:                  ptr.To(common.SentinelPort),
		HostPort:              cr.Spec.HostPort,
		AdditionalEnvVariable: getSentinelEnvVariable(cr),
	}
	if cr.Spec.EnvVars != nil {
		containerProp.EnvVars = cr.Spec.EnvVars
	}
	if cr.Spec.VolumeMount != nil {
		containerProp.AdditionalVolume = cr.Spec.VolumeMount.Volume
		containerProp.AdditionalMountPath = cr.Spec.VolumeMount.MountPath
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
		containerProp.RedisExporterSecurityContext = cr.Spec.RedisExporter.SecurityContext

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

	return containerProp, nil
}

// Get the Count of the Sentinel
func (service RedisSentinelSTS) getSentinelCount(cr *rsvb2.RedisSentinel) int32 {
	return cr.Spec.GetSentinelCounts(service.RedisStateFulType)
}

// Create the Service for redis sentinel
func (service RedisSentinelService) CreateRedisSentinelService(ctx context.Context, cr *rsvb2.RedisSentinel, cl kubernetes.Interface) error {
	serviceName := cr.Name + "-" + service.RedisServiceRole
	labels := getRedisLabels(serviceName, sentinel, service.RedisServiceRole, cr.Labels)
	var epp exporterPortProvider
	if cr.Spec.RedisExporter != nil {
		epp = func() (port int, enable bool) {
			defaultP := ptr.To(common.RedisExporterPort)
			return *util.Coalesce(cr.Spec.RedisExporter.Port, defaultP), cr.Spec.RedisExporter.Enabled
		}
	} else {
		epp = disableMetrics
	}
	annotations := generateServiceAnots(cr.ObjectMeta, nil, epp)
	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, annotations)
	additionalObjectMetaInfo := generateObjectMetaInformation(serviceName+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetServiceAnnotations(), epp))

	err := CreateOrUpdateService(ctx, cr.Namespace, headlessObjectMetaInfo, redisSentinelAsOwner(cr), disableMetrics, true, "ClusterIP", common.SentinelPort, cl)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	err = CreateOrUpdateService(ctx, cr.Namespace, objectMetaInfo, redisSentinelAsOwner(cr), epp, false, "ClusterIP", common.SentinelPort, cl)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}

	err = CreateOrUpdateService(
		ctx,
		cr.Namespace,
		additionalObjectMetaInfo,
		redisSentinelAsOwner(cr),
		disableMetrics,
		false,
		cr.Spec.KubernetesConfig.GetServiceType(),
		common.SentinelPort,
		cl,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create additional service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}

func getSentinelEnvVariable(cr *rsvb2.RedisSentinel) *[]corev1.EnvVar {
	if cr.Spec.RedisSentinelConfig == nil {
		return &[]corev1.EnvVar{}
	}

	envVar := &[]corev1.EnvVar{
		{
			Name:  "MASTER_GROUP_NAME",
			Value: cr.Spec.RedisSentinelConfig.MasterGroupName,
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
		{
			Name:  "RESOLVE_HOSTNAMES",
			Value: cr.Spec.RedisSentinelConfig.ResolveHostnames,
		},
		{
			Name:  "ANNOUNCE_HOSTNAMES",
			Value: cr.Spec.RedisSentinelConfig.AnnounceHostnames,
		},
	}

	if cr.Spec.RedisSentinelConfig != nil && cr.Spec.RedisSentinelConfig.RedisReplicationPassword != nil {
		*envVar = append(*envVar, corev1.EnvVar{
			Name:      "MASTER_PASSWORD",
			ValueFrom: cr.Spec.RedisSentinelConfig.RedisReplicationPassword,
		})
	}
	return envVar
}

func getRedisReplicationMasterPod(ctx context.Context, client kubernetes.Interface, cr *rsvb2.RedisSentinel, ctrlClient client.Client) RedisDetails {
	replicationName := cr.Spec.RedisSentinelConfig.RedisReplicationName
	replicationNamespace := cr.Namespace

	var replicationInstance rrvb2.RedisReplication
	var realMasterPod string

	emptyRedisInfo := RedisDetails{
		PodName:   "",
		Namespace: "",
	}

	// Get RedisReplication using controller-runtime client
	err := ctrlClient.Get(ctx, types.NamespacedName{
		Namespace: replicationNamespace,
		Name:      replicationName,
	}, &replicationInstance)

	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get RedisReplication", "replication name", replicationName, "namespace", replicationNamespace)
		return emptyRedisInfo
	} else {
		log.FromContext(ctx).V(1).Info("Successfully got RedisReplication", "replication name", replicationName, "namespace", replicationNamespace)
	}

	masterPods, err := GetRedisNodesByRole(ctx, client, &replicationInstance, "master")
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get RedisReplication master pods", "replication name", replicationName, "namespace", replicationNamespace)
		return emptyRedisInfo
	}
	if len(masterPods) == 0 {
		log.FromContext(ctx).Error(errors.New("no master pods found"), "")
		return emptyRedisInfo
	}
	for _, podName := range masterPods {
		redisClient := configureRedisReplicationClient(ctx, client, &replicationInstance, podName)
		defer redisClient.Close()

		if checkAttachedSlave(ctx, redisClient, podName) > 0 {
			realMasterPod = podName
			break
		}
	}
	if realMasterPod == "" {
		log.FromContext(ctx).Error(errors.New("no real master pod found"), "")
		return emptyRedisInfo
	}

	return RedisDetails{
		PodName:   realMasterPod,
		Namespace: replicationNamespace,
	}
}

func getRedisReplicationMasterIP(ctx context.Context, client kubernetes.Interface, cr *rsvb2.RedisSentinel, ctrlClient client.Client) string {
	RedisDetails := getRedisReplicationMasterPod(ctx, client, cr, ctrlClient)
	if RedisDetails.PodName == "" || RedisDetails.Namespace == "" {
		return ""
	} else {
		return getRedisServerIP(ctx, client, RedisDetails)
	}
}
