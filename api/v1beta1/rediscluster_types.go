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

// RedisClusterSpec defines the desired state of RedisCluster
type RedisClusterSpec struct {
	// +kubebuilder:validation:Minimum=3
	Size              *int32                       `json:"clusterSize"`
	KubernetesConfig  KubernetesConfig             `json:"kubernetesConfig"`
	RedisLeader       RedisLeader                  `json:"redisLeader,omitempty"`
	RedisFollower     RedisFollower                `json:"redisFollower,omitempty"`
	RedisExporter     *RedisExporter               `json:"redisExporter,omitempty"`
	Storage           *Storage                     `json:"storage,omitempty"`
	NodeSelector      map[string]string            `json:"nodeSelector,omitempty"`
	SecurityContext   *corev1.PodSecurityContext   `json:"securityContext,omitempty"`
	PriorityClassName string                       `json:"priorityClassName,omitempty"`
	Tolerations       *[]corev1.Toleration         `json:"tolerations,omitempty"`
	Resources         *corev1.ResourceRequirements `json:"resources,omitempty"`
	Sidecars          *[]Sidecar                   `json:"sidecars,omitempty"`
}

// RedisLeader interface will have the redis leader configuration
type RedisLeader struct {
	// +kubebuilder:validation:Minimum=3
	Replicas    *int32           `json:"replicas,omitempty"`
	RedisConfig *RedisConfig     `json:"redisConfig,omitempty"`
	Affinity    *corev1.Affinity `json:"affinity,omitempty"`
}

// RedisFollower interface will have the redis follower configuration
type RedisFollower struct {
	Replicas    *int32           `json:"replicas,omitempty"`
	RedisConfig *RedisConfig     `json:"redisConfig,omitempty"`
	Affinity    *corev1.Affinity `json:"affinity,omitempty"`
}

// RedisClusterStatus defines the observed state of RedisCluster
type RedisClusterStatus struct {
	RedisCluster RedisCluster `json:"redisCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ClusterSize",type=integer,JSONPath=`.spec.clusterSize`,description=Current cluster node count
// +kubebuilder:printcolumn:name="LeaderReplicas",type=integer,JSONPath=`.spec.redisLeader.replicas`,description=Overridden Leader replica count
// +kubebuilder:printcolumn:name="FollowerReplicas",type=integer,JSONPath=`.spec.redisFollower.replicas`,description=Overridden Follower replica count
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description=Age of Cluster

// RedisCluster is the Schema for the redisclusters API
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec `json:"spec"`
	Status RedisClusterSpec `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RedisClusterList contains a list of RedisCluster
type RedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisCluster{}, &RedisClusterList{})
}
