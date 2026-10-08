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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisbackup/v1alpha1"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/backuputil"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// RedisBackupFinalizer is the finalizer added to RedisBackup resources
	// to ensure cleanup is performed before deletion.
	RedisBackupFinalizer = "redisbackup.redis.redis.opstreelabs.in/finalizer"

	// snapshotTimeout bounds how long BGSAVE / BGREWRITEAOF may take.
	snapshotTimeout = 10 * time.Minute

	// validationRetryDelay is how long to wait before retrying a spec that
	// referenced something that does not exist yet, such as a Secret.
	validationRetryDelay = 30 * time.Second
)

// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis;rediss;redisreplications;redisclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch

// Reconciler reconciles a RedisBackup object.
type Reconciler struct {
	client.Client
	K8sClient  kubernetes.Interface
	RESTConfig *rest.Config

	// backupFn performs the backup and returns its storage location. It
	// defaults to performBackup and is only overridden in tests, which have
	// no live Redis pod or S3 bucket to talk to.
	backupFn func(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error)
}

// Reconcile drives a RedisBackup to completion exactly once.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &redisv1alpha1.RedisBackup{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get RedisBackup instance")
	}

	// ── Deletion ─────────────────────────────────────────────────────────────
	if instance.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(instance, RedisBackupFinalizer) {
			if err := r.cleanup(ctx, instance); err != nil {
				// Never block deletion on storage cleanup; surface it and move on.
				logger.Error(err, "backup cleanup failed; removing finalizer anyway")
			}
			controllerutil.RemoveFinalizer(instance, RedisBackupFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return intctrlutil.RequeueE(ctx, err, "failed to remove finalizer")
			}
		}
		return intctrlutil.Reconciled()
	}

	if !controllerutil.ContainsFinalizer(instance, RedisBackupFinalizer) {
		controllerutil.AddFinalizer(instance, RedisBackupFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to add finalizer")
		}
		return intctrlutil.Requeue()
	}

	// ── Idempotency ──────────────────────────────────────────────────────────
	// A completed backup is terminal. Without this the controller would redo
	// the whole BGSAVE and upload on every requeue. The only thing left to do
	// for a completed backup is to honour its retention, if it has one.
	if instance.Status.Phase == redisv1alpha1.BackupPhaseCompleted {
		return r.reconcileRetention(ctx, instance)
	}

	// ── Validation ───────────────────────────────────────────────────────────
	if err := r.validateSpec(ctx, instance); err != nil {
		logger.Error(err, "RedisBackup spec validation failed")
		if statusErr := r.markFailed(ctx, instance, err.Error()); statusErr != nil {
			return intctrlutil.RequeueE(ctx, statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: validationRetryDelay}, nil
	}

	if err := r.setPhase(ctx, instance, redisv1alpha1.BackupPhaseRunning, "Backup is in progress"); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to set Running phase")
	}

	// ── Execute ──────────────────────────────────────────────────────────────
	location, err := r.backup(ctx, instance)
	if err != nil {
		logger.Error(err, "Backup execution failed")
		if statusErr := r.markFailed(ctx, instance, fmt.Sprintf("Backup failed: %v", err)); statusErr != nil {
			return intctrlutil.RequeueE(ctx, statusErr, "failed to update status")
		}
		// Return the error so controller-runtime retries with backoff. Returning
		// nil here would make a transient S3 or Redis hiccup permanent, and yet
		// leave the resource eligible to be re-run by whatever unrelated event
		// next enqueues it (a Secret change, the informer resync, a restart).
		return ctrl.Result{}, err
	}

	// ── Complete ─────────────────────────────────────────────────────────────
	// The status write re-reads the object first: this reconcile can run for
	// minutes, during which the copy held here goes stale and a plain update
	// would fail the conflict check, leaving the backup un-completed and
	// looping forever.
	if err := r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisBackup) {
		now := metav1.NewTime(time.Now().UTC())
		cur.Status.Phase = redisv1alpha1.BackupPhaseCompleted
		cur.Status.Message = "Backup completed successfully"
		cur.Status.BackupLocation = location
		cur.Status.LastBackupTime = &now
		meta.SetStatusCondition(&cur.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cur.Generation,
			Reason:             "BackupSucceeded",
			Message:            fmt.Sprintf("Backup completed: %s", location),
		})
	}); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to update status to Completed")
	}

	logger.Info("RedisBackup completed successfully", "location", location)
	return intctrlutil.Reconciled()
}

// reconcileRetention deletes a completed backup's objects once retentionDays
// has elapsed, and otherwise schedules the reconcile that will.
//
// retentionDays is opt-in: zero means keep forever. It used to default to 7
// while nothing implemented it, so the field promised an expiry that never
// came; now it either does exactly what it says or does nothing.
func (r *Reconciler) reconcileRetention(ctx context.Context, instance *redisv1alpha1.RedisBackup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if instance.Spec.RetentionDays <= 0 || instance.Status.ExpiredTime != nil || instance.Status.LastBackupTime == nil {
		return intctrlutil.Reconciled()
	}
	expiry := instance.Status.LastBackupTime.Add(time.Duration(instance.Spec.RetentionDays) * 24 * time.Hour)
	if remaining := time.Until(expiry); remaining > 0 {
		return intctrlutil.RequeueAfter(ctx, remaining, "backup retention not yet reached", "expiresAt", expiry.UTC().Format(time.RFC3339))
	}

	cfg := instance.Spec.S3
	if cfg == nil || cfg.Bucket == "" {
		return intctrlutil.Reconciled()
	}
	accessKey, secretKey, err := r.credentials(ctx, instance.Namespace, cfg.SecretName)
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "cannot expire backup: credentials unavailable")
	}
	s3c, err := backuputil.NewS3Client(ctx, backuputil.S3Params{
		Bucket: cfg.Bucket, Region: cfg.Region, Endpoint: cfg.Endpoint,
		AccessKey: accessKey, SecretKey: secretKey,
	})
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "cannot expire backup: S3 client")
	}
	deleted, err := backuputil.DeletePrefix(ctx, s3c, cfg.Bucket, destinationPrefix(instance))
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to delete expired backup objects")
	}

	location := instance.Status.BackupLocation
	if err := r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisBackup) {
		now := metav1.NewTime(time.Now().UTC())
		cur.Status.ExpiredTime = &now
		cur.Status.BackupLocation = ""
		cur.Status.Message = fmt.Sprintf("Backup expired after %d day(s); %d object(s) deleted from %s",
			instance.Spec.RetentionDays, deleted, location)
		meta.SetStatusCondition(&cur.Status.Conditions, metav1.Condition{
			Type:               "Expired",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cur.Generation,
			Reason:             "RetentionElapsed",
			Message:            cur.Status.Message,
		})
	}); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to record expiry")
	}
	logger.Info("Backup expired and deleted", "location", location, "objects", deleted, "retentionDays", instance.Spec.RetentionDays)
	return intctrlutil.Reconciled()
}

// validateSpec checks the spec and everything it references.
func (r *Reconciler) validateSpec(ctx context.Context, instance *redisv1alpha1.RedisBackup) error {
	if instance.Spec.RedisClusterName == "" {
		return fmt.Errorf("spec.redisClusterName must not be empty")
	}

	// Reject rather than ignore. Silently accepting a cron expression that
	// nothing will ever act on makes users believe backups are scheduled.
	if instance.Spec.Schedule != "" {
		return fmt.Errorf("spec.schedule is not implemented; remove it and create RedisBackup resources from a CronJob instead")
	}

	if instance.Spec.StorageType != redisv1alpha1.StorageTypeS3 {
		return fmt.Errorf("unsupported storageType %q — currently supported: s3", instance.Spec.StorageType)
	}
	cfg := instance.Spec.S3
	if cfg == nil {
		return fmt.Errorf("spec.s3 is required when storageType is 's3'")
	}
	if cfg.Bucket == "" {
		return fmt.Errorf("spec.s3.bucket must not be empty")
	}
	if cfg.Region == "" {
		return fmt.Errorf("spec.s3.region must not be empty")
	}
	if cfg.SecretName == "" {
		return fmt.Errorf("spec.s3.secretName must not be empty")
	}
	if _, _, err := r.credentials(ctx, instance.Namespace, cfg.SecretName); err != nil {
		return err
	}

	// Fail fast with an actionable message instead of an opaque
	// `pods "<name>-0" not found` from the first exec.
	kind, err := backuputil.ResolveKind(ctx, r.Client, instance.Namespace,
		instance.Spec.RedisClusterName, backuputil.TargetKind(instance.Spec.TargetKind))
	if err != nil {
		return err
	}
	if _, err := backuputil.Resolve(ctx, r.Client, instance.Namespace, instance.Spec.RedisClusterName, kind); err != nil {
		return err
	}
	return nil
}

// credentials reads and validates the referenced S3 secret through the
// uncached clientset (see backuputil.ReadS3Credentials for why).
func (r *Reconciler) credentials(ctx context.Context, namespace, name string) (string, string, error) {
	return backuputil.ReadS3Credentials(ctx, r.K8sClient, namespace, name)
}

func (r *Reconciler) backup(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error) {
	if r.backupFn != nil {
		return r.backupFn(ctx, instance)
	}
	return r.performBackup(ctx, instance)
}

// destinationPrefix is stable for the lifetime of the resource.
//
// A wall-clock prefix gives every retry a fresh destination, so a backup that
// fails and retries scatters partial copies across the bucket and no key is
// ever idempotent. Keying on the resource UID means a retry overwrites its own
// previous attempt and cleanup can find exactly what this resource wrote.
func destinationPrefix(instance *redisv1alpha1.RedisBackup) string {
	return fmt.Sprintf("backups/%s/%s-%s",
		instance.Spec.RedisClusterName, instance.Name, instance.UID)
}

// performBackup snapshots every data-bearing shard and uploads the result.
func (r *Reconciler) performBackup(ctx context.Context, instance *redisv1alpha1.RedisBackup) (string, error) {
	logger := log.FromContext(ctx)
	cfg := instance.Spec.S3

	kind, err := backuputil.ResolveKind(ctx, r.Client, instance.Namespace,
		instance.Spec.RedisClusterName, backuputil.TargetKind(instance.Spec.TargetKind))
	if err != nil {
		return "", err
	}
	targets, err := backuputil.Resolve(ctx, r.Client, instance.Namespace, instance.Spec.RedisClusterName, kind)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "redis-backup-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	x := &backuputil.Executor{K8sClient: r.K8sClient, RESTConfig: r.RESTConfig}

	// A replication's replicas hold a copy that can be arbitrarily stale if
	// their link to the master is down, so the master is identified rather
	// than assumed to be pod-0.
	if kind == backuputil.KindRedisReplication {
		targets, err = backuputil.ResolveReplicationPrimary(targets, func(t backuputil.Target) (string, int, error) {
			return x.ReplicationRole(ctx, t)
		})
		if err != nil {
			return "", err
		}
	}

	primaries := backuputil.Primaries(targets)
	if len(primaries) == 0 {
		return "", fmt.Errorf("no data-bearing pod found for %s/%s", kind, instance.Spec.RedisClusterName)
	}

	// After a cluster failover a leader pod can be serving as a replica of a
	// follower pod. Its on-disk copy is then derived, possibly stale, and its
	// slot ranges belong to someone else, so the backup must not proceed.
	if kind == backuputil.KindRedisCluster {
		for _, t := range primaries {
			isMaster, roleErr := x.IsMaster(ctx, t)
			if roleErr != nil {
				return "", fmt.Errorf("shard %d (%s): %w", t.Shard, t.Pod, roleErr)
			}
			if !isMaster {
				return "", fmt.Errorf("shard %d: leader pod %s is currently a replica (the cluster has failed over); "+
					"run `redis-cli --cluster failover` on it or wait for the operator to restore leadership, then retry", t.Shard, t.Pod)
			}
		}
	}

	manifest := backuputil.Manifest{
		Version:   backuputil.ManifestVersion,
		Kind:      kind,
		OwnerName: instance.Spec.RedisClusterName,
		Shards:    len(primaries),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for _, t := range primaries {
		shardDir := filepath.Join(tmpDir, fmt.Sprintf("shard-%d", t.Shard))
		dataDir := filepath.Join(shardDir, "data")
		if err := os.MkdirAll(dataDir, 0o750); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", dataDir, err)
		}

		layout, err := x.DiscoverLayout(ctx, t)
		if err != nil {
			return "", fmt.Errorf("shard %d (%s): %w", t.Shard, t.Pod, err)
		}
		logger.Info("Resolved Redis layout", "pod", t.Pod, "dir", layout.Dir,
			"appendOnly", layout.AppendOnly, "clusterEnabled", layout.ClusterEnabled)

		if err := x.Snapshot(ctx, t, layout, snapshotTimeout); err != nil {
			return "", fmt.Errorf("shard %d (%s): %w", t.Shard, t.Pod, err)
		}

		// Capture whichever persistence artefacts this server actually uses.
		// With appendonly yes the AOF is the file Redis reloads, so omitting
		// it produces an archive that restores to an empty database.
		entries := layout.DataEntries()
		present := make([]string, 0, len(entries))
		for _, name := range entries {
			if x.FileExists(ctx, t, filepath.Join(layout.Dir, name)) {
				present = append(present, name)
				continue
			}
			if name == layout.DBFilename {
				logger.Info("RDB file absent after BGSAVE; continuing", "pod", t.Pod, "file", name)
				continue
			}
			return "", fmt.Errorf("shard %d (%s): expected %s in %s but it is missing",
				t.Shard, t.Pod, name, layout.Dir)
		}
		if len(present) == 0 {
			return "", fmt.Errorf("shard %d (%s): no persistence files found in %s", t.Shard, t.Pod, layout.Dir)
		}
		if err := x.CopyFrom(ctx, t, layout.Dir, present, dataDir); err != nil {
			return "", fmt.Errorf("shard %d (%s): %w", t.Shard, t.Pod, err)
		}

		// Recorded as close to the copy as possible so the restore can verify
		// the loaded dataset, and for a cluster, rebuild slot ownership
		// without shipping nodes.conf. On a busy master a few writes can
		// still land between the AOF read and this call.
		if err := x.CaptureShardInfo(ctx, t, &layout); err != nil {
			return "", fmt.Errorf("shard %d (%s): %w", t.Shard, t.Pod, err)
		}

		// The cluster node file lives on its own mount (/node-conf/nodes.conf
		// under this operator), not in the data directory.
		if layout.ClusterEnabled && layout.NodeConfigFile != "" {
			nodeDir := filepath.Join(shardDir, "node")
			if err := os.MkdirAll(nodeDir, 0o750); err != nil {
				return "", fmt.Errorf("failed to create %s: %w", nodeDir, err)
			}
			if x.FileExists(ctx, t, layout.NodeConfigFile) {
				if err := x.CopyFrom(ctx, t, filepath.Dir(layout.NodeConfigFile),
					[]string{filepath.Base(layout.NodeConfigFile)}, nodeDir); err != nil {
					return "", fmt.Errorf("shard %d (%s): failed to copy %s: %w",
						t.Shard, t.Pod, layout.NodeConfigFile, err)
				}
			} else {
				logger.Info("cluster-config-file not present on disk", "pod", t.Pod, "path", layout.NodeConfigFile)
			}
		}

		manifest.PerShard = append(manifest.PerShard, layout)
	}

	blob, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), blob, 0o600); err != nil {
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	accessKey, secretKey, err := r.credentials(ctx, instance.Namespace, cfg.SecretName)
	if err != nil {
		return "", err
	}
	s3c, err := backuputil.NewS3Client(ctx, backuputil.S3Params{
		Bucket: cfg.Bucket, Region: cfg.Region, Endpoint: cfg.Endpoint,
		AccessKey: accessKey, SecretKey: secretKey,
	})
	if err != nil {
		return "", err
	}

	prefix := destinationPrefix(instance)
	// Clear any partial content from a previous attempt so the destination
	// reflects this run alone.
	if _, err := backuputil.DeletePrefix(ctx, s3c, cfg.Bucket, prefix); err != nil {
		logger.Info("could not clear previous attempt; continuing", "error", err.Error())
	}
	if err := backuputil.UploadDir(ctx, s3c, cfg.Bucket, prefix, tmpDir); err != nil {
		return "", err
	}

	location := fmt.Sprintf("s3://%s/%s", cfg.Bucket, prefix)
	logger.Info("Backup uploaded", "location", location, "shards", len(primaries))
	return location, nil
}

// cleanup removes stored objects when the resource is deleted and the user
// asked for that. The default is to retain, because deleting a backup resource
// should not silently destroy the only copy of the data.
func (r *Reconciler) cleanup(ctx context.Context, instance *redisv1alpha1.RedisBackup) error {
	if instance.Spec.CleanupPolicy != redisv1alpha1.CleanupPolicyDelete {
		return nil
	}
	cfg := instance.Spec.S3
	if cfg == nil || cfg.Bucket == "" {
		return nil
	}
	accessKey, secretKey, err := r.credentials(ctx, instance.Namespace, cfg.SecretName)
	if err != nil {
		return err
	}
	s3c, err := backuputil.NewS3Client(ctx, backuputil.S3Params{
		Bucket: cfg.Bucket, Region: cfg.Region, Endpoint: cfg.Endpoint,
		AccessKey: accessKey, SecretKey: secretKey,
	})
	if err != nil {
		return err
	}
	n, err := backuputil.DeletePrefix(ctx, s3c, cfg.Bucket, destinationPrefix(instance))
	if err != nil {
		return err
	}
	log.FromContext(ctx).Info("Deleted backup objects", "count", n)
	return nil
}

// updateStatus re-reads the resource before mutating status, so a long-running
// reconcile cannot fail on a stale resourceVersion.
func (r *Reconciler) updateStatus(ctx context.Context, instance *redisv1alpha1.RedisBackup, mutate func(*redisv1alpha1.RedisBackup)) error {
	key := client.ObjectKeyFromObject(instance)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cur := &redisv1alpha1.RedisBackup{}
		if err := r.Get(ctx, key, cur); err != nil {
			return err
		}
		mutate(cur)
		if err := r.Status().Update(ctx, cur); err != nil {
			return err
		}
		cur.DeepCopyInto(instance)
		return nil
	})
}

func (r *Reconciler) setPhase(ctx context.Context, instance *redisv1alpha1.RedisBackup, phase redisv1alpha1.BackupPhase, msg string) error {
	if instance.Status.Phase == phase && instance.Status.Message == msg {
		return nil
	}
	return r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisBackup) {
		cur.Status.Phase = phase
		cur.Status.Message = msg
	})
}

func (r *Reconciler) markFailed(ctx context.Context, instance *redisv1alpha1.RedisBackup, reason string) error {
	return r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisBackup) {
		cur.Status.Phase = redisv1alpha1.BackupPhaseFailed
		cur.Status.Message = reason
		meta.SetStatusCondition(&cur.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cur.Generation,
			Reason:             "BackupFailed",
			Message:            reason,
		})
	})
}

// SetupWithManager registers this controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Status writes must not re-trigger reconciliation. Without this the
		// controller wakes itself on its own status update and the delayed
		// retry below never actually delays anything.
		//
		// There is deliberately no Watches(&corev1.Secret{}) here. A Secret
		// watch through the manager starts an informer over every Secret in
		// every namespace the operator can see — held in its memory, against a
		// 100Mi limit — for the sole benefit of noticing a credentials Secret
		// created after the RedisBackup. The validation-failure path already
		// requeues every 30s for exactly that case.
		For(&redisv1alpha1.RedisBackup{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(opts).
		Complete(r)
}
