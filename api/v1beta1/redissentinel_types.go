package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisSentinelSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Size                *int32                     `json:"clusterSize"`
	KubernetesConfig    KubernetesConfig           `json:"kubernetesConfig"`
	RedisExporter       *RedisExporter             `json:"redisExporter,omitempty"`
	RedisSentinelConfig *RedisSentinelConfig       `json:"redisSentinelConfig,omitempty"`
	NodeSelector        map[string]string          `json:"nodeSelector,omitempty"`
	PodSecurityContext  *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	SecurityContext     *corev1.SecurityContext    `json:"securityContext,omitempty"`
	PriorityClassName   string                     `json:"priorityClassName,omitempty"`
	Affinity            *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations         *[]corev1.Toleration       `json:"tolerations,omitempty"`
	TLS                 *TLSConfig                 `json:"TLS,omitempty"`
	PodDisruptionBudget *RedisPodDisruptionBudget  `json:"pdb,omitempty"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	LivenessProbe                 *Probe         `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
	InitContainer                 *InitContainer `json:"initContainer,omitempty"`
	Sidecars                      *[]Sidecar     `json:"sidecars,omitempty"`
	ServiceAccountName            *string        `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64         `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	NetworkPolicy                 *NetworkPolicy `json:"networkPolicy,omitempty"`
}

func (cr *RedisSentinelSpec) GetSentinelCounts(t string) int32 {
	replica := cr.Size
	return *replica
}

type RedisSentinelConfig struct {
	AdditionalSentinelConfig *string `json:"additionalSentinelConfig,omitempty"`
	RedisReplicationName     string  `json:"redisReplicationName"`
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

type RedisSentinelStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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

func init() {
	SchemeBuilder.Register(&RedisSentinel{}, &RedisSentinelList{})
}
