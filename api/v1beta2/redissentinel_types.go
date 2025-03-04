package v1beta2

import (
	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisSentinelSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Size                          *int32                            `json:"clusterSize"`
	KubernetesConfig              KubernetesConfig                  `json:"kubernetesConfig"`
	RedisExporter                 *RedisExporter                    `json:"redisExporter,omitempty"`
	RedisSentinelConfig           *RedisSentinelConfig              `json:"redisSentinelConfig,omitempty"`
	NodeSelector                  map[string]string                 `json:"nodeSelector,omitempty"`
	PodSecurityContext            *corev1.PodSecurityContext        `json:"podSecurityContext,omitempty"`
	SecurityContext               *corev1.SecurityContext           `json:"securityContext,omitempty"`
	PriorityClassName             string                            `json:"priorityClassName,omitempty"`
	Affinity                      *corev1.Affinity                  `json:"affinity,omitempty"`
	Tolerations                   *[]corev1.Toleration              `json:"tolerations,omitempty"`
	TLS                           *TLSConfig                        `json:"TLS,omitempty"`
	PodDisruptionBudget           *common.RedisPodDisruptionBudget  `json:"pdb,omitempty"`
	ReadinessProbe                *corev1.Probe                     `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe                 *corev1.Probe                     `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	InitContainer                 *InitContainer                    `json:"initContainer,omitempty"`
	Sidecars                      *[]Sidecar                        `json:"sidecars,omitempty"`
	ServiceAccountName            *string                           `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64                            `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	EnvVars                       *[]corev1.EnvVar                  `json:"env,omitempty"`
	VolumeMount                   *common.AdditionalVolume          `json:"volumeMount,omitempty"`
	TopologySpreadConstrains      []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	HostPort                      *int                              `json:"hostPort,omitempty"`
}

func (cr *RedisSentinelSpec) GetSentinelCounts(t string) int32 {
	replica := cr.Size
	return *replica
}

type RedisSentinelConfig struct {
	common.RedisSentinelConfig `json:",inline"`
}

type RedisSentinelStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
//+kubebuilder:storageversion

// Redis is the Schema for the redis API
type RedisSentinel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSentinelSpec   `json:"spec"`
	Status RedisSentinelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisList contains a list of Redis
type RedisSentinelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisSentinel `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&RedisSentinel{}, &RedisSentinelList{})
}
