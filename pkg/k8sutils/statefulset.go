package k8sutils

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/env"
	"k8s.io/utils/ptr"
)

type StatefulSet interface {
	IsStatefulSetReady(ctx context.Context, namespace, name string) bool
}

type StatefulSetService struct {
	kubeClient kubernetes.Interface
	log        logr.Logger
}

func NewStatefulSetService(kubeClient kubernetes.Interface, log logr.Logger) *StatefulSetService {
	log = log.WithValues("service", "k8s.statefulset")
	return &StatefulSetService{
		kubeClient: kubeClient,
		log:        log,
	}
}

func (s *StatefulSetService) IsStatefulSetReady(ctx context.Context, namespace, name string) bool {
	var (
		partition = 0
		replicas  = 1

		logger = s.log.WithValues("namespace", namespace, "name", name)
	)

	sts, err := s.kubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "failed to get statefulset")
		return false
	}

	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition = int(*sts.Spec.UpdateStrategy.RollingUpdate.Partition)
	}
	if sts.Spec.Replicas != nil {
		replicas = int(*sts.Spec.Replicas)
	}

	if expectedUpdateReplicas := replicas - partition; sts.Status.UpdatedReplicas < int32(expectedUpdateReplicas) {
		logger.V(1).Info("StatefulSet is not ready", "Status.UpdatedReplicas", sts.Status.UpdatedReplicas, "ExpectedUpdateReplicas", expectedUpdateReplicas)
		return false
	}
	if partition == 0 && sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		logger.V(1).Info("StatefulSet is not ready", "Status.CurrentRevision", sts.Status.CurrentRevision, "Status.UpdateRevision", sts.Status.UpdateRevision)
		return false
	}
	if sts.Status.ObservedGeneration != sts.ObjectMeta.Generation {
		logger.V(1).Info("StatefulSet is not ready", "Status.ObservedGeneration", sts.Status.ObservedGeneration, "ObjectMeta.Generation", sts.ObjectMeta.Generation)
		return false
	}
	if int(sts.Status.ReadyReplicas) != replicas {
		logger.V(1).Info("StatefulSet is not ready", "Status.ReadyReplicas", sts.Status.ReadyReplicas, "Replicas", replicas)
		return false
	}
	return true
}

const (
	redisExporterContainer = "redis-exporter"
)

// statefulSetParameters will define statefulsets input params
type statefulSetParameters struct {
	Replicas                      *int32
	ClusterMode                   bool
	ClusterVersion                *string
	NodeConfVolume                bool
	NodeSelector                  map[string]string
	PodSecurityContext            *corev1.PodSecurityContext
	PriorityClassName             string
	Affinity                      *corev1.Affinity
	Tolerations                   *[]corev1.Toleration
	EnableMetrics                 bool
	PersistentVolumeClaim         corev1.PersistentVolumeClaim
	NodeConfPersistentVolumeClaim corev1.PersistentVolumeClaim
	ImagePullSecrets              *[]corev1.LocalObjectReference
	ExternalConfig                *string
	ServiceAccountName            *string
	UpdateStrategy                appsv1.StatefulSetUpdateStrategy
	RecreateStatefulSet           bool
	TerminationGracePeriodSeconds *int64
	IgnoreAnnotations             []string
	HostNetwork                   bool
	MinReadySeconds               int32
}

// containerParameters will define container input params
type containerParameters struct {
	Image                        string
	ImagePullPolicy              corev1.PullPolicy
	Resources                    *corev1.ResourceRequirements
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
	TLSConfig                    *redisv1beta2.TLSConfig
	ACLConfig                    *redisv1beta2.ACLConfig
	ReadinessProbe               *corev1.Probe
	LivenessProbe                *corev1.Probe
	AdditionalEnvVariable        *[]corev1.EnvVar
	AdditionalVolume             []corev1.Volume
	AdditionalMountPath          []corev1.VolumeMount
	EnvVars                      *[]corev1.EnvVar
	Port                         *int
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
func CreateOrUpdateStateFul(cl kubernetes.Interface, logger logr.Logger, namespace string, stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, initcontainerParams initContainerParameters, containerParams containerParameters, sidecars *[]redisv1beta2.Sidecar) error {
	storedStateful, err := GetStatefulSet(cl, logger, namespace, stsMeta.Name)
	statefulSetDef := generateStatefulSetsDef(stsMeta, params, ownerDef, initcontainerParams, containerParams, getSidecars(sidecars))
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(statefulSetDef); err != nil { //nolint
			logger.Error(err, "Unable to patch redis statefulset with comparison object")
			return err
		}
		if apierrors.IsNotFound(err) {
			return createStatefulSet(cl, logger, namespace, statefulSetDef)
		}
		return err
	}
	return patchStatefulSet(storedStateful, statefulSetDef, namespace, params.RecreateStatefulSet, cl)
}

// patchStateFulSet will patch Redis Kubernetes StateFulSet
func patchStatefulSet(storedStateful *appsv1.StatefulSet, newStateful *appsv1.StatefulSet, namespace string, recreateStateFulSet bool, cl kubernetes.Interface) error {
	logger := statefulSetLogger(namespace, storedStateful.Name)
	// We want to try and keep this atomic as possible.
	newStateful.ResourceVersion = storedStateful.ResourceVersion
	newStateful.CreationTimestamp = storedStateful.CreationTimestamp
	newStateful.ManagedFields = storedStateful.ManagedFields

	patchResult, err := patch.DefaultPatchMaker.Calculate(storedStateful, newStateful,
		patch.IgnoreStatusFields(),
		patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus(),
		patch.IgnoreField("kind"),
		patch.IgnoreField("apiVersion"),
	)
	if err != nil {
		logger.Error(err, "Unable to patch redis statefulset with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		logger.V(1).Info("Changes in statefulset Detected, Updating...", "patch", string(patchResult.Patch))
		if len(newStateful.Spec.VolumeClaimTemplates) >= 1 && len(newStateful.Spec.VolumeClaimTemplates) == len(storedStateful.Spec.VolumeClaimTemplates) {
			// Field is immutable therefore we MUST keep it as is.
			if !apiequality.Semantic.DeepEqual(newStateful.Spec.VolumeClaimTemplates[0].Spec, storedStateful.Spec.VolumeClaimTemplates[0].Spec) {
				// resize pvc
				// 1.Get the data already stored internally
				// 2.Get the desired data
				// 3.Start querying the pvc list when you find data inconsistencies
				// 3.1 Comparison using real pvc capacity and desired data
				// 3.1.1 Update if you find inconsistencies
				// 3.2 Writing successful updates to internal
				// 4. Set to old VolumeClaimTemplates to update.Prevent update error reporting
				// 5. Set to old annotations to update
				annotations := storedStateful.Annotations
				if annotations == nil {
					annotations = map[string]string{
						"storageCapacity": "0",
					}
				}
				storedCapacity, _ := strconv.ParseInt(annotations["storageCapacity"], 0, 64)
				if len(newStateful.Spec.VolumeClaimTemplates) != 0 {
					stateCapacity := newStateful.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value()
					if storedCapacity != stateCapacity {
						listOpt := metav1.ListOptions{
							LabelSelector: labels.FormatLabels(
								map[string]string{
									"app":                         storedStateful.Name,
									"app.kubernetes.io/component": "redis",
								},
							),
						}
						pvcs, err := cl.CoreV1().PersistentVolumeClaims(storedStateful.Namespace).List(context.Background(), listOpt)
						if err != nil {
							return err
						}
						updateFailed := false
						realUpdate := false
						for i := range pvcs.Items {
							pvc := &pvcs.Items[i]
							realCapacity := pvc.Spec.Resources.Requests.Storage().Value()
							if realCapacity != stateCapacity {
								realUpdate = true
								pvc.Spec.Resources.Requests = newStateful.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests
								_, err = cl.CoreV1().PersistentVolumeClaims(storedStateful.Namespace).Update(context.Background(), pvc, metav1.UpdateOptions{})
								if err != nil {
									if !updateFailed {
										updateFailed = true
									}
									logger.Error(fmt.Errorf("redis:%s resize pvc failed:%s", storedStateful.Name, err.Error()), "")
								}
							}
						}

						if !updateFailed && len(pvcs.Items) != 0 {
							annotations["storageCapacity"] = fmt.Sprintf("%d", stateCapacity)
							storedStateful.Annotations = annotations
							if realUpdate {
								logger.Info(fmt.Sprintf("redis:%s resize pvc from  %d to %d", storedStateful.Name, storedCapacity, stateCapacity))
							} else {
								logger.Info(fmt.Sprintf("redis:%s resize noting,just set annotations", storedStateful.Name))
							}
						}
					}
				}
			}
			newStateful.Annotations["storageCapacity"] = storedStateful.Annotations["storageCapacity"]
			// set stored.volumeClaimTemplates
			newStateful.Spec.VolumeClaimTemplates = storedStateful.Spec.VolumeClaimTemplates
		}

		for key, value := range storedStateful.Annotations {
			if _, present := newStateful.Annotations[key]; !present {
				newStateful.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newStateful); err != nil {
			logger.Error(err, "Unable to patch redis statefulset with comparison object")
			return err
		}
		return updateStatefulSet(cl, logger, namespace, newStateful, recreateStateFulSet)
	}
	logger.V(1).Info("Reconciliation Complete, no Changes required.")
	return nil
}

// generateStatefulSetsDef generates the statefulsets definition of Redis
func generateStatefulSetsDef(stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, initcontainerParams initContainerParameters, containerParams containerParameters, sidecars []redisv1beta2.Sidecar) *appsv1.StatefulSet {
	statefulset := &appsv1.StatefulSet{
		TypeMeta:   generateMetaInformation("StatefulSet", "apps/v1"),
		ObjectMeta: stsMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector:        LabelSelectors(stsMeta.GetLabels()),
			ServiceName:     fmt.Sprintf("%s-headless", stsMeta.Name),
			Replicas:        params.Replicas,
			UpdateStrategy:  params.UpdateStrategy,
			MinReadySeconds: params.MinReadySeconds,
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
					SecurityContext:               params.PodSecurityContext,
					PriorityClassName:             params.PriorityClassName,
					Affinity:                      params.Affinity,
					TerminationGracePeriodSeconds: params.TerminationGracePeriodSeconds,
					HostNetwork:                   params.HostNetwork,
				},
			},
		},
	}

	if initcontainerParams.Enabled != nil && *initcontainerParams.Enabled {
		statefulset.Spec.Template.Spec.InitContainers = generateInitContainerDef(stsMeta.GetName(), initcontainerParams, initcontainerParams.AdditionalMountPath)
	}
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
		pvcTplName := env.GetString(EnvOperatorSTSPVCTemplateName, stsMeta.GetName())
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
func generateContainerDef(name string, containerParams containerParameters, clusterMode, nodeConfVolume, enableMetrics bool, externalConfig, clusterVersion *string, mountpath []corev1.VolumeMount, sidecars []redisv1beta2.Sidecar) []corev1.Container {
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

func generateInitContainerDef(name string, initcontainerParams initContainerParameters, mountpath []corev1.VolumeMount) []corev1.Container {
	initcontainerDefinition := []corev1.Container{
		{
			Name:            "init" + name,
			Image:           initcontainerParams.Image,
			ImagePullPolicy: initcontainerParams.ImagePullPolicy,
			Command:         initcontainerParams.Command,
			Args:            initcontainerParams.Arguments,
			VolumeMounts:    getVolumeMount(name, initcontainerParams.PersistenceEnabled, false, false, nil, mountpath, nil, nil),
			SecurityContext: initcontainerParams.SecurityContext,
		},
	}

	if initcontainerParams.Resources != nil {
		initcontainerDefinition[0].Resources = *initcontainerParams.Resources
	}

	if initcontainerParams.AdditionalEnvVariable != nil {
		initcontainerDefinition[0].Env = append(initcontainerDefinition[0].Env, *initcontainerParams.AdditionalEnvVariable...)
	}

	return initcontainerDefinition
}

func GenerateTLSEnvironmentVariables(tlsconfig *redisv1beta2.TLSConfig) []corev1.EnvVar {
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
				Name:          redisExporterPortName,
				ContainerPort: int32(*util.Coalesce(params.RedisExporterPort, ptr.To(redisExporterPort))),
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
func getVolumeMount(name string, persistenceEnabled *bool, clusterMode bool, nodeConfVolume bool, externalConfig *string, mountpath []corev1.VolumeMount, tlsConfig *redisv1beta2.TLSConfig, aclConfig *redisv1beta2.ACLConfig) []corev1.VolumeMount {
	var VolumeMounts []corev1.VolumeMount

	if persistenceEnabled != nil && clusterMode && nodeConfVolume {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "node-conf",
			MountPath: "/node-conf",
		})
	}

	if persistenceEnabled != nil && *persistenceEnabled {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      env.GetString(EnvOperatorSTSPVCTemplateName, name),
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

	VolumeMounts = append(VolumeMounts, mountpath...)

	return VolumeMounts
}

// getProbeInfo generate probe for Redis StatefulSet
func getProbeInfo(probe *corev1.Probe, sentinel, enableTLS, enableAuth bool) *corev1.Probe {
	if probe == nil {
		probe = &corev1.Probe{}
	}
	if probe.ProbeHandler.Exec == nil && probe.ProbeHandler.HTTPGet == nil && probe.ProbeHandler.TCPSocket == nil && probe.ProbeHandler.GRPC == nil {
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
	secretKey *string, persistenceEnabled *bool, tlsConfig *redisv1beta2.TLSConfig,
	aclConfig *redisv1beta2.ACLConfig, envVar *[]corev1.EnvVar, port *int, clusterVersion *string,
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
		redisHost = "redis://localhost:" + strconv.Itoa(sentinelPort)
		if port != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "SENTINEL_PORT", Value: strconv.Itoa(*port),
			})
		}
	} else {
		redisHost = "redis://localhost:" + strconv.Itoa(redisPort)
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
func createStatefulSet(cl kubernetes.Interface, logger logr.Logger, namespace string, stateful *appsv1.StatefulSet) error {
	_, err := cl.AppsV1().StatefulSets(namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis stateful creation failed")
		return err
	}
	logger.V(1).Info("Redis stateful successfully created")
	return nil
}

// updateStatefulSet is a method to update statefulset in Kubernetes
func updateStatefulSet(cl kubernetes.Interface, logger logr.Logger, namespace string, stateful *appsv1.StatefulSet, recreateStateFulSet bool) error {
	_, err := cl.AppsV1().StatefulSets(namespace).Update(context.TODO(), stateful, metav1.UpdateOptions{})
	if recreateStateFulSet {
		sErr, ok := err.(*apierrors.StatusError)
		if ok && sErr.ErrStatus.Code == 422 && sErr.ErrStatus.Reason == metav1.StatusReasonInvalid {
			failMsg := make([]string, len(sErr.ErrStatus.Details.Causes))
			for messageCount, cause := range sErr.ErrStatus.Details.Causes {
				failMsg[messageCount] = cause.Message
			}
			logger.V(1).Info("recreating StatefulSet because the update operation wasn't possible", "reason", strings.Join(failMsg, ", "))
			propagationPolicy := metav1.DeletePropagationForeground
			if err := cl.AppsV1().StatefulSets(namespace).Delete(context.TODO(), stateful.GetName(), metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil { //nolint
				return errors.Wrap(err, "failed to delete StatefulSet to avoid forbidden action")
			}
		}
	}
	if err != nil {
		logger.Error(err, "Redis statefulset update failed")
		return err
	}
	logger.V(1).Info("Redis statefulset successfully updated ")
	return nil
}

// GetStateFulSet is a method to get statefulset in Kubernetes
func GetStatefulSet(cl kubernetes.Interface, logger logr.Logger, namespace string, name string) (*appsv1.StatefulSet, error) {
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("StatefulSet", "apps/v1"),
	}
	statefulInfo, err := cl.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, getOpts)
	if err != nil {
		logger.V(1).Info("Redis statefulset get action failed")
		return nil, err
	}
	logger.V(1).Info("Redis statefulset get action was successful")
	return statefulInfo, nil
}

// statefulSetLogger will generate logging interface for Statfulsets
func statefulSetLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.StatefulSet.Namespace", namespace, "Request.StatefulSet.Name", name)
	return reqLogger
}

func getSidecars(sidecars *[]redisv1beta2.Sidecar) []redisv1beta2.Sidecar {
	if sidecars == nil {
		return []redisv1beta2.Sidecar{}
	}
	return *sidecars
}
