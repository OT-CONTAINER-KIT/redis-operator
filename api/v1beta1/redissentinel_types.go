package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisSentinelSpec struct {
	// +kubebuilder:validation:Minimum=3
	Size             *int32           `json:"clusterSize"`
	KubernetesConfig KubernetesConfig `json:"kubernetesConfig"`
	// RedisExporter    *RedisExporter   `json:"redisExporter,omitempty"`
	RedisSnt *RedisSnt `json:"redisSentinel,omitempty"`
	// Storage in Sentinel to be removed
	// Storage           *Storage                   `json:"storage,omitempty"`
	NodeSelector      map[string]string          `json:"nodeSelector,omitempty"`
	SecurityContext   *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	PriorityClassName string                     `json:"priorityClassName,omitempty"`
	Affinity          *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations       *[]corev1.Toleration       `json:"tolerations,omitempty"`
	TLS               *TLSConfig                 `json:"TLS,omitempty"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	LivenessProbe      *Probe     `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
	Sidecars           *[]Sidecar `json:"sidecars,omitempty"`
	ServiceAccountName *string    `json:"serviceAccountName,omitempty"`
}

func (cr *RedisSentinelSpec) GetSentinelCounts(t string) int32 {
	replica := cr.Size
	if t == "sentinel" && cr.RedisSnt.Replicas != nil {
		replica = cr.RedisSnt.Replicas
	}
	return *replica
}

type RedisSnt struct {
	RedisClusterName string `json:"redisClusterName,omitempty"`
	// +kubebuilder:validation:Minimum=3
	Replicas            *int32                    `json:"replicas,omitempty"`
	RedisSentinelConfig *RedisSentinelConfig      `json:"redisSentinelConfig,omitempty"`
	RedisConfig         *RedisConfig              `json:"redisConfig,omitempty"`
	Affinity            *corev1.Affinity          `json:"affinity,omitempty"`
	PodDisruptionBudget *RedisPodDisruptionBudget `json:"pdb,omitempty"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	// +kubebuilder:default:={initialDelaySeconds: 1, timeoutSeconds: 1, periodSeconds: 10, successThreshold: 1, failureThreshold:3}
	LivenessProbe *Probe `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
}

type RedisSentinelConfig struct {
	MasterGroupName       string `json:"masterGroupName,omitempty"`
	RedisName             string `json:"redisName,omitempty"`
	RedisPort             int32  `json:"redisPort,omitempty"`
	Quorum                int32  `json:"quorum,omitempty"`
	ParallelSyncs         int32  `json:"parallelSyncs,omitempty"`
	FailoverTimeout       int32  `json:"failoverTimeout,omitempty"`
	DownAfterMilliseconds int32  `json:"downAfterMilliseconds,omitempty"`
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
	Items           []Redis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisSentinel{}, &RedisSentinelList{})
}
