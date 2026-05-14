package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageType defines the backend storage provider for backups
// +kubebuilder:validation:Enum=s3;gcs;azure
type StorageType string

const (
	StorageTypeS3    StorageType = "s3"
	StorageTypeGCS   StorageType = "gcs"
	StorageTypeAzure StorageType = "azure"
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
	// that this backup targets. Must match an existing Redis/RedisCluster resource.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RedisClusterName string `json:"redisClusterName"`

	// StorageType is the backend storage provider. Currently supported: s3
	// +kubebuilder:validation:Required
	StorageType StorageType `json:"storageType"`

	// S3 holds configuration specific to AWS S3 storage.
	// Required when storageType is "s3".
	// +optional
	S3 *S3StorageConfig `json:"s3,omitempty"`

	// Schedule is an optional cron expression for recurring backups.
	// Example: "0 2 * * *" means every day at 2 AM UTC.
	// Leave empty to trigger a one-time backup immediately on creation.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// RetentionDays defines how many days backup files are kept in storage.
	// Defaults to 7 days if not specified. Minimum value is 1.
	// +optional
	// +kubebuilder:default=7
	// +kubebuilder:validation:Minimum=1
	RetentionDays int32 `json:"retentionDays,omitempty"`
}

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

func init() {
	SchemeBuilder.Register(&RedisBackup{}, &RedisBackupList{})
}
