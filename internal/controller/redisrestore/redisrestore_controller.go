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

package redisrestore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisbackup/v1alpha1"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/backuputil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	RedisRestoreFinalizer = "redisrestore.redis.redis.opstreelabs.in/finalizer"

	scaleDownTimeout     = 5 * time.Minute
	readyTimeout         = 10 * time.Minute
	replicaSyncTimeout   = 10 * time.Minute
	rollbackTimeout      = 5 * time.Minute
	clusterFormTimeout   = 3 * time.Minute
	pollInterval         = 2 * time.Second
	validationRetryDelay = 30 * time.Second
)

// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis;rediss;redisreplications;redisclusters;redissentinels,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets/scale,verbs=get;update;patch

type Reconciler struct {
	client.Client
	K8sClient  kubernetes.Interface
	RESTConfig *rest.Config
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &redisv1alpha1.RedisRestore{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get RedisRestore instance")
	}

	if instance.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(instance, RedisRestoreFinalizer) {
			controllerutil.RemoveFinalizer(instance, RedisRestoreFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return intctrlutil.RequeueE(ctx, err, "failed to remove finalizer")
			}
		}
		return intctrlutil.Reconciled()
	}

	if !controllerutil.ContainsFinalizer(instance, RedisRestoreFinalizer) {
		controllerutil.AddFinalizer(instance, RedisRestoreFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to add finalizer")
		}
		return intctrlutil.Requeue()
	}

	// A restore is destructive and must never run twice off one resource.
	switch instance.Status.Phase {
	case redisv1alpha1.RestorePhaseCompleted:
		return intctrlutil.Reconciled()
	case redisv1alpha1.RestorePhaseFailed:
		// Require an explicit spec change before touching the workload again.
		if instance.Status.ObservedGeneration == instance.Generation {
			return intctrlutil.Reconciled()
		}
	case redisv1alpha1.RestorePhaseRunning:
		// A Running restore with a rollback checkpoint is one the operator was
		// restarted in the middle of. Its defers never ran. Put the workload
		// back from the checkpoint rather than blindly starting over.
		if instance.Status.Rollback != nil {
			return r.recoverInterrupted(ctx, instance)
		}
	}

	if err := r.validateSpec(ctx, instance); err != nil {
		logger.Error(err, "RedisRestore spec validation failed")
		// Not terminal: the Secret or target may simply not exist yet, and the
		// requeue below re-validates. Stamping ObservedGeneration here would
		// make the terminal check above swallow that retry.
		if statusErr := r.markFailed(ctx, instance, err.Error(), false); statusErr != nil {
			return intctrlutil.RequeueE(ctx, statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: validationRetryDelay}, nil
	}

	if err := r.setPhase(ctx, instance, redisv1alpha1.RestorePhaseRunning, "Restore is in progress"); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to set Running phase")
	}

	if err := r.performRestore(ctx, instance); err != nil {
		logger.Error(err, "Restore failed")
		// Terminal: the workload was touched. A destructive restore must not
		// re-run itself; the user changes the spec to try again.
		return ctrl.Result{}, r.markFailed(ctx, instance, fmt.Sprintf("Restore failed: %v", err), true)
	}

	if err := r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisRestore) {
		now := metav1.NewTime(time.Now().UTC())
		cur.Status.Phase = redisv1alpha1.RestorePhaseCompleted
		cur.Status.Message = "Restore completed successfully"
		cur.Status.RestoreCompletedTime = &now
		cur.Status.ObservedGeneration = cur.Generation
		cur.Status.Rollback = nil
		meta.SetStatusCondition(&cur.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cur.Generation,
			Reason:             "RestoreSucceeded",
			Message:            "Restore completed successfully",
		})
	}); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to update status to Completed")
	}

	logger.Info("RedisRestore completed successfully")
	return intctrlutil.Reconciled()
}

func (r *Reconciler) validateSpec(ctx context.Context, instance *redisv1alpha1.RedisRestore) error {
	if instance.Spec.RedisClusterName == "" {
		return fmt.Errorf("spec.redisClusterName must not be empty")
	}
	if instance.Spec.StorageType != redisv1alpha1.StorageTypeS3 {
		return fmt.Errorf("unsupported storageType %q — currently supported: s3", instance.Spec.StorageType)
	}
	cfg := instance.Spec.S3
	if cfg == nil {
		return fmt.Errorf("spec.s3 is required when storageType is 's3'")
	}
	if cfg.SecretName == "" {
		return fmt.Errorf("spec.s3.secretName must not be empty")
	}
	if _, _, err := r.credentials(ctx, instance.Namespace, cfg.SecretName); err != nil {
		return err
	}
	if _, _, err := backuputil.ParseS3URI(instance.Spec.BackupLocation, cfg.Bucket); err != nil {
		return err
	}
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

func (r *Reconciler) credentials(ctx context.Context, namespace, name string) (string, string, error) {
	return backuputil.ReadS3Credentials(ctx, r.K8sClient, namespace, name)
}

// performRestore downloads the backup and pushes it into the target workload.
//
// The whole sequence is guarded: the owning controller is suspended for the
// duration and the original replica count is always put back, on every exit
// path, so a failure cannot leave Redis scaled to zero.
func (r *Reconciler) performRestore(ctx context.Context, instance *redisv1alpha1.RedisRestore) (err error) {
	logger := log.FromContext(ctx)
	cfg := instance.Spec.S3
	ns := instance.Namespace

	kind, err := backuputil.ResolveKind(ctx, r.Client, ns, instance.Spec.RedisClusterName,
		backuputil.TargetKind(instance.Spec.TargetKind))
	if err != nil {
		return err
	}
	targets, err := backuputil.Resolve(ctx, r.Client, ns, instance.Spec.RedisClusterName, kind)
	if err != nil {
		return err
	}

	x := &backuputil.Executor{K8sClient: r.K8sClient, RESTConfig: r.RESTConfig}

	// Rollback must run even if the reconcile's context is cancelled, or the
	// workload is left scaled to zero with the owning controller suspended.
	// It carries no deadline of its own: each rollback step takes a fresh
	// rollbackTimeout when it runs, so a long (even successful) restore does
	// not reach its defers with an already-expired context.
	rbBase := context.WithoutCancel(ctx)
	rollback := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(rbBase, rollbackTimeout)
	}

	tmpDir, err := os.MkdirTemp("", "redis-restore-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// ── Fetch and validate the archive before touching the workload ──────────
	bucket, prefix, err := backuputil.ParseS3URI(instance.Spec.BackupLocation, cfg.Bucket)
	if err != nil {
		return err
	}
	accessKey, secretKey, err := r.credentials(ctx, ns, cfg.SecretName)
	if err != nil {
		return err
	}
	s3c, err := backuputil.NewS3Client(ctx, backuputil.S3Params{
		Bucket: bucket, Region: cfg.Region, Endpoint: cfg.Endpoint,
		AccessKey: accessKey, SecretKey: secretKey,
	})
	if err != nil {
		return err
	}
	n, err := backuputil.DownloadPrefix(ctx, s3c, bucket, prefix, tmpDir)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no objects found at s3://%s/%s", bucket, prefix)
	}
	logger.Info("Downloaded backup", "objects", n, "location", instance.Spec.BackupLocation)

	manifest, err := readManifest(tmpDir)
	if err != nil {
		return err
	}
	if manifest.Kind != kind {
		return fmt.Errorf("backup was taken from a %s but target %q is a %s; refusing to restore across kinds",
			manifest.Kind, instance.Spec.RedisClusterName, kind)
	}
	primaries := backuputil.Primaries(targets)
	secondaries := backuputil.Secondaries(targets)
	// Every shard's files are checked here, before anything is suspended or
	// scaled. A partially uploaded backup (manifest present, a shard missing)
	// must be rejected while the workload is still intact, not discovered
	// after earlier shards have already been wiped.
	for i := range manifest.PerShard {
		dataDir := filepath.Join(tmpDir, fmt.Sprintf("shard-%d", i), "data")
		if _, statErr := os.Stat(dataDir); statErr != nil {
			return fmt.Errorf("backup is incomplete: shard-%d/data is missing from %s", i, instance.Spec.BackupLocation)
		}
		shard := manifest.PerShard[i]
		if aof := shard.AOFEntry(); aof != "" {
			if _, statErr := os.Stat(filepath.Join(dataDir, aof)); statErr != nil {
				return fmt.Errorf("backup is incomplete: shard-%d was taken with appendonly=yes but has no %s", i, aof)
			}
		} else if _, statErr := os.Stat(filepath.Join(dataDir, shard.DBFilename)); statErr != nil {
			return fmt.Errorf("backup is incomplete: shard-%d has no %s", i, shard.DBFilename)
		}
	}
	// Every primary is checked for compatibility with its shard BEFORE the
	// destructive flag is raised, so a mismatch is a clean refusal, not a
	// "target left suspended". The pods have to be up to be asked — a target
	// mid-rollout (an operator upgrade recreating its pods, say) is waited for
	// rather than failed on a transient "container not found".
	for _, t := range targets {
		if err := r.waitPodReady(ctx, x, t); err != nil {
			return err
		}
	}
	for i, t := range primaries {
		dst, err := x.DiscoverLayout(ctx, t)
		if err != nil {
			return fmt.Errorf("shard %d (%s): %w", i, t.Pod, err)
		}
		if err := checkShardCompatible(t, manifest.PerShard[i], dst); err != nil {
			return err
		}
	}
	if manifest.Shards != len(primaries) {
		return fmt.Errorf("backup has %d shard(s) but target %q has %d data-bearing pod(s); "+
			"refusing to restore a mismatched topology",
			manifest.Shards, instance.Spec.RedisClusterName, len(primaries))
	}

	// ── Suspend the owning controller ────────────────────────────────────────
	// Scaling an owned StatefulSet directly is otherwise reverted within
	// seconds by whichever controller owns the CR, and the scale-down never
	// completes.
	// Sentinels first. Annotating the RedisReplication below emits an event
	// the sentinel controller watches, so it has to be suspended before that
	// write lands, and its daemons have to be gone before any pod goes down —
	// otherwise a sentinel promotes a not-yet-emptied replica mid-restore.
	// Every change to the target is checkpointed into status BEFORE it is made,
	// so a reconcile after an operator restart knows exactly what to undo.
	owner := string(instance.UID)
	checkpoint := func(mutate func(rb *redisv1alpha1.RestoreRollbackState)) error {
		return r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisRestore) {
			if cur.Status.Rollback == nil {
				cur.Status.Rollback = &redisv1alpha1.RestoreRollbackState{TargetKind: string(kind)}
			}
			mutate(cur.Status.Rollback)
		})
	}

	var sentinels *sentinelSuspension
	if kind == backuputil.KindRedisReplication {
		sentinels, err = r.pauseSentinelControllers(ctx, ns, instance.Spec.RedisClusterName, owner)
		if err != nil {
			return err
		}
		defer func() {
			rbCtx, cancel := rollback()
			defer cancel()
			if resumeErr := sentinels.resume(rbCtx); resumeErr != nil {
				logger.Error(resumeErr, "failed to resume sentinels")
				if err == nil {
					err = resumeErr
				}
			}
		}()
		if err = checkpoint(func(rb *redisv1alpha1.RestoreRollbackState) {
			rb.SentinelsAnnotated = append([]string(nil), sentinels.annotated...)
		}); err != nil {
			return err
		}
	}

	restoreOwner, heldBy, err := r.setSkipReconcile(ctx, kind, ns, instance.Spec.RedisClusterName, owner, true)
	if err != nil {
		return err
	}
	if !restoreOwner && heldBy != owner {
		// Either a user paused this workload deliberately or another restore
		// holds it. Neither is ours to clear, so do not start.
		who := "a user pause"
		if heldBy != "" {
			who = "RedisRestore " + heldBy
		}
		return fmt.Errorf("%s %q is already suspended (%s); refusing to restore a paused or already-restoring target",
			kind, instance.Spec.RedisClusterName, who)
	}
	if err = checkpoint(func(rb *redisv1alpha1.RestoreRollbackState) { rb.Annotated = true }); err != nil {
		return err
	}
	// Once a restore has destroyed the old dataset, a failure must NOT hand
	// the half-rebuilt workload back to its controller — for a cluster that
	// controller's recovery path runs CLUSTER RESET and FLUSHALL. The flag is
	// raised at the point of no return and leaves the annotation in place.
	keepSuspended := false
	defer func() {
		if err != nil && keepSuspended {
			logger.Error(err, "restore failed after the point of no return; leaving the target suspended for manual recovery",
				"annotation", func() string { a, _ := backuputil.SkipReconcileAnnotation(kind); return a }())
			err = fmt.Errorf("%w (target left suspended: remove the skip-reconcile annotation on %s %q only after recovering it by hand)",
				err, kind, instance.Spec.RedisClusterName)
			return
		}
		rbCtx, cancel := rollback()
		defer cancel()
		if _, _, clearErr := r.setSkipReconcile(rbCtx, kind, ns, instance.Spec.RedisClusterName, owner, false); clearErr != nil {
			logger.Error(clearErr, "failed to clear skip-reconcile annotation")
			if err == nil {
				err = clearErr
			}
		}
	}()

	// Only now, with the replication controller suspended, can the embedded
	// sentinel StatefulSet be scaled away — that controller re-applies its
	// replica count every reconcile, and would have reverted an earlier
	// scale-down (or raced it, leaving live sentinels for the whole restore).
	if sentinels != nil {
		if err := sentinels.scaleDown(ctx, r); err != nil {
			return err
		}
		if err = checkpoint(func(rb *redisv1alpha1.RestoreRollbackState) {
			rb.SentinelReplicas = map[string]int32{}
			for k, v := range sentinels.sizes {
				rb.SentinelReplicas[k] = v
			}
		}); err != nil {
			return err
		}
	}

	// Name the restore destination as master before anything destructive
	// happens, so that if the restore fails partway the resumed controller
	// does not elect whichever pod its stale status still names.
	if kind == backuputil.KindRedisReplication {
		if mErr := r.setReplicationMaster(ctx, ns, instance.Spec.RedisClusterName, primaries[0].Pod); mErr != nil {
			return mErr
		}
	}

	// ── Scale down, remembering the original size ────────────────────────────
	// Every StatefulSet backing the target is scaled, not just the one holding
	// the primary. A cluster's followers live in their own StatefulSet, and
	// leaving them running would keep pre-restore data serving and replicating.
	stsNames := backuputil.StatefulSetNames(targets)
	original := make(map[string]int32, len(stsNames))
	for _, name := range stsNames {
		sts, getErr := r.K8sClient.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("failed to get statefulset %q: %w", name, getErr)
		}
		size := int32(1)
		if sts.Spec.Replicas != nil {
			size = *sts.Spec.Replicas
		}
		original[name] = size
	}
	if err = checkpoint(func(rb *redisv1alpha1.RestoreRollbackState) {
		rb.StatefulSetReplicas = map[string]int32{}
		for k, v := range original {
			rb.StatefulSetReplicas[k] = v
		}
	}); err != nil {
		return err
	}

	// Restoring the original size is registered before the first scale-down,
	// so every later failure still brings the workload back up.
	defer func() {
		rbCtx, cancel := rollback()
		defer cancel()
		for name, size := range original {
			if scaleErr := r.scale(rbCtx, ns, name, size); scaleErr != nil {
				logger.Error(scaleErr, "failed to restore replica count", "statefulset", name, "replicas", size)
				if err == nil {
					err = scaleErr
				}
			}
		}
	}()

	// A cluster is rebuilt rather than restored in place: followers go down
	// first, leaders are dissolved and reloaded, slot ownership is rebuilt from
	// the manifest, and followers rejoin last. That sequence owns its own
	// scaling, so it branches here.
	markDestructive := func() error {
		keepSuspended = true
		return checkpoint(func(rb *redisv1alpha1.RestoreRollbackState) { rb.Destructive = true })
	}
	if kind == backuputil.KindRedisCluster {
		return r.restoreCluster(ctx, x, instance, manifest, tmpDir, primaries, secondaries, original, markDestructive)
	}

	// Standalone and replication are restored in place. Scaling the
	// StatefulSet to zero and back would make every pod load its old dataset
	// once more only to have it replaced, doubling the outage; the pods are
	// dissolved where they run and restarted exactly once below.
	for _, t := range targets {
		if waitErr := r.waitPodReady(ctx, x, t); waitErr != nil {
			return waitErr
		}
	}

	// Every pod whose persistence we switch off is recorded so a failure
	// before the restart can switch it back on; otherwise the pod keeps
	// serving with disk and memory diverging until its next container start.
	type touchedPod struct {
		t backuputil.Target
		l backuputil.Layout
	}
	var touched []touchedPod
	restarted := false
	defer func() {
		if err == nil || restarted {
			return
		}
		rbCtx, cancel := rollback()
		defer cancel()
		for _, tp := range touched {
			if pErr := x.EnablePersistence(rbCtx, tp.t, tp.l); pErr != nil {
				logger.Error(pErr, "failed to re-enable persistence during rollback", "pod", tp.t.Pod)
			}
		}
	}()

	// ── Replace the on-disk state, shard by shard ────────────────────────────
	// From here the old dataset is gone; an interrupted restore must not be
	// auto-rolled-back past this point.
	if err = markDestructive(); err != nil {
		return err
	}
	for i, t := range primaries {
		shardDir := filepath.Join(tmpDir, fmt.Sprintf("shard-%d", t.Shard))
		if _, statErr := os.Stat(shardDir); statErr != nil {
			return fmt.Errorf("backup is missing shard-%d", t.Shard)
		}
		dstLayout, restoreErr := r.restoreShard(ctx, x, t, shardDir, manifest.PerShard[i])
		if dstLayout != nil {
			touched = append(touched, touchedPod{t: t, l: *dstLayout})
		}
		if restoreErr != nil {
			return restoreErr
		}
	}

	// ── Empty every pod that is not a restore destination ────────────────────
	// A replica or follower keeps its own PVC through the scale cycle. Left
	// alone it comes back holding pre-restore data, can serve stale reads, and
	// can be elected master — replicating the old dataset back over the one we
	// just restored.
	for _, t := range secondaries {
		layout, layoutErr := x.DiscoverLayout(ctx, t)
		if layoutErr != nil {
			return fmt.Errorf("%s: %w", t.Pod, layoutErr)
		}
		touched = append(touched, touchedPod{t: t, l: layout})
		if clearErr := x.ClearData(ctx, t, layout); clearErr != nil {
			return clearErr
		}
		logger.Info("Cleared non-primary pod", "pod", t.Pod)
	}

	// ── Restart so Redis loads what we just wrote ────────────────────────────
	// Copying files under a running server changes nothing by itself: the
	// dataset in memory is authoritative and would be flushed back over the
	// restored files. Persistence was disabled per pod above, so terminating
	// now cannot overwrite them.
	restarted = true
	for _, t := range targets {
		if delErr := r.K8sClient.CoreV1().Pods(ns).Delete(ctx, t.Pod, metav1.DeleteOptions{}); delErr != nil && !apierrors.IsNotFound(delErr) {
			return fmt.Errorf("failed to restart pod %q: %w", t.Pod, delErr)
		}
	}
	for _, t := range targets {
		if waitErr := r.waitPodReady(ctx, x, t); waitErr != nil {
			return waitErr
		}
	}

	// ── Rebuild the topology the restore just tore down ──────────────────────
	if kind == backuputil.KindRedisReplication {
		if topoErr := r.rebuildReplication(ctx, x, instance, primaries[0], secondaries); topoErr != nil {
			return topoErr
		}
	}

	// ── Prove the data is actually there ─────────────────────────────────────
	for i, t := range primaries {
		if verifyErr := r.verifyShard(ctx, x, t, manifest.PerShard[i]); verifyErr != nil {
			return verifyErr
		}
	}
	return nil
}

// verifyShard asserts the pod actually holds the dataset the backup recorded.
func (r *Reconciler) verifyShard(ctx context.Context, x *backuputil.Executor, t backuputil.Target, expected backuputil.Layout) error {
	out, err := x.RedisCLI(ctx, t, "DBSIZE")
	if err != nil {
		return fmt.Errorf("restore finished but %s is not answering: %w", t.Pod, err)
	}
	got, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return fmt.Errorf("restore finished but %s returned an unexpected DBSIZE %q", t.Pod, out)
	}
	// An archive either loads or it does not, so zero is the failure that
	// matters. The recorded count was read from a live server and can differ
	// by the writes or expiries that landed around the copy; a gross shortfall
	// still fails, a small drift is reported.
	if got == 0 && expected.DBSize > 0 {
		return fmt.Errorf("restore finished but %s is empty where the backup recorded %d keys; the archive did not load",
			t.Pod, expected.DBSize)
	}
	if expected.DBSize > 0 && got < expected.DBSize/2 {
		return fmt.Errorf("restore finished but %s holds %d keys where the backup recorded %d; the archive did not load correctly",
			t.Pod, got, expected.DBSize)
	}
	if got != expected.DBSize {
		log.FromContext(ctx).Info("Restored key count differs from the count recorded at backup time",
			"pod", t.Pod, "restored", got, "recorded", expected.DBSize)
	}
	log.FromContext(ctx).Info("Verified shard", "pod", t.Pod, "keys", got)
	return nil
}

// rebuildReplication makes the restored pod the master and re-attaches every
// replica to it.
//
// After the restart each pod is an independent master holding whatever is on
// its own disk. Handing that state back to the replication controller lets it
// elect whichever pod its stale status field names, and if that is not the
// restored one the replicas' PSYNC overwrites the restored data. The topology
// is therefore established here, while the owning controller is still
// suspended, and the CR's status is corrected before it resumes.
func (r *Reconciler) rebuildReplication(ctx context.Context, x *backuputil.Executor, instance *redisv1alpha1.RedisRestore, master backuputil.Target, replicas []backuputil.Target) error {
	logger := log.FromContext(ctx)

	if err := x.PromoteMaster(ctx, master); err != nil {
		return err
	}

	masterPod, err := r.K8sClient.CoreV1().Pods(instance.Namespace).Get(ctx, master.Pod, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get restored master %q: %w", master.Pod, err)
	}
	if masterPod.Status.PodIP == "" {
		return fmt.Errorf("restored master %q has no pod IP", master.Pod)
	}
	port, err := x.RedisPort(ctx, master)
	if err != nil {
		return err
	}

	for _, t := range replicas {
		if err := x.FollowMaster(ctx, t, masterPod.Status.PodIP, port, replicaSyncTimeout); err != nil {
			return err
		}
		logger.Info("Replica re-attached", "pod", t.Pod, "master", master.Pod)
	}

	// The <name>-master Service selects on the redis-role label, which only
	// the (suspended) replication controller normally maintains. Set it here
	// so the Service follows the restored master immediately.
	if err := r.setRoleLabels(ctx, instance.Namespace, master, replicas); err != nil {
		return err
	}

	// Point the CR at the restored master so the replication controller does
	// not fall back to the pre-restore value it still holds in status.
	if err := r.setReplicationMaster(ctx, instance.Namespace, instance.Spec.RedisClusterName, master.Pod); err != nil {
		return err
	}
	logger.Info("Replication rebuilt", "master", master.Pod, "replicas", len(replicas))
	return nil
}

// setReplicationMaster records the restored pod as the master on the CR.
func (r *Reconciler) setReplicationMaster(ctx context.Context, namespace, name, masterPod string) error {
	key := types.NamespacedName{Namespace: namespace, Name: name}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cur := &rrvb2.RedisReplication{}
		if err := r.Get(ctx, key, cur); err != nil {
			return err
		}
		if cur.Status.MasterNode == masterPod {
			return nil
		}
		cur.Status.MasterNode = masterPod
		return r.Status().Update(ctx, cur)
	})
}

// recoverInterrupted handles a restore the operator was restarted in the
// middle of. Its deferred rollback never ran; the checkpoint in status says
// exactly what had been changed.
//
// Before the destructive step the target still holds its data, so everything
// recorded is simply put back and the restore is failed with a message that
// says what happened. After it, the old data is gone and the archive is the
// only copy: nothing is touched, the target stays suspended, and the message
// tells the operator how to finish by hand — handing a half-restored workload
// back to its controller would let that controller reset it.
func (r *Reconciler) recoverInterrupted(ctx context.Context, instance *redisv1alpha1.RedisRestore) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	rb := instance.Status.Rollback
	ns := instance.Namespace
	kind := backuputil.TargetKind(rb.TargetKind)
	owner := string(instance.UID)

	if rb.Destructive {
		msg := fmt.Sprintf("Restore failed: the operator restarted after the target's data had already been replaced. "+
			"%s %q is left suspended with its skip-reconcile annotation set; verify the restored data by hand, "+
			"then remove the annotation to hand it back to its controller", kind, instance.Spec.RedisClusterName)
		logger.Error(nil, "interrupted restore found past its point of no return; leaving target suspended",
			"kind", kind, "target", instance.Spec.RedisClusterName)
		return ctrl.Result{}, r.markFailed(ctx, instance, msg, true)
	}

	logger.Info("Rolling back an interrupted restore", "kind", kind, "target", instance.Spec.RedisClusterName)
	rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
	defer cancel()

	var problems []string
	for name, size := range rb.StatefulSetReplicas {
		if err := r.scale(rbCtx, ns, name, size); err != nil {
			problems = append(problems, fmt.Sprintf("scale %s back to %d: %v", name, size, err))
		}
	}
	for name, size := range rb.SentinelReplicas {
		if size == 0 {
			continue
		}
		if err := r.scale(rbCtx, ns, name, size); err != nil {
			problems = append(problems, fmt.Sprintf("scale sentinel %s back to %d: %v", name, size, err))
		}
	}
	for _, name := range rb.SentinelsAnnotated {
		if _, heldBy, err := r.setSkipReconcile(rbCtx, backuputil.KindRedisSentinel, ns, name, owner, false); err != nil {
			problems = append(problems, fmt.Sprintf("clear skip-reconcile on RedisSentinel %s: %v", name, err))
		} else if heldBy != "" && heldBy != owner {
			problems = append(problems, fmt.Sprintf("skip-reconcile on RedisSentinel %s is held by %s and was left in place", name, heldBy))
		}
	}
	if rb.Annotated {
		if _, heldBy, err := r.setSkipReconcile(rbCtx, kind, ns, instance.Spec.RedisClusterName, owner, false); err != nil {
			problems = append(problems, fmt.Sprintf("clear skip-reconcile on %s %s: %v", kind, instance.Spec.RedisClusterName, err))
		} else if heldBy != "" && heldBy != owner {
			// Someone else took the lock after our attempt died — a user
			// pause or another restore. It is not ours to remove, but the
			// message must not claim the target was fully handed back.
			problems = append(problems, fmt.Sprintf("skip-reconcile on %s %s is held by %s and was left in place", kind, instance.Spec.RedisClusterName, heldBy))
		}
	}

	msg := "Restore failed: the operator restarted mid-restore before any data was changed; the target was rolled back. Edit the spec to retry"
	if len(problems) > 0 {
		msg = fmt.Sprintf("%s. Rollback was incomplete: %s", msg, strings.Join(problems, "; "))
		logger.Error(nil, "interrupted restore rolled back with problems", "problems", problems)
	}
	return ctrl.Result{}, r.markFailed(ctx, instance, msg, true)
}

// restoreCluster rebuilds a RedisCluster from a per-shard archive.
//
// nodes.conf is never restored. It carries node IDs and peer addresses that
// belong to the pods the backup was taken from, Redis rewrites it from memory
// on every state change, and with the CRD default (storage.nodeConfVolume:
// false) it lives on the ephemeral layer and would not survive the restart
// anyway. The manifest records each shard's slot ranges instead, and the
// topology is rebuilt from those: a leader restarted on its archive with a
// fresh nodes.conf claims every slot that holds a key on its own; the rest of
// its range is assigned explicitly, epochs are set, the leaders are meshed,
// and followers are attached last.
func (r *Reconciler) restoreCluster(
	ctx context.Context,
	x *backuputil.Executor,
	instance *redisv1alpha1.RedisRestore,
	manifest backuputil.Manifest,
	tmpDir string,
	leaders, followers []backuputil.Target,
	original map[string]int32,
	markDestructive func() error,
) error {
	logger := log.FromContext(ctx)
	ns := instance.Namespace
	n := len(leaders)

	// ── Pre-flight: the manifest must describe a complete, disjoint keyspace ─
	expected := make([]backuputil.SlotSet, n)
	var union backuputil.SlotSet
	for i, shard := range manifest.PerShard {
		set, err := backuputil.ParseSlotRanges(shard.Slots)
		if err != nil {
			return fmt.Errorf("shard %d: %w", i, err)
		}
		if set.Count() == 0 {
			return fmt.Errorf("shard %d owned no slots when it was backed up; cannot rebuild the keyspace from it", i)
		}
		for slot := range set {
			if set[slot] && union[slot] {
				return fmt.Errorf("shard %d and an earlier shard both claim slot %d; the backup's slot map is inconsistent", i, slot)
			}
			union[slot] = union[slot] || set[slot]
		}
		expected[i] = set
	}
	if got := union.Count(); got != backuputil.TotalSlots {
		return fmt.Errorf("backup covers %d of %d slots; a cluster cannot be rebuilt from an incomplete keyspace", got, backuputil.TotalSlots)
	}

	var followerSts string
	if len(followers) > 0 {
		followerSts = followers[0].StatefulSet
		// The follower StatefulSet is the one thing this path scales to zero.
		// Under whenScaled=Delete that would destroy the followers' PVCs; they
		// are re-synced from the leaders afterwards anyway, but refusing keeps
		// the restore from ever being the thing that deletes a volume.
		sts, err := r.K8sClient.AppsV1().StatefulSets(ns).Get(ctx, followerSts, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get statefulset %q: %w", followerSts, err)
		}
		if pol := sts.Spec.PersistentVolumeClaimRetentionPolicy; pol != nil &&
			pol.WhenScaled == appsv1.DeletePersistentVolumeClaimRetentionPolicyType {
			return fmt.Errorf("statefulset %q has persistentVolumeClaimRetentionPolicy.whenScaled=Delete; "+
				"a cluster restore scales it to zero and would destroy its volumes. Set whenScaled to Retain first", followerSts)
		}
	}

	// ── 1. Followers down; leaders stay up ───────────────────────────────────
	// The leader preStop hook runs `cluster failover` on the best replica, so
	// followers must be gone before any leader restarts. The leaders
	// themselves are dissolved in place rather than scaled away: cycling them
	// would make each one load its old AOF (sequentially, under OrderedReady)
	// only to be flushed, and then load the archive on the way back — twice
	// the outage for no gain. Waiting for the follower scale-down is also the
	// proof that the suspension took, since the cluster controller reverts
	// follower replicas whenever the leader StatefulSet is Ready.
	if followerSts != "" {
		if err := r.scale(ctx, ns, followerSts, 0); err != nil {
			return err
		}
		if err := r.waitScaledDown(ctx, ns, followerSts); err != nil {
			return err
		}
		logger.Info("Followers scaled down", "statefulset", followerSts)
	}
	for _, t := range leaders {
		if err := r.waitPodReady(ctx, x, t); err != nil {
			return err
		}
	}

	// ── 3. Dissolve each leader and load its shard ───────────────────────────
	// From the first dissolve on, the old dataset is gone. A failure after
	// this point leaves the target suspended rather than handing a half-built
	// cluster to a controller that would reset it.
	if err := markDestructive(); err != nil {
		return err
	}
	for i, t := range leaders {
		shardDir := filepath.Join(tmpDir, fmt.Sprintf("shard-%d", t.Shard))
		dataDir := filepath.Join(shardDir, "data")
		if _, err := os.Stat(dataDir); err != nil {
			return fmt.Errorf("backup is missing shard-%d/data", t.Shard)
		}
		layout, err := x.DiscoverLayout(ctx, t)
		if err != nil {
			return fmt.Errorf("shard %d (%s): %w", i, t.Pod, err)
		}
		if layout.AppendOnly {
			if _, statErr := os.Stat(filepath.Join(dataDir, layout.AppendDirname)); statErr != nil {
				return fmt.Errorf("shard %d (%s): target loads its append-only file on start but the backup has no %s; "+
					"it cannot be restored", i, t.Pod, layout.AppendDirname)
			}
		}
		if err := x.DissolveClusterNode(ctx, t, layout); err != nil {
			return fmt.Errorf("shard %d: %w", i, err)
		}
		if err := x.CopyTo(ctx, t, dataDir, layout.Dir); err != nil {
			return fmt.Errorf("shard %d (%s): failed to load data: %w", i, t.Pod, err)
		}
		// Same rule as the standalone path: the archive's file names are the
		// source server's; the leader loads whatever its own config names.
		if err := x.AlignNames(ctx, t, manifest.PerShard[i], layout); err != nil {
			return fmt.Errorf("shard %d (%s): %w", i, t.Pod, err)
		}
		logger.Info("Leader dissolved and reloaded", "pod", t.Pod, "shard", t.Shard)
	}

	// ── 4. Restart leaders so Redis loads the archives ───────────────────────
	if err := r.restartPods(ctx, x, leaders); err != nil {
		return err
	}

	// ── 5. Slot ownership and epochs, before any node knows a peer ───────────
	// SET-CONFIG-EPOCH is refused once a node has met anyone, so this runs
	// while every leader is still alone.
	ids := make([]string, n)
	for i, t := range leaders {
		owned, err := x.OwnedSlots(ctx, t)
		if err != nil {
			return err
		}
		if !owned.SubsetOf(expected[i]) {
			return fmt.Errorf("shard %d (%s) claimed slots outside its backed-up range after loading; "+
				"the archive holds keys that do not belong to this shard", i, t.Pod)
		}
		if err := x.AddSlots(ctx, t, expected[i].Minus(owned)); err != nil {
			return fmt.Errorf("shard %d: %w", i, err)
		}
		if err := x.SetConfigEpoch(ctx, t, i+1); err != nil {
			return fmt.Errorf("shard %d: %w", i, err)
		}
		if ids[i], err = x.NodeID(ctx, t); err != nil {
			return err
		}
		logger.Info("Slot ownership rebuilt", "pod", t.Pod, "slots", expected[i].Count(), "autoClaimed", owned.Count(), "nodeId", ids[i])
	}

	// ── 6. Mesh the leaders ──────────────────────────────────────────────────
	addrs := make([]backuputil.ClusterAddress, n)
	for i, t := range leaders {
		pod, err := r.K8sClient.CoreV1().Pods(ns).Get(ctx, t.Pod, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get %s: %w", t.Pod, err)
		}
		if addrs[i], err = x.AnnouncedAddress(ctx, t, pod.Status.PodIP); err != nil {
			return err
		}
	}
	for i := 1; i < n; i++ {
		if err := x.Meet(ctx, leaders[0], addrs[i]); err != nil {
			return err
		}
	}
	if err := x.WaitClusterOK(ctx, leaders[0], n, clusterFormTimeout); err != nil {
		return err
	}
	logger.Info("Leaders meshed", "nodes", n)

	// ── 7. Verify every leader holds its shard ───────────────────────────────
	for i, t := range leaders {
		if err := r.verifyShard(ctx, x, t, manifest.PerShard[i]); err != nil {
			return err
		}
	}

	// ── 8. Followers: back, dissolved, restarted, attached ───────────────────
	if followerSts != "" && len(followers) > 0 {
		if err := r.scale(ctx, ns, followerSts, original[followerSts]); err != nil {
			return err
		}
		for _, t := range followers {
			if err := r.waitPodReady(ctx, x, t); err != nil {
				return err
			}
		}
		for _, t := range followers {
			layout, err := x.DiscoverLayout(ctx, t)
			if err != nil {
				return fmt.Errorf("%s: %w", t.Pod, err)
			}
			if err := x.DissolveClusterNode(ctx, t, layout); err != nil {
				return err
			}
		}
		if err := r.restartPods(ctx, x, followers); err != nil {
			return err
		}
		for _, t := range followers {
			// The operator's own mapping: follower j replicates leader j % n.
			l := int(t.Ordinal) % n
			if err := x.Meet(ctx, t, addrs[l]); err != nil {
				return err
			}
			if err := x.WaitKnownMaster(ctx, t, ids[l], clusterFormTimeout); err != nil {
				return err
			}
			if err := x.Replicate(ctx, t, ids[l]); err != nil {
				return err
			}
			if err := x.WaitReplicaSynced(ctx, t, addrs[l].IP, addrs[l].Port, replicaSyncTimeout); err != nil {
				return err
			}
			logger.Info("Follower attached", "pod", t.Pod, "leader", leaders[l].Pod)
		}
		if err := x.WaitClusterOK(ctx, leaders[0], n+len(followers), clusterFormTimeout); err != nil {
			return err
		}
	}

	// ── 9. Labels the cluster controller would otherwise maintain ───────────
	for i, t := range leaders {
		var mine []backuputil.Target
		for _, f := range followers {
			if int(f.Ordinal)%n == i {
				mine = append(mine, f)
			}
		}
		if err := r.setRoleLabels(ctx, ns, t, mine); err != nil {
			return err
		}
	}

	logger.Info("Cluster restored", "leaders", n, "followers", len(followers))
	return nil
}

// restartPods deletes the pods and waits for their Redis containers to answer.
func (r *Reconciler) restartPods(ctx context.Context, x *backuputil.Executor, pods []backuputil.Target) error {
	for _, t := range pods {
		if err := r.K8sClient.CoreV1().Pods(t.Namespace).Delete(ctx, t.Pod, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to restart pod %q: %w", t.Pod, err)
		}
	}
	// A deleted pod can still report Ready for a moment; give the kubelet a
	// beat to tear it down before polling for the replacement.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(pollInterval):
	}
	for _, t := range pods {
		if err := r.waitPodReady(ctx, x, t); err != nil {
			return err
		}
	}
	return nil
}

// restoreShard replaces one pod's persistence files with the backed-up copy.
func (r *Reconciler) restoreShard(ctx context.Context, x *backuputil.Executor, t backuputil.Target, shardDir string, src backuputil.Layout) (*backuputil.Layout, error) {
	logger := log.FromContext(ctx)

	dst, err := x.DiscoverLayout(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("shard %d (%s): %w", t.Shard, t.Pod, err)
	}

	dataDir := filepath.Join(shardDir, "data")
	if err := checkShardCompatible(t, src, dst); err != nil {
		return &dst, err
	}

	// Stop Redis writing to disk so shutdown cannot clobber the restored files.
	if _, err := x.RedisCLI(ctx, t, "CONFIG", "SET", "appendonly", "no"); err != nil {
		return &dst, fmt.Errorf("shard %d (%s): failed to disable AOF before restore: %w", t.Shard, t.Pod, err)
	}
	if _, err := x.RedisCLI(ctx, t, "CONFIG", "SET", "save", ""); err != nil {
		return &dst, fmt.Errorf("shard %d (%s): failed to disable RDB snapshots before restore: %w", t.Shard, t.Pod, err)
	}

	// Clear the existing dataset files. Arguments are passed as argv, never
	// interpolated into a shell string.
	stale := []string{filepath.Join(dst.Dir, dst.DBFilename)}
	if dst.AppendDirname != "" {
		stale = append(stale, filepath.Join(dst.Dir, dst.AppendDirname))
	}
	if _, err := x.Exec(ctx, t, append([]string{"rm", "-rf"}, stale...)...); err != nil {
		return &dst, fmt.Errorf("shard %d (%s): failed to clear existing data: %w", t.Shard, t.Pod, err)
	}

	if err := x.CopyTo(ctx, t, dataDir, dst.Dir); err != nil {
		return &dst, fmt.Errorf("shard %d (%s): failed to restore data files: %w", t.Shard, t.Pod, err)
	}
	// The archive carries files under the names the SOURCE server used. The
	// target loads whatever its own config names — dbfilename, appendfilename,
	// appenddirname — and those can differ (a config change between backup and
	// restore, a Redis 6 archive into a Redis 7 server). A file left under the
	// wrong name is simply never read and the server starts empty.
	if err := x.AlignNames(ctx, t, src, dst); err != nil {
		return &dst, fmt.Errorf("shard %d (%s): %w", t.Shard, t.Pod, err)
	}

	logger.Info("Shard files replaced", "pod", t.Pod, "dir", dst.Dir, "appendOnly", dst.AppendOnly)
	return &dst, nil
}

// checkShardCompatible decides whether a shard archive can be loaded by the
// target server as it is configured now.
//
// The rule is about what Redis reads on start. A target with appendonly=yes
// loads its append-only log and ignores dump.rdb, so the archive must carry
// an AOF. Redis 7 keeps that log as a directory and can also load the single
// appendonly.aof a Redis 6 wrote (it converts it on the way in); Redis 6 can
// only load the single file. A target with appendonly=no loads dump.rdb.
func checkShardCompatible(t backuputil.Target, src, dst backuputil.Layout) error {
	if !dst.AppendOnly {
		return nil // dump.rdb is always present in the archive
	}
	srcAOF := src.AOFEntry()
	if srcAOF == "" {
		return fmt.Errorf("shard %d (%s): target has appendonly enabled but the backup only contains an RDB; "+
			"Redis would ignore it and start empty. Re-take the backup with an operator build that captures "+
			"the append-only file", t.Shard, t.Pod)
	}
	srcIsDir := src.AppendDirname != ""
	dstIsDir := dst.AppendDirname != ""
	if srcIsDir && !dstIsDir {
		return fmt.Errorf("shard %d (%s): the backup holds a Redis 7 multi-part AOF (%s) but the target is a "+
			"Redis 6 server that can only load a single %s; it cannot be restored here",
			t.Shard, t.Pod, src.AppendDirname, dst.AppendFilename)
	}
	return nil
}

func readManifest(dir string) (backuputil.Manifest, error) {
	var m backuputil.Manifest
	// #nosec G304 -- dir is a temp directory this process created.
	blob, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return m, fmt.Errorf("backup has no manifest.json; it was produced by an older, " +
				"incompatible operator build and cannot be restored")
		}
		return m, fmt.Errorf("failed to read manifest.json: %w", err)
	}
	if err := json.Unmarshal(blob, &m); err != nil {
		return m, fmt.Errorf("failed to parse manifest.json: %w", err)
	}
	if m.Version != backuputil.ManifestVersion {
		return m, fmt.Errorf("backup manifest version %d is not supported by this operator (expected %d)",
			m.Version, backuputil.ManifestVersion)
	}
	if len(m.PerShard) != m.Shards {
		return m, fmt.Errorf("manifest declares %d shards but describes %d", m.Shards, len(m.PerShard))
	}
	return m, nil
}

func (r *Reconciler) scale(ctx context.Context, namespace, name string, replicas int32) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		scale, err := r.K8sClient.AppsV1().StatefulSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if scale.Spec.Replicas == replicas {
			return nil
		}
		scale.Spec.Replicas = replicas
		_, err = r.K8sClient.AppsV1().StatefulSets(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		return err
	})
}

func (r *Reconciler) waitScaledDown(ctx context.Context, namespace, name string) error {
	deadline := time.Now().Add(scaleDownTimeout)
	for {
		sts, err := r.K8sClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to check statefulset %q: %w", name, err)
		}
		if sts.Spec.Replicas != nil && *sts.Spec.Replicas != 0 {
			return fmt.Errorf("statefulset %q was scaled back to %d while waiting for it to stop; "+
				"the owning controller is still reconciling it", name, *sts.Spec.Replicas)
		}
		if sts.Status.Replicas == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for statefulset %q to scale down", scaleDownTimeout, name)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (r *Reconciler) waitPodReady(ctx context.Context, x *backuputil.Executor, t backuputil.Target) error {
	deadline := time.Now().Add(readyTimeout)
	for {
		pod, err := r.K8sClient.CoreV1().Pods(t.Namespace).Get(ctx, t.Pod, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get pod %q: %w", t.Pod, err)
		}
		if err == nil && pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodRunning {
			// The Redis container specifically. A sidecar with no readiness
			// probe (the exporter, for one) is Ready the instant it starts,
			// which says nothing about whether Redis is up.
			ready := false
			for _, c := range pod.Status.ContainerStatuses {
				if c.Name != t.Container {
					continue
				}
				// A container Redis refuses to start in (an archive it cannot
				// load, say) sits in CrashLoopBackOff for the whole timeout.
				// Surface the real reason immediately instead.
				if c.State.Waiting != nil && c.State.Waiting.Reason == "CrashLoopBackOff" {
					reason := c.State.Waiting.Message
					if term := c.LastTerminationState.Terminated; term != nil {
						reason = fmt.Sprintf("exit %d: %s", term.ExitCode, strings.TrimSpace(term.Message))
					}
					return fmt.Errorf("container %q in pod %q is crash-looping (%s)", t.Container, t.Pod, reason)
				}
				ready = c.Ready
				break
			}
			if ready {
				if pong, pingErr := x.RedisCLI(ctx, t, "PING"); pingErr == nil && pong == "PONG" {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			// Reporting success here would mark the restore Completed while
			// Redis is down, which is the outage the user needs to see.
			return fmt.Errorf("timed out after %s waiting for pod %q container %q to become ready and answer PING",
				readyTimeout, t.Pod, t.Container)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// setSkipReconcile toggles the owning controller's skip-reconcile annotation.
func (r *Reconciler) setSkipReconcile(ctx context.Context, kind backuputil.TargetKind, namespace, name, owner string, on bool) (changed bool, heldBy string, err error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}
	annotation, ok := backuputil.SkipReconcileAnnotation(kind)
	if !ok {
		return false, "", fmt.Errorf("no skip-reconcile annotation is defined for kind %q", kind)
	}

	var obj client.Object
	switch kind {
	case backuputil.KindRedis:
		obj = &rvb2.Redis{}
	case backuputil.KindRedisReplication:
		obj = &rrvb2.RedisReplication{}
	case backuputil.KindRedisCluster:
		obj = &rcvb2.RedisCluster{}
	case backuputil.KindRedisSentinel:
		obj = &rsvb2.RedisSentinel{}
	default:
		return false, "", fmt.Errorf("unsupported kind %q", kind)
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		changed, heldBy = false, ""
		if err := r.Get(ctx, key, obj); err != nil {
			return err
		}
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		if on {
			if annotations[annotation] == "true" {
				// Someone holds it. Report who, so the caller can tell its own
				// interrupted attempt from a user pause or another restore.
				heldBy = annotations[backuputil.RestoreOwnerAnnotation]
				return nil
			}
			annotations[annotation] = "true"
			annotations[backuputil.RestoreOwnerAnnotation] = owner
		} else {
			if _, present := annotations[annotation]; !present {
				return nil
			}
			if held := annotations[backuputil.RestoreOwnerAnnotation]; held != "" && held != owner {
				// Not ours: a user pause set after we finished, or another
				// restore's lock. Leave it alone.
				heldBy = held
				return nil
			}
			delete(annotations, annotation)
			delete(annotations, backuputil.RestoreOwnerAnnotation)
		}
		obj.SetAnnotations(annotations)
		changed = true
		return r.Update(ctx, obj)
	})
	return changed, heldBy, err
}

// sentinelSuspension tracks what pauseSentinelControllers and scaleDown did,
// so resume can undo exactly that and nothing more.
type sentinelSuspension struct {
	namespace  string
	rr         *rrvb2.RedisReplication
	annotated  []string         // RedisSentinel CRs whose annotation we set
	stsToScale []string         // StatefulSets to take down once it is safe
	sizes      map[string]int32 // original replicas of those we did scale

	scaler     func(ctx context.Context, sts string, replicas int32) error
	unannotate func(ctx context.Context, name string) error
}

// pauseSentinelControllers annotates every RedisSentinel that watches the
// replication. It runs BEFORE the RedisReplication is annotated, because that
// write emits an event the sentinel controller watches and would act on.
//
// It deliberately does not scale anything yet: the embedded sentinel
// StatefulSet is owned by the replication controller, which is still active
// at this point and re-applies the StatefulSet's replica count on every pass.
func (r *Reconciler) pauseSentinelControllers(ctx context.Context, namespace, replicationName, owner string) (*sentinelSuspension, error) {
	logger := log.FromContext(ctx)
	sus := &sentinelSuspension{namespace: namespace, sizes: map[string]int32{}}
	sus.scaler = func(c context.Context, sts string, n int32) error { return r.scale(c, namespace, sts, n) }
	sus.unannotate = func(c context.Context, name string) error {
		_, _, err := r.setSkipReconcile(c, backuputil.KindRedisSentinel, namespace, name, owner, false)
		return err
	}

	rr := &rrvb2.RedisReplication{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: replicationName}, rr); err != nil {
		return nil, fmt.Errorf("failed to get RedisReplication %q: %w", replicationName, err)
	}
	sus.rr = rr
	if rr.EnableSentinel() {
		sus.stsToScale = append(sus.stsToScale, rr.SentinelStatefulSet())
	}

	list := &rsvb2.RedisSentinelList{}
	if err := r.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list RedisSentinels: %w", err)
	}
	for i := range list.Items {
		sent := &list.Items[i]
		if sent.Spec.RedisSentinelConfig == nil || sent.Spec.RedisSentinelConfig.RedisReplicationName != replicationName {
			continue
		}
		changed, heldBy, err := r.setSkipReconcile(ctx, backuputil.KindRedisSentinel, namespace, sent.Name, owner, true)
		if err != nil {
			_ = sus.resume(ctx)
			return nil, fmt.Errorf("failed to suspend sentinel %q: %w", sent.Name, err)
		}
		if !changed && heldBy != owner {
			// Paused by a user or another restore; not ours to resume.
			_ = sus.resume(ctx)
			return nil, fmt.Errorf("RedisSentinel %q already carries its skip-reconcile annotation; refusing to restore a paused target", sent.Name)
		}
		sus.annotated = append(sus.annotated, sent.Name)
		sus.stsToScale = append(sus.stsToScale, sent.Name+"-sentinel")
		logger.Info("Suspended sentinel controller for the restore", "sentinel", sent.Name)
	}
	return sus, nil
}

// scaleDown takes every sentinel daemon out for the duration of the restore
// and waits until they are actually gone. Sentinel pods are stateless here,
// so scaling them away and back yields fresh daemons that the (resumed)
// sentinel controller points at whichever pod really is the master — which,
// after rebuildReplication, is the restored one.
func (sus *sentinelSuspension) scaleDown(ctx context.Context, r *Reconciler) error {
	logger := log.FromContext(ctx)
	for _, stsName := range sus.stsToScale {
		sts, err := r.K8sClient.AppsV1().StatefulSets(sus.namespace).Get(ctx, stsName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// A RedisSentinel whose StatefulSet has not been created yet
				// has no daemons to stop and nothing to put back later.
				continue
			}
			return fmt.Errorf("failed to get sentinel statefulset %q: %w", stsName, err)
		}
		size := int32(0)
		if sts.Spec.Replicas != nil {
			size = *sts.Spec.Replicas
		}
		sus.sizes[stsName] = size
		if size == 0 {
			continue
		}
		if err := r.scale(ctx, sus.namespace, stsName, 0); err != nil {
			return fmt.Errorf("failed to scale down sentinel %q: %w", stsName, err)
		}
		logger.Info("Sentinel daemons scaled down for the restore", "statefulset", stsName, "replicas", size)
	}
	// Requesting the scale-down is not the same as the daemons being gone.
	for stsName, size := range sus.sizes {
		if size == 0 {
			continue
		}
		if err := r.waitScaledDown(ctx, sus.namespace, stsName); err != nil {
			return err
		}
	}
	return nil
}

// resume puts back exactly what was changed: replica counts we recorded, and
// annotations we set. It is safe to call after a partial pause.
func (sus *sentinelSuspension) resume(ctx context.Context) error {
	if sus == nil {
		return nil
	}
	var firstErr error
	// r is reachable through the closure that created us; keep the method
	// self-contained by re-deriving nothing and using the recorded state only.
	for stsName, size := range sus.sizes {
		if size == 0 {
			continue
		}
		if err := sus.scaler(ctx, stsName, size); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to scale sentinel %q back to %d: %w", stsName, size, err)
		}
	}
	for _, name := range sus.annotated {
		if err := sus.unannotate(ctx, name); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to clear skip-reconcile on sentinel %q: %w", name, err)
		}
	}
	return firstErr
}

// setRoleLabels marks the restored master and its replicas the way the
// replication controller would, so the <name>-master Service resolves.
func (r *Reconciler) setRoleLabels(ctx context.Context, namespace string, master backuputil.Target, replicas []backuputil.Target) error {
	set := func(pod, role string) error {
		patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, common.RedisRoleLabelKey, role))
		_, err := r.K8sClient.CoreV1().Pods(namespace).Patch(ctx, pod, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to label %s as %s: %w", pod, role, err)
		}
		return nil
	}
	if err := set(master.Pod, common.RedisRoleLabelMaster); err != nil {
		return err
	}
	for _, t := range replicas {
		if err := set(t.Pod, common.RedisRoleLabelSlave); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, instance *redisv1alpha1.RedisRestore, mutate func(*redisv1alpha1.RedisRestore)) error {
	key := client.ObjectKeyFromObject(instance)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cur := &redisv1alpha1.RedisRestore{}
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

func (r *Reconciler) setPhase(ctx context.Context, instance *redisv1alpha1.RedisRestore, phase redisv1alpha1.RestorePhase, msg string) error {
	if instance.Status.Phase == phase && instance.Status.Message == msg {
		return nil
	}
	return r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisRestore) {
		cur.Status.Phase = phase
		cur.Status.Message = msg
	})
}

func (r *Reconciler) markFailed(ctx context.Context, instance *redisv1alpha1.RedisRestore, reason string, terminal bool) error {
	return r.updateStatus(ctx, instance, func(cur *redisv1alpha1.RedisRestore) {
		cur.Status.Phase = redisv1alpha1.RestorePhaseFailed
		cur.Status.Message = reason
		if terminal {
			cur.Status.ObservedGeneration = cur.Generation
		}
		meta.SetStatusCondition(&cur.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cur.Generation,
			Reason:             "RestoreFailed",
			Message:            reason,
		})
	})
}

// SetupWithManager registers this controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1alpha1.RedisRestore{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(opts).
		Complete(r)
}
