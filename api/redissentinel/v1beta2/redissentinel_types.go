package v1beta2

import (
	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisSentinelSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Size                          *int32                            `json:"clusterSize"`
	KubernetesConfig              common.KubernetesConfig           `json:"kubernetesConfig"`
	RedisExporter                 *common.RedisExporter             `json:"redisExporter,omitempty"`
	RedisSentinelConfig           *RedisSentinelConfig              `json:"redisSentinelConfig,omitempty"`
	NodeSelector                  map[string]string                 `json:"nodeSelector,omitempty"`
	PodSecurityContext            *corev1.PodSecurityContext        `json:"podSecurityContext,omitempty"`
	SecurityContext               *corev1.SecurityContext           `json:"securityContext,omitempty"`
	PriorityClassName             string                            `json:"priorityClassName,omitempty"`
	Affinity                      *corev1.Affinity                  `json:"affinity,omitempty"`
	Tolerations                   *[]corev1.Toleration              `json:"tolerations,omitempty"`
	TLS                           *common.TLSConfig                 `json:"TLS,omitempty"`
	PodDisruptionBudget           *common.RedisPodDisruptionBudget  `json:"pdb,omitempty"`
	ReadinessProbe                *corev1.Probe                     `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe                 *corev1.Probe                     `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	InitContainer                 *common.InitContainer             `json:"initContainer,omitempty"`
	Sidecars                      *[]common.Sidecar                 `json:"sidecars,omitempty"`
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

// RedisSentinelStatus defines the observed state of RedisSentinel
type RedisSentinelStatus struct {
	// State is the current lifecycle state of the RedisSentinel resource.
	State RedisSentinelState `json:"state,omitempty"`
	// Reason provides a human-readable explanation of the current state.
	Reason string `json:"reason,omitempty"`
	// ReadyReplicas is the number of sentinel pods that are currently ready.
	// +kubebuilder:default=0
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// RedisSentinelState describes the lifecycle state of a RedisSentinel resource.
type RedisSentinelState string

const (
	// RedisSentinelInitializing means the sentinel StatefulSet and pods are being created.
	RedisSentinelInitializing RedisSentinelState = "Initializing"
	// RedisSentinelReady means all sentinel pods are ready and monitoring a master.
	RedisSentinelReady RedisSentinelState = "Ready"
	// RedisSentinelFailed means the sentinel setup is unhealthy.
	RedisSentinelFailed RedisSentinelState = "Failed"
)

const (
	InitializingSentinelReason = "RedisSentinel is initializing"
	ReadySentinelReason        = "RedisSentinel is ready"
	FailedSentinelReason       = "RedisSentinel pods are not ready"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="The current state of the RedisSentinel"
// +kubebuilder:printcolumn:name="ReadyReplicas",type="integer",JSONPath=".status.readyReplicas",description="Number of ready sentinel replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RedisSentinel is the Schema for the redis sentinel API
type RedisSentinel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSentinelSpec   `json:"spec"`
	Status RedisSentinelStatus `json:"status,omitempty"`
}

func (rs *RedisSentinel) GetStatefulSetName() string {
	return rs.Name + "-sentinel"
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
