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

// RestorePhase represents the current lifecycle phase of a restore operation
type RestorePhase string

const (
	RestorePhasePending   RestorePhase = "Pending"
	RestorePhaseRunning   RestorePhase = "Running"
	RestorePhaseCompleted RestorePhase = "Completed"
	RestorePhaseFailed    RestorePhase = "Failed"
)

// RedisRestoreSpec defines the desired state of a Redis restore operation
type RedisRestoreSpec struct {
	// RedisClusterName is the name of the target Redis resource to restore to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RedisClusterName string `json:"redisClusterName"`

	// TargetKind is the kind of the referenced Redis resource. Leave it empty
	// to discover the kind by looking the resource up.
	// +optional
	// +kubebuilder:validation:Enum=Redis;RedisReplication;RedisCluster
	TargetKind string `json:"targetKind,omitempty"`

	// StorageType is the backend storage provider where the backup is stored
	// +kubebuilder:validation:Required
	StorageType StorageType `json:"storageType"`

	// S3 holds configuration specific to AWS S3 storage
	// +optional
	S3 *S3StorageConfig `json:"s3,omitempty"`

	// BackupLocation is the full path to the backup directory in storage
	// Example: s3://my-bucket/backups/redis-cluster/2026-06-01T02-00-00Z
	// +kubebuilder:validation:Required
	BackupLocation string `json:"backupLocation"`
}

// RedisRestoreStatus reflects the observed state of a Redis restore
type RedisRestoreStatus struct {
	// Phase is the current stage: Pending, Running, Completed, or Failed
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// Message is a human-readable explanation of the current phase
	// +optional
	Message string `json:"message,omitempty"`

	// RestoreCompletedTime is the timestamp when the restore finished
	// +optional
	RestoreCompletedTime *metav1.Time `json:"restoreCompletedTime,omitempty"`

	// ObservedGeneration is the spec generation this status refers to. A
	// restore is destructive, so a failed one is not retried against the same
	// spec until the user changes it.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is a standard Kubernetes condition list
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Rollback records what the in-flight restore has changed on the target,
	// so that if the operator restarts mid-restore the next reconcile can put
	// the workload back instead of guessing. It is cleared on completion.
	// +optional
	Rollback *RestoreRollbackState `json:"rollback,omitempty"`
}

// RestoreRollbackState is the checkpointed footprint of an in-flight restore.
type RestoreRollbackState struct {
	// TargetKind is the resolved kind of the target resource.
	TargetKind string `json:"targetKind,omitempty"`
	// Annotated is true once the target's skip-reconcile annotation was set.
	Annotated bool `json:"annotated,omitempty"`
	// StatefulSetReplicas holds the original replica count of every
	// StatefulSet the restore scales, keyed by name.
	// +optional
	StatefulSetReplicas map[string]int32 `json:"statefulSetReplicas,omitempty"`
	// SentinelsAnnotated lists RedisSentinel resources whose skip-reconcile
	// annotation the restore set.
	// +optional
	SentinelsAnnotated []string `json:"sentinelsAnnotated,omitempty"`
	// SentinelReplicas holds the original replica count of every sentinel
	// StatefulSet the restore scaled to zero, keyed by name.
	// +optional
	SentinelReplicas map[string]int32 `json:"sentinelReplicas,omitempty"`
	// Destructive is true once the target's existing data has been removed.
	// From then on an interrupted restore must not be rolled back
	// automatically: the old data is gone and the archive is the only copy.
	Destructive bool `json:"destructive,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rr;redisrestore,scope=Namespaced
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.redisClusterName`,description="Target Redis cluster name"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="Current restore phase"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RedisRestore defines a restore operation for a Redis cluster from a backup.
// When created, the operator downloads the backup from storage, scales down the
// target Redis, restores dump.rdb and node.conf, and scales it back up.
type RedisRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisRestoreSpec   `json:"spec,omitempty"`
	Status RedisRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisRestoreList contains a list of RedisRestore resources
type RedisRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisRestore `json:"items"`
}

func init() { //nolint:gochecknoinits // required by kubebuilder for type registration
	SchemeBuilder.Register(&RedisRestore{}, &RedisRestoreList{})
}
