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

	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("RedisBackup Controller", func() {
	Context("When a valid RedisBackup is created with an existing Secret", func() {
		It("Should reach the Completed phase and record the backup location", func() {
			ctx := context.Background()

			// Create the Secret that the backup references
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-aws-secret-valid",
					Namespace: ns,
				},
				Data: map[string][]byte{
					"AWS_ACCESS_KEY_ID":     []byte("test-key-id"),
					"AWS_SECRET_ACCESS_KEY": []byte("test-secret-key"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-valid-backup",
					Namespace: ns,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeS3,
					S3: &redisv1alpha1.S3StorageConfig{
						Bucket:     "test-bucket",
						Region:     "ap-south-1",
						SecretName: "test-aws-secret-valid",
					},
					RetentionDays: 7,
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-valid-backup", Namespace: ns}
			result := &redisv1alpha1.RedisBackup{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseCompleted
			}, timeout, interval).Should(BeTrue(), "expected backup to reach Completed phase")

			Expect(result.Status.BackupLocation).To(ContainSubstring("s3://test-bucket/backups/test-redis-cluster"))
			Expect(result.Status.LastBackupTime).NotTo(BeNil())
			Expect(result.Status.Message).To(ContainSubstring("alpha"))
		})
	})

	Context("When a RedisBackup is missing S3 config", func() {
		It("Should reach the Failed phase with a clear validation error", func() {
			ctx := context.Background()

			invalidBackup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-missing-s3-backup",
					Namespace: ns,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeS3,
					// S3 block intentionally omitted
				},
			}
			Expect(k8sClient.Create(ctx, invalidBackup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-missing-s3-backup", Namespace: ns}
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

	Context("When a RedisBackup references a non-existent Secret", func() {
		It("Should reach the Failed phase with a secret not found error", func() {
			ctx := context.Background()

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-missing-secret-backup",
					Namespace: ns,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeS3,
					S3: &redisv1alpha1.S3StorageConfig{
						Bucket:     "test-bucket",
						Region:     "ap-south-1",
						SecretName: "nonexistent-secret",
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-missing-secret-backup", Namespace: ns}
			result := &redisv1alpha1.RedisBackup{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseFailed
			}, timeout, interval).Should(BeTrue(), "expected backup to reach Failed phase")

			Expect(result.Status.Message).To(ContainSubstring("not found"))
		})
	})

	Context("When a RedisBackup uses an unsupported storage type", func() {
		It("Should reach the Failed phase", func() {
			ctx := context.Background()

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-unsupported-storage",
					Namespace: ns,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeGCS,
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-unsupported-storage", Namespace: ns}
			result := &redisv1alpha1.RedisBackup{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseFailed
			}, timeout, interval).Should(BeTrue(), "expected backup to reach Failed phase")

			Expect(result.Status.Message).To(ContainSubstring("unsupported storageType"))
		})
	})

	Context("When a RedisBackup has an empty cluster name", func() {
		It("Should be rejected by the API server due to CRD validation", func() {
			ctx := context.Background()

			// MinLength=1 on redisClusterName means the API server rejects
			// this at the CRD validation layer — the controller never sees it
			emptyClusterBackup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-empty-cluster-backup",
					Namespace: ns,
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

	Context("When a completed RedisBackup is reconciled again", func() {
		It("Should remain completed without changes (idempotent)", func() {
			ctx := context.Background()

			// Create the Secret for this test
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-aws-secret-idempotent",
					Namespace: ns,
				},
				Data: map[string][]byte{
					"AWS_ACCESS_KEY_ID":     []byte("test-key-id"),
					"AWS_SECRET_ACCESS_KEY": []byte("test-secret-key"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-idempotent-backup",
					Namespace: ns,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeS3,
					S3: &redisv1alpha1.S3StorageConfig{
						Bucket:     "test-bucket",
						Region:     "ap-south-1",
						SecretName: "test-aws-secret-idempotent",
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-idempotent-backup", Namespace: ns}
			result := &redisv1alpha1.RedisBackup{}

			// Wait for completion
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseCompleted
			}, timeout, interval).Should(BeTrue(), "expected backup to reach Completed phase")

			// Record the backup location
			originalLocation := result.Status.BackupLocation

			// Verify it stays completed with the same location (idempotent)
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseCompleted &&
					result.Status.BackupLocation == originalLocation
			}, timeout/2, interval).Should(BeTrue(), "backup should remain completed with same location")
		})
	})
})
