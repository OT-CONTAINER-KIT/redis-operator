package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisReplicationSpec struct {
	Size               *int32                     `json:"clusterSize"`
	KubernetesConfig   KubernetesConfig           `json:"kubernetesConfig"`
	RedisExporter      *RedisExporter             `json:"redisExporter,omitempty"`
	RedisConfig        *RedisConfig               `json:"redisConfig,omitempty"`
	Storage            *Storage                   `json:"storage,omitempty"`
	NodeSelector       map[string]string          `json:"nodeSelector,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	SecurityContext    *corev1.SecurityContext    `json:"securityContext,omitempty"`
	PriorityClassName  string                     `json:"priorityClassName,omitempty"`
	Affinity           *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations        *[]corev1.Toleration       `json:"tolerations,omitempty"`
	TLS                *TLSConfig                 `json:"TLS,omitempty"`
	ACL                *ACLConfig                 `json:"acl,omitempty"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	LivenessProbe                 *Probe           `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
	InitContainer                 *InitContainer   `json:"initContainer,omitempty"`
	Sidecars                      *[]Sidecar       `json:"sidecars,omitempty"`
	ServiceAccountName            *string          `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64           `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	EnvVars                       *[]corev1.EnvVar `json:"env,omitempty"`
}

func (cr *RedisReplicationSpec) GetReplicationCounts(t string) int32 {
	replica := cr.Size
	return *replica
}

// RedisStatus defines the observed state of Redis
type RedisReplicationStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Redis is the Schema for the redis API
type RedisReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisReplicationSpec   `json:"spec"`
	Status RedisReplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisList contains a list of Redis
type RedisReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisReplication `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&RedisReplication{}, &RedisReplicationList{})
}
