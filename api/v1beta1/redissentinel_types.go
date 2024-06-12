package v1beta1

import (
	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisSentinelSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Default=3
	// +kubebuilder:validation:Not=2
	Size                *int32                           `json:"clusterSize"`
	KubernetesConfig    KubernetesConfig                 `json:"kubernetesConfig"`
	RedisSentinelConfig *RedisSentinelConfig             `json:"redisSentinelConfig,omitempty"`
	NodeSelector        map[string]string                `json:"nodeSelector,omitempty"`
	SecurityContext     *corev1.PodSecurityContext       `json:"securityContext,omitempty"`
	PriorityClassName   string                           `json:"priorityClassName,omitempty"`
	Affinity            *corev1.Affinity                 `json:"affinity,omitempty"`
	Tolerations         *[]corev1.Toleration             `json:"tolerations,omitempty"`
	TLS                 *TLSConfig                       `json:"TLS,omitempty"`
	PodDisruptionBudget *common.RedisPodDisruptionBudget `json:"pdb,omitempty"`
	ReadinessProbe      *corev1.Probe                    `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe       *corev1.Probe                    `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	Sidecars            *[]Sidecar                       `json:"sidecars,omitempty"`
	ServiceAccountName  *string                          `json:"serviceAccountName,omitempty"`
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
