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

package redisbackup

import (
	"context"
	"fmt"
	"time"

	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1alpha1"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// RedisBackupFinalizer is the finalizer added to RedisBackup resources
	// to ensure cleanup is performed before deletion.
	RedisBackupFinalizer = "redisbackup.redis.redis.opstreelabs.in/finalizer"
)

// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconciler reconciles a RedisBackup object.
type Reconciler struct {
	client.Client
	K8sClient kubernetes.Interface
}

// RBAC permissions required by this controller.
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is the main reconciliation loop for RedisBackup resources.
// It validates the spec, ensures referenced Secrets exist and contain the
// required keys, and performs the backup operation.
//
// NOTE (alpha): The actual S3 upload (BGSAVE → copy RDB → upload) is not yet
// implemented. The controller currently validates inputs and resolves the
// expected backup path. Full upload support is planned for a follow-up PR.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling RedisBackup", "name", req.Name, "namespace", req.Namespace)

	// ── 1. Fetch the RedisBackup resource ────────────────────────────────────
	instance := &redisv1alpha1.RedisBackup{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get RedisBackup instance")
	}

	// ── 2. Handle deletion — clean up via finalizer ──────────────────────────
	if instance.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(instance, RedisBackupFinalizer) {
			logger.Info("RedisBackup is being deleted, running finalizer cleanup")
			// TODO: Add cleanup logic here (e.g., delete S3 objects based on retention policy)
			controllerutil.RemoveFinalizer(instance, RedisBackupFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return intctrlutil.RequeueE(ctx, err, "failed to remove finalizer")
			}
		}
		return intctrlutil.Reconciled()
	}

	// ── 3. Add finalizer if not present ──────────────────────────────────────
	if !controllerutil.ContainsFinalizer(instance, RedisBackupFinalizer) {
		controllerutil.AddFinalizer(instance, RedisBackupFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to add finalizer")
		}
		return intctrlutil.Requeue()
	}

	// ── 4. Idempotency — skip if already completed ───────────────────────────
	if instance.Status.Phase == redisv1alpha1.BackupPhaseCompleted {
		logger.Info("RedisBackup already completed, nothing to do")
		return intctrlutil.Reconciled()
	}

	// ── 5. Log warnings for features not yet implemented ─────────────────────
	if instance.Spec.Schedule != "" {
		logger.Info("WARNING: schedule field is not yet implemented — ignoring",
			"schedule", instance.Spec.Schedule)
	}
	if instance.Spec.RetentionDays > 0 {
		logger.V(1).Info("NOTE: retentionDays is accepted but cleanup is not yet implemented",
			"retentionDays", instance.Spec.RetentionDays)
	}

	// ── 6. Validate the spec ─────────────────────────────────────────────────
	if err := r.validateSpec(ctx, instance); err != nil {
		logger.Error(err, "RedisBackup spec validation failed")
		return ctrl.Result{}, r.setFailedStatus(ctx, instance, err.Error())
	}

	// ── 7. Set Running phase ─────────────────────────────────────────────────
	if instance.Status.Phase != redisv1alpha1.BackupPhaseRunning {
		if err := r.setPhase(ctx, instance, redisv1alpha1.BackupPhaseRunning, "Backup is in progress"); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to set Running phase")
		}
	}

	// ── 8. Execute the backup ────────────────────────────────────────────────
	backupLocation, err := r.performBackup(ctx, instance)
	if err != nil {
		logger.Error(err, "Backup execution failed")
		return ctrl.Result{}, r.setFailedStatus(ctx, instance, fmt.Sprintf("Backup failed: %v", err))
	}

	// ── 9. Mark as Completed ─────────────────────────────────────────────────
	now := metav1.NewTime(time.Now().UTC())
	instance.Status.Phase = redisv1alpha1.BackupPhaseCompleted
	instance.Status.Message = "Backup location resolved (alpha: S3 upload not yet implemented)"
	instance.Status.BackupLocation = backupLocation
	instance.Status.LastBackupTime = &now

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: instance.Generation,
		Reason:             "BackupSucceeded",
		Message:            fmt.Sprintf("Backup location resolved: %s (upload pending implementation)", backupLocation),
	})

	if err := r.Status().Update(ctx, instance); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to update status to Completed")
	}

	logger.Info("RedisBackup completed successfully", "location", backupLocation)
	return intctrlutil.Reconciled()
}

// validateSpec checks all required fields and validates that referenced
// Kubernetes resources (e.g., Secrets) actually exist and contain the
// expected keys.
func (r *Reconciler) validateSpec(ctx context.Context, instance *redisv1alpha1.RedisBackup) error {
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

		// Validate that the referenced Secret actually exists
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Name:      instance.Spec.S3.SecretName,
			Namespace: instance.Namespace,
		}
		if err := r.Get(ctx, secretKey, secret); err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("secret %q not found in namespace %q", instance.Spec.S3.SecretName, instance.Namespace)
			}
			return fmt.Errorf("failed to look up secret %q: %w", instance.Spec.S3.SecretName, err)
		}

		// Validate the Secret contains the expected AWS credential keys
		requiredKeys := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"}
		for _, key := range requiredKeys {
			if _, ok := secret.Data[key]; !ok {
				return fmt.Errorf("secret %q is missing required key %q", instance.Spec.S3.SecretName, key)
			}
		}

	default:
		return fmt.Errorf("unsupported storageType %q — currently supported: s3", instance.Spec.StorageType)
	}
	return nil
}

// performBackup dispatches to the correct storage backend.
//
// NOTE (alpha): Only S3 is currently supported. GCS and Azure backends are
// defined in the CRD enum for forward compatibility but are not yet implemented.
func (r *Reconciler) performBackup(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error) {
	switch instance.Spec.StorageType {
	case redisv1alpha1.StorageTypeS3:
		return r.backupToS3(ctx, instance)
	default:
		return "", fmt.Errorf("no backup implementation for storageType %q", instance.Spec.StorageType)
	}
}

// backupToS3 resolves the S3 backup location.
//
// TODO(alpha): This currently only constructs the expected S3 path and
// validates that credentials exist. The full implementation should:
//  1. Trigger BGSAVE on the target Redis instance
//  2. Wait for the RDB snapshot to complete
//  3. Copy the RDB file from the Redis pod
//  4. Upload it to S3 using the AWS SDK with credentials from the referenced Secret
//  5. Verify the upload integrity (e.g., checksum)
func (r *Reconciler) backupToS3(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error) {
	logger := log.FromContext(ctx)
	cfg := instance.Spec.S3

	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	objectKey := fmt.Sprintf("backups/%s/%s.rdb", instance.Spec.RedisClusterName, timestamp)
	backupLocation := fmt.Sprintf("s3://%s/%s", cfg.Bucket, objectKey)

	logger.Info("S3 backup location resolved (alpha: upload not yet implemented)",
		"cluster", instance.Spec.RedisClusterName,
		"bucket", cfg.Bucket,
		"region", cfg.Region,
		"objectKey", objectKey,
		"secretName", cfg.SecretName,
	)

	return backupLocation, nil
}

// setPhase updates Phase and Message in the resource's Status.
func (r *Reconciler) setPhase(ctx context.Context, instance *redisv1alpha1.RedisBackup, phase redisv1alpha1.BackupPhase, msg string) error {
	instance.Status.Phase = phase
	instance.Status.Message = msg
	return r.Status().Update(ctx, instance)
}

// setFailedStatus marks the backup as Failed, sets a message, and records
// a "Ready=False" condition with the failure reason.
func (r *Reconciler) setFailedStatus(ctx context.Context, instance *redisv1alpha1.RedisBackup, reason string) error {
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

// SetupWithManager registers this controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1alpha1.RedisBackup{}).
		WithOptions(opts).
		Complete(r)
}
