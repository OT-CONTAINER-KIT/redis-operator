package k8sutils

import (
	"context"
	"fmt"
	"path"
	redisv1beta1 "redis-operator/api/v1beta1"
	"sort"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	redisExporterContainer = "redis-exporter"
)

// statefulSetParameters will define statefulsets input params
type statefulSetParameters struct {
	Replicas              *int32
	Metadata              metav1.ObjectMeta
	NodeSelector          map[string]string
	SecurityContext       *corev1.PodSecurityContext
	PriorityClassName     string
	Affinity              *corev1.Affinity
	Tolerations           *[]corev1.Toleration
	EnableMetrics         bool
	PersistentVolumeClaim corev1.PersistentVolumeClaim
	ImagePullSecrets      *[]corev1.LocalObjectReference
	ExternalConfig        *string
}

// containerParameters will define container input params
type containerParameters struct {
	Image                        string
	ImagePullPolicy              corev1.PullPolicy
	Resources                    *corev1.ResourceRequirements
	RedisExporterImage           string
	RedisExporterImagePullPolicy corev1.PullPolicy
	RedisExporterResources       *corev1.ResourceRequirements
	RedisExporterEnv             *[]corev1.EnvVar
	Role                         string
	EnabledPassword              *bool
	SecretName                   *string
	SecretKey                    *string
	PersistenceEnabled           *bool
	TLSConfig                    *redisv1beta1.TLSConfig
	ReadinessProbe               *redisv1beta1.Probe
	LivenessProbe                *redisv1beta1.Probe
}

// CreateOrUpdateStateFul method will create or update Redis service
func CreateOrUpdateStateFul(namespace string, stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters, sidecars *[]redisv1beta1.Sidecar) error {
	logger := statefulSetLogger(namespace, stsMeta.Name)
	storedStateful, err := GetStatefulSet(namespace, stsMeta.Name)
	statefulSetDef := generateStatefulSetsDef(stsMeta, params, ownerDef, containerParams, getSidecars(sidecars))
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(statefulSetDef); err != nil {
			logger.Error(err, "Unable to patch redis statefulset with comparison object")
			return err
		}
		if errors.IsNotFound(err) {
			return createStatefulSet(namespace, statefulSetDef)
		}
		return err
	}
	return patchStatefulSet(storedStateful, statefulSetDef, namespace)
}

// patchStateFulSet will patch Redis Kubernetes StateFulSet
func patchStatefulSet(storedStateful *appsv1.StatefulSet, newStateful *appsv1.StatefulSet, namespace string) error {
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
		logger.Info("Changes in statefulset Detected, Updating...", "patch", string(patchResult.Patch))
		// Field is immutable therefore we MUST keep it as is.
		if !apiequality.Semantic.DeepEqual(newStateful.Spec.VolumeClaimTemplates, storedStateful.Spec.VolumeClaimTemplates) {
			logger.Error(fmt.Errorf("ignored change in cr.spec.storage.volumeClaimTemplate because it is not supported by statefulset"),
				"Redis statefulset is patched partially")
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
		return updateStatefulSet(namespace, newStateful)
	}
	logger.Info("Reconciliation Complete, no Changes required.")
	return nil
}

// generateStatefulSetsDef generates the statefulsets definition of Redis
func generateStatefulSetsDef(stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters, sidecars []redisv1beta1.Sidecar) *appsv1.StatefulSet {
	statefulset := &appsv1.StatefulSet{
		TypeMeta:   generateMetaInformation("StatefulSet", "apps/v1"),
		ObjectMeta: stsMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector:    LabelSelectors(stsMeta.GetLabels()),
			ServiceName: stsMeta.Name,
			Replicas:    params.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      stsMeta.GetLabels(),
					Annotations: generateStatefulSetsAnots(stsMeta),
					// Annotations: stsMeta.Annotations,
				},
				Spec: corev1.PodSpec{
					Containers:        generateContainerDef(stsMeta.GetName(), containerParams, params.EnableMetrics, params.ExternalConfig, sidecars),
					NodeSelector:      params.NodeSelector,
					SecurityContext:   params.SecurityContext,
					PriorityClassName: params.PriorityClassName,
					Affinity:          params.Affinity,
				},
			},
		},
	}
	if params.Tolerations != nil {
		statefulset.Spec.Template.Spec.Tolerations = *params.Tolerations
	}
	if params.ImagePullSecrets != nil {
		statefulset.Spec.Template.Spec.ImagePullSecrets = *params.ImagePullSecrets
	}
	if containerParams.PersistenceEnabled != nil && *containerParams.PersistenceEnabled {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, createPVCTemplate(stsMeta, params.PersistentVolumeClaim))
	}
	if params.ExternalConfig != nil {
		statefulset.Spec.Template.Spec.Volumes = getExternalConfig(*params.ExternalConfig)
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
func createPVCTemplate(stsMeta metav1.ObjectMeta, storageSpec corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	pvcTemplate := storageSpec
	pvcTemplate.CreationTimestamp = metav1.Time{}
	pvcTemplate.Name = stsMeta.GetName()
	pvcTemplate.Labels = stsMeta.GetLabels()
	// We want the same annoations as the StatefulSet here
	pvcTemplate.Annotations = generateStatefulSetsAnots(stsMeta)
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
func generateContainerDef(name string, containerParams containerParameters, enableMetrics bool, externalConfig *string, sidecars []redisv1beta1.Sidecar) []corev1.Container {
	containerDefinition := []corev1.Container{
		{
			Name:            name,
			Image:           containerParams.Image,
			ImagePullPolicy: containerParams.ImagePullPolicy,
			Env: getEnvironmentVariables(
				containerParams.Role,
				false,
				containerParams.EnabledPassword,
				containerParams.SecretName,
				containerParams.SecretKey,
				containerParams.PersistenceEnabled,
				containerParams.RedisExporterEnv,
				containerParams.TLSConfig,
			),
			ReadinessProbe: getProbeInfo(containerParams.ReadinessProbe),
			LivenessProbe:  getProbeInfo(containerParams.LivenessProbe),
			VolumeMounts:   getVolumeMount(name, containerParams.PersistenceEnabled, externalConfig, containerParams.TLSConfig),
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
		}
		if sidecar.Resources != nil {
			container.Resources = *sidecar.Resources
		}
		if sidecar.EnvVars != nil {
			container.Env = *sidecar.EnvVars
		}
		containerDefinition = append(containerDefinition, container)
	}
	return containerDefinition
}

func GenerateTLSEnvironmentVariables(tlsconfig *redisv1beta1.TLSConfig) []corev1.EnvVar {
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
		Env: getEnvironmentVariables(
			params.Role,
			true,
			params.EnabledPassword,
			params.SecretName,
			params.SecretKey,
			params.PersistenceEnabled,
			params.RedisExporterEnv,
			params.TLSConfig,
		),
		VolumeMounts: getVolumeMount("", nil, nil, params.TLSConfig), // We need/want the tls-certs but we DON'T need the PVC (if one is available)
	}
	if params.RedisExporterResources != nil {
		exporterDefinition.Resources = *params.RedisExporterResources
	}
	return exporterDefinition
}

// getVolumeMount gives information about persistence mount
func getVolumeMount(name string, persistenceEnabled *bool, externalConfig *string, tlsConfig *redisv1beta1.TLSConfig) []corev1.VolumeMount {
	var VolumeMounts []corev1.VolumeMount

	if persistenceEnabled != nil && *persistenceEnabled {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      name,
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

	if externalConfig != nil {
		VolumeMounts = append(VolumeMounts, corev1.VolumeMount{
			Name:      "external-config",
			MountPath: "/etc/redis/external.conf.d",
		})
	}

	return VolumeMounts
}

// getProbeInfo generate probe for Redis StatefulSet
func getProbeInfo(probe *redisv1beta1.Probe) *corev1.Probe {
	if probe == nil {
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"bash",
						"/usr/bin/healthcheck.sh",
					},
				},
			},
		}
	}
	return &corev1.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		FailureThreshold:    probe.FailureThreshold,
		TimeoutSeconds:      probe.TimeoutSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"/usr/bin/healthcheck.sh",
				},
			},
		},
	}
}

// getEnvironmentVariables returns all the required Environment Variables
func getEnvironmentVariables(role string, enabledMetric bool, enabledPassword *bool, secretName *string, secretKey *string, persistenceEnabled *bool, extraEnv *[]corev1.EnvVar, tlsConfig *redisv1beta1.TLSConfig) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{Name: "SERVER_MODE", Value: role},
		{Name: "SETUP_MODE", Value: role},
	}

	redisHost := "redis://localhost:6379"
	if tlsConfig != nil {
		redisHost = "rediss://localhost:6379"
		envVars = append(envVars, GenerateTLSEnvironmentVariables(tlsConfig)...)
		if enabledMetric {
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
		}
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

	if extraEnv != nil {
		envVars = append(envVars, *extraEnv...)
	}
	sort.SliceStable(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})
	return envVars
}

// createStatefulSet is a method to create statefulset in Kubernetes
func createStatefulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := statefulSetLogger(namespace, stateful.Name)
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis stateful creation failed")
		return err
	}
	logger.Info("Redis stateful successfully created")
	return nil
}

// updateStatefulSet is a method to update statefulset in Kubernetes
func updateStatefulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := statefulSetLogger(namespace, stateful.Name)
	// logger.Info(fmt.Sprintf("Setting Statefulset to the following: %s", stateful))
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Update(context.TODO(), stateful, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis stateful update failed")
		return err
	}
	logger.Info("Redis stateful successfully updated ")
	return nil
}

// GetStateFulSet is a method to get statefulset in Kubernetes
func GetStatefulSet(namespace string, stateful string) (*appsv1.StatefulSet, error) {
	logger := statefulSetLogger(namespace, stateful)
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("StatefulSet", "apps/v1"),
	}
	statefulInfo, err := generateK8sClient().AppsV1().StatefulSets(namespace).Get(context.TODO(), stateful, getOpts)
	if err != nil {
		logger.Info("Redis statefulset get action failed")
		return nil, err
	}
	logger.Info("Redis statefulset get action was successful")
	return statefulInfo, nil
}

// statefulSetLogger will generate logging interface for Statfulsets
func statefulSetLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.StatefulSet.Namespace", namespace, "Request.StatefulSet.Name", name)
	return reqLogger
}

func getSidecars(sidecars *[]redisv1beta1.Sidecar) []redisv1beta1.Sidecar {
	if sidecars == nil {
		return []redisv1beta1.Sidecar{}
	}
	return *sidecars
}
