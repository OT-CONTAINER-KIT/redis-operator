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
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	RedisRestoreFinalizer = "redisrestore.redis.redis.opstreelabs.in/finalizer"
)

// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redisrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
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
	logger.Info("Reconciling RedisRestore", "name", req.Name, "namespace", req.Namespace)

	// Fetch the RedisRestore resource
	instance := &redisv1alpha1.RedisRestore{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get RedisRestore instance")
	}

	// Handle deletion
	if instance.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(instance, RedisRestoreFinalizer) {
			controllerutil.RemoveFinalizer(instance, RedisRestoreFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return intctrlutil.RequeueE(ctx, err, "failed to remove finalizer")
			}
		}
		return intctrlutil.Reconciled()
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(instance, RedisRestoreFinalizer) {
		controllerutil.AddFinalizer(instance, RedisRestoreFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to add finalizer")
		}
		return intctrlutil.Requeue()
	}

	// Skip if already completed
	if instance.Status.Phase == redisv1alpha1.RestorePhaseCompleted {
		return intctrlutil.Reconciled()
	}

	// Validate
	if err := r.validateSpec(ctx, instance); err != nil {
		logger.Error(err, "RedisRestore spec validation failed")
		if statusErr := r.setFailedStatus(ctx, instance, err.Error()); statusErr != nil {
			return intctrlutil.RequeueE(ctx, statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Set Running phase
	if instance.Status.Phase != redisv1alpha1.RestorePhaseRunning {
		if err := r.setPhase(ctx, instance, redisv1alpha1.RestorePhaseRunning, "Restore is in progress"); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to set Running phase")
		}
	}

	// Perform restore
	if err := r.performRestore(ctx, instance); err != nil {
		logger.Error(err, "Restore failed")
		return ctrl.Result{}, r.setFailedStatus(ctx, instance, fmt.Sprintf("Restore failed: %v", err))
	}

	// Mark completed
	now := metav1.NewTime(time.Now().UTC())
	instance.Status.Phase = redisv1alpha1.RestorePhaseCompleted
	instance.Status.Message = "Restore completed successfully"
	instance.Status.RestoreCompletedTime = &now

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: instance.Generation,
		Reason:             "RestoreSucceeded",
		Message:            "Restore completed successfully",
	})

	if err := r.Status().Update(ctx, instance); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to update status to Completed")
	}

	logger.Info("RedisRestore completed successfully")
	return intctrlutil.Reconciled()
}

func (r *Reconciler) validateSpec(ctx context.Context, instance *redisv1alpha1.RedisRestore) error {
	if instance.Spec.RedisClusterName == "" {
		return fmt.Errorf("spec.redisClusterName must not be empty")
	}
	if instance.Spec.BackupLocation == "" {
		return fmt.Errorf("spec.backupLocation must not be empty")
	}

	switch instance.Spec.StorageType {
	case redisv1alpha1.StorageTypeS3:
		if instance.Spec.S3 == nil {
			return fmt.Errorf("spec.s3 is required when storageType is 's3'")
		}
		if instance.Spec.S3.SecretName == "" {
			return fmt.Errorf("spec.s3.secretName must not be empty")
		}
		// Validate secret exists
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.S3.SecretName, Namespace: instance.Namespace}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("secret %q not found", instance.Spec.S3.SecretName)
			}
			return fmt.Errorf("failed to look up secret: %w", err)
		}
	default:
		return fmt.Errorf("unsupported storageType %q", instance.Spec.StorageType)
	}
	return nil
}

func (r *Reconciler) performRestore(ctx context.Context, instance *redisv1alpha1.RedisRestore) error {
	logger := log.FromContext(ctx)
	cfg := instance.Spec.S3
	clusterName := instance.Spec.RedisClusterName

	// Step 1: Download backup files from S3
	tmpDir, err := os.MkdirTemp("", "redis-restore-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	logger.Info("Downloading backup from S3", "location", instance.Spec.BackupLocation)
	if err := r.downloadFromS3(ctx, instance, tmpDir); err != nil {
		return fmt.Errorf("failed to download from S3: %w", err)
	}

	// Step 2: Scale down StatefulSet
	logger.Info("Scaling down Redis StatefulSet", "name", clusterName)
	scale, err := r.K8sClient.AppsV1().StatefulSets(instance.Namespace).GetScale(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet scale: %w", err)
	}
	originalReplicas := scale.Spec.Replicas
	scale.Spec.Replicas = 0
	_, err = r.K8sClient.AppsV1().StatefulSets(instance.Namespace).UpdateScale(ctx, clusterName, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale down: %w", err)
	}

	// Wait for scale down
	logger.Info("Waiting for StatefulSet to scale down")
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		ss, err := r.K8sClient.AppsV1().StatefulSets(instance.Namespace).Get(ctx, clusterName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to check StatefulSet: %w", err)
		}
		if ss.Status.Replicas == 0 {
			break
		}
		if i == 29 {
			return fmt.Errorf("timed out waiting for StatefulSet to scale down")
		}
	}

	// Step 3: Scale up to 1 replica to get the PVC attached
	logger.Info("Scaling up Redis to 1 replica for restore")
	scale, err = r.K8sClient.AppsV1().StatefulSets(instance.Namespace).GetScale(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet scale: %w", err)
	}
	scale.Spec.Replicas = 1
	_, err = r.K8sClient.AppsV1().StatefulSets(instance.Namespace).UpdateScale(ctx, clusterName, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale up: %w", err)
	}

	// Wait for pod ready
	podName := fmt.Sprintf("%s-0", clusterName)
	logger.Info("Waiting for pod to be ready", "pod", podName)
	for i := 0; i < 60; i++ {
		time.Sleep(2 * time.Second)
		pod, err := r.K8sClient.CoreV1().Pods(instance.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		ready := false
		for _, c := range pod.Status.ContainerStatuses {
			if c.Ready {
				ready = true
				break
			}
		}
		if ready {
			break
		}
		if i == 59 {
			return fmt.Errorf("timed out waiting for pod %s to be ready", podName)
		}
	}

	// Step 4: Stop Redis in the pod, copy files, restart
	logger.Info("Stopping Redis for restore")
	backupController := &backupReconcilerHelper{K8sClient: r.K8sClient, RESTConfig: r.RESTConfig}
	_, _ = backupController.execInPod(ctx, instance.Namespace, podName, clusterName, []string{"redis-cli", "SHUTDOWN", "NOSAVE"})
	time.Sleep(2 * time.Second)

	// Copy dump.rdb
	dumpPath := filepath.Join(tmpDir, "dump.rdb")
	if _, err := os.Stat(dumpPath); err == nil {
		logger.Info("Copying dump.rdb to pod")
		if err := backupController.copyFileToPod(ctx, instance.Namespace, podName, clusterName, dumpPath, "/data/dump.rdb"); err != nil {
			return fmt.Errorf("failed to copy dump.rdb: %w", err)
		}
	}

	// Copy node.conf and patch IPs for cross-cluster restore
	nodeConfPath := filepath.Join(tmpDir, "node.conf")
	if _, err := os.Stat(nodeConfPath); err == nil {
		logger.Info("Copying node.conf to pod (cluster mode)")
		if err := backupController.copyFileToPod(ctx, instance.Namespace, podName, clusterName, nodeConfPath, "/data/node.conf"); err != nil {
			logger.Info("Failed to copy node.conf, continuing", "error", err.Error())
		} else {
			// Dynamically patch node.conf to replace stale IPs with current pod IP
			logger.Info("Patching node.conf with current pod IP")
			if err := r.patchNodeConf(ctx, instance.Namespace, podName, clusterName, backupController); err != nil {
				logger.Info("Failed to patch node.conf IPs (non-fatal for same-topology restore)", "error", err.Error())
			}
		}
	}

	// Step 5: Scale back to original replicas
	logger.Info("Scaling Redis back to original replicas", "replicas", originalReplicas)
	scale, err = r.K8sClient.AppsV1().StatefulSets(instance.Namespace).GetScale(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet scale: %w", err)
	}
	scale.Spec.Replicas = originalReplicas
	_, err = r.K8sClient.AppsV1().StatefulSets(instance.Namespace).UpdateScale(ctx, clusterName, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to restore replicas: %w", err)
	}

	// Wait for all pods ready
	for i := 0; i < 60; i++ {
		time.Sleep(2 * time.Second)
		ss, err := r.K8sClient.AppsV1().StatefulSets(instance.Namespace).Get(ctx, clusterName, metav1.GetOptions{})
		if err == nil && ss.Status.ReadyReplicas == originalReplicas {
			break
		}
		if i == 59 {
			logger.Info("Warning: timed out waiting for all replicas to be ready")
		}
	}

	_ = cfg // referenced via instance.Spec.S3
	logger.Info("Restore completed successfully")
	return nil
}

func (r *Reconciler) downloadFromS3(ctx context.Context, instance *redisv1alpha1.RedisRestore, destDir string) error {
	cfg := instance.Spec.S3

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: cfg.SecretName, Namespace: instance.Namespace}, secret); err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			string(secret.Data["AWS_ACCESS_KEY_ID"]),
			string(secret.Data["AWS_SECRET_ACCESS_KEY"]),
			"",
		)),
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

	// Parse the backup location to get bucket and prefix
	// Expected format: s3://bucket/prefix
	location := instance.Spec.BackupLocation
	if len(location) > 5 && location[:5] == "s3://" {
		location = location[5:]
	}
	// Split into bucket/prefix
	var bucket, prefix string
	for i, c := range location {
		if c == '/' {
			bucket = location[:i]
			prefix = location[i+1:]
			break
		}
	}
	if bucket == "" {
		bucket = cfg.Bucket
		prefix = location
	}

	// Download dump.rdb
	for _, fileName := range []string{"dump.rdb", "node.conf"} {
		key := fmt.Sprintf("%s/%s", prefix, fileName)
		resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			if fileName == "node.conf" {
				// node.conf is optional
				continue
			}
			return fmt.Errorf("failed to download %s: %w", fileName, err)
		}
		defer resp.Body.Close()

		outFile, err := os.Create(filepath.Join(destDir, fileName))
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", fileName, err)
		}
		if _, err := io.Copy(outFile, resp.Body); err != nil {
			outFile.Close()
			return fmt.Errorf("failed to write %s: %w", fileName, err)
		}
		outFile.Close()
	}

	return nil
}

// patchNodeConf reads node.conf from the pod, replaces all stale IP addresses
// with the pod's current IP, and writes it back. This enables cross-cluster
// restores where Pod IPs differ from the backup source.
//
// Redis node.conf format per line:
//
//	<node-id> <ip>:<port>@<bus-port> <flags> ...
//
// The function rewrites only the <ip> portion, preserving port and bus-port.
func (r *Reconciler) patchNodeConf(ctx context.Context, namespace, podName, containerName string, helper *backupReconcilerHelper) error {
	logger := log.FromContext(ctx)

	// Get the current pod IP
	pod, err := r.K8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s: %w", podName, err)
	}
	currentIP := pod.Status.PodIP
	if currentIP == "" {
		return fmt.Errorf("pod %s has no IP assigned yet", podName)
	}

	// Read node.conf from the pod
	nodeConfContent, err := helper.execInPod(ctx, namespace, podName, containerName, []string{"cat", "/data/node.conf"})
	if err != nil {
		return fmt.Errorf("failed to read node.conf: %w", err)
	}

	// Patch IP addresses: match <ip>:<port>@<bus-port> pattern
	// Replace the IP part with the current pod IP
	ipPortPattern := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+@\d+)`)

	var patched bool
	lines := strings.Split(nodeConfContent, "\n")
	for i, line := range lines {
		if ipPortPattern.MatchString(line) {
			newLine := ipPortPattern.ReplaceAllString(line, currentIP+"${2}")
			if newLine != line {
				lines[i] = newLine
				patched = true
			}
		}
	}

	if !patched {
		logger.Info("No IP addresses to patch in node.conf")
		return nil
	}

	// Write patched content back via exec
	patchedContent := strings.Join(lines, "\n")
	// Use printf to write the patched content (avoids echo -e issues)
	writeCmd := []string{"sh", "-c", fmt.Sprintf("cat > /data/node.conf << 'NODECONF_EOF'\n%s\nNODECONF_EOF", patchedContent)}
	if _, err := helper.execInPod(ctx, namespace, podName, containerName, writeCmd); err != nil {
		return fmt.Errorf("failed to write patched node.conf: %w", err)
	}

	logger.Info("Successfully patched node.conf with current pod IP", "ip", currentIP)
	return nil
}

// backupReconcilerHelper provides exec helpers shared with the backup controller.
type backupReconcilerHelper struct {
	K8sClient  kubernetes.Interface
	RESTConfig *rest.Config
}

func (h *backupReconcilerHelper) execInPod(ctx context.Context, namespace, podName, containerName string, command []string) (string, error) {
	req := h.K8sClient.CoreV1().RESTClient().Post().
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

	exec, err := remotecommand.NewSPDYExecutor(h.RESTConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return "", fmt.Errorf("exec failed: %w", err)
	}

	return stdout.String(), nil
}

func (h *backupReconcilerHelper) copyFileToPod(ctx context.Context, namespace, podName, containerName, srcPath, destPath string) error {
	// Read the local file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", srcPath, err)
	}

	// Create tar archive
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	hdr := &tar.Header{
		Name: filepath.Base(destPath),
		Mode: 0o644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	tw.Close()

	// Execute tar extract in pod
	req := h.K8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   []string{"tar", "xf", "-", "-C", filepath.Dir(destPath)},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
	}, k8sscheme.ParameterCodec)

	execStream, err := remotecommand.NewSPDYExecutor(h.RESTConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var stderr bytes.Buffer
	if err := execStream.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  &tarBuf,
		Stdout: io.Discard,
		Stderr: &stderr,
	}); err != nil {
		return fmt.Errorf("tar extract failed: %w", err)
	}

	return nil
}

func (r *Reconciler) setPhase(ctx context.Context, instance *redisv1alpha1.RedisRestore, phase redisv1alpha1.RestorePhase, msg string) error {
	instance.Status.Phase = phase
	instance.Status.Message = msg
	return r.Status().Update(ctx, instance)
}

func (r *Reconciler) setFailedStatus(ctx context.Context, instance *redisv1alpha1.RedisRestore, reason string) error {
	instance.Status.Phase = redisv1alpha1.RestorePhaseFailed
	instance.Status.Message = reason
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: instance.Generation,
		Reason:             "RestoreFailed",
		Message:            reason,
	})
	return r.Status().Update(ctx, instance)
}

// SetupWithManager registers this controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1alpha1.RedisRestore{}).
		WithOptions(opts).
		Complete(r)
}
