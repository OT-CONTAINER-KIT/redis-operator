package k8sutils

import (
	"strconv"
	"strings"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// RedisClusterSTS is a interface to call Redis Statefulset function
type RedisClusterSTS struct {
	RedisStateFulType             string
	ExternalConfig                *string
	SecurityContext               *corev1.SecurityContext
	Affinity                      *corev1.Affinity `json:"affinity,omitempty"`
	TerminationGracePeriodSeconds *int64           `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	ReadinessProbe                *corev1.Probe
	LivenessProbe                 *corev1.Probe
	NodeSelector                  map[string]string
	Tolerations                   *[]corev1.Toleration
}

// RedisClusterService is a interface to call Redis Service function
type RedisClusterService struct {
	RedisServiceRole string
}

// generateRedisClusterParams generates Redis cluster information
func generateRedisClusterParams(cr *redisv1beta2.RedisCluster, replicas int32, externalConfig *string, params RedisClusterSTS) statefulSetParameters {
	var minreadyseconds int32 = 0
	if cr.Spec.KubernetesConfig.MinReadySeconds != nil {
		minreadyseconds = *cr.Spec.KubernetesConfig.MinReadySeconds
	}
	res := statefulSetParameters{
		Replicas:                      &replicas,
		ClusterMode:                   true,
		ClusterVersion:                cr.Spec.ClusterVersion,
		NodeSelector:                  params.NodeSelector,
		PodSecurityContext:            cr.Spec.PodSecurityContext,
		PriorityClassName:             cr.Spec.PriorityClassName,
		Affinity:                      params.Affinity,
		TerminationGracePeriodSeconds: params.TerminationGracePeriodSeconds,
		Tolerations:                   params.Tolerations,
		ServiceAccountName:            cr.Spec.ServiceAccountName,
		UpdateStrategy:                cr.Spec.KubernetesConfig.UpdateStrategy,
		IgnoreAnnotations:             cr.Spec.KubernetesConfig.IgnoreAnnotations,
		HostNetwork:                   cr.Spec.HostNetwork,
		MinReadySeconds:               minreadyseconds,
	}
	if cr.Spec.RedisExporter != nil {
		res.EnableMetrics = cr.Spec.RedisExporter.Enabled
	}
	if cr.Spec.KubernetesConfig.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.KubernetesConfig.ImagePullSecrets
	}
	if cr.Spec.Storage != nil {
		res.PersistentVolumeClaim = cr.Spec.Storage.VolumeClaimTemplate
		res.NodeConfVolume = cr.Spec.Storage.NodeConfVolume
		res.NodeConfPersistentVolumeClaim = cr.Spec.Storage.NodeConfVolumeClaimTemplate
	}
	if externalConfig != nil {
		res.ExternalConfig = externalConfig
	}
	if _, found := cr.ObjectMeta.GetAnnotations()[AnnotationKeyRecreateStatefulset]; found {
		res.RecreateStatefulSet = true
	}
	return res
}

func generateRedisClusterInitContainerParams(cr *redisv1beta2.RedisCluster) initContainerParameters {
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

// generateRedisClusterContainerParams generates Redis container information
func generateRedisClusterContainerParams(cl kubernetes.Interface, logger logr.Logger, cr *redisv1beta2.RedisCluster, securityContext *corev1.SecurityContext, readinessProbeDef *corev1.Probe, livenessProbeDef *corev1.Probe, role string) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "cluster",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resources,
		SecurityContext: securityContext,
		Port:            cr.Spec.Port,
	}
	if cr.Spec.EnvVars != nil {
		containerProp.EnvVars = cr.Spec.EnvVars
	}
	if cr.Spec.KubernetesConfig.Service != nil && cr.Spec.KubernetesConfig.Service.ServiceType == "NodePort" {
		envVars := util.Coalesce(containerProp.EnvVars, &[]corev1.EnvVar{})
		*envVars = append(*envVars, corev1.EnvVar{
			Name:  "NODEPORT",
			Value: "true",
		})
		*envVars = append(*envVars, corev1.EnvVar{
			Name: "HOST_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.hostIP",
				},
			},
		})

		type ports struct {
			announcePort    int
			announceBusPort int
		}
		nps := map[string]ports{} // pod name to ports
		replicas := cr.Spec.GetReplicaCounts(role)
		for i := 0; i < int(replicas); i++ {
			svc, err := getService(cl, logger, cr.Namespace, cr.ObjectMeta.Name+"-"+role+"-"+strconv.Itoa(i))
			if err != nil {
				log.Error(err, "Cannot get service for Redis", "Setup.Type", role)
			} else {
				nps[svc.Name] = ports{
					announcePort:    int(svc.Spec.Ports[0].NodePort),
					announceBusPort: int(svc.Spec.Ports[1].NodePort),
				}
			}
		}
		for name, np := range nps {
			*envVars = append(*envVars, corev1.EnvVar{
				Name:  "announce_port_" + strings.ReplaceAll(name, "-", "_"),
				Value: strconv.Itoa(np.announcePort),
			})
			*envVars = append(*envVars, corev1.EnvVar{
				Name:  "announce_bus_port_" + strings.ReplaceAll(name, "-", "_"),
				Value: strconv.Itoa(np.announceBusPort),
			})
		}
		containerProp.EnvVars = envVars
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
		if cr.Spec.RedisExporter.Port != nil {
			containerProp.RedisExporterPort = cr.Spec.RedisExporter.Port
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
func CreateRedisLeader(cr *redisv1beta2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterSTS{
		RedisStateFulType:             "leader",
		SecurityContext:               cr.Spec.RedisLeader.SecurityContext,
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
	return prop.CreateRedisClusterSetup(cr, cl)
}

// CreateRedisFollower will create a follower redis setup
func CreateRedisFollower(cr *redisv1beta2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterSTS{
		RedisStateFulType:             "follower",
		SecurityContext:               cr.Spec.RedisFollower.SecurityContext,
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
	return prop.CreateRedisClusterSetup(cr, cl)
}

// CreateRedisLeaderService method will create service for Redis Leader
func CreateRedisLeaderService(cr *redisv1beta2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterService{
		RedisServiceRole: "leader",
	}
	return prop.CreateRedisClusterService(cr, cl)
}

// CreateRedisFollowerService method will create service for Redis Follower
func CreateRedisFollowerService(cr *redisv1beta2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterService{
		RedisServiceRole: "follower",
	}
	return prop.CreateRedisClusterService(cr, cl)
}

func (service RedisClusterSTS) getReplicaCount(cr *redisv1beta2.RedisCluster) int32 {
	return cr.Spec.GetReplicaCounts(service.RedisStateFulType)
}

// CreateRedisClusterSetup will create Redis Setup for leader and follower
func (service RedisClusterSTS) CreateRedisClusterSetup(cr *redisv1beta2.RedisCluster, cl kubernetes.Interface) error {
	stateFulName := cr.ObjectMeta.Name + "-" + service.RedisStateFulType
	logger := statefulSetLogger(cr.Namespace, stateFulName)
	labels := getRedisLabels(stateFulName, cluster, service.RedisStateFulType, cr.ObjectMeta.Labels)
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStateFul(
		cl,
		logger,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisClusterParams(cr, service.getReplicaCount(cr), service.ExternalConfig, service),
		redisClusterAsOwner(cr),
		generateRedisClusterInitContainerParams(cr),
		generateRedisClusterContainerParams(cl, logger, cr, service.SecurityContext, service.ReadinessProbe, service.LivenessProbe, service.RedisStateFulType),
		cr.Spec.Sidecars,
	)
	if err != nil {
		logger.Error(err, "Cannot create statefulset for Redis", "Setup.Type", service.RedisStateFulType)
		return err
	}
	return nil
}

// CreateRedisClusterService method will create service for Redis
func (service RedisClusterService) CreateRedisClusterService(cr *redisv1beta2.RedisCluster, cl kubernetes.Interface) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole
	logger := serviceLogger(cr.Namespace, serviceName)
	labels := getRedisLabels(serviceName, cluster, service.RedisServiceRole, cr.ObjectMeta.Labels)
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
	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations)
	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, annotations)
	additionalObjectMetaInfo := generateObjectMetaInformation(serviceName+"-additional", cr.Namespace, labels, generateServiceAnots(cr.ObjectMeta, additionalServiceAnnotations, epp))
	err := CreateOrUpdateService(cr.Namespace, headlessObjectMetaInfo, redisClusterAsOwner(cr), disableMetrics, true, "ClusterIP", *cr.Spec.Port, cl)
	if err != nil {
		logger.Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisClusterAsOwner(cr), epp, false, "ClusterIP", *cr.Spec.Port, cl)
	if err != nil {
		logger.Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	additionalServiceType := "ClusterIP"
	if cr.Spec.KubernetesConfig.Service != nil {
		additionalServiceType = cr.Spec.KubernetesConfig.Service.ServiceType
		if additionalServiceType == "NodePort" {
			// If NodePort is enabled, we need to create a service for every redis pod.
			// Then use --cluster-announce-ip --cluster-announce-port --cluster-announce-bus-port to make cluster.
			err = service.createOrUpdateClusterNodePortService(cr, cl)
			if err != nil {
				logger.Error(err, "Cannot create nodeport service for Redis", "Setup.Type", service.RedisServiceRole)
				return err
			}
		}
	}
	err = CreateOrUpdateService(cr.Namespace, additionalObjectMetaInfo, redisClusterAsOwner(cr), disableMetrics, false, additionalServiceType, *cr.Spec.Port, cl)
	if err != nil {
		logger.Error(err, "Cannot create additional service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}

func (service RedisClusterService) createOrUpdateClusterNodePortService(cr *redisv1beta2.RedisCluster, cl kubernetes.Interface) error {
	replicas := cr.Spec.GetReplicaCounts(service.RedisServiceRole)

	for i := 0; i < int(replicas); i++ {
		serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole + "-" + strconv.Itoa(i)
		logger := serviceLogger(cr.Namespace, serviceName)
		labels := getRedisLabels(cr.ObjectMeta.Name+"-"+service.RedisServiceRole, cluster, service.RedisServiceRole, map[string]string{
			"statefulset.kubernetes.io/pod-name": serviceName,
		})
		annotations := generateServiceAnots(cr.ObjectMeta, nil, disableMetrics)
		objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations)
		busPort := corev1.ServicePort{
			Name:     "redis-bus",
			Port:     int32(*cr.Spec.Port + 10000),
			Protocol: corev1.ProtocolTCP,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(*cr.Spec.Port + 10000),
			},
		}
		err := CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisClusterAsOwner(cr), disableMetrics, false, "NodePort", *cr.Spec.Port, cl, busPort)
		if err != nil {
			logger.Error(err, "Cannot create nodeport service for Redis", "Setup.Type", service.RedisServiceRole)
			return err
		}
	}
	return nil
}
