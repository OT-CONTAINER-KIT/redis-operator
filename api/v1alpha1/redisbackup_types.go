/*
Copyright 2026.

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

// =========================================================================
// 1. THE SPEC (The Desired State)
// This is where we define what the user MUST provide in their YAML file
// when they want to trigger a backup.
// =========================================================================

// RedisBackupSpec defines the desired state of RedisBackup
type RedisBackupSpec struct {
	// The +kubebuilder tags act like validators. If a user tries to apply
	// a YAML without a cluster name, Kubernetes will instantly reject it.

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// RedisClusterName is the exact name of the target Redis cluster you want to backup.
	RedisClusterName string `json:"redisClusterName"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=s3;gcs
	// StorageType forces the user to choose only 's3' or 'gcs'.
	StorageType string `json:"storageType"`

	// +kubebuilder:validation:Required
	// S3Bucket specifies the name of the destination AWS S3 bucket.
	S3Bucket string `json:"s3Bucket"`

	// +kubebuilder:validation:Required
	// SecretName points to a Kubernetes Secret that holds the AWS Access Keys 
	// so your operator can securely upload the .rdb file.
	SecretName string `json:"secretName"`
}

// =========================================================================
// 2. THE STATUS (The Observed State)
// This is where YOUR controller writes data back to the cluster so the 
// user can see if their backup succeeded or failed.
// =========================================================================

// RedisBackupStatus defines the observed state of RedisBackup
type RedisBackupStatus struct {
	// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
	// Phase tracks the real-time status of the backup operation.
	Phase string `json:"phase,omitempty"`

	// LastBackupTime records a timestamp of when the snapshot was successfully uploaded.
	LastBackupTime string `json:"lastBackupTime,omitempty"`
}

// =========================================================================
// 3. THE MAIN CRD STRUCTURE
// This combines the Spec and Status into the final Kubernetes Object.
// =========================================================================

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RedisBackup is the Schema for the redisbackups API
type RedisBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisBackupSpec   `json:"spec,omitempty"`
	Status RedisBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisBackupList contains a list of RedisBackup objects
type RedisBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisBackup `json:"items"`
}

// =========================================================================
// 4. REGISTRATION
// This tells Kubernetes that this new Custom Resource actually exists.
// =========================================================================

func init() {
	SchemeBuilder.Register(&RedisBackup{}, &RedisBackupList{})
}
