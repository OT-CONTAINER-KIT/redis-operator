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
	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RedisSpec defines the desired state of Redis
type RedisSpec struct {
	KubernetesConfig              common.KubernetesConfig    `json:"kubernetesConfig"`
	RedisExporter                 *common.RedisExporter      `json:"redisExporter,omitempty"`
	RedisConfig                   *common.RedisConfig        `json:"redisConfig,omitempty"`
	Storage                       *common.Storage            `json:"storage,omitempty"`
	NodeSelector                  map[string]string          `json:"nodeSelector,omitempty"`
	PodSecurityContext            *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	SecurityContext               *corev1.SecurityContext    `json:"securityContext,omitempty"`
	PriorityClassName             string                     `json:"priorityClassName,omitempty"`
	Affinity                      *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations                   *[]corev1.Toleration       `json:"tolerations,omitempty"`
	TLS                           *common.TLSConfig          `json:"TLS,omitempty"`
	ACL                           *common.ACLConfig          `json:"acl,omitempty"`
	ReadinessProbe                *corev1.Probe              `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe                 *corev1.Probe              `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	InitContainer                 *common.InitContainer      `json:"initContainer,omitempty"`
	Sidecars                      *[]common.Sidecar          `json:"sidecars,omitempty"`
	ServiceAccountName            *string                    `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64                     `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	EnvVars                       *[]corev1.EnvVar           `json:"env,omitempty"`
	HostPort                      *int                       `json:"hostPort,omitempty"`
}

// RedisStatus defines the observed state of Redis
type RedisStatus struct {
	// State is the current lifecycle state of the Redis resource.
	State RedisState `json:"state,omitempty"`
	// Reason provides a human-readable explanation of the current state.
	Reason string `json:"reason,omitempty"`
}

// RedisState describes the lifecycle state of a Redis standalone resource.
type RedisState string

const (
	// RedisInitializing means the StatefulSet and pod are being created.
	RedisInitializing RedisState = "Initializing"
	// RedisReady means the pod is running and ready to serve traffic.
	RedisReady RedisState = "Ready"
	// RedisFailed means the pod is not ready.
	RedisFailed RedisState = "Failed"
)

const (
	InitializingRedisReason = "Redis is initializing"
	ReadyRedisReason        = "Redis is ready"
	FailedRedisReason       = "Redis pod is not ready"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="The current state of the Redis instance"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Redis is the Schema for the redis API
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec"`
	Status RedisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisList contains a list of Redis
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}
