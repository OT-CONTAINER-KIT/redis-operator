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

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	redisv1alpha1 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisbackup/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ensureRedisTarget creates the Redis CR and the StatefulSet the controller
// resolves the backup target from. envtest runs no other controllers, so the
// StatefulSet the Redis controller would normally create is made here.
func ensureRedisTarget(ctx context.Context, name string) {
	redis := &rvb2.Redis{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: rvb2.RedisSpec{
			KubernetesConfig: commonapi.KubernetesConfig{
				Image: "quay.io/opstree/redis:v7.0.12",
			},
		},
	}
	if err := k8sClient.Create(ctx, redis); err != nil {
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue(), "unexpected error creating Redis: %v", err)
	}

	replicas := int32(1)
	labels := map[string]string{"app": name}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: name,
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  name,
						Image: "quay.io/opstree/redis:v7.0.12",
					}},
				},
			},
		},
	}
	if err := k8sClient.Create(ctx, sts); err != nil {
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue(), "unexpected error creating StatefulSet: %v", err)
	}
}

var _ = Describe("RedisBackup Controller", func() {
	Context("When a valid RedisBackup is created with an existing Secret", func() {
		It("Should reach the Completed phase and record the backup location", func() {
			ctx := context.Background()
			ensureRedisTarget(ctx, "test-redis-cluster")

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
			Expect(result.Status.Message).To(ContainSubstring("Backup completed successfully"))
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
		// spec.storageType carries an Enum marker, so the API server rejects an
		// unsupported value at admission and the controller never sees the object.
		It("Should be rejected by the API server due to CRD validation", func() {
			ctx := context.Background()

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-unsupported-storage",
					Namespace: ns,
				},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageType("ftp"),
				},
			}
			err := k8sClient.Create(ctx, backup)
			Expect(err).To(HaveOccurred(), "expected the API server to reject an unsupported storageType")
			Expect(err.Error()).To(ContainSubstring("spec.storageType"))
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
			ensureRedisTarget(ctx, "test-redis-cluster")

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

	Context("When a RedisBackup sets spec.schedule", func() {
		// schedule was previously accepted and silently ignored, which made a
		// one-shot backup look like a recurring one.
		It("Should fail with an explicit not-implemented error", func() {
			ctx := context.Background()
			ensureRedisTarget(ctx, "test-redis-cluster")

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-aws-secret-sched", Namespace: ns},
				Data: map[string][]byte{
					"AWS_ACCESS_KEY_ID":     []byte("k"),
					"AWS_SECRET_ACCESS_KEY": []byte("s"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-scheduled-backup", Namespace: ns},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "test-redis-cluster",
					StorageType:      redisv1alpha1.StorageTypeS3,
					Schedule:         "0 2 * * *",
					S3: &redisv1alpha1.S3StorageConfig{
						Bucket: "test-bucket", Region: "ap-south-1", SecretName: "test-aws-secret-sched",
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-scheduled-backup", Namespace: ns}
			result := &redisv1alpha1.RedisBackup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseFailed
			}, timeout, interval).Should(BeTrue(), "expected schedule to be rejected")

			Expect(result.Status.Message).To(ContainSubstring("spec.schedule is not implemented"))
		})
	})

	Context("When a RedisBackup targets a resource that does not exist", func() {
		// Previously this surfaced as `pods "<name>-0" not found` from the
		// first exec, after the controller had already started work.
		It("Should fail during validation with an actionable message", func() {
			ctx := context.Background()

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-aws-secret-notarget", Namespace: ns},
				Data: map[string][]byte{
					"AWS_ACCESS_KEY_ID":     []byte("k"),
					"AWS_SECRET_ACCESS_KEY": []byte("s"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			backup := &redisv1alpha1.RedisBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-no-target-backup", Namespace: ns},
				Spec: redisv1alpha1.RedisBackupSpec{
					RedisClusterName: "does-not-exist",
					StorageType:      redisv1alpha1.StorageTypeS3,
					S3: &redisv1alpha1.S3StorageConfig{
						Bucket: "test-bucket", Region: "ap-south-1", SecretName: "test-aws-secret-notarget",
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			key := types.NamespacedName{Name: "test-no-target-backup", Namespace: ns}
			result := &redisv1alpha1.RedisBackup{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == redisv1alpha1.BackupPhaseFailed
			}, timeout, interval).Should(BeTrue(), "expected an unresolvable target to fail validation")

			Expect(result.Status.Message).To(ContainSubstring("no Redis, RedisReplication or RedisCluster named"))
		})
	})
})
