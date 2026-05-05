package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1alpha1"
)

var _ = Describe("RedisBackup Controller", func() {

	const (
		backupNamespace = "default"
		timeout         = time.Second * 20
		interval        = time.Millisecond * 250
	)

	Context("When a valid RedisBackup is created", func() {
		It("Should reach the Completed phase and record the backup location", func() {
			ctx := context.Background()

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-valid-backup",
					Namespace: backupNamespace,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeS3,
					S3: &redisv1alpha1.S3StorageConfig{
						Bucket:     "test-bucket",
						Region:     "ap-south-1",
						SecretName: "test-aws-secret",
					},
					RetentionDays: 7,
				},
			}

			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-valid-backup", Namespace: backupNamespace}
			result := &redisv1alpha1.RedisBackup{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseCompleted
			}, timeout, interval).Should(BeTrue(), "expected backup to reach Completed phase")

			Expect(result.Status.BackupLocation).To(ContainSubstring("s3://test-bucket/backups/test-redis-cluster"))
			Expect(result.Status.LastBackupTime).NotTo(BeNil())
			Expect(result.Status.Message).To(Equal("Backup completed successfully"))
		})
	})

	Context("When a RedisBackup is missing S3 config", func() {
		It("Should reach the Failed phase with a clear validation error", func() {
			ctx := context.Background()

			invalidBackup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-missing-s3-backup",
					Namespace: backupNamespace,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeS3,
					// S3 block intentionally omitted
				},
			}

			Expect(k8sClient.Create(ctx, invalidBackup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-missing-s3-backup", Namespace: backupNamespace}
			result := &redisv1alpha1.RedisBackup{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseFailed
			}, timeout, interval).Should(BeTrue(), "expected backup to reach Failed phase")

			Expect(result.Status.Message).To(ContainSubstring("spec.s3 is required"))
		})
	})

	Context("When a RedisBackup has an empty cluster name", func() {
		It("Should be rejected by the API server due to validation", func() {
			ctx := context.Background()

			// MinLength=1 on redisClusterName means the API server rejects
			// this at the CRD validation layer — the controller never sees it
			emptyClusterBackup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-empty-cluster-backup",
					Namespace: backupNamespace,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "",
					StorageType:      redisv1alpha1.StorageTypeS3,
					S3: &redisv1alpha1.S3StorageConfig{
						Bucket:     "test-bucket",
						Region:     "ap-south-1",
						SecretName: "test-secret",
					},
				},
			}

			// Expect Create to fail at the API level — not reach the controller
			err := k8sClient.Create(ctx, emptyClusterBackup)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("redisClusterName"))
		})
	})
})
