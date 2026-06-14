package k8sutils

import (
	"context"
	"fmt"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func CreateReplicationSentinel(ctx context.Context, cr *rrvb2.RedisReplication, cl kubernetes.Interface) error {
	stateFulName := cr.SentinelStatefulSet()
	labels := getRedisLabels(stateFulName, sentinel, "sentinel", cr.GetLabels())
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)

	err := CreateOrUpdateStateFul(
		ctx,
		cl,
		cr.GetNamespace(),
		objectMetaInfo,
		generateReplicationSentinelParams(cr),
		redisReplicationAsOwner(cr),
		generateReplicationSentinelInitContainerParams(cr),
		generateReplicationSentinelContainerParams(cr),
		nil,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create sentinel statefulset for RedisReplication")
		return err
	}
	return nil
}

func generateReplicationSentinelParams(cr *rrvb2.RedisReplication) statefulSetParameters {
	replicas := cr.Spec.Sentinel.Size
	res := statefulSetParameters{
		Replicas:                             &replicas,
		ClusterMode:                          false,
		NodeConfVolume:                       false,
		ServiceAccountName:                   cr.Spec.ServiceAccountName,
		UpdateStrategy:                       cr.Spec.Sentinel.UpdateStrategy,
		PersistentVolumeClaimRetentionPolicy: cr.Spec.Sentinel.PersistentVolumeClaimRetentionPolicy,
		IgnoreAnnotations:                    cr.Spec.Sentinel.IgnoreAnnotations,
		MinReadySeconds:                      ptr.Deref(cr.Spec.Sentinel.MinReadySeconds, 0),
		RecreateStatefulSet:                  true,
		ServiceName:                          cr.SentinelHLService(),
	}
	if cr.Spec.Sentinel.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.Sentinel.ImagePullSecrets
	}
	if cr.Spec.Sentinel.AdditionalSentinelConfig != nil {
		res.ExternalConfig = cr.Spec.Sentinel.AdditionalSentinelConfig
	}
	// Pod placement settings for the Sentinel StatefulSet. A Sentinel-specific
	// ServiceAccountName takes precedence over the replication-level one.
	if cr.Spec.Sentinel.ServiceAccountName != nil {
		res.ServiceAccountName = cr.Spec.Sentinel.ServiceAccountName
	}
	if cr.Spec.Sentinel.Affinity != nil {
		res.Affinity = cr.Spec.Sentinel.Affinity
	}
	if cr.Spec.Sentinel.Tolerations != nil {
		res.Tolerations = cr.Spec.Sentinel.Tolerations
	}
	if cr.Spec.Sentinel.NodeSelector != nil {
		res.NodeSelector = cr.Spec.Sentinel.NodeSelector
	}
	if len(cr.Spec.Sentinel.TopologySpreadConstraints) > 0 {
		res.TopologySpreadConstraints = cr.Spec.Sentinel.TopologySpreadConstraints
	}
	if cr.Spec.Sentinel.PodSecurityContext != nil {
		res.PodSecurityContext = cr.Spec.Sentinel.PodSecurityContext
	}
	if cr.Spec.Sentinel.PriorityClassName != "" {
		res.PriorityClassName = cr.Spec.Sentinel.PriorityClassName
	}
	if cr.Spec.Sentinel.TerminationGracePeriodSeconds != nil {
		res.TerminationGracePeriodSeconds = cr.Spec.Sentinel.TerminationGracePeriodSeconds
	}
	return res
}

func generateReplicationSentinelInitContainerParams(cr *rrvb2.RedisReplication) initContainerParameters {
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
		}
	}
	return initcontainerProp
}

func generateReplicationSentinelContainerParams(cr *rrvb2.RedisReplication) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:                  "sentinel",
		Image:                 cr.Spec.Sentinel.Image,
		ImagePullPolicy:       cr.Spec.Sentinel.ImagePullPolicy,
		Resources:             cr.Spec.Sentinel.Resources,
		Port:                  ptr.To(common.SentinelPort),
		AdditionalEnvVariable: getReplicationSentinelEnvVariable(cr),
	}
	if cr.Spec.Sentinel.SecurityContext != nil {
		containerProp.SecurityContext = cr.Spec.Sentinel.SecurityContext
	}
	if cr.Spec.Sentinel.ExistingPasswordSecret != nil {
		containerProp.EnabledPassword = &trueProperty
		containerProp.SecretName = cr.Spec.Sentinel.ExistingPasswordSecret.Name
		containerProp.SecretKey = cr.Spec.Sentinel.ExistingPasswordSecret.Key
	} else {
		containerProp.EnabledPassword = &falseProperty
	}
	if cr.Spec.TLS != nil {
		containerProp.TLSConfig = cr.Spec.TLS
	}
	return containerProp
}

func getReplicationSentinelEnvVariable(cr *rrvb2.RedisReplication) *[]corev1.EnvVar {
	envVar := &[]corev1.EnvVar{
		{Name: "MASTER_GROUP_NAME", Value: "mymaster"},
		{Name: "QUORUM", Value: fmt.Sprintf("%d", cr.Spec.Sentinel.Size/2+1)},
		{Name: "DOWN_AFTER_MILLISECONDS", Value: cr.Spec.Sentinel.DownAfterMilliseconds},
		{Name: "PARALLEL_SYNCS", Value: cr.Spec.Sentinel.ParallelSyncs},
		{Name: "FAILOVER_TIMEOUT", Value: cr.Spec.Sentinel.FailoverTimeout},
		{Name: "RESOLVE_HOSTNAMES", Value: cr.Spec.Sentinel.ResolveHostnames},
		{Name: "ANNOUNCE_HOSTNAMES", Value: cr.Spec.Sentinel.AnnounceHostnames},
	}
	// MASTER_PASSWORD lets Sentinel authenticate to the monitored master. A
	// Sentinel-specific redisSecret takes precedence over the replication-level
	// one, falling back to the latter when unset.
	passwordSecret := cr.Spec.KubernetesConfig.ExistingPasswordSecret
	if cr.Spec.Sentinel.ExistingPasswordSecret != nil {
		passwordSecret = cr.Spec.Sentinel.ExistingPasswordSecret
	}
	if passwordSecret != nil {
		*envVar = append(*envVar, corev1.EnvVar{
			Name: "MASTER_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *passwordSecret.Name,
					},
					Key: *passwordSecret.Key,
				},
			},
		})
	}
	return envVar
}
