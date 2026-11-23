package k8sutils

import (
	"context"
	"fmt"
	"strings"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateReplicationService method will create replication service for Redis
func CreateReplicationService(ctx context.Context, cr *rrvb2.RedisReplication, cl kubernetes.Interface) error {
	labels := getRedisLabels(cr.Name, replication, "replication", cr.Labels)

	epp := disableMetrics
	if cr.Spec.RedisExporter != nil {
		epp = func() (port int, enable bool) {
			defaultP := ptr.To(common.RedisExporterPort)
			return *util.Coalesce(cr.Spec.RedisExporter.Port, defaultP), cr.Spec.RedisExporter.Enabled
		}
	}

	annotations := generateServiceAnots(cr.ObjectMeta, nil, epp)
	objectMetaInfo := generateObjectMetaInformation(cr.Name, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(
		cr.Name+"-headless",
		cr.Namespace,
		labels,
		generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetHeadlessServiceAnnotations(), epp),
	)
	additionalObjectMetaInfo := generateObjectMetaInformation(cr.Name+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetServiceAnnotations(), epp))
	masterLabels := maps.Merge(
		labels, map[string]string{common.RedisRoleLabelKey: common.RedisRoleLabelMaster},
	)
	replicaLabels := maps.Merge(
		labels, map[string]string{common.RedisRoleLabelKey: common.RedisRoleLabelSlave},
	)
	masterObjectMetaInfo := generateObjectMetaInformation(cr.MasterService(), cr.Namespace, masterLabels, annotations)
	replicaObjectMetaInfo := generateObjectMetaInformation(cr.Name+"-replica", cr.Namespace, replicaLabels, annotations)

	if err := CreateOrUpdateService(ctx, cr.Namespace, headlessObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, true, "ClusterIP", common.RedisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replication headless service for Redis")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, objectMetaInfo, redisReplicationAsOwner(cr), epp, false, "ClusterIP", common.RedisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replication service for Redis")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, additionalObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, cr.Spec.KubernetesConfig.GetServiceType(), common.RedisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create additional service for Redis Replication")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, masterObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, "ClusterIP", common.RedisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create master service for Redis")
		return err
	}
	if err := CreateOrUpdateService(ctx, cr.Namespace, replicaObjectMetaInfo, redisReplicationAsOwner(cr), disableMetrics, false, "ClusterIP", common.RedisPort, cl); err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replica service for Redis")
		return err
	}
	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		exporterPort := *util.Coalesce(cr.Spec.RedisExporter.Port, ptr.To(common.RedisExporterPort))
		selectorLabels := getRedisStableLabels(cr.Name, string(replication), "replication")
		if err := CreateOrUpdateMetricsService(ctx, cr.Namespace, cr.Name+"-metrics", selectorLabels, redisReplicationAsOwner(cr), exporterPort, cl); err != nil {
			log.FromContext(ctx).Error(err, "Cannot create metrics service for Redis Replication")
			return err
		}
	}

	return nil
}

// CreateReplicationRedis will create a replication redis setup
func CreateReplicationRedis(ctx context.Context, cr *rrvb2.RedisReplication, cl kubernetes.Interface, ctrlClient client.Reader) error {
	if cr.EnableSentinel() && (cr.Annotations[common.SentinelOwnershipAnnotation] == "sentinel" || cr.Annotations[common.SentinelStartupAnnotation] == "enabled") {
		return fmt.Errorf("activated standalone Sentinel authority does not permit embedded Sentinel")
	}
	stateFulName := cr.Name
	labels := getRedisLabels(cr.Name, replication, "replication", cr.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	containerParams := generateRedisReplicationContainerParams(ctx, cr, ctrlClient)
	if containerParams.SentinelBootstrap && cr.Spec.InitContainer != nil && ptr.Deref(cr.Spec.InitContainer.Enabled, false) {
		return fmt.Errorf("protected Sentinel startup does not permit a later user init container")
	}
	if containerParams.SentinelBootstrap && cr.Spec.ReadinessProbe != nil && (cr.Spec.ReadinessProbe.Exec != nil || cr.Spec.ReadinessProbe.HTTPGet != nil || cr.Spec.ReadinessProbe.TCPSocket != nil || cr.Spec.ReadinessProbe.GRPC != nil) {
		return fmt.Errorf("protected Sentinel startup requires the managed discovery readiness probe")
	}
	if !cr.EnableSentinel() && cr.Annotations[common.SentinelStartupAnnotation] == "enabled" {
		configured := false
		for _, env := range ptr.Deref(containerParams.EnvVars, []corev1.EnvVar{}) {
			if env.Name == "SENTINEL_BOOTSTRAP_ADDRS" && env.Value != "" {
				configured = true
			}
		}
		if !configured {
			return fmt.Errorf("cannot update activated replication without one standalone Sentinel configuration")
		}
	}

	err := CreateOrUpdateStateFul(
		ctx,
		cl,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisReplicationParams(cr),
		redisReplicationAsOwner(cr),
		generateRedisReplicationInitContainerParams(cr),
		containerParams,
		cr.Spec.Sidecars,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create replication statefulset for Redis")
		return err
	}
	return nil
}

func generateRedisReplicationParams(cr *rrvb2.RedisReplication) statefulSetParameters {
	replicas := cr.Spec.GetReplicationCounts("Replication")
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
		Affinity:                             cr.Spec.Affinity,
		Tolerations:                          cr.Spec.Tolerations,
		TopologySpreadConstraints:            cr.Spec.TopologySpreadConstrains,
		TerminationGracePeriodSeconds:        cr.Spec.TerminationGracePeriodSeconds,
		UpdateStrategy:                       cr.Spec.KubernetesConfig.UpdateStrategy,
		PersistentVolumeClaimRetentionPolicy: cr.Spec.KubernetesConfig.PersistentVolumeClaimRetentionPolicy,
		IgnoreAnnotations:                    cr.Spec.KubernetesConfig.IgnoreAnnotations,
		MinReadySeconds:                      minreadyseconds,
	}

	if cr.Spec.PodManagementPolicy != nil {
		res.PodManagementPolicy = cr.Spec.PodManagementPolicy
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
	if value, found := cr.GetAnnotations()[common.AnnotationKeyRecreateStatefulset]; found && value == "true" {
		res.RecreateStatefulSet = true
		res.RecreateStatefulsetStrategy = getDeletionPropagationStrategy(cr.GetAnnotations())
	}
	return res
}

// generateRedisReplicationContainerParams generates Redis container information
func generateRedisReplicationContainerParams(ctx context.Context, cr *rrvb2.RedisReplication, ctrlClient client.Reader) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "replication",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resources,
		SecurityContext: cr.Spec.SecurityContext,
		Port:            ptr.To(common.RedisPort),
		HostPort:        cr.Spec.HostPort,
	}
	if cr.Spec.RedisConfig != nil {
		containerProp.MaxMemoryPercentOfLimit = cr.Spec.RedisConfig.MaxMemoryPercentOfLimit
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
	if storageHasVolumeClaimTemplate(cr.Spec.Storage) {
		containerProp.PersistenceEnabled = &trueProperty
	}
	if cr.Spec.TLS != nil {
		containerProp.TLSConfig = cr.Spec.TLS
	}
	if cr.Spec.ACL != nil {
		containerProp.ACLConfig = cr.Spec.ACL
	}
	// Only wire up the Sentinel failover preStop hook when an embedded Sentinel
	// is actually managing this replication. The service name and master group
	// are sourced from the CR so they match the deployed Sentinel topology, and
	// the demotion wait is bounded by the grace period so it cannot run past it.
	if cr.EnableSentinel() {
		containerProp.SentinelService = cr.SentinelHLService()
		containerProp.SentinelMasterName = cr.SentinelMasterName()
		containerProp.SentinelPort = 26379
		containerProp.PreStopWaitSeconds = preStopWaitSeconds(cr.Spec.TerminationGracePeriodSeconds)
	}
	resolve, announce := sentinelHostnameSettings(ctx, ctrlClient, cr)
	hostnameEnv := []corev1.EnvVar{
		{Name: "RESOLVE_HOSTNAMES", Value: resolve},
		{Name: "ANNOUNCE_HOSTNAMES", Value: announce},
	}
	if containerProp.EnvVars != nil {
		hostnameEnv = append(append([]corev1.EnvVar{}, *containerProp.EnvVars...), hostnameEnv...)
	}
	containerProp.EnvVars = &hostnameEnv
	if !cr.EnableSentinel() && cr.Annotations[common.SentinelStartupAnnotation] == "enabled" && ctrlClient != nil {
		containerProp.SentinelBootstrap = true
		hostnameEnv = append(hostnameEnv, corev1.EnvVar{Name: "SENTINEL_BOOTSTRAP_REQUIRED", Value: "true"})
		hostnameEnv = append(hostnameEnv, corev1.EnvVar{Name: "SENTINEL_MEMBER_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}})
		containerProp.EnvVars = &hostnameEnv
		var linked rsvb2.RedisSentinelList
		if err := ctrlClient.List(ctx, &linked, client.InNamespace(cr.Namespace)); err == nil {
			matches := 0
			for _, rs := range linked.Items {
				if rs.Spec.RedisSentinelConfig != nil && rs.Spec.RedisSentinelConfig.RedisReplicationName == cr.Name {
					matches++
				}
			}
			for _, rs := range linked.Items {
				if matches != 1 || rs.Spec.RedisSentinelConfig == nil || rs.Spec.RedisSentinelConfig.RedisReplicationName != cr.Name {
					continue
				}
				containerProp.SentinelBootstrap = true
				var addresses []string
				for i := int32(0); i < rs.Spec.GetSentinelCounts("sentinel"); i++ {
					addresses = append(addresses, fmt.Sprintf("%s-%d.%s-headless.%s.svc.%s", rs.GetStatefulSetName(), i, rs.GetStatefulSetName(), rs.Namespace, envs.GetServiceDNSDomain()))
				}
				hostnameEnv = append(hostnameEnv, corev1.EnvVar{Name: "SENTINEL_BOOTSTRAP_ADDRS", Value: strings.Join(addresses, ",")}, corev1.EnvVar{Name: "SENTINEL_BOOTSTRAP_GROUP", Value: rs.Spec.RedisSentinelConfig.MasterGroupName})
				var members []string
				for i := int32(0); cr.Spec.Size != nil && i < *cr.Spec.Size; i++ {
					members = append(members, fmt.Sprintf("%s-%d.%s-headless.%s.svc.%s", cr.Name, i, cr.Name, cr.Namespace, envs.GetServiceDNSDomain()))
				}
				hostnameEnv = append(hostnameEnv, corev1.EnvVar{Name: "SENTINEL_BOOTSTRAP_MEMBERS", Value: strings.Join(members, ",")})
				if secret := rs.Spec.KubernetesConfig.ExistingPasswordSecret; secret != nil && secret.Name != nil && secret.Key != nil {
					hostnameEnv = append(hostnameEnv, corev1.EnvVar{Name: "SENTINEL_BOOTSTRAP_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: *secret.Name}, Key: *secret.Key}}})
				}
				containerProp.EnvVars = &hostnameEnv
				break
			}
		}
	}

	return containerProp
}

func sentinelHostnameSettings(ctx context.Context, ctrlClient client.Reader, cr *rrvb2.RedisReplication) (string, string) {
	if cr.EnableSentinel() {
		return sentinelHostnameFlag(cr.Spec.Sentinel.ResolveHostnames), sentinelHostnameFlag(cr.Spec.Sentinel.AnnounceHostnames)
	}
	if ctrlClient == nil {
		return "no", "no"
	}
	var sentinels rsvb2.RedisSentinelList
	if err := ctrlClient.List(ctx, &sentinels, client.InNamespace(cr.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "failed to list RedisSentinel, assuming hostnames are disabled")
		return "no", "no"
	}
	for i := range sentinels.Items {
		cfg := sentinels.Items[i].Spec.RedisSentinelConfig
		if cfg == nil || cfg.RedisReplicationName != cr.Name {
			continue
		}
		return sentinelHostnameFlag(cfg.ResolveHostnames), sentinelHostnameFlag(cfg.AnnounceHostnames)
	}
	return "no", "no"
}

// generateRedisReplicationInitContainerParams generates Redis Replication initcontainer information
func generateRedisReplicationInitContainerParams(cr *rrvb2.RedisReplication) initContainerParameters {
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
		if storageHasVolumeClaimTemplate(cr.Spec.Storage) {
			initcontainerProp.PersistenceEnabled = &trueProperty
		}
	}
	return initcontainerProp
}

func IsRedisReplicationReady(ctx context.Context, client kubernetes.Interface, ctrlClient client.Client, rs *rsvb2.RedisSentinel) bool {
	if _, err := GetStatefulSet(ctx, client, rs.GetNamespace(), rs.Spec.RedisSentinelConfig.RedisReplicationName); err != nil {
		return false
	}
	return getRedisReplicationMasterIP(ctx, client, rs, ctrlClient) != ""
}

func sentinelHostnameFlag(v string) string {
	if v == "" {
		return "no"
	}
	return v
}
