package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RedisSpec will define the interface for Redis Configuration Input Values
type RedisSpec struct {
	Mode              string                     `json:"mode"`
	Size              *int32                     `json:"size,omitempty"`
	GlobalConfig      GlobalConfig               `json:"global"`
	Master            RedisMaster                `json:"master,omitempty"`
	Slave             RedisSlave                 `json:"slave,omitempty"`
	RedisExporter     *RedisExporter             `json:"redisExporter,omitempty"`
	RedisConfig       map[string]string          `json:"redisConfig"`
	Resources         *Resources                 `json:"resources,omitempty"`
	Storage           *Storage                   `json:"storage,omitempty"`
	NodeSelector      map[string]string          `json:"nodeSelector,omitempty"`
	SecurityContext   *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	PriorityClassName string                     `json:"priorityClassName,omitempty"`
	Affinity          *corev1.Affinity           `json:"affinity,omitempty"`
}

// Storage is the inteface to add pvc and pv support in redis
type Storage struct {
	VolumeClaimTemplate corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
}

// RedisMaster interface will have the redis master configuration
type RedisMaster struct {
	Resources   Resources         `json:"resources,omitempty"`
	RedisConfig map[string]string `json:"redisConfig"`
}

// RedisExporter interface will have the information for redis exporter related stuff
type RedisExporter struct {
	Enabled         bool              `json:"enabled,omitempty"`
	Image           string            `json:"image"`
	Resources       *Resources        `json:"resources,omitempty"`
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// GlobalConfig will be the JSON struct for Basic Redis Config
type GlobalConfig struct {
	Image           string            `json:"image"`
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	Password        *string           `json:"password,omitempty"`
	Resources       *Resources        `json:"resources,omitempty"`
}

// RedisSlave interface will have the redis slave configuration
type RedisSlave struct {
	Resources   Resources         `json:"resources,omitempty"`
	RedisConfig map[string]string `json:"redisConfig"`
}

// ResourceDescription describes CPU and memory resources defined for a cluster.
type ResourceDescription struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// Resources describes requests and limits for the cluster resouces.
type Resources struct {
	ResourceRequests ResourceDescription `json:"requests,omitempty"`
	ResourceLimits   ResourceDescription `json:"limits,omitempty"`
}

// RedisStatus will give the descriptive information for Redis status
type RedisStatus struct {
	Cluster RedisSpec `json:"cluster,omitempty"`
}

// Redis will give the descriptive information for Redis
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec,omitempty"`
	Status RedisStatus `json:"status,omitempty"`
}

// RedisList will give the list of Redis
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}
