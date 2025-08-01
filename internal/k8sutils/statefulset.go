package k8sutils

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/consts"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	internalenv "github.com/OT-CONTAINER-KIT/redis-operator/internal/env"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/image"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/env"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type StatefulSet interface {
	IsStatefulSetReady(ctx context.Context, namespace, name string) bool
	GetStatefulSetReplicas(ctx context.Context, namespace, name string) int32
}

type StatefulSetService struct {
	kubeClient kubernetes.Interface
}

func NewStatefulSetService(kubeClient kubernetes.Interface) *StatefulSetService {
	return &StatefulSetService{
		kubeClient: kubeClient,
	}
}

func (s *StatefulSetService) IsStatefulSetReady(ctx context.Context, namespace, name string) bool {
	var (
		partition = 0
		replicas  = 1
	)

	sts, err := s.kubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to get statefulset")
		return false
	}

	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition = int(*sts.Spec.UpdateStrategy.RollingUpdate.Partition)
	}
	if sts.Spec.Replicas != nil {
		replicas = int(*sts.Spec.Replicas)
	}

	if expectedUpdateReplicas := replicas - partition; sts.Status.UpdatedReplicas < int32(expectedUpdateReplicas) {
		log.FromContext(ctx).V(1).Info("StatefulSet is not ready", "Status.UpdatedReplicas", sts.Status.UpdatedReplicas, "ExpectedUpdateReplicas", expectedUpdateReplicas)
		return false
	}
	if partition == 0 && sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		log.FromContext(ctx).V(1).Info("StatefulSet is not ready", "Status.CurrentRevision", sts.Status.CurrentRevision, "Status.UpdateRevision", sts.Status.UpdateRevision)
		return false
	}
	if sts.Status.ObservedGeneration != sts.Generation {
		log.FromContext(ctx).V(1).Info("StatefulSet is not ready", "Status.ObservedGeneration", sts.Status.ObservedGeneration, "Generation", sts.Generation)
		return false
	}
	if int(sts.Status.ReadyReplicas) != replicas {
		log.FromContext(ctx).V(1).Info("StatefulSet is not ready", "Status.ReadyReplicas", sts.Status.ReadyReplicas, "Replicas", replicas)
		return false
	}
	return true
}

func (s *StatefulSetService) GetStatefulSetReplicas(ctx context.Context, namespace, name string) int32 {
	sts, err := s.kubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0
	}
	if sts.Spec.Replicas == nil {
		return 0
	}
	return *sts.Spec.Replicas
}

const (
	redisExporterContainer = "redis-exporter"
)

// statefulSetParameters will define statefulsets input params
type statefulSetParameters struct {
	Replicas                             *int32
	ClusterMode                          bool
	ClusterVersion                       *string
	NodeConfVolume                       bool
	NodeSelector                         map[string]string
	TopologySpreadConstraints            []corev1.TopologySpreadConstraint
	PodSecurityContext                   *corev1.PodSecurityContext
	PriorityClassName                    string
	Affinity                             *corev1.Affinity
	Tolerations                          *[]corev1.Toleration
	EnableMetrics                        bool
	PersistentVolumeClaim                corev1.PersistentVolumeClaim
	NodeConfPersistentVolumeClaim        corev1.PersistentVolumeClaim
	ImagePullSecrets                     *[]corev1.LocalObjectReference
	ExternalConfig                       *string
	ServiceAccountName                   *string
	UpdateStrategy                       appsv1.StatefulSetUpdateStrategy
	PersistentVolumeClaimRetentionPolicy *appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy
	RecreateStatefulSet                  bool
	RecreateStatefulsetStrategy          *metav1.DeletionPropagation
	TerminationGracePeriodSeconds        *int64
	IgnoreAnnotations                    []string
	HostNetwork                          bool
	MinReadySeconds                      int32
}

// containerParameters will define container input params
type containerParameters struct {
	Image                        string
	ImagePullPolicy              corev1.PullPolicy
	Resources                    *corev1.ResourceRequirements
	MaxMemoryPercentOfLimit      *int
	SecurityContext              *corev1.SecurityContext
	RedisExporterImage           string
	RedisExporterImagePullPolicy corev1.PullPolicy
	RedisExporterResources       *corev1.ResourceRequirements
	RedisExporterEnv             *[]corev1.EnvVar
	RedisExporterPort            *int
	RedisExporterSecurityContext *corev1.SecurityContext
	Role                         string
	EnabledPassword              *bool
	SecretName                   *string
	SecretKey                    *string
	PersistenceEnabled           *bool
	TLSConfig                    *commonapi.TLSConfig
	ACLConfig                    *commonapi.ACLConfig
	ReadinessProbe               *corev1.Probe
	LivenessProbe                *corev1.Probe
	AdditionalEnvVariable        *[]corev1.EnvVar
	AdditionalVolume             []corev1.Volume
	AdditionalMountPath          []corev1.VolumeMount
	EnvVars                      *[]corev1.EnvVar
	Port                         *int
	HostPort                     *int
}

type initContainerParameters struct {
	Enabled               *bool
	Image                 string
	ImagePullPolicy       corev1.PullPolicy
	Resources             *corev1.ResourceRequirements
	Role                  string
	Command               []string
	Arguments             []string
	PersistenceEnabled    *bool
	AdditionalEnvVariable *[]corev1.EnvVar
	AdditionalVolume      []corev1.Volume
	AdditionalMountPath   []corev1.VolumeMount
	SecurityContext       *corev1.SecurityContext
}

// CreateOrUpdateStateFul method will create or update Redis service
func CreateOrUpdateStateFul(ctx context.Context, cl kubernetes.Interface, namespace string, stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, initcontainerParams initContainerParameters, containerParams containerParameters, sidecars *[]commonapi.Sidecar) error {
	storedStateful, err := GetStatefulSet(ctx, cl, namespace, stsMeta.Name)
	statefulSetDef := generateStatefulSetsDef(stsMeta, params, ownerDef, initcontainerParams, containerParams, getSidecars(sidecars))
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(statefulSetDef); err != nil { //nolint:gocritic
			log.FromContext(ctx).Error(err, "Unable to patch redis statefulset with comparison object")
			return err
		}
		if apierrors.IsNotFound(err) {
			return createStatefulSet(ctx, cl, namespace, statefulSetDef)
		}
		return err
	}
	return patchStatefulSet(ctx, storedStateful, statefulSetDef, namespace, params.RecreateStatefulSet, params.RecreateStatefulsetStrategy, cl)
}

// patchStatefulSet patches the Redis StatefulSet by applying changes while maintaining atomicity.
func patchStatefulSet(ctx context.Context, storedStateful, newStateful *appsv1.StatefulSet, namespace string, recreateStatefulSet bool, deletePropagation *metav1.DeletionPropagation, cl kubernetes.Interface) error {
	// Sync system-managed fields to ensure atomic update.
	syncManagedFields(storedStateful, newStateful)

	vctModified := false
	if hasVolumeClaimTemplates(newStateful, storedStateful) {
		originalCap := storedStateful.Annotations["storageCapacity"]
		if err := HandlePVCResizing(ctx, storedStateful, newStateful, cl); err != nil {
			return err
		}
		// NOTE: this way of detecting changes is hacky because we rely on
		// HandlePVCResizing updating the storedStateful.Annotations as a side
		// effect.  Also, the code will not detect when other VCT fields change
		vctModified = storedStateful.Annotations["storageCapacity"] != originalCap
		if !recreateStatefulSet {
			// Since VolumeClaimTemplate fields are immutable, revert to the stored configuration
			// if we are not recreating the StatefulSet.
			if newStateful.Annotations == nil {
				newStateful.Annotations = make(map[string]string)
			}
			newStateful.Annotations["storageCapacity"] = storedStateful.Annotations["storageCapacity"]
			newStateful.Spec.VolumeClaimTemplates = storedStateful.Spec.VolumeClaimTemplates
			if vctModified {
				log.FromContext(ctx).V(1).Info("VolumeClaimTemplate change is being ignored because the field is immutable. Consider enabling recreating the statefulset option.")
			}
		}
	}

	// Calculate the patch between the stored and new objects, ignoring immutable or unnecessary fields.
	patchResult, err := patch.DefaultPatchMaker.Calculate(storedStateful, newStateful,
		patch.IgnoreStatusFields(),
		patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus(),
		patch.IgnoreField("kind"),
		patch.IgnoreField("apiVersion"),
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Unable to calculate patch for redis statefulset")
		return err
	}

	if patchResult.IsEmpty() && !vctModified {
		log.FromContext(ctx).V(1).Info("Reconciliation complete, no changes required.")
		return nil
	}

	log.FromContext(ctx).V(1).Info("Changes detected in statefulset, updating...", "patch", string(patchResult.Patch), "VCT modified", vctModified)

	// Merge missing annotations from the stored object into the new object.
	mergeAnnotations(storedStateful, newStateful)

	// Set the last applied annotation for future patch comparisons.
	if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newStateful); err != nil {
		log.FromContext(ctx).Error(err, "Failed to set last applied annotation for redis statefulset")
		return err
	}

	return updateStatefulSet(ctx, cl, namespace, newStateful, recreateStatefulSet, deletePropagation)
}

// syncManagedFields syncs system-managed fields from the stored object to the new object.
func syncManagedFields(stored, new *appsv1.StatefulSet) {
	new.ResourceVersion = stored.ResourceVersion
	new.CreationTimestamp = stored.CreationTimestamp
	new.ManagedFields = stored.ManagedFields
}

// hasVolumeClaimTemplates checks if the StatefulSet has VolumeClaimTemplates and if their counts match.
func hasVolumeClaimTemplates(new, stored *appsv1.StatefulSet) bool {
	return len(new.Spec.VolumeClaimTemplates) >= 1 && len(new.Spec.VolumeClaimTemplates) == len(stored.Spec.VolumeClaimTemplates)
}

// mergeAnnotations merges annotations from the stored object into the new object if missing.
func mergeAnnotations(stored, new *appsv1.StatefulSet) {
	if new.Annotations == nil {
		new.Annotations = make(map[string]string)
	}
	for key, value := range stored.Annotations {
		if _, exists := new.Annotations[key]; !exists {
			new.Annotations[key] = value
		}
	}
}

// generateStatefulSetsDef generates the statefulsets definition of Redis
func generateStatefulSetsDef(stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, initcontainerParams initContainerParameters, containerParams containerParameters, sidecars []commonapi.Sidecar) *appsv1.StatefulSet {
	// Generate stable selector labels (only core labels that won't change)
	selectorLabels := extractStatefulSetSelectorLabels(stsMeta.GetLabels())

	statefulset := &appsv1.StatefulSet{
		TypeMeta:   generateMetaInformation("StatefulSet", "apps/v1"),
		ObjectMeta: stsMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector:                             LabelSelectors(selectorLabels),
			ServiceName:                          fmt.Sprintf("%s-headless", stsMeta.Name),
			Replicas:                             params.Replicas,
			UpdateStrategy:                       params.UpdateStrategy,
			PersistentVolumeClaimRetentionPolicy: params.PersistentVolumeClaimRetentionPolicy,
			MinReadySeconds:                      params.MinReadySeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      stsMeta.GetLabels(),
					Annotations: generateStatefulSetsAnots(stsMeta, params.IgnoreAnnotations),
				},
				Spec: corev1.PodSpec{
					Containers: generateContainerDef(
						stsMeta.GetName(),
						containerParams,
						params.ClusterMode,
						params.NodeConfVolume,
						params.EnableMetrics,
						params.ExternalConfig,
						params.ClusterVersion,
						containerParams.AdditionalMountPath,
						sidecars,
					),
					NodeSelector:                  params.NodeSelector,
					TopologySpreadConstraints:     params.TopologySpreadConstraints,
					SecurityContext:               params.PodSecurityContext,
					PriorityClassName:             params.PriorityClassName,
					Affinity:                      params.Affinity,
					TerminationGracePeriodSeconds: params.TerminationGracePeriodSeconds,
					HostNetwork:                   params.HostNetwork,
					Volumes:                       []corev1.Volume{generateConfigVolume(common.VolumeNameConfig)},
				},
			},
		},
	}

	statefulset.Spec.Template.Spec.InitContainers = generateInitContainerDef(containerParams.Role, stsMeta.GetName(), initcontainerParams, initcontainerParams.AdditionalMountPath, containerParams, params.ClusterVersion)

	if params.Tolerations != nil {
		statefulset.Spec.Template.Spec.Tolerations = *params.Tolerations
	}
	if params.ImagePullSecrets != nil {
		statefulset.Spec.Template.Spec.ImagePullSecrets = *params.ImagePullSecrets
	}
	if containerParams.PersistenceEnabled != nil && params.ClusterMode && params.NodeConfVolume {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, createPVCTemplate("node-conf", stsMeta, params.NodeConfPersistentVolumeClaim))
	}
	if containerParams.PersistenceEnabled != nil && *containerParams.PersistenceEnabled {
		pvcTplName := env.GetString(common.EnvOperatorSTSPVCTemplateName, stsMeta.GetName())
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, createPVCTemplate(pvcTplName, stsMeta, params.PersistentVolumeClaim))
	}
	if params.ExternalConfig != nil {
		statefulset.Spec.Template.Spec.Volumes = getExternalConfig(*params.ExternalConfig)
	}
	if containerParams.AdditionalVolume != nil {
		statefulset.Spec.Template.Spec.Volumes = append(statefulset.Spec.Template.Spec.Volumes, containerParams.AdditionalVolume...)
	}

	if containerParams.TLSConfig != nil {
		statefulset.Spec.Template.Spec.Volumes = append(statefulset.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &containerParams.TLSConfig.Secret,
				},
			})
	}

	if containerParams.ACLConfig != nil && containerParams.ACLConfig.Secret != nil {
		statefulset.Spec.Template.Spec.Volumes = append(statefulset.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "acl-secret",
				VolumeSource: corev1.VolumeSource{
					Secret: containerParams.ACLConfig.Secret,
				},
			})
	}

	if params.ServiceAccountName != nil {
		statefulset.Spec.Template.Spec.ServiceAccountName = *params.ServiceAccountName
	}

	AddOwnerRefToObject(statefulset, ownerDef)
	return statefulset
}

func generateConfigVolume(volumeName string) corev1.Volume {
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func generateConfigVolumeMount(volumeName string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/etc/redis",
	}
}

// getExternalConfig will return the redis external configuration
func getExternalConfig(configMapName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "external-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		},
	}
}

// createPVCTemplate will create the persistent volume claim template
func createPVCTemplate(volumeName string, stsMeta metav1.ObjectMeta, storageSpec corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	pvcTemplate := storageSpec
	pvcTemplate.CreationTimestamp = metav1.Time{}
	pvcTemplate.Name = volumeName
	pvcTemplate.Labels = stsMeta.GetLabels()
	// We want the same annotation as the StatefulSet here
	pvcTemplate.Annotations = generateStatefulSetsAnots(stsMeta, nil)
	if storageSpec.Spec.AccessModes == nil {
		pvcTemplate.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	} else {
		pvcTemplate.Spec.AccessModes = storageSpec.Spec.AccessModes
	}
	pvcVolumeMode := corev1.PersistentVolumeFilesystem
	if storageSpec.Spec.VolumeMode != nil {
		pvcVolumeMode = *storageSpec.Spec.VolumeMode
	}
	pvcTemplate.Spec.VolumeMode = &pvcVolumeMode
	pvcTemplate.Spec.Resources = storageSpec.Spec.Resources
	pvcTemplate.Spec.Selector = storageSpec.Spec.Selector
	return pvcTemplate
}

// generateContainerDef generates container definition for Redis
func generateContainerDef(name string, containerParams containerParameters, clusterMode, nodeConfVolume, enableMetrics bool, externalConfig, clusterVersion *string, mountpath []corev1.VolumeMount, sidecars []commonapi.Sidecar) []corev1.Container {
	sentinelCntr := containerParams.Role == "sentinel"
	enableTLS := containerParams.TLSConfig != nil
	enableAuth := containerParams.EnabledPassword != nil && *containerParams.EnabledPassword
	containerDefinition := []corev1.Container{
		{
			Name:            name,
			Image:           containerParams.Image,
			ImagePullPolicy: containerParams.ImagePullPolicy,
			SecurityContext: containerParams.SecurityContext,
			Env: getEnvironmentVariables(
				containerParams.Role,
				containerParams.EnabledPassword,
				containerParams.SecretName,
				containerParams.SecretKey,
				containerParams.PersistenceEnabled,
				containerParams.TLSConfig,
				containerParams.ACLConfig,
				containerParams.EnvVars,
				containerParams.Port,
				clusterVersion,
			),
			ReadinessProbe: getProbeInfo(containerParams.ReadinessProbe, sentinelCntr, enableTLS, enableAuth),
			LivenessProbe:  getProbeInfo(containerParams.LivenessProbe, sentinelCntr, enableTLS, enableAuth),
			VolumeMounts:   getVolumeMount(name, containerParams.PersistenceEnabled, clusterMode, nodeConfVolume, externalConfig, mountpath, containerParams.TLSConfig, containerParams.ACLConfig),
		},
	}

	if features.Enabled(features.GenerateConfigInInitContainer) {
		if sentinelCntr {
			containerDefinition[0].Command = []string{"redis-sentinel"}
			containerDefinition[0].Args = []string{"/etc/redis/sentinel.conf"}
		} else {
			containerDefinition[0].Command = []string{"redis-server"}
			containerDefinition[0].Args = []string{"/etc/redis/redis.conf"}
		}
	}

	if preStopCmd := GeneratePreStopCommand(containerParams.Role, enableAuth, enableTLS); preStopCmd != "" {
		containerDefinition[0].Lifecycle = &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"sh", "-c", preStopCmd},
				},
			},
		}
	}

	if containerParams.HostPort != nil {
		containerDefinition[0].Ports = []corev1.ContainerPort{
			{
				HostPort:      int32(*containerParams.HostPort),
				ContainerPort: int32(*containerParams.Port),
			},
		}
	}

	if containerParams.Resources != nil {
		containerDefinition[0].Resources = *containerParams.Resources
	}
	if enableMetrics {
		containerDefinition = append(containerDefinition, enableRedisMonitoring(containerParams))
	}
	for _, sidecar := range sidecars {
		container := corev1.Container{
			Name:            sidecar.Name,
			Image:           sidecar.Image,
			ImagePullPolicy: sidecar.ImagePullPolicy,
			SecurityContext: sidecar.SecurityContext,
		}
		if sidecar.Command != nil {
			container.Command = sidecar.Command
		}
		if sidecar.Ports != nil {
			container.Ports = append(container.Ports, *sidecar.Ports...)
		}
		if sidecar.Volumes != nil {
			container.VolumeMounts = *sidecar.Volumes
		}
		if sidecar.Resources != nil {
			container.Resources = *sidecar.Resources
		}
		if sidecar.EnvVars != nil {
			container.Env = *sidecar.EnvVars
		}
		containerDefinition = append(containerDefinition, container)
	}

	if containerParams.AdditionalEnvVariable != nil {
		containerDefinition[0].Env = append(containerDefinition[0].Env, *containerParams.AdditionalEnvVariable...)
	}

	return containerDefinition
}

// GeneratePreStopCommand generates the preStop script based on the Redis role.
// Only "cluster" role is supported for now; other roles return an empty string.
func GeneratePreStopCommand(role string, enableAuth, enableTLS bool) string {
	authArgs, tlsArgs := GenerateAuthAndTLSArgs(enableAuth, enableTLS)

	switch role {
	case "cluster":
		return generateClusterPreStop(authArgs, tlsArgs)
	default:
		return ""
	}
}

// GenerateAuthAndTLSArgs constructs authentication and TLS arguments for redis-cli.
func GenerateAuthAndTLSArgs(enableAuth, enableTLS bool) (string, string) {
	authArgs := ""
	tlsArgs := ""

	if enableAuth {
		authArgs = " -a \"${REDIS_PASSWORD}\""
	}
	if enableTLS {
		tlsArgs = " --tls --cert \"${REDIS_TLS_CERT}\" --key \"${REDIS_TLS_CERT_KEY}\" --cacert \"${REDIS_TLS_CA_KEY}\""
	}
	return authArgs, tlsArgs
}

// generateClusterPreStop generates the preStop script for Redis cluster mode.
// It identifies the master node and triggers a failover to the best available slave before shutdown.
func generateClusterPreStop(authArgs, tlsArgs string) string {
	return fmt.Sprintf(`#!/bin/sh
ROLE=$(redis-cli -h $(hostname) -p ${REDIS_PORT} %s %s info replication | awk -F: '/role:master/ {print "master"}')

if [ "$ROLE" = "master" ]; then
    BEST_SLAVE=$(redis-cli -h $(hostname) -p ${REDIS_PORT} %s %s info replication | awk -F: '
        BEGIN { maxOffset = -1; bestSlave = "" }
        /slave[0-9]+:ip/ {
            split($2, a, ",");
            split(a[1], ip_arr, "=");
            split(a[4], offset_arr, "=");
            ip = ip_arr[2];
            offset = offset_arr[2] + 0;
            if (offset > maxOffset) {
                maxOffset = offset;
                bestSlave = ip;
            }
        }
        END { print bestSlave }
    ')

    if [ -n "$BEST_SLAVE" ]; then
        redis-cli -h "$BEST_SLAVE" -p ${REDIS_PORT} %s %s cluster failover
    fi
fi`, authArgs, tlsArgs, authArgs, tlsArgs, authArgs, tlsArgs)
}

func generateInitContainerDef(role, name string, initcontainerParams initContainerParameters, mountpath []corev1.VolumeMount, containerParams containerParameters, clusterVersion *string) []corev1.Container {
	containers := []corev1.Container{}

	if features.Enabled(features.GenerateConfigInInitContainer) {
		image, _ := util.CoalesceEnv(internalenv.OperatorImageEnv, image.GetOperatorImage())
		// give all container env vars to init container
		envVars := append(
			ptr.Deref(containerParams.EnvVars, []corev1.EnvVar{}),
			ptr.Deref(containerParams.AdditionalEnvVariable, []corev1.EnvVar{})...,
		)
		if containerParams.Resources != nil && containerParams.MaxMemoryPercentOfLimit != nil {
			memLimit := containerParams.Resources.Limits.Memory().Value()
			if memLimit != 0 {
				maxMem := int(float64(memLimit) * float64(*containerParams.MaxMemoryPercentOfLimit) / 100)
				envVars = append(envVars, corev1.EnvVar{
					Name:  consts.ENV_KEY_REDIS_MAX_MEMORY,
					Value: fmt.Sprintf("%d", maxMem),
				})
			}
		}
		container := corev1.Container{
			Name:            "init-config",
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"/operator", "agent"},
			Env: getEnvironmentVariables(
				containerParams.Role,
				containerParams.EnabledPassword,
				containerParams.SecretName,
				containerParams.SecretKey,
				containerParams.PersistenceEnabled,
				containerParams.TLSConfig,
				containerParams.ACLConfig,
				&envVars,
				containerParams.Port,
				clusterVersion,
			),
			VolumeMounts: []corev1.VolumeMount{
				generateConfigVolumeMount(common.VolumeNameConfig),
			},
		}
		if role == "sentinel" {
			container.Args = []string{"bootstrap", "--sentinel"}
		} else {
			container.Args = []string{"bootstrap"}
		}
		containers = append(containers, container)
	}

	if initcontainerParams.Enabled != nil && *initcontainerParams.Enabled {
		containers = append(containers, corev1.Container{
			Name:            "init" + name,
			Image:           initcontainerParams.Image,
			ImagePullPolicy: initcontainerParams.ImagePullPolicy,
			Command:         initcontainerParams.Command,
			Args:            initcontainerParams.Arguments,
			VolumeMounts:    getVolumeMount(name, initcontainerParams.PersistenceEnabled, false, false, nil, mountpath, nil, nil),
			SecurityContext: initcontainerParams.SecurityContext,
			Resources:       ptr.Deref(initcontainerParams.Resources, corev1.ResourceRequirements{}),
			Env:             ptr.Deref(initcontainerParams.AdditionalEnvVariable, []corev1.EnvVar{}),
		})
	}
	return containers
}

func GenerateTLSEnvironmentVariables(tlsconfig *commonapi.TLSConfig) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	root := "/tls/"

	// get and set Defaults
	caCert := "ca.crt"
	tlsCert := "tls.crt"
	tlsCertKey := "tls.key"

	if tlsconfig.CaKeyFile != "" {
		caCert = tlsconfig.CaKeyFile
	}
	if tlsconfig.CertKeyFile != "" {
		tlsCert = tlsconfig.CertKeyFile
	}
	if tlsconfig.KeyFile != "" {
		tlsCertKey = tlsconfig.KeyFile
	}

	envVars = append(envVars, corev1.EnvVar{
		Name:  "TLS_MODE",
		Value: "true",
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_TLS_CA_KEY",
		Value: path.Join(root, caCert),
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_TLS_CERT",
		Value: path.Join(root, tlsCert),
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_TLS_CERT_KEY",
		Value: path.Join(root, tlsCertKey),
	})
	return envVars
}

// enableRedisMonitoring will add Redis Exporter as sidecar container
func enableRedisMonitoring(params containerParameters) corev1.Container {
	exporterDefinition := corev1.Container{
		Name:            redisExporterContainer,
		Image:           params.RedisExporterImage,
		ImagePullPolicy: params.RedisExporterImagePullPolicy,
		Env:             getExporterEnvironmentVariables(params),
		VolumeMounts:    getVolumeMount("", nil, false, false, nil, params.AdditionalMountPath, params.TLSConfig, params.ACLConfig), // We need/want the tls-certs but we DON'T need the PVC (if one is available)
		Ports: []corev1.ContainerPort{
			{
				Name:          common.RedisExporterPortName,
				ContainerPort: int32(*util.Coalesce(params.RedisExporterPort, ptr.To(common.RedisExporterPort))),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		SecurityContext: params.RedisExporterSecurityContext,
	}
	if params.RedisExporterResources != nil {
		exporterDefinition.Resources = *params.RedisExporterResources
	}
	return exporterDefinition
}

func getExporterEnvironmentVariables(params containerParameters) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	redisHost := "redis://localhost:"
	if params.TLSConfig != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REDIS_EXPORTER_TLS_CLIENT_KEY_FILE",
			Value: "/tls/tls.key",
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REDIS_EXPORTER_TLS_CLIENT_CERT_FILE",
			Value: "/tls/tls.crt",
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REDIS_EXPORTER_TLS_CA_CERT_FILE",
			Value: "/tls/ca.crt",
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REDIS_EXPORTER_SKIP_TLS_VERIFICATION",
			Value: "true",
		})
		redisHost = "rediss://localhost:"
	}
	if params.RedisExporterPort != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REDIS_EXPORTER_WEB_LISTEN_ADDRESS",
			Value: fmt.Sprintf(":%d", *params.RedisExporterPort),
		})
	}
	if params.Port != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REDIS_ADDR",
			Value: redisHost + strconv.Itoa(*params.Port),
		})
	}
	if params.EnabledPassword != nil && *params.EnabledPassword {
		envVars = append(envVars, corev1.EnvVar{
			Name: "REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *params.SecretName,
					},
					Key: *params.SecretKey,
				},
			},
		})
	}
	if params.RedisExporterEnv != nil {
		envVars = append(envVars, *params.RedisExporterEnv...)
	}

	sort.SliceStable(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})
	return envVars
}

// getVolumeMount gives information about persistence mount
func getVolumeMount(name string, persistenceEnabled *bool, clusterMode bool, nodeConfVolume bool, externalConfig *string, mountpath []corev1.VolumeMount, tlsConfig *commonapi.TLSConfig, aclConfig *commonapi.ACLConfig) []corev1.VolumeMount {
	var VolumeMounts []corev1.VolumeMount

	if persistenceEnabled != nil && clusterMode && nodeConfVolume {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "node-conf",
			MountPath: "/node-conf",
		})
	}

	if persistenceEnabled != nil && *persistenceEnabled {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      env.GetString(common.EnvOperatorSTSPVCTemplateName, name),
			MountPath: "/data",
		})
	}

	if tlsConfig != nil {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "tls-certs",
			ReadOnly:  true,
			MountPath: "/tls",
		})
	}

	if aclConfig != nil {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "acl-secret",
			MountPath: "/etc/redis/user.acl",
			SubPath:   "user.acl",
		})
	}

	if externalConfig != nil {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "external-config",
			MountPath: "/etc/redis/external.conf.d",
		})
	}

	if features.Enabled(features.GenerateConfigInInitContainer) {
		VolumeMounts = append(VolumeMounts, generateConfigVolumeMount(common.VolumeNameConfig))
	}

	VolumeMounts = append(VolumeMounts, mountpath...)

	return VolumeMounts
}

// getProbeInfo generate probe for Redis StatefulSet
func getProbeInfo(probe *corev1.Probe, sentinel, enableTLS, enableAuth bool) *corev1.Probe {
	if probe == nil {
		probe = &corev1.Probe{}
	}
	if probe.Exec == nil && probe.HTTPGet == nil && probe.TCPSocket == nil && probe.GRPC == nil {
		healthChecker := []string{
			"redis-cli",
			"-h", "$(hostname)",
		}
		if sentinel {
			healthChecker = append(healthChecker, "-p", "${SENTINEL_PORT}")
		} else {
			healthChecker = append(healthChecker, "-p", "${REDIS_PORT}")
		}
		if enableAuth {
			healthChecker = append(healthChecker, "-a", "${REDIS_PASSWORD}")
		}
		if enableTLS {
			healthChecker = append(healthChecker, "--tls", "--cert", "${REDIS_TLS_CERT}", "--key", "${REDIS_TLS_CERT_KEY}", "--cacert", "${REDIS_TLS_CA_KEY}")
		}
		healthChecker = append(healthChecker, "ping")
		probe.ProbeHandler = corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"sh", "-c", strings.Join(healthChecker, " ")},
			},
		}
	}
	return probe
}

// getEnvironmentVariables returns all the required Environment Variables
func getEnvironmentVariables(role string, enabledPassword *bool, secretName *string,
	secretKey *string, persistenceEnabled *bool, tlsConfig *commonapi.TLSConfig,
	aclConfig *commonapi.ACLConfig, envVar *[]corev1.EnvVar, port *int, clusterVersion *string,
) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{Name: "SERVER_MODE", Value: role},
		{Name: "SETUP_MODE", Value: role},
	}

	if clusterVersion != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REDIS_MAJOR_VERSION",
			Value: *clusterVersion,
		})
	}

	var redisHost string
	if role == "sentinel" {
		redisHost = "redis://localhost:" + strconv.Itoa(common.SentinelPort)
		if port != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "SENTINEL_PORT", Value: strconv.Itoa(*port),
			})
		}
	} else {
		redisHost = "redis://localhost:" + strconv.Itoa(common.RedisPort)
		if port != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "REDIS_PORT", Value: strconv.Itoa(*port),
			})
		}
	}

	if tlsConfig != nil {
		envVars = append(envVars, GenerateTLSEnvironmentVariables(tlsConfig)...)
	}

	if aclConfig != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "ACL_MODE",
			Value: "true",
		})
	}

	envVars = append(envVars, corev1.EnvVar{
		Name:  "REDIS_ADDR",
		Value: redisHost,
	})

	if enabledPassword != nil && *enabledPassword {
		envVars = append(envVars, corev1.EnvVar{
			Name: "REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *secretName,
					},
					Key: *secretKey,
				},
			},
		})
	}
	if persistenceEnabled != nil && *persistenceEnabled {
		envVars = append(envVars, corev1.EnvVar{Name: "PERSISTENCE_ENABLED", Value: "true"})
	}

	if envVar != nil {
		envVars = append(envVars, *envVar...)
	}

	sort.SliceStable(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})
	return envVars
}

// createStatefulSet is a method to create statefulset in Kubernetes
func createStatefulSet(ctx context.Context, cl kubernetes.Interface, namespace string, stateful *appsv1.StatefulSet) error {
	_, err := cl.AppsV1().StatefulSets(namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Redis stateful creation failed")
		return err
	}
	log.FromContext(ctx).V(1).Info("Redis stateful successfully created")
	return nil
}

// updateStatefulSet is a method to update statefulset in Kubernetes
func updateStatefulSet(ctx context.Context, cl kubernetes.Interface, namespace string, stateful *appsv1.StatefulSet, recreateStateFulSet bool, deletePropagation *metav1.DeletionPropagation) error {
	_, err := cl.AppsV1().StatefulSets(namespace).Update(context.TODO(), stateful, metav1.UpdateOptions{})
	if recreateStateFulSet {
		sErr, ok := err.(*apierrors.StatusError)
		if ok && sErr.ErrStatus.Code == 422 && sErr.ErrStatus.Reason == metav1.StatusReasonInvalid {
			failMsg := make([]string, len(sErr.ErrStatus.Details.Causes))
			for messageCount, cause := range sErr.ErrStatus.Details.Causes {
				failMsg[messageCount] = cause.Message
			}
			log.FromContext(ctx).V(1).Info("recreating StatefulSet because the update operation wasn't possible", "reason", strings.Join(failMsg, ", "))
			if err := cl.AppsV1().StatefulSets(namespace).Delete(context.TODO(), stateful.GetName(), metav1.DeleteOptions{PropagationPolicy: deletePropagation}); err != nil { //nolint:gocritic
				return errors.Wrap(err, "failed to delete StatefulSet to avoid forbidden action")
			}
			return nil // rely on the controller to recreate the StatefulSet
		}
	}
	if err != nil {
		log.FromContext(ctx).Error(err, "Redis statefulset update failed")
		return err
	}
	log.FromContext(ctx).V(1).Info("Redis statefulset successfully updated ")
	return nil
}

// GetStateFulSet is a method to get statefulset in Kubernetes
func GetStatefulSet(ctx context.Context, cl kubernetes.Interface, namespace string, name string) (*appsv1.StatefulSet, error) {
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("StatefulSet", "apps/v1"),
	}
	statefulInfo, err := cl.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, getOpts)
	if err != nil {
		log.FromContext(ctx).V(1).Info("Redis statefulset get action failed")
		return nil, err
	}
	log.FromContext(ctx).V(1).Info("Redis statefulset get action was successful")
	return statefulInfo, nil
}

func getSidecars(sidecars *[]commonapi.Sidecar) []commonapi.Sidecar {
	if sidecars == nil {
		return []commonapi.Sidecar{}
	}
	return *sidecars
}

// getDeletionPropagationStrategy returns the deletion propagation strategy based on the annotation
func getDeletionPropagationStrategy(annotations map[string]string) *metav1.DeletionPropagation {
	if annotations == nil {
		return nil
	}

	if strategy, exists := annotations[common.AnnotationKeyRecreateStatefulsetStrategy]; exists {
		var propagation metav1.DeletionPropagation

		switch strings.ToLower(strategy) {
		case "orphan":
			propagation = metav1.DeletePropagationOrphan
		case "background":
			propagation = metav1.DeletePropagationBackground
		default:
			propagation = metav1.DeletePropagationForeground
		}

		return &propagation
	}

	return nil
}
