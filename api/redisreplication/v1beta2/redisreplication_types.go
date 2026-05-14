package v1beta2

import (
	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisReplicationSpec struct {
	Size                          *int32                            `json:"clusterSize"`
	KubernetesConfig              common.KubernetesConfig           `json:"kubernetesConfig"`
	RedisExporter                 *common.RedisExporter             `json:"redisExporter,omitempty"`
	RedisConfig                   *common.RedisConfig               `json:"redisConfig,omitempty"`
	Storage                       *common.Storage                   `json:"storage,omitempty"`
	NodeSelector                  map[string]string                 `json:"nodeSelector,omitempty"`
	PodSecurityContext            *corev1.PodSecurityContext        `json:"podSecurityContext,omitempty"`
	SecurityContext               *corev1.SecurityContext           `json:"securityContext,omitempty"`
	PriorityClassName             string                            `json:"priorityClassName,omitempty"`
	Affinity                      *corev1.Affinity                  `json:"affinity,omitempty"`
	Tolerations                   *[]corev1.Toleration              `json:"tolerations,omitempty"`
	TLS                           *common.TLSConfig                 `json:"TLS,omitempty"`
	PodDisruptionBudget           *common.RedisPodDisruptionBudget  `json:"pdb,omitempty"`
	ACL                           *common.ACLConfig                 `json:"acl,omitempty"`
	ReadinessProbe                *corev1.Probe                     `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe                 *corev1.Probe                     `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	InitContainer                 *common.InitContainer             `json:"initContainer,omitempty"`
	Sidecars                      *[]common.Sidecar                 `json:"sidecars,omitempty"`
	ServiceAccountName            *string                           `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64                            `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	EnvVars                       *[]corev1.EnvVar                  `json:"env,omitempty"`
	TopologySpreadConstrains      []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	HostPort                      *int                              `json:"hostPort,omitempty"`
	Sentinel                      *Sentinel                         `json:"sentinel,omitempty"`
}

type Sentinel struct {
	common.KubernetesConfig `json:",inline"`
	common.SentinelConfig   `json:",inline"`
	Size                    int32 `json:"size"`
}

func (cr *RedisReplicationSpec) GetReplicationCounts(t string) int32 {
	replica := cr.Size
	return *replica
}

// ConnectionInfo provides connection details for clients to connect to Redis
type ConnectionInfo struct {
	// Host is the service FQDN
	Host string `json:"host,omitempty"`
	// Port is the service port
	Port int `json:"port,omitempty"`
	// MasterName is the Sentinel master group name, only set when Sentinel mode is enabled
	// +optional
	MasterName string `json:"masterName,omitempty"`
}

// RedisReplicationStatus defines the observed state of RedisReplication
type RedisReplicationStatus struct {
	// State is the current lifecycle state of the RedisReplication resource.
	State RedisReplicationState `json:"state,omitempty"`
	// Reason provides a human-readable explanation of the current state.
	Reason string `json:"reason,omitempty"`
	// ReadyReplicas is the number of pods that are currently ready.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// MasterNode is the pod name of the current Redis master.
	MasterNode string `json:"masterNode,omitempty"`
	// ConnectionInfo provides connection details for clients to connect to Redis.
	// +optional
	ConnectionInfo *ConnectionInfo `json:"connectionInfo,omitempty"`
}

// RedisReplicationState describes the lifecycle state of a RedisReplication resource.
type RedisReplicationState string

const (
	// RedisReplicationInitializing means the StatefulSet and pods are being created.
	RedisReplicationInitializing RedisReplicationState = "Initializing"
	// RedisReplicationReady means all replicas are up and a master has been elected.
	RedisReplicationReady RedisReplicationState = "Ready"
	// RedisReplicationFailed means the replication setup is unhealthy.
	RedisReplicationFailed RedisReplicationState = "Failed"
)

const (
	InitializingReplicationReason = "RedisReplication is initializing"
	ReadyReplicationReason        = "RedisReplication is ready"
	FailedReplicationReason       = "RedisReplication has no master"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="The current state of the RedisReplication"
// +kubebuilder:printcolumn:name="ReadyReplicas",type="integer",JSONPath=".status.readyReplicas",description="Number of ready replicas"
// +kubebuilder:printcolumn:name="Master",type="string",JSONPath=".status.masterNode"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Redis is the Schema for the redis API
type RedisReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisReplicationSpec   `json:"spec"`
	Status RedisReplicationStatus `json:"status,omitempty"`
}

func (rr *RedisReplication) GetStatefulSetName() string {
	return rr.Name
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
