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
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisbackup/v1alpha1"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create

// Reconciler reconciles a RedisBackup object.
type Reconciler struct {
	client.Client
	K8sClient  kubernetes.Interface
	RESTConfig *rest.Config
}

// Reconcile is the main reconciliation loop for RedisBackup resources.
// It validates the spec, ensures referenced Secrets exist and contain the
// required keys, triggers BGSAVE, copies dump.rdb and node.conf, and uploads
// the backup to S3.
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
		if statusErr := r.setFailedStatus(ctx, instance, err.Error()); statusErr != nil {
			return intctrlutil.RequeueE(ctx, statusErr, "failed to update status")
		}
		// Requeue after 30s so the backup re-reconciles when the Secret is created
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
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
	instance.Status.Message = "Backup completed successfully"
	instance.Status.BackupLocation = backupLocation
	instance.Status.LastBackupTime = &now

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: instance.Generation,
		Reason:             "BackupSucceeded",
		Message:            fmt.Sprintf("Backup completed: %s", backupLocation),
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

// backupToS3 performs the complete backup pipeline:
// 1. BGSAVE → 2. Wait LASTSAVE → 3. Copy dump.rdb & node.conf → 4. Upload to S3
func (r *Reconciler) backupToS3(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error) {
	logger := log.FromContext(ctx)
	cfg := instance.Spec.S3
	podName := fmt.Sprintf("%s-0", instance.Spec.RedisClusterName)
	containerName := instance.Spec.RedisClusterName

	// Step 1: Trigger BGSAVE
	logger.Info("Triggering BGSAVE", "pod", podName)
	if _, err := r.execInPod(ctx, instance.Namespace, podName, containerName, []string{"redis-cli", "BGSAVE"}); err != nil {
		return "", fmt.Errorf("failed to trigger BGSAVE: %w", err)
	}

	// Step 2: Wait for BGSAVE to complete by polling LASTSAVE
	logger.Info("Waiting for BGSAVE to complete")
	if err := r.waitForBGSAVE(ctx, instance.Namespace, podName, containerName); err != nil {
		return "", fmt.Errorf("BGSAVE did not complete: %w", err)
	}

	// Step 3: Copy dump.rdb and node.conf from pod
	tmpDir, err := os.MkdirTemp("", "redis-backup-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	logger.Info("Copying dump.rdb from pod")
	if err := r.copyFileFromPod(ctx, instance.Namespace, podName, containerName, "/data/dump.rdb", filepath.Join(tmpDir, "dump.rdb")); err != nil {
		return "", fmt.Errorf("failed to copy dump.rdb: %w", err)
	}

	logger.Info("Copying node.conf from pod (optional for cluster mode)")
	if err := r.copyFileFromPod(ctx, instance.Namespace, podName, containerName, "/data/node.conf", filepath.Join(tmpDir, "node.conf")); err != nil {
		logger.Info("node.conf not found (non-cluster mode), skipping", "error", err.Error())
	}

	// Step 4: Upload to S3
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	prefix := fmt.Sprintf("backups/%s/%s", instance.Spec.RedisClusterName, timestamp)

	logger.Info("Uploading backup files to S3", "bucket", cfg.Bucket, "prefix", prefix)
	if err := r.uploadToS3(ctx, instance, tmpDir, prefix); err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	backupLocation := fmt.Sprintf("s3://%s/%s", cfg.Bucket, prefix)
	logger.Info("Backup completed successfully", "location", backupLocation)
	return backupLocation, nil
}

// execInPod executes a command in a pod and returns stdout.
func (r *Reconciler) execInPod(ctx context.Context, namespace, podName, containerName string, command []string) (string, error) {
	req := r.K8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
	}, k8sscheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.RESTConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return "", fmt.Errorf("exec failed (stderr: %s): %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// waitForBGSAVE polls LASTSAVE until it changes, indicating BGSAVE completed.
func (r *Reconciler) waitForBGSAVE(ctx context.Context, namespace, podName, containerName string) error {
	initialSave, err := r.execInPod(ctx, namespace, podName, containerName, []string{"redis-cli", "LASTSAVE"})
	if err != nil {
		return fmt.Errorf("failed to get initial LASTSAVE: %w", err)
	}

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timed out waiting for BGSAVE to complete after 60s")
		case <-ticker.C:
			currentSave, err := r.execInPod(ctx, namespace, podName, containerName, []string{"redis-cli", "LASTSAVE"})
			if err != nil {
				return fmt.Errorf("failed to poll LASTSAVE: %w", err)
			}
			if strings.TrimSpace(currentSave) != strings.TrimSpace(initialSave) {
				return nil // BGSAVE completed
			}
		}
	}
}

// copyFileFromPod copies a file from a pod to a local path using tar.
func (r *Reconciler) copyFileFromPod(ctx context.Context, namespace, podName, containerName, srcPath, destPath string) error {
	cmd := []string{"tar", "cf", "-", "-C", filepath.Dir(srcPath), filepath.Base(srcPath)}
	req := r.K8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
	}, k8sscheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.RESTConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var tarBuf bytes.Buffer
	var stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &tarBuf,
		Stderr: &stderr,
	}); err != nil {
		return fmt.Errorf("tar exec failed (stderr: %s): %w", stderr.String(), err)
	}

	// Extract tar
	tr := tar.NewReader(&tarBuf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			outFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", destPath, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", destPath, err)
			}
			outFile.Close()
		}
	}
	return nil
}

// uploadToS3 uploads all files from the local directory to S3.
func (r *Reconciler) uploadToS3(ctx context.Context, instance *redisv1alpha1.RedisBackup, localDir, s3Prefix string) error {
	logger := log.FromContext(ctx)
	cfg := instance.Spec.S3

	// Get AWS credentials from the referenced Secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: cfg.SecretName, Namespace: instance.Namespace}, secret); err != nil {
		return fmt.Errorf("failed to get secret %q: %w", cfg.SecretName, err)
	}

	accessKey := string(secret.Data["AWS_ACCESS_KEY_ID"])
	secretKey := string(secret.Data["AWS_SECRET_ACCESS_KEY"])

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Opts := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
			o.UsePathStyle = true
		}
	}
	s3Client := s3.NewFromConfig(awsCfg, s3Opts)

	// Upload each file
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("failed to read local dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(localDir, entry.Name())
		objectKey := fmt.Sprintf("%s/%s", s3Prefix, entry.Name())

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", filePath, err)
		}

		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &cfg.Bucket,
			Key:    &objectKey,
			Body:   file,
		})
		file.Close()
		if err != nil {
			return fmt.Errorf("failed to upload %s to s3://%s/%s: %w", entry.Name(), cfg.Bucket, objectKey, err)
		}

		logger.Info("Uploaded file to S3", "file", entry.Name(), "key", objectKey)
	}

	return nil
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
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []ctrl.Request {
				var backupList redisv1alpha1.RedisBackupList
				if err := mgr.GetClient().List(ctx, &backupList, client.InNamespace(obj.GetNamespace())); err != nil {
					return nil
				}
				var requests []ctrl.Request
				for _, backup := range backupList.Items {
					if backup.Spec.S3 != nil && backup.Spec.S3.SecretName == obj.GetName() {
						requests = append(requests, ctrl.Request{
							NamespacedName: types.NamespacedName{
								Name:      backup.Name,
								Namespace: backup.Namespace,
							},
						})
					}
				}
				return requests
			},
		)).
		WithOptions(opts).
		Complete(r)
}
