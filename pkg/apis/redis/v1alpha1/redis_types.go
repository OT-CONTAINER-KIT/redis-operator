package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RedisSpec will define the interface for Redis Configuration Input Values
type RedisSpec struct {
	Mode               string            `json:"mode"`
	ImageName          string            `json:"imageName,omitempty"`
	Size               *int32            `json:"size,omitempty"`
	ImagePullPolicy    corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	Master             RedisMaster       `json:"master,omitempty"`
	Slave              RedisSlave        `json:"slave,omitempty"`
	RedisPassword      *string           `json:"redisPassword,omitempty"`
	RedisExporter      bool              `json:"exporter"`
	RedisExporterImage string            `json:"redisExporterImage"`
	RedisConfig        map[string]string `json:"redisConfig"`
}

// RedisMaster interface will have the redis master configuration
type RedisMaster struct {
}

// RedisMaster interface will have the redis master configuration
type RedisSlave struct {
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
