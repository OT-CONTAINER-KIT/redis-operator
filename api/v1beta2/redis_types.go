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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RedisSpec defines the desired state of Redis
type RedisSpec struct {
	KubernetesConfig              KubernetesConfig           `json:"kubernetesConfig"`
	RedisExporter                 *RedisExporter             `json:"redisExporter,omitempty"`
	RedisConfig                   *RedisConfig               `json:"redisConfig,omitempty"`
	Storage                       *Storage                   `json:"storage,omitempty"`
	NodeSelector                  map[string]string          `json:"nodeSelector,omitempty"`
	PodSecurityContext            *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	SecurityContext               *corev1.SecurityContext    `json:"securityContext,omitempty"`
	PriorityClassName             string                     `json:"priorityClassName,omitempty"`
	Affinity                      *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations                   *[]corev1.Toleration       `json:"tolerations,omitempty"`
	TLS                           *TLSConfig                 `json:"TLS,omitempty"`
	ACL                           *ACLConfig                 `json:"acl,omitempty"`
	ReadinessProbe                *corev1.Probe              `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe                 *corev1.Probe              `json:"livenessProbe,omitempty" protobuf:"bytes,12,opt,name=livenessProbe"`
	InitContainer                 *InitContainer             `json:"initContainer,omitempty"`
	Sidecars                      *[]Sidecar                 `json:"sidecars,omitempty"`
	ServiceAccountName            *string                    `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64                     `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,4,opt,name=terminationGracePeriodSeconds"`
	EnvVars                       *[]corev1.EnvVar           `json:"env,omitempty"`
	HostPort                      *int                       `json:"hostPort,omitempty"`
}

// RedisStatus defines the observed state of Redis
type RedisStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

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
