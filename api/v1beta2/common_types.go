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
	corev1 "k8s.io/api/core/v1"
)

// KubernetesConfig will be the JSON struct for Basic Redis Config
type KubernetesConfig struct {
	common.KubernetesConfig `json:",inline"`
}

// ServiceConfig define the type of service to be created and its annotations
type ServiceConfig struct {
	// +kubebuilder:validation:Enum=LoadBalancer;NodePort;ClusterIP
	ServiceType        string            `json:"serviceType,omitempty"`
	ServiceAnnotations map[string]string `json:"annotations,omitempty"`
}

// RedisConfig defines the external configuration of Redis
type RedisConfig struct {
	common.RedisConfig `json:",inline"`
}

// ExistingPasswordSecret is the struct to access the existing secret
type ExistingPasswordSecret struct {
	Name *string `json:"name,omitempty"`
	Key  *string `json:"key,omitempty"`
}

// Storage is the inteface to add pvc and pv support in redis
type Storage struct {
	common.Storage `json:",inline"`
}

// Node-conf needs to be added only in redis cluster
type ClusterStorage struct {
	// +kubebuilder:default=false
	NodeConfVolume              bool                         `json:"nodeConfVolume,omitempty"`
	NodeConfVolumeClaimTemplate corev1.PersistentVolumeClaim `json:"nodeConfVolumeClaimTemplate,omitempty"`
	common.Storage              `json:",inline"`
}

// RedisExporter interface will have the information for redis exporter related stuff
type RedisExporter struct {
	common.RedisExporter `json:",inline"`
}

// TLS Configuration for redis instances
type TLSConfig struct {
	common.TLSConfig `json:",inline"`
}

type ACLConfig struct {
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty"`
}

// Sidecar for each Redis pods
type Sidecar struct {
	common.Sidecar  `json:",inline"`
	Volumes         *[]corev1.VolumeMount   `json:"mountPath,omitempty"`
	Command         []string                `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	Ports           *[]corev1.ContainerPort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"containerPort" protobuf:"bytes,6,rep,name=ports"`
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// InitContainer for each Redis pods
type InitContainer struct {
	Enabled         *bool                        `json:"enabled,omitempty"`
	Image           string                       `json:"image"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	Resources       *corev1.ResourceRequirements `json:"resources,omitempty"`
	EnvVars         *[]corev1.EnvVar             `json:"env,omitempty"`
	Command         []string                     `json:"command,omitempty"`
	Args            []string                     `json:"args,omitempty"`
	SecurityContext *corev1.SecurityContext      `json:"securityContext,omitempty"`
}
