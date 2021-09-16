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
)

// KubernetesConfig will be the JSON struct for Basic Redis Config
type KubernetesConfig struct {
	Image                  string                         `json:"image"`
	ImagePullPolicy        corev1.PullPolicy              `json:"imagePullPolicy,omitempty"`
	Resources              *corev1.ResourceRequirements   `json:"resources,omitempty"`
	ExistingPasswordSecret *ExistingPasswordSecret        `json:"redisSecret,omitempty"`
	ImagePullSecrets       *[]corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
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
}

// RedisExporter interface will have the information for redis exporter related stuff
type RedisExporter struct {
	Enabled         bool                         `json:"enabled,omitempty"`
	Image           string                       `json:"image"`
	Resources       *corev1.ResourceRequirements `json:"resources,omitempty"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	EnvVars         *[]corev1.EnvVar             `json:"env,omitempty"`
}
