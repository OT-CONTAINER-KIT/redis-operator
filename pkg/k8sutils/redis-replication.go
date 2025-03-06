package k8sutils

import (
	"context"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateReplicationService method will create replication service for Redis
func CreateReplicationService(ctx context.Context, cr *redisv1beta2.RedisReplication, cl kubernetes.Interface) error {
	labels := getRedisLabels(cr.ObjectMeta.Name, replication, "replication", cr.ObjectMeta.Labels)

	epp := disableMetrics
	if cr.Spec.RedisExporter != nil {
		epp = func() (port int, enable bool) {
			defaultP := ptr.To(redisExporterPort)
			return *util.Coalesce(cr.Spec.RedisExporter.Port, defaultP), cr.Spec.RedisExporter.Enabled
		}
	}

	annotations := generateServiceAnots(cr.ObjectMeta, nil, epp)
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-headless", cr.Namespace, labels, annotations)
	additionalObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetServiceAnnotations(), epp))
	masterLabels := util.MergeMap(
		labels, map[string]string{RedisRoleLabelKey: RedisRoleLabelMaster},
	)
	replicaLabels := util.MergeMap(
		labels, map[string]string{RedisRoleLabelKey: RedisRoleLabelSlave},
	)
	masterObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-master", cr.Namespace, masterLabels, annotations)
	replicaObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-replica", cr.Namespace, replicaLabels, annotations)

	if err := CreateOrUpdateService(ctx, cr.Namespace, headlessObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, true, "ClusterIP", redisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replication headless service for Redis")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, objectMetaInfo, redisReplicationAsOwner(cr), epp, false, "ClusterIP", redisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replication service for Redis")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, additionalObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, cr.Spec.KubernetesConfig.GetServiceType(), redisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create additional service for Redis Replication")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, masterObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, "ClusterIP", redisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create master service for Redis")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, replicaObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, "ClusterIP", redisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replica service for Redis")
		return err
	}

	return nil
}

// CreateReplicationRedis will create a replication redis setup
func CreateReplicationRedis(ctx context.Context, cr *redisv1beta2.RedisReplication, cl kubernetes.Interface) error {
	stateFulName := cr.ObjectMeta.Name
	labels := getRedisLabels(cr.ObjectMeta.Name, replication, "replication", cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)

	err := CreateOrUpdateStateFul(
		ctx,
		cl,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisReplicationParams(cr),
		redisReplicationAsOwner(cr),
		generateRedisReplicationInitContainerParams(cr),
		generateRedisReplicationContainerParams(cr),
		cr.Spec.Sidecars,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replication statefulset for Redis")
		return err
	}
	return nil
}

func generateRedisReplicationParams(cr *redisv1beta2.RedisReplication) statefulSetParameters {
	replicas := cr.Spec.GetReplicationCounts("Replication")
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
		Affinity:                      cr.Spec.Affinity,
		Tolerations:                   cr.Spec.Tolerations,
		TopologySpreadConstraints:     cr.Spec.TopologySpreadConstrains,
		TerminationGracePeriodSeconds: cr.Spec.TerminationGracePeriodSeconds,
		UpdateStrategy:                cr.Spec.KubernetesConfig.UpdateStrategy,
		IgnoreAnnotations:             cr.Spec.KubernetesConfig.IgnoreAnnotations,
		MinReadySeconds:               minreadyseconds,
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
	if cr.Spec.ServiceAccountName != nil {
		res.ServiceAccountName = cr.Spec.ServiceAccountName
	}
	if value, found := cr.ObjectMeta.GetAnnotations()[AnnotationKeyRecreateStatefulset]; found && value == "true" {
		res.RecreateStatefulSet = true
	}
	return res
}

// generateRedisReplicationContainerParams generates Redis container information
func generateRedisReplicationContainerParams(cr *redisv1beta2.RedisReplication) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "replication",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resources,
		SecurityContext: cr.Spec.SecurityContext,
		Port:            ptr.To(redisPort),
		HostPort:        cr.Spec.HostPort,
	}
	if cr.Spec.EnvVars != nil {
		containerProp.EnvVars = cr.Spec.EnvVars
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
		containerProp.RedisExporterSecurityContext = cr.Spec.RedisExporter.SecurityContext

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
	if cr.Spec.TLS != nil {
		containerProp.TLSConfig = cr.Spec.TLS
	}
	if cr.Spec.ACL != nil {
		containerProp.ACLConfig = cr.Spec.ACL
	}
	return containerProp
}

// generateRedisReplicationInitContainerParams generates Redis Replication initcontainer information
func generateRedisReplicationInitContainerParams(cr *redisv1beta2.RedisReplication) initContainerParameters {
	trueProperty := true
	initcontainerProp := initContainerParameters{}

	if cr.Spec.InitContainer != nil {
		initContainer := cr.Spec.InitContainer

		initcontainerProp = initContainerParameters{
			Enabled:               initContainer.Enabled,
			Role:                  "replication",
			Image:                 initContainer.Image,
			ImagePullPolicy:       initContainer.ImagePullPolicy,
			Resources:             initContainer.Resources,
			AdditionalEnvVariable: initContainer.EnvVars,
			Command:               initContainer.Command,
			Arguments:             initContainer.Args,
			SecurityContext:       initContainer.SecurityContext,
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

func IsRedisReplicationReady(ctx context.Context, client kubernetes.Interface, dClient dynamic.Interface, rs *redisv1beta2.RedisSentinel) bool {
	// statefulset name the same as the redis replication name
	sts, err := GetStatefulSet(ctx, client, rs.GetNamespace(), rs.Spec.RedisSentinelConfig.RedisReplicationName)
	if err != nil {
		return false
	}
	if sts.Status.ReadyReplicas != *sts.Spec.Replicas {
		return false
	}
	if sts.Status.ObservedGeneration != sts.Generation {
		return false
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return false
	}
	// Enhanced check: When the pod is ready, it may not have been
	// created as part of a replication cluster, so we should verify
	// whether there is an actual master node.
	if master := getRedisReplicationMasterIP(ctx, client, rs, dClient); master == "" {
		return false
	}
	return true
}
