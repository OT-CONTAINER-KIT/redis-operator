package v1beta2

import (
	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExternalMaster defines an external Redis master endpoint for slave-only deployments.
// When set (non-nil), all pods in this deployment become read-replicas of the specified
// external master instead of having a local master elected within the cluster.
// This is useful for cross-cluster replication where the primary cluster runs a full
// RedisReplication deployment and secondary clusters run slave-only deployments.
// Cannot be combined with Sentinel.
// +k8s:deepcopy-gen=true
type ExternalMaster struct {
	// Host is the DNS name or IP address of the external Redis master.
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// Port is the port of the external Redis master.
	// Defaults to 6379 when omitted.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default:=6379
	Port *int32 `json:"port,omitempty"`
}

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
	// PodManagementPolicy controls how pods are created during initial scale up,
	// when replacing pods on nodes, or when scaling down. This field is immutable
	// on an existing StatefulSet; changing it for a running cluster requires
	// recreating the StatefulSet (e.g. via the
	// redis.opstreelabs.in/recreate-statefulset annotation), otherwise the change
	// is ignored.
	// +optional
	// +kubebuilder:validation:Enum=OrderedReady;Parallel
	PodManagementPolicy *string `json:"podManagementPolicy,omitempty"`
	// ExternalMaster configures slave-only mode where all pods replicate from an
	// external Redis master residing in another cluster. When enabled, no local
	// master is elected, the master-role service is not created, and all
	// leader-election / failover logic is skipped.
	// Cannot be combined with Sentinel.
	ExternalMaster *ExternalMaster `json:"externalMaster,omitempty"`
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

// RedisStatus defines the observed state of Redis
type RedisReplicationStatus struct {
	MasterNode string `json:"masterNode,omitempty"`
	// ConnectionInfo provides connection details for clients to connect to Redis
	// +optional
	ConnectionInfo *ConnectionInfo `json:"connectionInfo,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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
