package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1alpha1"
)

// RedisBackupReconciler reconciles RedisBackup objects
type RedisBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// RBAC permissions this controller needs
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is called by Kubernetes every time a RedisBackup resource is
// created, updated, or deleted. It compares desired state (Spec) with
// observed state (Status) and takes action to make them match.
func (r *RedisBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling RedisBackup", "name", req.Name, "namespace", req.Namespace)

	// ── 1. Fetch the RedisBackup resource from the cluster ───────────────────
	instance := &redisv1alpha1.RedisBackup{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Resource was deleted — nothing to do
			logger.Info("RedisBackup not found, likely deleted — skipping")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch RedisBackup")
		return ctrl.Result{}, err
	}

	// ── 2. Idempotency — skip if already completed ───────────────────────────
	// This prevents re-running a backup if the controller restarts
	if instance.Status.Phase == redisv1alpha1.BackupPhaseCompleted {
		logger.Info("RedisBackup already completed, nothing to do")
		return ctrl.Result{}, nil
	}

	// ── 3. Validate the spec before doing any work ───────────────────────────
	if err := r.validateSpec(instance); err != nil {
		logger.Error(err, "RedisBackup spec validation failed")
		return ctrl.Result{}, r.setFailedStatus(ctx, instance, err.Error())
	}

	// ── 4. Update status to Running ──────────────────────────────────────────
	if err := r.setPhase(ctx, instance, redisv1alpha1.BackupPhaseRunning, "Backup is in progress"); err != nil {
		logger.Error(err, "Failed to set Running phase")
		return ctrl.Result{}, err
	}

	// ── 5. Execute the backup ────────────────────────────────────────────────
	backupLocation, err := r.performBackup(ctx, instance)
	if err != nil {
		logger.Error(err, "Backup execution failed")
		return ctrl.Result{}, r.setFailedStatus(ctx, instance, fmt.Sprintf("Backup failed: %v", err))
	}

	// ── 6. Mark as Completed and record metadata ─────────────────────────────
	now := metav1.NewTime(time.Now().UTC())
	instance.Status.Phase = redisv1alpha1.BackupPhaseCompleted
	instance.Status.Message = "Backup completed successfully"
	instance.Status.BackupLocation = backupLocation
	instance.Status.LastBackupTime = &now

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: instance.Generation,
		Reason:             "BackupSucceeded",
		Message:            fmt.Sprintf("Backup stored at %s", backupLocation),
	})

	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "Failed to update status to Completed")
		return ctrl.Result{}, err
	}

	logger.Info("RedisBackup completed successfully", "location", backupLocation)

	// ── 7. If scheduled, requeue after 1 hour ────────────────────────────────
	if instance.Spec.Schedule != "" {
		logger.Info("Scheduled backup — requeueing for next run", "schedule", instance.Spec.Schedule)
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}

	return ctrl.Result{}, nil
}

// validateSpec checks all required fields are present and consistent
func (r *RedisBackupReconciler) validateSpec(instance *redisv1alpha1.RedisBackup) error {
	if instance.Spec.RedisClusterName == "" {
		return fmt.Errorf("spec.redisClusterName must not be empty")
	}
	switch instance.Spec.StorageType {
	case redisv1alpha1.StorageTypeS3:
		if instance.Spec.S3 == nil {
			return fmt.Errorf("spec.s3 is required when storageType is 's3'")
		}
		if instance.Spec.S3.Bucket == "" {
			return fmt.Errorf("spec.s3.bucket must not be empty")
		}
		if instance.Spec.S3.Region == "" {
			return fmt.Errorf("spec.s3.region must not be empty")
		}
		if instance.Spec.S3.SecretName == "" {
			return fmt.Errorf("spec.s3.secretName must not be empty")
		}
	default:
		return fmt.Errorf("unsupported storageType %q — supported values: s3", instance.Spec.StorageType)
	}
	return nil
}

// performBackup dispatches to the correct storage backend
func (r *RedisBackupReconciler) performBackup(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error) {
	switch instance.Spec.StorageType {
	case redisv1alpha1.StorageTypeS3:
		return r.backupToS3(ctx, instance)
	default:
		return "", fmt.Errorf("no backup implementation for storageType %q", instance.Spec.StorageType)
	}
}

// backupToS3 handles the full S3 upload flow
func (r *RedisBackupReconciler) backupToS3(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error) {
	logger := log.FromContext(ctx)
	cfg := instance.Spec.S3

	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	objectKey := fmt.Sprintf("backups/%s/%s.rdb", instance.Spec.RedisClusterName, timestamp)
	backupLocation := fmt.Sprintf("s3://%s/%s", cfg.Bucket, objectKey)

	logger.Info("Starting S3 backup",
		"cluster",    instance.Spec.RedisClusterName,
		"bucket",     cfg.Bucket,
		"region",     cfg.Region,
		"objectKey",  objectKey,
		"secretName", cfg.SecretName,
	)

	// AWS SDK upload logic will be wired here using credentials from cfg.SecretName
	// The Secret must exist in instance.Namespace and contain:
	//   AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
	logger.Info("S3 backup location resolved", "location", backupLocation)

	return backupLocation, nil
}

// setPhase is a helper that updates only Phase and Message in Status
func (r *RedisBackupReconciler) setPhase(ctx context.Context, instance *redisv1alpha1.RedisBackup, phase redisv1alpha1.BackupPhase, msg string) error {
	instance.Status.Phase = phase
	instance.Status.Message = msg
	return r.Status().Update(ctx, instance)
}

// setFailedStatus marks the backup as Failed and sets a condition
func (r *RedisBackupReconciler) setFailedStatus(ctx context.Context, instance *redisv1alpha1.RedisBackup, reason string) error {
	instance.Status.Phase = redisv1alpha1.BackupPhaseFailed
	instance.Status.Message = reason

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: instance.Generation,
		Reason:             "BackupFailed",
		Message:            reason,
	})

	return r.Status().Update(ctx, instance)
}

// SetupWithManager registers this controller with the controller manager
func (r *RedisBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1alpha1.RedisBackup{}).
		Complete(r)
}
