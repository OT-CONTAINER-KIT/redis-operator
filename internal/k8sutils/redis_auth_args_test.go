package k8sutils

import (
	"context"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

const (
	authTestNamespace  = "default"
	authTestClusterCR  = "test-cluster"
	authTestLeaderPod  = "test-cluster-leader-0"
	authTestSecretName = "redis-secret"
	authTestSecretKey  = "password"
	authTestPassword   = "s3cr3t"
)

// newAuthTestCluster builds a minimal RedisCluster referencing an existing
// password secret, which is the only case in which authentication args matter.
func newAuthTestCluster(withSecret bool) *rcvb2.RedisCluster {
	cr := &rcvb2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      authTestClusterCR,
			Namespace: authTestNamespace,
		},
	}
	if withSecret {
		cr.Spec.KubernetesConfig.ExistingPasswordSecret = &common.ExistingPasswordSecret{
			Name: ptr.To(authTestSecretName),
			Key:  ptr.To(authTestSecretKey),
		}
	}
	return cr
}

// leaderPod returns a leader pod whose redis container optionally carries the
// REDISCLI_AUTH env var sourced from the given secret name/key.
func leaderPod(withRedisCLIAuth bool, secretName, secretKey string) *corev1.Pod {
	container := corev1.Container{Name: authTestClusterCR + "-leader"}
	if withRedisCLIAuth {
		container.Env = []corev1.EnvVar{
			{
				Name: "REDISCLI_AUTH",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  secretKey,
					},
				},
			},
		}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: authTestLeaderPod, Namespace: authTestNamespace},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{container}},
	}
}

func authTestSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authTestSecretName, Namespace: authTestNamespace},
		Data:       map[string][]byte{authTestSecretKey: []byte(authTestPassword)},
	}
}

// setAvoidCommandLinePassword toggles the feature gate for the duration of a
// test and restores its previous value afterwards.
func setAvoidCommandLinePassword(t *testing.T, enabled bool) {
	t.Helper()
	previous := features.Enabled(features.AvoidCommandLinePassword)
	require.NoError(t, features.MutableFeatureGate.SetFromMap(map[string]bool{
		string(features.AvoidCommandLinePassword): enabled,
	}))
	t.Cleanup(func() {
		require.NoError(t, features.MutableFeatureGate.SetFromMap(map[string]bool{
			string(features.AvoidCommandLinePassword): previous,
		}))
	})
}

func TestGetRedisClusterAuthArgs(t *testing.T) {
	t.Run("no existing password secret returns no args", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		args, err := getRedisClusterAuthArgs(context.Background(), client, newAuthTestCluster(false), authTestLeaderPod)
		require.NoError(t, err)
		assert.Empty(t, args)
	})

	t.Run("REDISCLI_AUTH present and matching returns no -a", func(t *testing.T) {
		client := fake.NewSimpleClientset(authTestSecret(), leaderPod(true, authTestSecretName, authTestSecretKey))
		args, err := getRedisClusterAuthArgs(context.Background(), client, newAuthTestCluster(true), authTestLeaderPod)
		require.NoError(t, err)
		assert.Empty(t, args)
	})

	t.Run("REDISCLI_AUTH absent with gate off returns -a <password>", func(t *testing.T) {
		setAvoidCommandLinePassword(t, false)
		client := fake.NewSimpleClientset(authTestSecret(), leaderPod(false, "", ""))
		args, err := getRedisClusterAuthArgs(context.Background(), client, newAuthTestCluster(true), authTestLeaderPod)
		require.NoError(t, err)
		assert.Equal(t, []string{"-a", authTestPassword}, args)
	})

	t.Run("REDISCLI_AUTH absent with gate on returns error", func(t *testing.T) {
		setAvoidCommandLinePassword(t, true)
		client := fake.NewSimpleClientset(authTestSecret(), leaderPod(false, "", ""))
		args, err := getRedisClusterAuthArgs(context.Background(), client, newAuthTestCluster(true), authTestLeaderPod)
		require.Error(t, err)
		assert.Empty(t, args)
	})
}

func TestCheckRedisCLIAuthInEnv(t *testing.T) {
	t.Run("matching REDISCLI_AUTH", func(t *testing.T) {
		client := fake.NewSimpleClientset(leaderPod(true, authTestSecretName, authTestSecretKey))
		ok, err := checkRedisCLIAuthInEnv(context.Background(), client, newAuthTestCluster(true), authTestLeaderPod, authTestSecretName, authTestSecretKey)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("REDISCLI_AUTH sourced from a different secret", func(t *testing.T) {
		client := fake.NewSimpleClientset(leaderPod(true, "other-secret", authTestSecretKey))
		ok, err := checkRedisCLIAuthInEnv(context.Background(), client, newAuthTestCluster(true), authTestLeaderPod, authTestSecretName, authTestSecretKey)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("leader container without REDISCLI_AUTH", func(t *testing.T) {
		client := fake.NewSimpleClientset(leaderPod(false, "", ""))
		ok, err := checkRedisCLIAuthInEnv(context.Background(), client, newAuthTestCluster(true), authTestLeaderPod, authTestSecretName, authTestSecretKey)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("pod does not exist returns error", func(t *testing.T) {
		client := fake.NewSimpleClientset()
		ok, err := checkRedisCLIAuthInEnv(context.Background(), client, newAuthTestCluster(true), authTestLeaderPod, authTestSecretName, authTestSecretKey)
		require.Error(t, err)
		assert.False(t, ok)
	})
}
