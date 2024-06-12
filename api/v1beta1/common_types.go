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
	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
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

// RedisExporter interface will have the information for redis exporter related stuff
type RedisExporter struct {
	common.RedisExporter `json:",inline"`
}

// TLS Configuration for redis instances
type TLSConfig struct {
	common.TLSConfig `json:",inline"`
}

// Sidecar for each Redis pods
type Sidecar struct {
	common.Sidecar `json:",inline"`
}
