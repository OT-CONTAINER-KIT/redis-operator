package k8sutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
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

type RedisReplicationObject struct {
	RedisReplication *redisv1beta2.RedisReplication
}

// Redis Sentinel Create the Redis Sentinel Setup
func CreateRedisSentinel(ctx context.Context, client kubernetes.Interface, cr *redisv1beta2.RedisSentinel, cl kubernetes.Interface, dcl dynamic.Interface) error {
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

	return prop.CreateRedisSentinelSetup(ctx, client, cr, cl, dcl)
}

// Create RedisSentinel Service
func CreateRedisSentinelService(ctx context.Context, cr *redisv1beta2.RedisSentinel, cl kubernetes.Interface) error {
	prop := RedisSentinelService{
		RedisServiceRole: "sentinel",
	}
	return prop.CreateRedisSentinelService(ctx, cr, cl)
}

// Create Redis Sentinel Cluster Setup
func (service RedisSentinelSTS) CreateRedisSentinelSetup(ctx context.Context, client kubernetes.Interface, cr *redisv1beta2.RedisSentinel, cl kubernetes.Interface, dcl dynamic.Interface) error {
	stateFulName := cr.ObjectMeta.Name + "-" + service.RedisStateFulType
	labels := getRedisLabels(stateFulName, sentinel, service.RedisStateFulType, cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(
		ctx,
		cl,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisSentinelParams(ctx, cr, service.getSentinelCount(cr), service.ExternalConfig, service.Affinity),
		redisSentinelAsOwner(cr),
		generateRedisSentinelInitContainerParams(cr),
		generateRedisSentinelContainerParams(ctx, client, cr, service.ReadinessProbe, service.LivenessProbe, dcl),
		cr.Spec.Sidecars,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create Sentinel statefulset for Redis")
		return err
	}
	return nil
}

// Create Redis Sentile Params for the statefulset
func generateRedisSentinelParams(ctx context.Context, cr *redisv1beta2.RedisSentinel, replicas int32, externalConfig *string, affinity *corev1.Affinity) statefulSetParameters {
	var minreadyseconds int32 = 0
	if cr.Spec.KubernetesConfig.MinReadySeconds != nil {
		minreadyseconds = *cr.Spec.KubernetesConfig.MinReadySeconds
	}
	res := statefulSetParameters{
		Replicas:                      &replicas,
		ClusterMode:                   false,
		NodeConfVolume:                false,
		NodeSelector:                  cr.Spec.NodeSelector,
		PodSecurityContext:            cr.Spec.PodSecurityContext,
		PriorityClassName:             cr.Spec.PriorityClassName,
		Affinity:                      affinity,
		TerminationGracePeriodSeconds: cr.Spec.TerminationGracePeriodSeconds,
		Tolerations:                   cr.Spec.Tolerations,
		TopologySpreadConstraints:     cr.Spec.TopologySpreadConstrains,
		ServiceAccountName:            cr.Spec.ServiceAccountName,
		UpdateStrategy:                cr.Spec.KubernetesConfig.UpdateStrategy,
		IgnoreAnnotations:             cr.Spec.KubernetesConfig.IgnoreAnnotations,
		MinReadySeconds:               minreadyseconds,
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
	if value, found := cr.ObjectMeta.GetAnnotations()[AnnotationKeyRecreateStatefulset]; found && value == "true" {
		res.RecreateStatefulSet = true
	}
	return res
}

// generateRedisSentinelInitContainerParams generates Redis sentinel initcontainer information
func generateRedisSentinelInitContainerParams(cr *redisv1beta2.RedisSentinel) initContainerParameters {
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
func generateRedisSentinelContainerParams(ctx context.Context, client kubernetes.Interface, cr *redisv1beta2.RedisSentinel, readinessProbeDef *corev1.Probe, livenessProbeDef *corev1.Probe, dcl dynamic.Interface) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:                  "sentinel",
		Image:                 cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy:       cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:             cr.Spec.KubernetesConfig.Resources,
		SecurityContext:       cr.Spec.SecurityContext,
		Port:                  ptr.To(sentinelPort),
		HostPort:              cr.Spec.HostPort,
		AdditionalEnvVariable: getSentinelEnvVariable(ctx, client, cr, dcl),
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

	return containerProp
}

// Get the Count of the Sentinel
func (service RedisSentinelSTS) getSentinelCount(cr *redisv1beta2.RedisSentinel) int32 {
	return cr.Spec.GetSentinelCounts(service.RedisStateFulType)
}

// Create the Service for redis sentinel
func (service RedisSentinelService) CreateRedisSentinelService(ctx context.Context, cr *redisv1beta2.RedisSentinel, cl kubernetes.Interface) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole
	labels := getRedisLabels(serviceName, sentinel, service.RedisServiceRole, cr.ObjectMeta.Labels)
	var epp exporterPortProvider
	if cr.Spec.RedisExporter != nil {
		epp = func() (port int, enable bool) {
			defaultP := ptr.To(redisExporterPort)
			return *util.Coalesce(cr.Spec.RedisExporter.Port, defaultP), cr.Spec.RedisExporter.Enabled
		}
	} else {
		epp = disableMetrics
	}
	annotations := generateServiceAnots(cr.ObjectMeta, nil, epp)
	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, annotations)
	additionalObjectMetaInfo := generateObjectMetaInformation(serviceName+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetServiceAnnotations(), epp))

	err := CreateOrUpdateService(ctx, cr.Namespace, headlessObjectMetaInfo, redisSentinelAsOwner(cr), disableMetrics, true, "ClusterIP", sentinelPort, cl)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	err = CreateOrUpdateService(ctx, cr.Namespace, objectMetaInfo, redisSentinelAsOwner(cr), epp, false, "ClusterIP", sentinelPort, cl)
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
		sentinelPort,
		cl,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create additional service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}

func getSentinelEnvVariable(ctx context.Context, client kubernetes.Interface, cr *redisv1beta2.RedisSentinel, dcl dynamic.Interface) *[]corev1.EnvVar {
	if cr.Spec.RedisSentinelConfig == nil {
		return &[]corev1.EnvVar{}
	}

	var IP string
	if cr.Spec.RedisSentinelConfig.ResolveHostnames == "yes" {
		IP = getRedisReplicationMasterName(ctx, client, cr, dcl)
	} else {
		IP = getRedisReplicationMasterIP(ctx, client, cr, dcl)
	}

	envVar := &[]corev1.EnvVar{
		{
			Name:  "MASTER_GROUP_NAME",
			Value: cr.Spec.RedisSentinelConfig.MasterGroupName,
		},
		{
			Name:  "IP",
			Value: IP,
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

func getRedisReplicationMasterPod(ctx context.Context, client kubernetes.Interface, cr *redisv1beta2.RedisSentinel, dcl dynamic.Interface) RedisDetails {
	replicationName := cr.Spec.RedisSentinelConfig.RedisReplicationName
	replicationNamespace := cr.Namespace

	var replicationInstance redisv1beta2.RedisReplication
	var realMasterPod string

	emptyRedisInfo := RedisDetails{
		PodName:   "",
		Namespace: "",
	}

	// Get Request on Dynamic Client
	customObject, err := dcl.Resource(schema.GroupVersionResource{
		Group:    "redis.redis.opstreelabs.in",
		Version:  "v1beta2",
		Resource: "redisreplications",
	}).Namespace(replicationNamespace).Get(context.TODO(), replicationName, v1.GetOptions{})

	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to Execute Get Request", "replication name", replicationName, "namespace", replicationNamespace)
		return emptyRedisInfo
	} else {
		log.FromContext(ctx).V(1).Info("Successfully Execute the Get Request", "replication name", replicationName, "namespace", replicationNamespace)
	}

	// Marshal CustomObject to JSON
	replicationJSON, err := customObject.MarshalJSON()
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed To Load JSON")
		return emptyRedisInfo
	}

	// Unmarshal The JSON on Object
	if err := json.Unmarshal(replicationJSON, &replicationInstance); err != nil {
		log.FromContext(ctx).Error(err, "Failed To Unmarshal JSON over the Object")
		return emptyRedisInfo
	}

	masterPods := GetRedisNodesByRole(ctx, client, &replicationInstance, "master")
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

func getRedisReplicationMasterIP(ctx context.Context, client kubernetes.Interface, cr *redisv1beta2.RedisSentinel, dcl dynamic.Interface) string {
	RedisDetails := getRedisReplicationMasterPod(ctx, client, cr, dcl)
	if RedisDetails.PodName == "" || RedisDetails.Namespace == "" {
		return ""
	} else {
		return getRedisServerIP(ctx, client, RedisDetails)
	}
}

func getRedisReplicationMasterName(ctx context.Context, client kubernetes.Interface, cr *redisv1beta2.RedisSentinel, dcl dynamic.Interface) string {
	RedisDetails := getRedisReplicationMasterPod(ctx, client, cr, dcl)
	if RedisDetails.PodName == "" || RedisDetails.Namespace == "" {
		return ""
	} else {
		return fmt.Sprintf("%s.%s-headless.%s.svc", RedisDetails.PodName, cr.Spec.RedisSentinelConfig.RedisReplicationName, RedisDetails.Namespace)
	}
}
