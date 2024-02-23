package k8sutils

import (
	"context"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// CreateReplicationService method will create replication service for Redis
func CreateReplicationService(cr *redisv1beta2.RedisReplication, cl kubernetes.Interface) error {
	logger := serviceLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getRedisLabels(cr.ObjectMeta.Name, replication, "replication", cr.ObjectMeta.Labels)
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
	additionalServiceAnnotations := map[string]string{}
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceAnnotations = cr.Spec.KubernetesConfig.Service.ServiceAnnotations
	}
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-headless", cr.Namespace, labels, annotations)
	masterObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-leader", cr.Namespace, labels, annotations)
	slaveObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-follower", cr.Namespace, labels, annotations)
	additionalObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, additionalServiceAnnotations, epp))
	err := CreateOrUpdateService(cr.Namespace, headlessObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, true, "ClusterIP", redisPort, cl)
	if err != nil {
		logger.Error(err, "Cannot create replication headless service for Redis")
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisReplicationAsOwner(cr), epp, false, "ClusterIP", redisPort, cl)
	if err != nil {
		logger.Error(err, "Cannot create replication service for Redis")
		return err
	}
	additionalServiceType := "ClusterIP"
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceType = cr.Spec.KubernetesConfig.Service.ServiceType
	}
	err = CreateOrUpdateService(cr.Namespace, additionalObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, additionalServiceType, redisPort, cl)
	if err != nil {
		logger.Error(err, "Cannot create additional service for Redis Replication")
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, masterObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, additionalServiceType, redisPort, cl)
	if err != nil {
		logger.Error(err, "Cannot create additional service for Redis Replication")
	}
	err = CreateOrUpdateService(cr.Namespace, slaveObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, additionalServiceType, redisPort, cl)
	if err != nil {
		logger.Error(err, "Cannot create additional service for Redis Replication")
	}
	return nil
}

// CreateReplicationRedis will create a replication redis setup
func CreateReplicationRedis(cr *redisv1beta2.RedisReplication, cl kubernetes.Interface) error {
	stateFulName := cr.ObjectMeta.Name
	logger := statefulSetLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getRedisLabels(cr.ObjectMeta.Name, replication, "replication", cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(
		cl,
		logger,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisReplicationParams(cr),
		redisReplicationAsOwner(cr),
		generateRedisReplicationInitContainerParams(cr),
		generateRedisReplicationContainerParams(cr),
		cr.Spec.Sidecars,
	)
	if err != nil {
		logger.Error(err, "Cannot create replication statefulset for Redis")
		return err
	}
	return nil
}

func generateRedisReplicationParams(cr *redisv1beta2.RedisReplication) statefulSetParameters {
	replicas := cr.Spec.GetReplicationCounts("Replication")
	res := statefulSetParameters{
		Replicas:                      &replicas,
		ClusterMode:                   false,
		NodeConfVolume:                false,
		NodeSelector:                  cr.Spec.NodeSelector,
		PodSecurityContext:            cr.Spec.PodSecurityContext,
		PriorityClassName:             cr.Spec.PriorityClassName,
		Affinity:                      cr.Spec.Affinity,
		Tolerations:                   cr.Spec.Tolerations,
		TerminationGracePeriodSeconds: cr.Spec.TerminationGracePeriodSeconds,
		UpdateStrategy:                cr.Spec.KubernetesConfig.UpdateStrategy,
		IgnoreAnnotations:             cr.Spec.KubernetesConfig.IgnoreAnnotations,
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
	if _, found := cr.ObjectMeta.GetAnnotations()[AnnotationKeyRecreateStatefulset]; found {
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
		Port:            ptr.To(6379),
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

		if cr.Spec.RedisExporter.Resources != nil {
			containerProp.RedisExporterResources = cr.Spec.RedisExporter.Resources
		}
		if cr.Spec.RedisExporter.EnvVars != nil {
			containerProp.RedisExporterEnv = cr.Spec.RedisExporter.EnvVars
		}
	}
	if cr.Spec.ReadinessProbe != nil {
		containerProp.ReadinessProbe = &cr.Spec.ReadinessProbe.Probe
	}
	if cr.Spec.LivenessProbe != nil {
		containerProp.LivenessProbe = &cr.Spec.LivenessProbe.Probe
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

func IsRedisReplicationReady(ctx context.Context, logger logr.Logger, client kubernetes.Interface, dClient dynamic.Interface, rs *redisv1beta2.RedisSentinel) bool {
	// statefulset name the same as the redis replication name
	sts, err := GetStatefulSet(client, logger, rs.GetNamespace(), rs.Spec.RedisSentinelConfig.RedisReplicationName)
	if err != nil {
		return false
	}
	if sts.Status.ReadyReplicas != *sts.Spec.Replicas {
		return false
	}
	// Enhanced check: When the pod is ready, it may not have been
	// created as part of a replication cluster, so we should verify
	// whether there is an actual master node.
	if master := getRedisReplicationMasterIP(ctx, client, logger, rs, dClient); master == "" {
		return false
	}
	return true
}

func updatePodLabel(ctx context.Context, cl kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication, role string, nodes []string) error {
	for _, node := range nodes {
		pod, err := cl.CoreV1().Pods(cr.Namespace).Get(context.TODO(), node, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "Cannot get redis replication pod")
			return err
		}
		// set Label redis-role
		metav1.SetMetaDataLabel(&pod.ObjectMeta, "redis-role", role)
		// update Label
		_, err = cl.CoreV1().Pods(cr.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "Cannot update redis replication pod")
			return err
		}
	}
	return nil
}

func UpdateRoleLabelPod(ctx context.Context, cl kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisReplication) error {
	// find realMaster, and label this pod: 'redis-role=master'
	role := "master"
	masterPods := GetRedisNodesByRole(ctx, cl, logger, cr, role)
	realMaster := checkAttachedSlave(ctx, cl, logger, cr, masterPods)
	err := updatePodLabel(ctx, cl, logger, cr, role, []string{realMaster})
	if err != nil {
		return err
	}

	// when configuring one size replication
	if cr.Spec.Size == pointer.Int32(1) {
		err = updatePodLabel(ctx, cl, logger, cr, role, masterPods)
		if err != nil {
			return err
		}
	}

	role = "slave"
	err = updatePodLabel(ctx, cl, logger, cr, role, GetRedisNodesByRole(ctx, cl, logger, cr, role))
	if err != nil {
		return err
	}
	return nil
}
