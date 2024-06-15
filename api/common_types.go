package api

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// KubernetesConfig will be the JSON struct for Basic Redis Config
// +k8s:deepcopy-gen=true
type KubernetesConfig struct {
	Image                  string                           `json:"image"`
	ImagePullPolicy        corev1.PullPolicy                `json:"imagePullPolicy,omitempty"`
	Resources              *corev1.ResourceRequirements     `json:"resources,omitempty"`
	ExistingPasswordSecret *ExistingPasswordSecret          `json:"redisSecret,omitempty"`
	ImagePullSecrets       *[]corev1.LocalObjectReference   `json:"imagePullSecrets,omitempty"`
	UpdateStrategy         appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	Service                *ServiceConfig                   `json:"service,omitempty"`
	IgnoreAnnotations      []string                         `json:"ignoreAnnotations,omitempty"`
}

// ServiceConfig define the type of service to be created and its annotations
// +k8s:deepcopy-gen=true
type ServiceConfig struct {
	// +kubebuilder:validation:Enum=LoadBalancer;NodePort;ClusterIP
	ServiceType        string            `json:"serviceType,omitempty"`
	ServiceAnnotations map[string]string `json:"annotations,omitempty"`
}

// ExistingPasswordSecret is the struct to access the existing secret
// +k8s:deepcopy-gen=true
type ExistingPasswordSecret struct {
	Name *string `json:"name,omitempty"`
	Key  *string `json:"key,omitempty"`
}

// RedisExporter interface will have the information for redis exporter related stuff
// +k8s:deepcopy-gen=true
type RedisExporter struct {
	Enabled bool `json:"enabled,omitempty"`
	// +kubebuilder:default:=9121
	Port            *int                         `json:"port,omitempty"`
	Image           string                       `json:"image"`
	Resources       *corev1.ResourceRequirements `json:"resources,omitempty"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	EnvVars         *[]corev1.EnvVar             `json:"env,omitempty"`
	SecurityContext *corev1.SecurityContext      `json:"securityContext,omitempty"`
}

// RedisConfig defines the external configuration of Redis
// +k8s:deepcopy-gen=true
type RedisConfig struct {
	AdditionalRedisConfig *string `json:"additionalRedisConfig,omitempty"`
}

// Storage is the inteface to add pvc and pv support in redis
// +k8s:deepcopy-gen=true
type Storage struct {
	KeepAfterDelete     bool                         `json:"keepAfterDelete,omitempty"`
	VolumeClaimTemplate corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
	VolumeMount         AdditionalVolume             `json:"volumeMount,omitempty"`
}

// Additional Volume is provided by user that is mounted on the pods
// +k8s:deepcopy-gen=true
type AdditionalVolume struct {
	Volume    []corev1.Volume      `json:"volume,omitempty"`
	MountPath []corev1.VolumeMount `json:"mountPath,omitempty"`
}

// TLS Configuration for redis instances
// +k8s:deepcopy-gen=true
type TLSConfig struct {
	CaKeyFile   string `json:"ca,omitempty"`
	CertKeyFile string `json:"cert,omitempty"`
	KeyFile     string `json:"key,omitempty"`
	// Reference to secret which contains the certificates
	Secret corev1.SecretVolumeSource `json:"secret"`
}

// Sidecar for each Redis pods
// +k8s:deepcopy-gen=true
type Sidecar struct {
	Name            string                       `json:"name"`
	Image           string                       `json:"image"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	Resources       *corev1.ResourceRequirements `json:"resources,omitempty"`
	EnvVars         *[]corev1.EnvVar             `json:"env,omitempty"`
}

// RedisLeader interface will have the redis leader configuration
// +k8s:deepcopy-gen=true
type RedisLeader struct {
	Replicas            *int32                    `json:"replicas,omitempty"`
	RedisConfig         *RedisConfig              `json:"redisConfig,omitempty"`
	Affinity            *corev1.Affinity          `json:"affinity,omitempty"`
	PodDisruptionBudget *RedisPodDisruptionBudget `json:"pdb,omitempty"`
	ReadinessProbe      *corev1.Probe             `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe       *corev1.Probe             `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	Tolerations         *[]corev1.Toleration      `json:"tolerations,omitempty"`
	NodeSelector        map[string]string         `json:"nodeSelector,omitempty"`
}

// RedisFollower interface will have the redis follower configuration
// +k8s:deepcopy-gen=true
type RedisFollower struct {
	Replicas            *int32                    `json:"replicas,omitempty"`
	RedisConfig         *RedisConfig              `json:"redisConfig,omitempty"`
	Affinity            *corev1.Affinity          `json:"affinity,omitempty"`
	PodDisruptionBudget *RedisPodDisruptionBudget `json:"pdb,omitempty"`
	ReadinessProbe      *corev1.Probe             `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe       *corev1.Probe             `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	Tolerations         *[]corev1.Toleration      `json:"tolerations,omitempty"`
	NodeSelector        map[string]string         `json:"nodeSelector,omitempty"`
}

// RedisPodDisruptionBudget configure a PodDisruptionBudget on the resource (leader/follower)
// +k8s:deepcopy-gen=true
type RedisPodDisruptionBudget struct {
	Enabled        bool   `json:"enabled,omitempty"`
	MinAvailable   *int32 `json:"minAvailable,omitempty"`
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

// +k8s:deepcopy-gen=true
type RedisSentinelConfig struct {
	AdditionalSentinelConfig *string              `json:"additionalSentinelConfig,omitempty"`
	RedisReplicationName     string               `json:"redisReplicationName"`
	RedisReplicationPassword *corev1.EnvVarSource `json:"redisReplicationPassword,omitempty"`
	// +kubebuilder:default:=myMaster
	MasterGroupName string `json:"masterGroupName,omitempty"`
	// +kubebuilder:default:="6379"
	RedisPort string `json:"redisPort,omitempty"`
	// +kubebuilder:default:="2"
	Quorum string `json:"quorum,omitempty"`
	// +kubebuilder:default:="1"
	ParallelSyncs string `json:"parallelSyncs,omitempty"`
	// +kubebuilder:default:="180000"
	FailoverTimeout string `json:"failoverTimeout,omitempty"`
	// +kubebuilder:default:="30000"
	DownAfterMilliseconds string `json:"downAfterMilliseconds,omitempty"`
}
