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

package v1beta2

import (
	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	status "github.com/OT-CONTAINER-KIT/redis-operator/api/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RedisClusterSpec defines the desired state of RedisCluster
type RedisClusterSpec struct {
	Size             *int32           `json:"clusterSize"`
	KubernetesConfig KubernetesConfig `json:"kubernetesConfig"`
	HostNetwork      bool             `json:"hostNetwork,omitempty"`
	// +kubebuilder:default:=6379
	Port *int `json:"port,omitempty"`
	// +kubebuilder:default:=v7
	ClusterVersion     *string                      `json:"clusterVersion,omitempty"`
	RedisConfig        *RedisConfig                 `json:"redisConfig,omitempty"`
	RedisLeader        RedisLeader                  `json:"redisLeader,omitempty"`
	RedisFollower      RedisFollower                `json:"redisFollower,omitempty"`
	RedisExporter      *RedisExporter               `json:"redisExporter,omitempty"`
	Storage            *ClusterStorage              `json:"storage,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext   `json:"podSecurityContext,omitempty"`
	PriorityClassName  string                       `json:"priorityClassName,omitempty"`
	Resources          *corev1.ResourceRequirements `json:"resources,omitempty"`
	TLS                *TLSConfig                   `json:"TLS,omitempty"`
	ACL                *ACLConfig                   `json:"acl,omitempty"`
	InitContainer      *InitContainer               `json:"initContainer,omitempty"`
	Sidecars           *[]Sidecar                   `json:"sidecars,omitempty"`
	ServiceAccountName *string                      `json:"serviceAccountName,omitempty"`
	PersistenceEnabled *bool                        `json:"persistenceEnabled,omitempty"`
	EnvVars            *[]corev1.EnvVar             `json:"env,omitempty"`
	HostPort           *int                         `json:"hostPort,omitempty"`
}

func (cr *RedisClusterSpec) GetReplicaCounts(t string) int32 {
	replica := cr.Size
	if t == "leader" && cr.RedisLeader.Replicas != nil {
		replica = cr.RedisLeader.Replicas
	} else if t == "follower" && cr.RedisFollower.Replicas != nil {
		replica = cr.RedisFollower.Replicas
	}
	return *replica
}

// GetRedisLeaderResources returns the resources for the redis leader, if not set, it will return the default resources
func (cr *RedisClusterSpec) GetRedisLeaderResources() *corev1.ResourceRequirements {
	if cr.RedisLeader.Resources != nil {
		return cr.RedisLeader.Resources
	}

	return cr.KubernetesConfig.Resources
}

// GetRedisDynamicConfig returns Redis dynamic configuration parameters.
// Priority: top-level config > leader config > follower config
func (cr *RedisClusterSpec) GetRedisDynamicConfig() []string {
	// Use top-level configuration if available
	if cr.RedisConfig != nil && len(cr.RedisConfig.DynamicConfig) > 0 {
		return cr.RedisConfig.DynamicConfig
	}
	// Return empty slice if no configuration is found
	return []string{}
}

// GetRedisFollowerResources returns the resources for the redis follower, if not set, it will return the default resources
func (cr *RedisClusterSpec) GetRedisFollowerResources() *corev1.ResourceRequirements {
	if cr.RedisFollower.Resources != nil {
		return cr.RedisFollower.Resources
	}

	return cr.KubernetesConfig.Resources
}

// RedisLeader interface will have the redis leader configuration
type RedisLeader struct {
	common.RedisLeader            `json:",inline"`
	SecurityContext               *corev1.SecurityContext      `json:"securityContext,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	Resources                     *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// RedisFollower interface will have the redis follower configuration
type RedisFollower struct {
	common.RedisFollower          `json:",inline"`
	SecurityContext               *corev1.SecurityContext      `json:"securityContext,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	Resources                     *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// RedisClusterStatus defines the observed state of RedisCluster
// +kubebuilder:subresource:status
type RedisClusterStatus struct {
	State  status.RedisClusterState `json:"state,omitempty"`
	Reason string                   `json:"reason,omitempty"`
	// +kubebuilder:default=0
	ReadyLeaderReplicas int32 `json:"readyLeaderReplicas,omitempty"`
	// +kubebuilder:default=0
	ReadyFollowerReplicas int32 `json:"readyFollowerReplicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="ClusterSize",type=integer,JSONPath=`.spec.clusterSize`,description=Current cluster node count
// +kubebuilder:printcolumn:name="ReadyLeaderReplicas",type="integer",JSONPath=".status.readyLeaderReplicas",description="Number of ready leader replicas"
// +kubebuilder:printcolumn:name="ReadyFollowerReplicas",type="integer",JSONPath=".status.readyFollowerReplicas",description="Number of ready follower replicas"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="The current state of the Redis Cluster",priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="Age of Cluster",priority=1
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.reason",description="The reason for the current state",priority=1

// RedisCluster is the Schema for the redisclusters API
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec   `json:"spec"`
	Status RedisClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RedisClusterList contains a list of RedisCluster
type RedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisCluster `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&RedisCluster{}, &RedisClusterList{})
}
