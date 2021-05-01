/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RedisSpec defines the desired state of Redis
type RedisSpec struct {
	Mode              string                     `json:"mode"`
	Size              *int32                     `json:"size,omitempty"`
	GlobalConfig      GlobalConfig               `json:"global"`
	Service           Service                    `json:"service"`
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
	Tolerations       *[]corev1.Toleration       `json:"tolerations,omitempty"`
}

// RedisStatus defines the observed state of Redis
type RedisStatus struct {
	Cluster RedisSpec `json:"cluster,omitempty"`
}

// Storage is the inteface to add pvc and pv support in redis
type Storage struct {
	VolumeClaimTemplate corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
}

// RedisMaster interface will have the redis master configuration
type RedisMaster struct {
	Resources   Resources         `json:"resources,omitempty"`
	RedisConfig map[string]string `json:"redisConfig,omitempty"`
	Service     Service           `json:"service,omitempty"`
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
	Image                  string                  `json:"image"`
	ImagePullPolicy        corev1.PullPolicy       `json:"imagePullPolicy,omitempty"`
	Password               *string                 `json:"password,omitempty"`
	Resources              *Resources              `json:"resources,omitempty"`
	ExistingPasswordSecret *ExistingPasswordSecret `json:"existingPasswordSecret,omitempty"`
}

type ExistingPasswordSecret struct {
	Name *string `json:"name,omitempty"`
	Key  *string `json:"key,omitempty"`
}

// RedisSlave interface will have the redis slave configuration
type RedisSlave struct {
	Resources   Resources         `json:"resources,omitempty"`
	RedisConfig map[string]string `json:"redisConfig,omitempty"`
	Service     Service           `json:"service,omitempty"`
}

// ResourceDescription describes CPU and memory resources defined for a cluster.
type ResourceDescription struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// Service is the struct for service definition
type Service struct {
	Type string `json:"type"`
}

// Resources describes requests and limits for the cluster resouces.
type Resources struct {
	ResourceRequests ResourceDescription `json:"requests,omitempty"`
	ResourceLimits   ResourceDescription `json:"limits,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Redis is the Schema for the redis API
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec,omitempty"`
	Status RedisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisList contains a list of Redis
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}
