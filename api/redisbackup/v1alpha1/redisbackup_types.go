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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageType defines the backend storage provider for backups
// +kubebuilder:validation:Enum=s3
type StorageType string

const (
	StorageTypeS3 StorageType = "s3"
)

// BackupPhase represents the current lifecycle phase of a backup operation
type BackupPhase string

const (
	BackupPhasePending   BackupPhase = "Pending"
	BackupPhaseRunning   BackupPhase = "Running"
	BackupPhaseCompleted BackupPhase = "Completed"
	BackupPhaseFailed    BackupPhase = "Failed"
)

// S3StorageConfig holds all configuration needed to upload a backup to AWS S3
type S3StorageConfig struct {
	// Bucket is the name of the S3 bucket where backups will be stored
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=3
	Bucket string `json:"bucket"`

	// Region is the AWS region where the S3 bucket lives (e.g. ap-south-1)
	// +kubebuilder:validation:Required
	Region string `json:"region"`

	// Endpoint is an optional custom S3-compatible endpoint URL (e.g. for MinIO)
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// SecretName is the name of the Kubernetes Secret in the same namespace
	// that contains AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY keys
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`
}

// RedisBackupSpec defines what the user wants — the desired state
type RedisBackupSpec struct {
	// RedisClusterName is the name of the Redis resource in the same namespace
	// that this backup targets. Must match an existing Redis, RedisReplication
	// or RedisCluster resource.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RedisClusterName string `json:"redisClusterName"`

	// TargetKind is the kind of the referenced Redis resource. Each kind lays
	// its StatefulSets and containers out differently, so the controller has to
	// know which one it is looking at. Leave it empty to discover the kind by
	// looking the resource up.
	// +optional
	// +kubebuilder:validation:Enum=Redis;RedisReplication;RedisCluster
	TargetKind string `json:"targetKind,omitempty"`

	// StorageType is the backend storage provider. Currently supported: s3
	// +kubebuilder:validation:Required
	StorageType StorageType `json:"storageType"`

	// S3 holds configuration specific to AWS S3 storage.
	// Required when storageType is "s3".
	// +optional
	S3 *S3StorageConfig `json:"s3,omitempty"`

	// Schedule is reserved for recurring backups and is NOT implemented yet.
	// Setting it is rejected rather than ignored, so a backup can never appear
	// to be scheduled when nothing will run it. Use a CronJob that creates
	// RedisBackup resources until this is supported.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// RetentionDays is how many days after completion the controller deletes
	// this backup's objects from storage. Zero (the default) keeps them
	// indefinitely. Expiry is recorded on the resource's status and conditions
	// when it happens.
	// +optional
	// +kubebuilder:validation:Minimum=0
	RetentionDays int32 `json:"retentionDays,omitempty"`

	// CleanupPolicy decides what happens to the stored objects when this
	// resource is deleted. "Retain" keeps them, which is the safe default;
	// "Delete" removes this backup's own prefix from storage.
	// +optional
	// +kubebuilder:default=Retain
	// +kubebuilder:validation:Enum=Retain;Delete
	CleanupPolicy CleanupPolicy `json:"cleanupPolicy,omitempty"`
}

// CleanupPolicy controls whether stored backup objects outlive the resource.
// +kubebuilder:validation:Enum=Retain;Delete
type CleanupPolicy string

const (
	CleanupPolicyRetain CleanupPolicy = "Retain"
	CleanupPolicyDelete CleanupPolicy = "Delete"
)

// RedisBackupStatus reflects what the controller has actually done — observed state
type RedisBackupStatus struct {
	// Phase is the current stage of the backup: Pending, Running, Completed, or Failed
	// +optional
	Phase BackupPhase `json:"phase,omitempty"`

	// Message is a human-readable explanation of the current phase
	// +optional
	Message string `json:"message,omitempty"`

	// BackupLocation is the full path where the backup file was stored
	// Example: s3://my-bucket/backups/redis-cluster/2026-05-05T02-00-00Z.rdb
	// +optional
	BackupLocation string `json:"backupLocation,omitempty"`

	// LastBackupTime is the timestamp of the most recent successful backup
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// ExpiredTime is set when retentionDays elapsed and the stored objects were
	// deleted. The resource stays Completed; BackupLocation is cleared.
	// +optional
	ExpiredTime *metav1.Time `json:"expiredTime,omitempty"`

	// Conditions is a standard Kubernetes condition list for this resource
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rb;redisbackup,scope=Namespaced
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.redisClusterName`,description="Target Redis cluster name"
// +kubebuilder:printcolumn:name="Storage",type=string,JSONPath=`.spec.storageType`,description="Storage backend type"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="Current backup phase"
// +kubebuilder:printcolumn:name="Last Backup",type=string,JSONPath=`.status.lastBackupTime`,description="Timestamp of last successful backup"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RedisBackup defines a backup operation for a Redis cluster.
// When created, the operator will snapshot the target Redis cluster
// and upload the backup file to the configured storage backend.
type RedisBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisBackupSpec   `json:"spec,omitempty"`
	Status RedisBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisBackupList contains a list of RedisBackup resources
type RedisBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisBackup `json:"items"`
}

func init() { //nolint:gochecknoinits // required by kubebuilder for type registration
	SchemeBuilder.Register(&RedisBackup{}, &RedisBackupList{})
}
