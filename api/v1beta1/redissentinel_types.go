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

// RedisSentinelSpec defines the desired state of RedisSentinel
type RedisSentinelSpec struct {
	// +kubebuilder:validation:Minimum=3
	Size              *int32                       `json:"clusterSize"`
	KubernetesConfig  KubernetesConfig             `json:"kubernetesConfig"`
	RedisReplica      RedisReplica                 `json:"redisReplica,omitempty"`
	RedisSentinel     RedisSentinelSentinel        `json:"redisSentinel,omitempty"`
	RedisExporter     *RedisExporter               `json:"redisExporter,omitempty"`
	Storage           *Storage                     `json:"storage,omitempty"`
	NodeSelector      map[string]string            `json:"nodeSelector,omitempty"`
	SecurityContext   *corev1.PodSecurityContext   `json:"securityContext,omitempty"`
	PriorityClassName string                       `json:"priorityClassName,omitempty"`
	Tolerations       *[]corev1.Toleration         `json:"tolerations,omitempty"`
	Resources         *corev1.ResourceRequirements `json:"resources,omitempty"`
	Sidecars          *[]Sidecar                   `json:"sidecars,omitempty"`
}

// RedisSentinelStsSpec defines the Statefulset values
type RedisSentinelSentinel struct {
	// +kubebuilder:validation:Minimum=3
	Replicas       *int32               `json:"replicas,omitempty"`
	Config         *RedisSentinelConfig `json:"sentinelConfig,omitempty"`
	Affinity       *corev1.Affinity     `json:"affinity,omitempty"`
	ReadinessProbe *corev1.Probe        `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe  *corev1.Probe        `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
}

// RedisSentinelConfig defines the config for sentinel
type RedisSentinelConfig struct {
	// +kubebuilder:validation:Minimum=1
	Quorum *int32 `json:"quorum,omitempty"`
	// +kubebuilder:validation:Minimum=1
	ParallelSyncs         *int32 `json:"parallelSyncs,omitempty"`
	FailoverTimeout       *int32 `json:"failoverTimeout,omitempty"`
	DownAfterMilliseconds *int32 `json:"downAfterMilliseconds,omitempty"`
}

// RedisSentinelStatus defines the observed state of RedisSentinel
type RedisSentinelStatus struct {
	// Leader stores the Current Elected Leader from Sentinel
	Leader string `json:"leader,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ClusterSize",type=integer,JSONPath=`.spec.clusterSize`,description=Current cluster node count
// +kubebuilder:printcolumn:name="LeaderReplicas",type=integer,JSONPath=`.spec.redisLeader.replicas`,description=Overridden Leader replica count
// +kubebuilder:printcolumn:name="FollowerReplicas",type=integer,JSONPath=`.spec.redisFollower.replicas`,description=Overridden Follower replica count
// +kubebuilder:printcolumn:name="Leader",type=integer,JSONPath=`.status.leader`,description=Overridden Follower replica count
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description=Age of Cluster

// RedisSentinel is the Schema for the RedisSentinels API
type RedisSentinel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSentinelSpec    `json:"spec"`
	Status *RedisSentinelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RedisSentinelList contains a list of RedisSentinel
type RedisSentinelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisSentinel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisSentinel{}, &RedisSentinelList{})
}
