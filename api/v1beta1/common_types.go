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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// KubernetesConfig will be the JSON struct for Basic Redis Config
type KubernetesConfig struct {
	Image                  string                           `json:"image"`
	ImagePullPolicy        corev1.PullPolicy                `json:"imagePullPolicy,omitempty"`
	Resources              *corev1.ResourceRequirements     `json:"resources,omitempty"`
	ExistingPasswordSecret *ExistingPasswordSecret          `json:"redisSecret,omitempty"`
	ImagePullSecrets       *[]corev1.LocalObjectReference   `json:"imagePullSecrets,omitempty"`
	UpdateStrategy         appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	Service                *ServiceConfig                   `json:"service,omitempty"`
}

// ServiceConfig define the type of service to be created and its annotations
type ServiceConfig struct {
	// +kubebuilder:validation:Enum=LoadBalancer;NodePort;ClusterIP
	ServiceType        string            `json:"serviceType,omitempty"`
	ServiceAnnotations map[string]string `json:"annotations,omitempty"`
}

// RedisConfig defines the external configuration of Redis
type RedisConfig struct {
	AdditionalRedisConfig *string `json:"additionalRedisConfig,omitempty"`
}

// ExistingPasswordSecret is the struct to access the existing secret
type ExistingPasswordSecret struct {
	Name *string `json:"name,omitempty"`
	Key  *string `json:"key,omitempty"`
}

// Storage is the inteface to add pvc and pv support in redis
type Storage struct {
	VolumeClaimTemplate corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
	VolumeMount         AdditionalVolume             `json:"volumeMount,omitempty"`
}

// Node-conf needs to be added only in redis cluster
type ClusterStorage struct {
	VolumeClaimTemplate         corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
	NodeConfVolumeClaimTemplate corev1.PersistentVolumeClaim `json:"nodeConfVolumeClaimTemplate,omitempty"`
	VolumeMount                 AdditionalVolume             `json:"volumeMount,omitempty"`
}

// Additional Volume is provided by user that is mounted on the pods
type AdditionalVolume struct {
	Volume    []corev1.Volume      `json:"volume,omitempty"`
	MountPath []corev1.VolumeMount `json:"mountPath,omitempty"`
}

// RedisExporter interface will have the information for redis exporter related stuff
type RedisExporter struct {
	Enabled         bool                         `json:"enabled,omitempty"`
	Image           string                       `json:"image"`
	Resources       *corev1.ResourceRequirements `json:"resources,omitempty"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	EnvVars         *[]corev1.EnvVar             `json:"env,omitempty"`
}

// TLS Configuration for redis instances
type TLSConfig struct {
	CaKeyFile   string `json:"ca,omitempty"`
	CertKeyFile string `json:"cert,omitempty"`
	KeyFile     string `json:"key,omitempty"`
	// Reference to secret which contains the certificates
	Secret corev1.SecretVolumeSource `json:"secret"`
}

type ACLConfig struct {
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty"`
}

// Probe is a interface for ReadinessProbe and LivenessProbe
type Probe struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty" protobuf:"varint,2,opt,name=initialDelaySeconds"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" protobuf:"varint,3,opt,name=timeoutSeconds"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	PeriodSeconds int32 `json:"periodSeconds,omitempty" protobuf:"varint,4,opt,name=periodSeconds"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	SuccessThreshold int32 `json:"successThreshold,omitempty" protobuf:"varint,5,opt,name=successThreshold"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	FailureThreshold int32 `json:"failureThreshold,omitempty" protobuf:"varint,6,opt,name=failureThreshold"`
}

// Sidecar for each Redis pods
type Sidecar struct {
	Name            string                       `json:"name"`
	Image           string                       `json:"image"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	Resources       *corev1.ResourceRequirements `json:"resources,omitempty"`
	EnvVars         *[]corev1.EnvVar             `json:"env,omitempty"`
	Volumes         *[]corev1.VolumeMount        `json:"mountPath,omitempty"`
	Command         []string                     `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	Ports           *[]corev1.ContainerPort      `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"containerPort" protobuf:"bytes,6,rep,name=ports"`
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
}
