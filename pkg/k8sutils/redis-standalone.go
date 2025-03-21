package k8sutils

import (
	"context"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateStandaloneService method will create standalone service for Redis
func CreateStandaloneService(ctx context.Context, cr *redisv1beta2.Redis, cl kubernetes.Interface) error {
	labels := getRedisLabels(cr.ObjectMeta.Name, standalone, "standalone", cr.ObjectMeta.Labels)
	var epp exporterPortProvider
	if cr.Spec.RedisExporter != nil {
		epp = func() (port int, enable bool) {
			defaultP := ptr.To(redisExporterPort)
			return *util.Coalesce(cr.Spec.RedisExporter.Port, defaultP), cr.Spec.RedisExporter.Enabled
		}
	} else {
		epp = disableMetrics
	}
	objectMetaInfo := generateObjectMetaInformation(
		cr.ObjectMeta.Name,
		cr.Namespace,
		labels,
		generateServiceAnots(cr.ObjectMeta, nil, epp),
	)
	headlessObjectMetaInfo := generateObjectMetaInformation(
		cr.ObjectMeta.Name+"-headless",
		cr.Namespace,
		labels,
		generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetHeadlessServiceAnnotations(), epp),
	)
	additionalObjectMetaInfo := generateObjectMetaInformation(
		cr.ObjectMeta.Name+"-additional",
		cr.Namespace,
		labels,
		generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetServiceAnnotations(), epp),
	)
	err := CreateOrUpdateService(ctx, cr.Namespace, headlessObjectMetaInfo, redisAsOwner(cr), disableMetrics, true, "ClusterIP", redisPort, cl)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create standalone headless service for Redis")
		return err
	}
	err = CreateOrUpdateService(ctx, cr.Namespace, objectMetaInfo, redisAsOwner(cr), epp, false, "ClusterIP", redisPort, cl)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create standalone service for Redis")
		return err
	}
	if cr.Spec.KubernetesConfig.ShouldCreateAdditionalService() {
		err = CreateOrUpdateService(
			ctx,
			cr.Namespace,
			additionalObjectMetaInfo,
			redisAsOwner(cr),
			disableMetrics,
			false,
			cr.Spec.KubernetesConfig.GetServiceType(),
			redisPort,
			cl,
		)
		if err != nil {
			log.FromContext(ctx).Error(err, "Cannot create additional service for Redis")
			return err
		}
	}
	return nil
}

// CreateStandaloneRedis will create a standalone redis setup
func CreateStandaloneRedis(ctx context.Context, cr *redisv1beta2.Redis, cl kubernetes.Interface) error {
	labels := getRedisLabels(cr.ObjectMeta.Name, standalone, "standalone", cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(
		ctx,
		cl,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisStandaloneParams(cr),
		redisAsOwner(cr),
		generateRedisStandaloneInitContainerParams(cr),
		generateRedisStandaloneContainerParams(cr),
		cr.Spec.Sidecars,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create standalone statefulset for Redis")
		return err
	}
	return nil
}

// generateRedisStandalone generates Redis standalone information
func generateRedisStandaloneParams(cr *redisv1beta2.Redis) statefulSetParameters {
	replicas := int32(1)
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
		TerminationGracePeriodSeconds: cr.Spec.TerminationGracePeriodSeconds,
		Tolerations:                   cr.Spec.Tolerations,
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
		res.RecreateStatefulsetStrategy = getDeletionPropagationStrategy(cr.ObjectMeta.GetAnnotations())
	}
	return res
}

// generateRedisStandaloneContainerParams generates Redis container information
func generateRedisStandaloneContainerParams(cr *redisv1beta2.Redis) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "standalone",
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

		if cr.Spec.RedisExporter.Resources != nil {
			containerProp.RedisExporterResources = cr.Spec.RedisExporter.Resources
		}

		if cr.Spec.RedisExporter.EnvVars != nil {
			containerProp.RedisExporterEnv = cr.Spec.RedisExporter.EnvVars
		}
		containerProp.RedisExporterSecurityContext = cr.Spec.RedisExporter.SecurityContext
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

// generateRedisStandaloneInitContainerParams generates Redis initcontainer information
func generateRedisStandaloneInitContainerParams(cr *redisv1beta2.Redis) initContainerParameters {
	trueProperty := true
	initcontainerProp := initContainerParameters{}

	if cr.Spec.InitContainer != nil {
		initContainer := cr.Spec.InitContainer

		initcontainerProp = initContainerParameters{
			Enabled:               initContainer.Enabled,
			Role:                  "standalone",
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
