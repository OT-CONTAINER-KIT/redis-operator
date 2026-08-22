package k8sutils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RedisClusterSTS is a interface to call Redis Statefulset function
type RedisClusterSTS struct {
	RedisStateFulType             string
	ExternalConfig                *string
	Resources                     *corev1.ResourceRequirements
	SecurityContext               *corev1.SecurityContext
	Affinity                      *corev1.Affinity `json:"affinity,omitempty"`
	TerminationGracePeriodSeconds *int64           `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	ReadinessProbe                *corev1.Probe
	LivenessProbe                 *corev1.Probe
	NodeSelector                  map[string]string
	TopologySpreadConstraints     []corev1.TopologySpreadConstraint
	Tolerations                   *[]corev1.Toleration
}

// RedisClusterService is a interface to call Redis Service function
type RedisClusterService struct {
	RedisServiceRole string
}

// generateRedisClusterParams generates Redis cluster information
func generateRedisClusterParams(ctx context.Context, cr *rcvb2.RedisCluster, replicas int32, externalConfig *string, params RedisClusterSTS) statefulSetParameters {
	var minreadyseconds int32 = 0
	if cr.Spec.KubernetesConfig.MinReadySeconds != nil {
		minreadyseconds = *cr.Spec.KubernetesConfig.MinReadySeconds
	}
	res := statefulSetParameters{
		Replicas:                             &replicas,
		ClusterMode:                          true,
		ClusterVersion:                       cr.Spec.ClusterVersion,
		NodeSelector:                         params.NodeSelector,
		TopologySpreadConstraints:            params.TopologySpreadConstraints,
		PodSecurityContext:                   cr.Spec.PodSecurityContext,
		PriorityClassName:                    cr.Spec.PriorityClassName,
		Affinity:                             params.Affinity,
		TerminationGracePeriodSeconds:        params.TerminationGracePeriodSeconds,
		Tolerations:                          params.Tolerations,
		ServiceAccountName:                   cr.Spec.ServiceAccountName,
		UpdateStrategy:                       cr.Spec.KubernetesConfig.UpdateStrategy,
		PersistentVolumeClaimRetentionPolicy: cr.Spec.KubernetesConfig.PersistentVolumeClaimRetentionPolicy,
		IgnoreAnnotations:                    cr.Spec.KubernetesConfig.IgnoreAnnotations,
		HostNetwork:                          cr.Spec.HostNetwork,
		MinReadySeconds:                      minreadyseconds,
	}

	if cr.Spec.PodManagementPolicy != nil {
		res.PodManagementPolicy = cr.Spec.PodManagementPolicy
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
	if value, found := cr.GetAnnotations()[common.AnnotationKeyRecreateStatefulset]; found && value == "true" {
		res.RecreateStatefulSet = true
		res.RecreateStatefulsetStrategy = getDeletionPropagationStrategy(cr.GetAnnotations())
	}
	return res
}

func generateRedisClusterInitContainerParams(cr *rcvb2.RedisCluster) initContainerParameters {
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
		if cr.Spec.Storage != nil && cr.Spec.PersistenceEnabled != nil && *cr.Spec.PersistenceEnabled {
			initcontainerProp.PersistenceEnabled = &trueProperty
		}
	}

	return initcontainerProp
}

// generateRedisClusterContainerParams generates Redis container information. It
// returns an error when the per-pod NodePort Services required to build the
// cluster announce variables cannot be read, so that an incomplete pod template
// is never handed to the StatefulSet reconciler.
func generateRedisClusterContainerParams(ctx context.Context, cl kubernetes.Interface, cr *rcvb2.RedisCluster, securityContext *corev1.SecurityContext, readinessProbeDef *corev1.Probe, livenessProbeDef *corev1.Probe, role string, resources *corev1.ResourceRequirements) (containerParameters, error) {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "cluster",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       resources,
		SecurityContext: securityContext,
		Port:            cr.Spec.Port,
		HostPort:        cr.Spec.HostPort,
	}
	if cr.Spec.RedisConfig != nil {
		containerProp.MaxMemoryPercentOfLimit = cr.Spec.RedisConfig.MaxMemoryPercentOfLimit
	}
	if cr.Spec.EnvVars != nil {
		containerProp.EnvVars = cr.Spec.EnvVars
	}
	if cr.Spec.KubernetesConfig.GetServiceType() == "NodePort" {
		// Start from a copy: containerProp.EnvVars may alias cr.Spec.EnvVars, which is
		// shared between the leader and the follower render of the same reconciliation.
		// Appending in place would leak one role's announce variables into the other.
		envVars := &[]corev1.EnvVar{}
		if containerProp.EnvVars != nil {
			*envVars = append(*envVars, *containerProp.EnvVars...)
		}
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

		replicas := cr.Spec.GetReplicaCounts(role)
		for i := 0; i < int(replicas); i++ {
			serviceName := cr.Name + "-" + role + "-" + strconv.Itoa(i)
			svc, err := getService(ctx, cl, cr.Namespace, serviceName)
			if err != nil {
				// Rendering the StatefulSet without the announce variables makes Redis
				// announce an address nothing listens on, so fail instead of shipping a
				// pod template that is known to be incomplete.
				return containerParameters{}, fmt.Errorf("cannot get nodeport service %s/%s: %w", cr.Namespace, serviceName, err)
			}
			announcePort, announceBusPort, err := clusterAnnounceNodePorts(svc)
			if err != nil {
				return containerParameters{}, err
			}
			envSuffix := strings.ReplaceAll(svc.Name, "-", "_")
			*envVars = append(*envVars, corev1.EnvVar{
				Name:  "announce_port_" + envSuffix,
				Value: strconv.Itoa(int(announcePort)),
			})
			*envVars = append(*envVars, corev1.EnvVar{
				Name:  "announce_bus_port_" + envSuffix,
				Value: strconv.Itoa(int(announceBusPort)),
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

	return containerProp, nil
}

// clusterAnnounceNodePorts returns the allocated client and cluster-bus node ports
// of a per-pod NodePort Service. Ports are matched by name because the order of
// the port list is not part of the API contract, and a zero node port means
// Kubernetes has not allocated one yet.
func clusterAnnounceNodePorts(svc *corev1.Service) (int32, int32, error) {
	var announcePort, announceBusPort int32
	for _, port := range svc.Spec.Ports {
		switch port.Name {
		case redisClientPortName:
			announcePort = port.NodePort
		case redisBusPortName:
			announceBusPort = port.NodePort
		}
	}
	if announcePort == 0 || announceBusPort == 0 {
		return 0, 0, fmt.Errorf("service %s/%s has no allocated %q and %q node port", svc.Namespace, svc.Name, redisClientPortName, redisBusPortName)
	}
	return announcePort, announceBusPort, nil
}

// CreateRedisLeader will create a leader redis setup
func CreateRedisLeader(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterSTS{
		RedisStateFulType:             "leader",
		Resources:                     cr.Spec.GetRedisLeaderResources(),
		SecurityContext:               cr.Spec.RedisLeader.SecurityContext,
		Affinity:                      cr.Spec.RedisLeader.Affinity,
		TerminationGracePeriodSeconds: cr.Spec.RedisLeader.TerminationGracePeriodSeconds,
		NodeSelector:                  cr.Spec.RedisLeader.NodeSelector,
		TopologySpreadConstraints:     cr.Spec.RedisLeader.TopologySpreadConstraints,

		Tolerations:    cr.Spec.RedisLeader.Tolerations,
		ReadinessProbe: cr.Spec.RedisLeader.ReadinessProbe,
		LivenessProbe:  cr.Spec.RedisLeader.LivenessProbe,
	}
	if cr.Spec.RedisLeader.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisClusterSetup(ctx, cr, cl)
}

// CreateRedisFollower will create a follower redis setup
func CreateRedisFollower(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterSTS{
		RedisStateFulType:             "follower",
		Resources:                     cr.Spec.GetRedisFollowerResources(),
		SecurityContext:               cr.Spec.RedisFollower.SecurityContext,
		Affinity:                      cr.Spec.RedisFollower.Affinity,
		TerminationGracePeriodSeconds: cr.Spec.RedisFollower.TerminationGracePeriodSeconds,
		NodeSelector:                  cr.Spec.RedisFollower.NodeSelector,
		TopologySpreadConstraints:     cr.Spec.RedisFollower.TopologySpreadConstraints,
		Tolerations:                   cr.Spec.RedisFollower.Tolerations,
		ReadinessProbe:                cr.Spec.RedisFollower.ReadinessProbe,
		LivenessProbe:                 cr.Spec.RedisFollower.LivenessProbe,
	}
	if cr.Spec.RedisFollower.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisFollower.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisClusterSetup(ctx, cr, cl)
}

// CreateRedisLeaderService method will create service for Redis Leader
func CreateRedisLeaderService(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterService{
		RedisServiceRole: "leader",
	}
	return prop.CreateRedisClusterService(ctx, cr, cl)
}

// CreateRedisFollowerService method will create service for Redis Follower
func CreateRedisFollowerService(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface) error {
	prop := RedisClusterService{
		RedisServiceRole: "follower",
	}
	return prop.CreateRedisClusterService(ctx, cr, cl)
}

func (service RedisClusterSTS) getReplicaCount(cr *rcvb2.RedisCluster) int32 {
	return cr.Spec.GetReplicaCounts(service.RedisStateFulType)
}

// CreateRedisClusterSetup will create Redis Setup for leader and follower
func (service RedisClusterSTS) CreateRedisClusterSetup(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface) error {
	stateFulName := cr.Name + "-" + service.RedisStateFulType
	labels := getRedisLabels(stateFulName, cluster, service.RedisStateFulType, cr.Labels)
	// add an common label for all pods in the cluster
	labels["cluster"] = cr.Name
	annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
	objectMetaInfo := generateObjectMetaInformation(stateFulName, cr.Namespace, labels, annotations)
	containerParams, err := generateRedisClusterContainerParams(ctx, cl, cr, service.SecurityContext, service.ReadinessProbe, service.LivenessProbe, service.RedisStateFulType, service.Resources)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot generate container parameters for Redis", "Setup.Type", service.RedisStateFulType)
		return err
	}
	err = CreateOrUpdateStateFul(
		ctx,
		cl,
		cr.GetNamespace(),
		objectMetaInfo,
		generateRedisClusterParams(ctx, cr, service.getReplicaCount(cr), service.ExternalConfig, service),
		redisClusterAsOwner(cr),
		generateRedisClusterInitContainerParams(cr),
		containerParams,
		cr.Spec.Sidecars,
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create statefulset for Redis", "Setup.Type", service.RedisStateFulType)
		return err
	}
	return nil
}

// CreateRedisClusterService method will create service for Redis
func (service RedisClusterService) CreateRedisClusterService(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface) error {
	serviceName := cr.Name + "-" + service.RedisServiceRole
	labels := getRedisLabels(serviceName, cluster, service.RedisServiceRole, cr.Labels)
	var epp exporterPortProvider
	if cr.Spec.RedisExporter != nil {
		epp = func() (port int, enable bool) {
			defaultP := ptr.To(common.RedisExporterPort)
			return *util.Coalesce(cr.Spec.RedisExporter.Port, defaultP), cr.Spec.RedisExporter.Enabled
		}
	} else {
		epp = disableMetrics
	}

	busPort := corev1.ServicePort{
		Name:     redisBusPortName,
		Port:     int32(*cr.Spec.Port + 10000),
		Protocol: corev1.ProtocolTCP,
		TargetPort: intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: int32(*cr.Spec.Port + 10000),
		},
	}

	objectMetaInfo := generateObjectMetaInformation(
		serviceName,
		cr.Namespace,
		labels,
		generateServiceAnots(cr.ObjectMeta, nil, epp),
	)
	headlessObjectMetaInfo := generateObjectMetaInformation(
		serviceName+"-headless",
		cr.Namespace,
		labels,
		generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetHeadlessServiceAnnotations(), epp),
	)
	additionalObjectMetaInfo := generateObjectMetaInformation(
		serviceName+"-additional",
		cr.Namespace,
		labels,
		generateServiceAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.GetServiceAnnotations(), epp),
	)
	headlessExtraPorts := []corev1.ServicePort{}
	if cr.Spec.KubernetesConfig.ShouldIncludeBusPortForHeadless() {
		headlessExtraPorts = append(headlessExtraPorts, busPort)
	}
	err := CreateOrUpdateService(ctx, cr.Namespace, headlessObjectMetaInfo, redisClusterAsOwner(cr), disableMetrics, true, "ClusterIP", *cr.Spec.Port, cl, headlessExtraPorts...)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create headless service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	extraPorts := []corev1.ServicePort{}
	if cr.Spec.KubernetesConfig.ShouldIncludeBusPort() {
		extraPorts = append(extraPorts, busPort)
	}
	err = CreateOrUpdateService(ctx, cr.Namespace, objectMetaInfo, redisClusterAsOwner(cr), epp, false, "ClusterIP", *cr.Spec.Port, cl, extraPorts...)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	if cr.Spec.KubernetesConfig.GetServiceType() == "NodePort" {
		err = service.createOrUpdateClusterNodePortService(ctx, cr, cl)
		if err != nil {
			log.FromContext(ctx).Error(err, "Cannot create nodeport service for Redis", "Setup.Type", service.RedisServiceRole)
			return err
		}
	}
	additionalExtraPorts := []corev1.ServicePort{}
	if cr.Spec.KubernetesConfig.ShouldIncludeBusPortForAdditional() {
		additionalExtraPorts = append(additionalExtraPorts, busPort)
	}
	if cr.Spec.KubernetesConfig.ShouldCreateAdditionalService() {
		err = CreateOrUpdateService(ctx, cr.Namespace, additionalObjectMetaInfo, redisClusterAsOwner(cr), disableMetrics, false, cr.Spec.KubernetesConfig.GetServiceType(), *cr.Spec.Port, cl, additionalExtraPorts...)
		if err != nil {
			log.FromContext(ctx).Error(err, "Cannot create additional service for Redis", "Setup.Type", service.RedisServiceRole)
			return err
		}
	}

	masterObjectMetaInfo := generateObjectMetaInformation(
		cr.Name+"-master",
		cr.Namespace,
		map[string]string{
			"cluster":                cr.Name,
			common.RedisRoleLabelKey: common.RedisRoleLabelMaster,
			"redis_setup_type":       "cluster",
		},
		generateServiceAnots(cr.ObjectMeta, nil, epp),
	)
	err = CreateOrUpdateService(ctx, cr.Namespace, masterObjectMetaInfo, redisClusterAsOwner(cr), disableMetrics, false, "ClusterIP", *cr.Spec.Port, cl)
	if err != nil {
		log.FromContext(ctx).Error(err, "Cannot create master service for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}

	if cr.Spec.RedisExporter != nil && cr.Spec.RedisExporter.Enabled {
		defaultP := ptr.To(common.RedisExporterPort)
		exporterPort := *util.Coalesce(cr.Spec.RedisExporter.Port, defaultP)
		selectorLabels := getRedisStableLabels(serviceName, string(cluster), service.RedisServiceRole)
		err = CreateOrUpdateMetricsService(ctx, cr.Namespace, serviceName+"-metrics", selectorLabels, redisClusterAsOwner(cr), exporterPort, cl)
		if err != nil {
			log.FromContext(ctx).Error(err, "Cannot create metrics service for Redis", "Setup.Type", service.RedisServiceRole)
			return err
		}
	}
	return nil
}

func (service RedisClusterService) createOrUpdateClusterNodePortService(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface) error {
	replicas := cr.Spec.GetReplicaCounts(service.RedisServiceRole)

	for i := 0; i < int(replicas); i++ {
		if err := service.createOrUpdateClusterNodePortServiceForReplica(ctx, cr, cl, i); err != nil {
			log.FromContext(ctx).Error(err, "Cannot create nodeport service for Redis", "Setup.Type", service.RedisServiceRole)
			return err
		}
	}
	if err := service.deleteStaleClusterNodePortServices(ctx, cr, cl, replicas); err != nil {
		log.FromContext(ctx).Error(err, "Cannot delete stale nodeport services for Redis", "Setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}

// deleteStaleClusterNodePortServices removes the per-pod NodePort Services left
// behind by a scale-down. Their node ports stay allocated for as long as the
// Services exist, so a cluster that is repeatedly scaled up and down eventually
// exhausts the node port range.
//
// This runs as part of the Service reconciliation, which the controller performs
// only after the StatefulSet has been updated to the smaller size. Deleting a
// Service whose pod is still meant to be running would strip that pod of the node
// port it announces to the cluster, so the ordering matters: never move this
// ahead of the StatefulSet update, and in particular never into the preflight in
// EnsureRedisClusterNodePortServices, which deliberately runs before it.
func (service RedisClusterService) deleteStaleClusterNodePortServices(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface, replicas int32) error {
	prefix := cr.Name + "-" + service.RedisServiceRole + "-"
	selector := labels.SelectorFromSet(getRedisStableLabels(cr.Name+"-"+service.RedisServiceRole, string(cluster), service.RedisServiceRole))

	services, err := cl.CoreV1().Services(cr.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	for i := range services.Items {
		stale := &services.Items[i]
		// The role wide Services carry the same labels; only the per-pod ones are
		// named after a pod and labelled with it.
		if _, perPod := stale.Labels["statefulset.kubernetes.io/pod-name"]; !perPod {
			continue
		}
		ordinal, err := strconv.Atoi(strings.TrimPrefix(stale.Name, prefix))
		if err != nil || ordinal < int(replicas) {
			continue
		}
		// Only ever delete Services this cluster owns.
		if !isOwnedByRedisCluster(stale, cr) {
			continue
		}
		log.FromContext(ctx).Info("Deleting nodeport service of a scaled down replica",
			"Service", stale.Name, "Setup.Type", service.RedisServiceRole, "Replicas", replicas)
		if err := deleteService(ctx, cl, cr.Namespace, stale.Name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// isOwnedByRedisCluster reports whether cr owns obj. The owner reference is
// matched on UID and name rather than on Kind, because the Kind of an object read
// through a typed client is empty, and redisClusterAsOwner copies it verbatim.
func isOwnedByRedisCluster(obj metav1.Object, cr *rcvb2.RedisCluster) bool {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.UID == cr.UID && owner.Name == cr.Name {
			return true
		}
	}
	return false
}

// clusterNodePortServiceParams returns the object metadata and the cluster-bus
// port of the per-pod NodePort Service of a single replica.
func (service RedisClusterService) clusterNodePortServiceParams(cr *rcvb2.RedisCluster, replica int) (metav1.ObjectMeta, corev1.ServicePort) {
	serviceName := cr.Name + "-" + service.RedisServiceRole + "-" + strconv.Itoa(replica)
	labels := getRedisLabels(cr.Name+"-"+service.RedisServiceRole, cluster, service.RedisServiceRole, map[string]string{
		"statefulset.kubernetes.io/pod-name": serviceName,
	})
	annotations := generateServiceAnots(cr.ObjectMeta, nil, disableMetrics)
	busPort := corev1.ServicePort{
		Name:     redisBusPortName,
		Port:     int32(*cr.Spec.Port + 10000),
		Protocol: corev1.ProtocolTCP,
		TargetPort: intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: int32(*cr.Spec.Port + 10000),
		},
	}
	return generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations), busPort
}

func (service RedisClusterService) createOrUpdateClusterNodePortServiceForReplica(ctx context.Context, cr *rcvb2.RedisCluster, cl kubernetes.Interface, replica int) error {
	objectMetaInfo, busPort := service.clusterNodePortServiceParams(cr, replica)
	return CreateOrUpdateService(ctx, cr.Namespace, objectMetaInfo, redisClusterAsOwner(cr), disableMetrics, false, "NodePort", *cr.Spec.Port, cl, busPort)
}

// EnsureRedisClusterNodePortServices creates missing per-pod NodePort Services
// before the StatefulSet is rendered, so that Kubernetes has allocated the node
// ports that the pod template announces.
//
// Missing Services are created, never updated: an update would move the selector
// of a Service that is already backing a running pod ahead of a StatefulSet update
// that may still fail on an immutable field, which is the regression fixed by
// upstream #1347/#1348. Creating directly rather than going through
// CreateOrUpdateService keeps that guarantee even if the Service appears between
// the lookup and the write.
func EnsureRedisClusterNodePortServices(ctx context.Context, cr *rcvb2.RedisCluster, role string, cl kubernetes.Interface) error {
	if cr.Spec.KubernetesConfig.GetServiceType() != "NodePort" {
		return nil
	}

	service := RedisClusterService{RedisServiceRole: role}
	replicas := cr.Spec.GetReplicaCounts(role)
	for i := 0; i < int(replicas); i++ {
		serviceName := cr.Name + "-" + role + "-" + strconv.Itoa(i)
		if _, err := getService(ctx, cl, cr.Namespace, serviceName); err == nil {
			continue
		} else if !apierrors.IsNotFound(err) {
			return err
		}

		objectMetaInfo, busPort := service.clusterNodePortServiceParams(cr, i)
		serviceDef := generateServiceDef(objectMetaInfo, disableMetrics, redisClusterAsOwner(cr), false, "NodePort", *cr.Spec.Port, busPort)
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil {
			return err
		}
		// A concurrent creation is not a failure: the Service exists, which is all
		// the StatefulSet render needs.
		if err := createService(ctx, cl, cr.Namespace, serviceDef); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}
