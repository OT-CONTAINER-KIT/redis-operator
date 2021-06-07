package k8sutils

import (
	"context"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	redisExporterContainer = "redis-exporter"
	graceTime              = 15
)

// statefulSetParameters will define statefulsets input params
type statefulSetParameters struct {
	Replicas              *int32
	NodeSelector          map[string]string
	SecurityContext       *corev1.PodSecurityContext
	PriorityClassName     string
	Affinity              *corev1.Affinity
	Tolerations           *[]corev1.Toleration
	EnableMetrics         bool
	PersistentVolumeClaim corev1.PersistentVolumeClaim
}

// containerParameters will define container input params
type containerParameters struct {
	Image                        string
	ImagePullPolicy              corev1.PullPolicy
	Resources                    *corev1.ResourceRequirements
	RedisExporterImage           string
	RedisExporterImagePullPolicy corev1.PullPolicy
	RedisExporterResources       *corev1.ResourceRequirements
	Role                         string
	EnabledPassword              *bool
	SecretName                   *string
	SecretKey                    *string
	PersistenceEnabled           *bool
}

// CreateOrUpdateService method will create or update Redis service
func CreateOrUpdateStateFul(namespace string, stsMeta metav1.ObjectMeta, labels map[string]string, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters) error {
	logger := stateFulSetLogger(namespace, stsMeta.Name)
	storedStateful, err := getStateFulSet(namespace, stsMeta.Name)
	statefulSetDef := generateStateFulSetsDef(stsMeta, labels, params, ownerDef, containerParams)
	if err != nil {
		if errors.IsNotFound(err) {
			return createStateFulSet(namespace, statefulSetDef)
		}
		return err
	}
	if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(statefulSetDef); err != nil {
		logger.Error(err, "Unable to patch redis statefulset with comparison object")
		return err
	}
	return patchStateFulSet(storedStateful, statefulSetDef, namespace)
}

// patchStateFulSet will patch Redis Kubernetes StateFulSet
func patchStateFulSet(storedStateful *appsv1.StatefulSet, newStateful *appsv1.StatefulSet, namespace string) error {
	logger := stateFulSetLogger(namespace, storedStateful.Name)
	patchResult, err := patch.DefaultPatchMaker.Calculate(storedStateful, newStateful)
	if err != nil {
		logger.Error(err, "Unable to patch redis statefulset with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newStateful); err != nil {
			logger.Error(err, "Unable to patch redis statefulset with comparison object")
			return err
		}
		return updateStateFulSet(namespace, newStateful)
	}
	return nil
}

// generateStateFulSetsDef generates the statefulsets definition of Redis
func generateStateFulSetsDef(stsMeta metav1.ObjectMeta, labels map[string]string, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters) *appsv1.StatefulSet {
	statefulset := &appsv1.StatefulSet{
		TypeMeta:   generateMetaInformation("StatefulSet", "apps/v1"),
		ObjectMeta: stsMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector:    LabelSelectors(labels),
			ServiceName: stsMeta.Name,
			Replicas:    params.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers:        generateContainerDef(stsMeta.Name, containerParams, params.EnableMetrics),
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
	if *containerParams.PersistenceEnabled {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, createPVCTemplate(stsMeta.Name, params.PersistentVolumeClaim))
	}
	AddOwnerRefToObject(statefulset, ownerDef)
	return statefulset
}

// createPVCTemplate will create the persistent volume claim template
func createPVCTemplate(name string, storageSpec corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	var pvcTemplate corev1.PersistentVolumeClaim

	pvcTemplate = storageSpec
	pvcTemplate.CreationTimestamp = metav1.Time{}
	pvcTemplate.Name = name
	if storageSpec.Spec.AccessModes == nil {
		pvcTemplate.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	} else {
		pvcTemplate.Spec.AccessModes = storageSpec.Spec.AccessModes
	}
	pvcTemplate.Spec.Resources = storageSpec.Spec.Resources
	pvcTemplate.Spec.Selector = storageSpec.Spec.Selector
	pvcTemplate.Spec.Selector = storageSpec.Spec.Selector
	return pvcTemplate
}

// generateContainerDef generates container fefinition for Redis
func generateContainerDef(name string, containerParams containerParameters, enableMetrics bool) []corev1.Container {
	containerDefinition := []corev1.Container{
		{
			Name:            name,
			Image:           containerParams.Image,
			ImagePullPolicy: containerParams.ImagePullPolicy,
			Env:             getEnvironmentVariables(containerParams.Role, containerParams.EnabledPassword, containerParams.SecretName, containerParams.SecretKey, containerParams.PersistenceEnabled),
			Resources:       *containerParams.Resources,
			ReadinessProbe:  getProbeInfo(),
			LivenessProbe:   getProbeInfo(),
			VolumeMounts:    getVolumeMount(name, containerParams.PersistenceEnabled),
		},
	}
	containerDefinition = append(containerDefinition, enableRedisMonitoring(containerParams))
	return containerDefinition
}

// enableRedisMonitoring will add Redis Exporter as sidecar container
func enableRedisMonitoring(params containerParameters) corev1.Container {
	exporterDefinition := corev1.Container{
		Name:            redisExporterContainer,
		Image:           params.RedisExporterImage,
		ImagePullPolicy: params.RedisExporterImagePullPolicy,
		Env:             getEnvironmentVariables(params.Role, params.EnabledPassword, params.SecretName, params.SecretKey, params.PersistenceEnabled),
		Resources:       *params.RedisExporterResources,
	}
	return exporterDefinition
}

// getVolumeMount gives information about persistence mount
func getVolumeMount(name string, persistenceEnabled *bool) []corev1.VolumeMount {
	var VolumeMounts []corev1.VolumeMount
	if *persistenceEnabled && persistenceEnabled != nil {
		VolumeMounts = []corev1.VolumeMount{
			{
				Name:      name,
				MountPath: "/data",
			},
		}
		return VolumeMounts
	}
	return VolumeMounts
}

// getProbeInfo generates probe information for Redis
func getProbeInfo() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: graceTime,
		PeriodSeconds:       15,
		FailureThreshold:    5,
		TimeoutSeconds:      5,
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"/usr/bin/healthcheck.sh",
				},
			},
		},
	}
}

// getResources will get the resource information for Redis container
// func getResources(resources *redisv1beta1.Resources) *corev1.ResourceRequirements {
// 	var resourceStruct *corev1.ResourceRequirements
// 	if resources != nil {
// 		resourceStruct.Limits[corev1.ResourceCPU] = resource.MustParse(resources.ResourceLimits.CPU)
// 		resourceStruct.Requests[corev1.ResourceCPU] = resource.MustParse(resources.ResourceRequests.CPU)
// 		resourceStruct.Limits[corev1.ResourceMemory] = resource.MustParse(resources.ResourceLimits.Memory)
// 		resourceStruct.Requests[corev1.ResourceMemory] = resource.MustParse(resources.ResourceRequests.Memory)
// 		return resourceStruct
// 	}
// 	return nil
// }

// getEnvironmentVariables returns all the required Environment Variables
func getEnvironmentVariables(role string, enabledPassword *bool, secretName *string, secretKey *string, persistenceEnabled *bool) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{Name: "SERVER_MODE", Value: role},
		{Name: "SETUP_MODE", Value: role},
		{Name: "REDIS_ADDR", Value: "redis://localhost:6379"},
	}
	if *enabledPassword && enabledPassword != nil {
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
	if *persistenceEnabled && persistenceEnabled != nil {
		envVars = append(envVars, corev1.EnvVar{Name: "PERSISTENCE_ENABLED", Value: "true"})
	}
	return envVars
}

// createStateFulSet is a method to create statefulset in Kubernetes
func createStateFulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := stateFulSetLogger(namespace, stateful.Name)
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis stateful creation is failed")
		return err
	}
	logger.Info("Redis stateful creation is successful")
	return nil
}

// updateStateFulSet is a method to update statefulset in Kubernetes
func updateStateFulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := stateFulSetLogger(namespace, stateful.Name)
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Update(context.TODO(), stateful, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis stateful updation is failed")
		return err
	}
	logger.Info("Redis stateful updation is successful")
	return nil
}

// getStateFulSet is a method to get statefulset in Kubernetes
func getStateFulSet(namespace string, stateful string) (*appsv1.StatefulSet, error) {
	logger := stateFulSetLogger(namespace, stateful)
	statefulInfo, err := generateK8sClient().AppsV1().StatefulSets(namespace).Get(context.TODO(), stateful, metav1.GetOptions{})
	if err != nil {
		logger.Info("Redis statefulset get action is failed")
		return nil, err
	}
	logger.Info("Redis statefulset get action is successful")
	return statefulInfo, err
}

// stateFulSetLogger will generate logging interface for Statfulsets
func stateFulSetLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.StateFulSet.Namespace", namespace, "Request.StateFulSet.Name", name)
	return reqLogger
}
