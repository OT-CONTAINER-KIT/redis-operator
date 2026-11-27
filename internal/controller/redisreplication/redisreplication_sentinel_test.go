package redisreplication

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

// TestSentinelExistingPasswordSecretResolvesToEmbeddedRedisSecret guards against the
// shadowing regression: the Sentinel struct must not declare its own outer
// ExistingPasswordSecret field, otherwise it shadows the embedded
// KubernetesConfig.ExistingPasswordSecret (json "redisSecret") and the
// controller silently drops the configured Sentinel password.
func TestSentinelExistingPasswordSecretResolvesToEmbeddedRedisSecret(t *testing.T) {
	secret := &commonapi.ExistingPasswordSecret{
		Name: ptr.To("sentinel-secret"),
		Key:  ptr.To("password"),
	}
	// Set only the embedded KubernetesConfig.ExistingPasswordSecret (json
	// "redisSecret") via a composite literal...
	sentinel := &rrvb2.Sentinel{
		KubernetesConfig: commonapi.KubernetesConfig{ExistingPasswordSecret: secret},
	}

	// ...and require that the promoted selector resolves to it, rather than to a
	// separate (nil) outer field.
	assert.Same(t, secret, sentinel.ExistingPasswordSecret)
}

func TestBuildSentinelPodTemplate(t *testing.T) {
	affinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{TopologyKey: "kubernetes.io/hostname"},
			},
		},
	}
	tolerations := []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}}
	nodeSelector := map[string]string{"disktype": "ssd"}
	topology := []corev1.TopologySpreadConstraint{{
		MaxSkew:           1,
		TopologyKey:       "kubernetes.io/hostname",
		WhenUnsatisfiable: corev1.DoNotSchedule,
	}}
	podSecurityContext := &corev1.PodSecurityContext{FSGroup: ptr.To(int64(1000))}
	imagePullSecrets := []corev1.LocalObjectReference{{Name: "registry-creds"}}
	graceperiod := int64(42)

	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size:                          3,
				Affinity:                      affinity,
				Tolerations:                   &tolerations,
				NodeSelector:                  nodeSelector,
				TopologySpreadConstraints:     topology,
				PodSecurityContext:            podSecurityContext,
				PriorityClassName:             "high-priority",
				TerminationGracePeriodSeconds: ptr.To(graceperiod),
				ServiceAccountName:            ptr.To("sentinel-sa"),
			},
		},
	}
	rr.Spec.Sentinel.ImagePullSecrets = &imagePullSecrets

	labels := map[string]string{"app": "sentinel"}
	template := buildSentinelPodTemplate(rr, labels)
	spec := template.Spec

	assert.Equal(t, labels, template.Labels)
	assert.Same(t, affinity, spec.Affinity)
	assert.Equal(t, tolerations, spec.Tolerations)
	assert.Equal(t, nodeSelector, spec.NodeSelector)
	assert.Equal(t, topology, spec.TopologySpreadConstraints)
	assert.Same(t, podSecurityContext, spec.SecurityContext)
	assert.Equal(t, "high-priority", spec.PriorityClassName)
	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	assert.Equal(t, graceperiod, *spec.TerminationGracePeriodSeconds)
	assert.Equal(t, imagePullSecrets, spec.ImagePullSecrets)
	assert.Equal(t, "sentinel-sa", spec.ServiceAccountName)
	require.Len(t, spec.Containers, 1)
	assert.Equal(t, "sentinel", spec.Containers[0].Name)
}

// TestBuildSentinelPodTemplateOmitsUnsetPlacement ensures nil/empty placement
// fields do not leak into the PodSpec (defaults stay zero-valued).
func TestBuildSentinelPodTemplateOmitsUnsetPlacement(t *testing.T) {
	rr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}

	spec := buildSentinelPodTemplate(rr, nil).Spec

	assert.Nil(t, spec.Affinity)
	assert.Nil(t, spec.Tolerations)
	assert.Nil(t, spec.NodeSelector)
	assert.Nil(t, spec.TopologySpreadConstraints)
	assert.Nil(t, spec.SecurityContext)
	assert.Empty(t, spec.PriorityClassName)
	assert.Nil(t, spec.TerminationGracePeriodSeconds)
	assert.Nil(t, spec.ImagePullSecrets)
	assert.Empty(t, spec.ServiceAccountName)
}

func TestBuildSentinelContainer(t *testing.T) {
	resources := &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	securityContext := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}

	rr := &rrvb2.RedisReplication{
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{
				Size:            3,
				SecurityContext: securityContext,
			},
		},
	}
	rr.Spec.Sentinel.Image = "quay.io/opstree/redis-sentinel:v7.0.15"
	rr.Spec.Sentinel.ImagePullPolicy = corev1.PullAlways
	rr.Spec.Sentinel.Resources = resources

	container := buildSentinelContainer(rr)

	assert.Equal(t, "sentinel", container.Name)
	assert.Equal(t, "quay.io/opstree/redis-sentinel:v7.0.15", container.Image)
	assert.Equal(t, corev1.PullAlways, container.ImagePullPolicy)
	assert.Equal(t, *resources, container.Resources)
	assert.Same(t, securityContext, container.SecurityContext)
	require.Len(t, container.Ports, 1)
	assert.Equal(t, int32(26379), container.Ports[0].ContainerPort)
}

func TestBuildSentinelContainerProbes(t *testing.T) {
	const sanitizer = `if [ -n "${REDISCLI_AUTH:-}" ]; then REDISCLI_AUTH="$(printf %s "$REDISCLI_AUTH" | tr -d '\r\n')"; export REDISCLI_AUTH; fi`
	tests := []struct {
		name string
		tls  *commonapi.TLSConfig
		want string
	}{
		{
			name: "without TLS",
			want: sanitizer + "\n" +
				`RESP="$(redis-cli -h $(hostname) -p ${SENTINEL_PORT} ping)"` + "\n" +
				`[ "$RESP" = "PONG" ]`,
		},
		{
			name: "with TLS",
			tls:  &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}},
			want: sanitizer + "\n" +
				`RESP="$(redis-cli -h $(hostname) -p ${SENTINEL_PORT} --tls --cert ${REDIS_TLS_CERT} --key ${REDIS_TLS_CERT_KEY} ${REDIS_TLS_CA_CERT:+--cacert} ${REDIS_TLS_CA_CERT} ping)"` + "\n" +
				`[ "$RESP" = "PONG" ]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := &rrvb2.RedisReplication{
				Spec: rrvb2.RedisReplicationSpec{TLS: tt.tls, Sentinel: &rrvb2.Sentinel{Size: 3}},
			}

			container := buildSentinelContainer(rr)

			for name, probe := range map[string]*corev1.Probe{"readiness": container.ReadinessProbe, "liveness": container.LivenessProbe} {
				require.NotNil(t, probe, name)
				require.NotNil(t, probe.Exec, name)
				assert.Equal(t, []string{"sh", "-ec", tt.want}, probe.Exec.Command, name)
			}
			assert.NotSame(t, container.ReadinessProbe, container.LivenessProbe)
			// Every variable the probe reads is set on the container.
			names := map[string]bool{}
			for _, env := range container.Env {
				names[env.Name] = true
			}
			assert.True(t, names["SENTINEL_PORT"])
			if tt.tls != nil {
				for _, name := range []string{"REDIS_TLS_CERT", "REDIS_TLS_CERT_KEY", "REDIS_TLS_CA_CERT"} {
					assert.True(t, names[name], name)
				}
			}
		})
	}
}

func TestBuildSentinelEnv(t *testing.T) {
	redisSecret := &commonapi.ExistingPasswordSecret{
		Name: ptr.To("redis-secret"),
		Key:  ptr.To("redis-password"),
	}
	sentinelSecret := &commonapi.ExistingPasswordSecret{
		Name: ptr.To("sentinel-secret"),
		Key:  ptr.To("sentinel-password"),
	}

	newRR := func(redisPwd, sentinelPwd *commonapi.ExistingPasswordSecret) *rrvb2.RedisReplication {
		rr := &rrvb2.RedisReplication{
			Spec: rrvb2.RedisReplicationSpec{
				Sentinel: &rrvb2.Sentinel{Size: 3},
			},
		}
		rr.Spec.KubernetesConfig.ExistingPasswordSecret = redisPwd
		rr.Spec.Sentinel.ExistingPasswordSecret = sentinelPwd
		return rr
	}

	envValue := func(envs []corev1.EnvVar, name string) (string, bool) {
		for _, e := range envs {
			if e.Name == name {
				return e.Value, true
			}
		}
		return "", false
	}

	t.Run("no password secret sets quorum, the hostname knobs and the probe port", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(nil, nil))
		require.Len(t, envs, 4)
		assert.Equal(t, "QUORUM", envs[0].Name)
		assert.Equal(t, "2", envs[0].Value) // size 3 -> 3/2+1
		// Without these the embedded Sentinel always rendered `resolve-hostnames no`
		// and spec.sentinel.resolveHostnames was silently a no-op.
		v, ok := envValue(envs, "RESOLVE_HOSTNAMES")
		require.True(t, ok, "RESOLVE_HOSTNAMES must be passed to the embedded Sentinel")
		assert.Equal(t, "no", v)
		v, ok = envValue(envs, "ANNOUNCE_HOSTNAMES")
		require.True(t, ok, "ANNOUNCE_HOSTNAMES must be passed to the embedded Sentinel")
		assert.Equal(t, "no", v)
		v, ok = envValue(envs, "SENTINEL_PORT")
		require.True(t, ok, "SENTINEL_PORT is read by the probes")
		assert.Equal(t, "26379", v)
		_, ok = envValue(envs, "REDISCLI_AUTH")
		assert.False(t, ok, "REDISCLI_AUTH must stay unset without a sentinel password")
	})

	t.Run("hostname knobs are propagated from the spec", func(t *testing.T) {
		rr := newRR(nil, nil)
		rr.Spec.Sentinel.ResolveHostnames = "yes"
		rr.Spec.Sentinel.AnnounceHostnames = "yes"
		envs := buildSentinelEnv(rr)
		v, _ := envValue(envs, "RESOLVE_HOSTNAMES")
		assert.Equal(t, "yes", v)
		v, _ = envValue(envs, "ANNOUNCE_HOSTNAMES")
		assert.Equal(t, "yes", v)
	})

	t.Run("top-level redis secret becomes the master auth-pass", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(redisSecret, nil))
		assertSecretEnv(t, envs, "MASTER_PASSWORD", "redis-secret", "redis-password")
		_, ok := envValue(envs, "REDIS_PASSWORD")
		assert.False(t, ok, "REDIS_PASSWORD must not be set without spec.sentinel.redisSecret")
	})

	t.Run("sentinel redisSecret protects the sentinel port only", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(redisSecret, sentinelSecret))
		assertSecretEnv(t, envs, "MASTER_PASSWORD", "redis-secret", "redis-password")
		assertSecretEnv(t, envs, "REDIS_PASSWORD", "sentinel-secret", "sentinel-password")
		// The probes authenticate to Sentinel, not to Redis.
		assertSecretEnv(t, envs, "REDISCLI_AUTH", "sentinel-secret", "sentinel-password")
	})

	t.Run("sentinel-only redisSecret leaves the master unauthenticated", func(t *testing.T) {
		envs := buildSentinelEnv(newRR(nil, sentinelSecret))
		assertSecretEnv(t, envs, "REDIS_PASSWORD", "sentinel-secret", "sentinel-password")
		_, ok := envValue(envs, "MASTER_PASSWORD")
		assert.False(t, ok, "MASTER_PASSWORD must not be set without spec.kubernetesConfig.redisSecret")
	})
}

func assertSecretEnv(t *testing.T, envs []corev1.EnvVar, name, wantName, wantKey string) {
	t.Helper()
	for _, e := range envs {
		if e.Name != name {
			continue
		}
		require.NotNil(t, e.ValueFrom)
		require.NotNil(t, e.ValueFrom.SecretKeyRef)
		assert.Equal(t, wantName, e.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, wantKey, e.ValueFrom.SecretKeyRef.Key)
		return
	}
	t.Fatalf("%s env var not found in %+v", name, envs)
}

// TestConfigureSentinelPodUsesSentinelRedisSecret exercises the controller path
// that regressed when an outer ExistingPasswordSecret shadowed the embedded
// redisSecret: the Sentinel connection must authenticate with the password read
// from the embedded redisSecret.
func TestConfigureSentinelPodUsesSentinelRedisSecret(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "sentinel-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("s3cr3t")},
	})

	inst := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Size:     ptr.To(int32(3)),
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}
	inst.Spec.Sentinel.ExistingPasswordSecret = &commonapi.ExistingPasswordSecret{
		Name: ptr.To("sentinel-secret"),
		Key:  ptr.To("password"),
	}

	r := &Reconciler{K8sClient: clientset}
	redisClient := &fakeSentinelRedisClient{
		svc: &fakeSentinelRedisService{slaves: 2, sentinels: 3},
	}
	pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

	err := r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", "")

	require.NoError(t, err)
	require.Len(t, redisClient.connections, 1)
	assert.Equal(t, "10.0.0.10", redisClient.connections[0].Host)
	assert.Equal(t, "s3cr3t", redisClient.connections[0].Password)
}

func TestConfigureSentinelKeepsSentinelAndMasterPasswordsApart(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-secret", Namespace: "default"},
		Data: map[string][]byte{
			"redis_password":    []byte("REDIS_PASS"),
			"sentinel_password": []byte("SENTINEL_PASS"),
		},
	})

	inst := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Size:     ptr.To(int32(3)),
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}
	inst.Spec.KubernetesConfig.ExistingPasswordSecret = &commonapi.ExistingPasswordSecret{
		Name: ptr.To("redis-secret"),
		Key:  ptr.To("redis_password"),
	}
	inst.Spec.Sentinel.ExistingPasswordSecret = &commonapi.ExistingPasswordSecret{
		Name: ptr.To("redis-secret"),
		Key:  ptr.To("sentinel_password"),
	}

	r := &Reconciler{K8sClient: clientset}

	masterPassword, err := r.masterPassword(context.Background(), inst)
	require.NoError(t, err)
	assert.Equal(t, "REDIS_PASS", masterPassword)

	svc := &fakeSentinelRedisService{slaves: 2, sentinels: 3}
	redisClient := &fakeSentinelRedisClient{svc: svc}
	pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

	err = r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", masterPassword)

	require.NoError(t, err)
	require.Len(t, redisClient.connections, 1)
	assert.Equal(t, "SENTINEL_PASS", redisClient.connections[0].Password)
	require.NotNil(t, svc.monitoredMaster)
	assert.Equal(t, "10.0.0.20", svc.monitoredMaster.Host)
	assert.Equal(t, "REDIS_PASS", svc.monitoredMaster.Password)
}

func TestConfigureSentinelPodWithoutSecretSendsEmptyPassword(t *testing.T) {
	inst := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
		Spec: rrvb2.RedisReplicationSpec{
			Size:     ptr.To(int32(3)),
			Sentinel: &rrvb2.Sentinel{Size: 3},
		},
	}

	r := &Reconciler{K8sClient: fake.NewSimpleClientset()}
	redisClient := &fakeSentinelRedisClient{
		svc: &fakeSentinelRedisService{slaves: 2, sentinels: 3},
	}
	pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

	err := r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", "")

	require.NoError(t, err)
	require.Len(t, redisClient.connections, 1)
	assert.Empty(t, redisClient.connections[0].Password)
}

// sentinelTLSEnvNames are the variables the sentinel image's entrypoint reads
// to configure its TLS listener.
var sentinelTLSEnvNames = []string{"TLS_MODE", "REDIS_TLS_CA_KEY", "REDIS_TLS_CERT", "REDIS_TLS_CERT_KEY"}

func TestBuildSentinelPodTemplateTLS(t *testing.T) {
	tests := []struct {
		name        string
		tls         *commonapi.TLSConfig
		wantVolumes []corev1.Volume
		wantMounts  []corev1.VolumeMount
		wantTLSEnv  map[string]string
	}{
		{
			name: "without TLS the pod has no TLS volumes, mounts or env",
		},
		{
			name: "default secret keys",
			tls: &commonapi.TLSConfig{
				Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"},
			},
			wantVolumes: []corev1.Volume{
				{Name: "tls-certs", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}}},
				{Name: "external-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "example-replication-s-tls"},
				}}},
			},
			wantMounts: []corev1.VolumeMount{
				{Name: "tls-certs", MountPath: "/tls", ReadOnly: true},
				{Name: "external-config", MountPath: "/etc/redis/external.conf.d", ReadOnly: true},
			},
			wantTLSEnv: map[string]string{
				"TLS_MODE":           "true",
				"REDIS_TLS_CA_KEY":   "/tls/ca.crt",
				"REDIS_TLS_CA_CERT":  "/tls/ca.crt",
				"REDIS_TLS_CERT":     "/tls/tls.crt",
				"REDIS_TLS_CERT_KEY": "/tls/tls.key",
			},
		},
		{
			name: "custom secret keys",
			tls: &commonapi.TLSConfig{
				CaCertFile:  "root.pem",
				CertKeyFile: "server.pem",
				KeyFile:     "server-key.pem",
				Secret:      corev1.SecretVolumeSource{SecretName: "custom-tls", DefaultMode: ptr.To(int32(0o440))},
			},
			wantVolumes: []corev1.Volume{
				{Name: "tls-certs", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "custom-tls", DefaultMode: ptr.To(int32(0o440))}}},
				{Name: "external-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "example-replication-s-tls"},
				}}},
			},
			wantMounts: []corev1.VolumeMount{
				{Name: "tls-certs", MountPath: "/tls", ReadOnly: true},
				{Name: "external-config", MountPath: "/etc/redis/external.conf.d", ReadOnly: true},
			},
			wantTLSEnv: map[string]string{
				"TLS_MODE":           "true",
				"REDIS_TLS_CA_KEY":   "/tls/root.pem",
				"REDIS_TLS_CA_CERT":  "/tls/root.pem",
				"REDIS_TLS_CERT":     "/tls/server.pem",
				"REDIS_TLS_CERT_KEY": "/tls/server-key.pem",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := &rrvb2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
				Spec: rrvb2.RedisReplicationSpec{
					TLS:      tt.tls,
					Sentinel: &rrvb2.Sentinel{Size: 3},
				},
			}

			spec := buildSentinelPodTemplate(rr, nil).Spec

			assert.Equal(t, tt.wantVolumes, spec.Volumes)
			require.Len(t, spec.Containers, 1)
			container := spec.Containers[0]
			assert.Equal(t, tt.wantMounts, container.VolumeMounts)
			for _, mount := range container.VolumeMounts {
				assert.Empty(t, mount.SubPath, "subPath mounts never see a reissued certificate")
			}
			gotTLSEnv := map[string]string{}
			for _, env := range container.Env {
				if slices.Contains(sentinelTLSEnvNames, env.Name) || strings.HasPrefix(env.Name, "REDIS_TLS_") {
					gotTLSEnv[env.Name] = env.Value
				}
			}
			if tt.wantTLSEnv == nil {
				assert.Empty(t, gotTLSEnv)
			} else {
				assert.Equal(t, tt.wantTLSEnv, gotTLSEnv)
			}
		})
	}
}

func TestNewSentinelTLSConfigMap(t *testing.T) {
	rr := &rrvb2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-replication",
			Namespace: "redis",
			Labels:    map[string]string{"team": "payments"},
		},
		Spec: rrvb2.RedisReplicationSpec{
			Sentinel: &rrvb2.Sentinel{Size: 3},
			TLS:      &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}},
		},
	}

	cm := newSentinelTLSConfigMap(rr)

	assert.Equal(t, "example-replication-s-tls", cm.Name)
	assert.Equal(t, "redis", cm.Namespace)
	assert.Equal(t, "payments", cm.Labels["team"])
	assert.Equal(t, newSentinelService(rr).Labels, cm.Labels)
	// The entrypoint includes exactly this file, as the last line of
	// sentinel.conf, so these lines override its "tls-auth-clients optional".
	require.Equal(t, []string{"redis-sentinel-additional.conf"}, slices.Collect(maps.Keys(cm.Data)))
	assert.Equal(t, []string{"tls-replication yes", "tls-auth-clients yes"},
		strings.Split(strings.TrimSpace(cm.Data["redis-sentinel-additional.conf"]), "\n"))
}

func TestConfigureSentinelPodTLS(t *testing.T) {
	ca := newTestCA(t)
	tlsSpec := &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}}

	tests := []struct {
		name    string
		tls     *commonapi.TLSConfig
		objects []runtime.Object
		wantTLS bool
		wantErr string
	}{
		{
			name:    "without TLS the sentinel is reached over plain TCP",
			wantTLS: false,
		},
		{
			name:    "with TLS the sentinel is reached with the replication certificate",
			tls:     tlsSpec,
			objects: []runtime.Object{ca.secret(t, "redis-tls-cert", "default", 1)},
			wantTLS: true,
		},
		{
			name:    "a TLS secret that cannot be read is an error, not a plain-text fallback",
			tls:     tlsSpec,
			wantErr: `build sentinel TLS config: get TLS secret default/redis-tls-cert`,
		},
		{
			// Hostname verification is skipped, so trusting the system roots
			// would accept any publicly issued certificate as the Sentinel.
			name:    "a TLS secret without the CA is an error, not a system trust store fallback",
			tls:     tlsSpec,
			objects: []runtime.Object{withoutCA(ca.secret(t, "redis-tls-cert", "default", 1))},
			wantErr: `TLS secret default/redis-tls-cert is missing the CA key "ca.crt"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := newSentinelReplicationInstanceForTest()
			inst.Spec.TLS = tt.tls
			r := &Reconciler{K8sClient: fake.NewSimpleClientset(tt.objects...)}
			// More slaves than expected, so the call also goes through SENTINEL RESET.
			svc := &fakeSentinelRedisService{slaves: 5, sentinels: 3}
			redisClient := &fakeSentinelRedisClient{svc: svc}
			pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

			err := r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", "")

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Empty(t, redisClient.connections)
				return
			}
			require.NoError(t, err)
			// MONITOR, SET and the reset check all share this one connection.
			require.Len(t, redisClient.connections, 1)
			assert.Equal(t, 1, svc.resets)
			conn := redisClient.connections[0]
			assert.Equal(t, "26379", conn.Port)
			if !tt.wantTLS {
				assert.Nil(t, conn.TLSConfig)
				return
			}
			require.NotNil(t, conn.TLSConfig)
			assert.Equal(t, secretLeafDER(t, r, "redis-tls-cert"), conn.TLSConfig.Certificates[0].Certificate[0])
			assert.NotNil(t, conn.TLSConfig.VerifyPeerCertificate, "the sentinel certificate chain must be verified")
		})
	}
}

func TestQuerySentinelMonitoredMasterTLS(t *testing.T) {
	ca := newTestCA(t)

	tests := []struct {
		name    string
		tls     *commonapi.TLSConfig
		secret  bool
		wantTLS bool
		wantErr string
	}{
		{name: "without TLS", wantTLS: false},
		{
			name:    "with TLS",
			tls:     &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}},
			secret:  true,
			wantTLS: true,
		},
		{
			name:    "with TLS and no usable secret",
			tls:     &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}},
			wantErr: `build sentinel TLS config: get TLS secret default/redis-tls-cert`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := newSentinelReplicationInstanceForTest()
			instance.Spec.TLS = tt.tls
			const candidate = "example-replication-0"
			objects := []runtime.Object{newRunningPodForTest(instance.Namespace, candidate, "10.0.0.10", nil, true)}
			sentinelLabels := common.GetRedisLabels(instance.SentinelStatefulSet(), common.SetupTypeSentinel, "sentinel", instance.GetLabels())
			for i, ip := range []string{"10.0.1.10", "10.0.1.11", "10.0.1.12"} {
				objects = append(objects, newRunningPodForTest(instance.Namespace, fmt.Sprintf("%s-%d", instance.SentinelStatefulSet(), i), ip, sentinelLabels, true))
			}
			if tt.secret {
				objects = append(objects, ca.secret(t, "redis-tls-cert", instance.Namespace, 1))
			}
			info := sentinelInfoMonitoring("10.0.0.10:6379")
			redisClient := &fakeSentinelInfoClient{infoByHost: map[string]*redis.InfoSentinelResult{
				"10.0.1.10": info, "10.0.1.11": info, "10.0.1.12": info,
			}}
			r := &Reconciler{K8sClient: fake.NewSimpleClientset(objects...)}

			got, err := r.querySentinelMonitoredMaster(context.Background(), redisClient, instance, []string{candidate})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Empty(t, redisClient.connections)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, candidate, got)
			require.Len(t, redisClient.connections, 3)
			for _, conn := range redisClient.connections {
				if tt.wantTLS {
					require.NotNil(t, conn.TLSConfig, "sentinel %s", conn.Host)
					assert.Equal(t, secretLeafDER(t, r, "redis-tls-cert"), conn.TLSConfig.Certificates[0].Certificate[0])
				} else {
					assert.Nil(t, conn.TLSConfig, "sentinel %s", conn.Host)
				}
			}
		})
	}
}

// TestSentinelTLSConfigFollowsReissuedCertificate covers certificate renewal:
// the operator must present the certificate currently in the secret, not the
// one it saw first, or its connections fail once the old one expires.
func TestSentinelTLSConfigFollowsReissuedCertificate(t *testing.T) {
	ca := newTestCA(t)
	secret := ca.secret(t, "redis-tls-cert", "default", 1)
	clientset := fake.NewSimpleClientset(secret)
	inst := newSentinelReplicationInstanceForTest()
	inst.Spec.TLS = &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}}
	r := &Reconciler{K8sClient: clientset}
	redisClient := &fakeSentinelRedisClient{svc: &fakeSentinelRedisService{slaves: 2, sentinels: 3}}
	pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

	require.NoError(t, r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", ""))
	reissued := ca.secret(t, "redis-tls-cert", "default", 2)
	_, err := clientset.CoreV1().Secrets("default").Update(context.Background(), reissued, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NoError(t, r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", ""))

	require.Len(t, redisClient.connections, 2)
	first := redisClient.connections[0].TLSConfig.Certificates[0].Certificate[0]
	second := redisClient.connections[1].TLSConfig.Certificates[0].Certificate[0]
	assert.NotEqual(t, first, second)
	assert.Equal(t, secretLeafDER(t, r, "redis-tls-cert"), second)
}

// TestSentinelTLSConfigFailsClosedWhenRenewalDropsCA covers a renewal that
// writes the secret without its CA: the operator must stop connecting rather
// than switch to the system trust store.
func TestSentinelTLSConfigFailsClosedWhenRenewalDropsCA(t *testing.T) {
	ca := newTestCA(t)
	clientset := fake.NewSimpleClientset(ca.secret(t, "redis-tls-cert", "default", 1))
	inst := newSentinelReplicationInstanceForTest()
	inst.Spec.TLS = &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-cert"}}
	r := &Reconciler{K8sClient: clientset}
	redisClient := &fakeSentinelRedisClient{svc: &fakeSentinelRedisService{slaves: 2, sentinels: 3}}
	pod := corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.10"}}

	require.NoError(t, r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", ""))
	reissued := withoutCA(ca.secret(t, "redis-tls-cert", "default", 2))
	_, err := clientset.CoreV1().Secrets("default").Update(context.Background(), reissued, metav1.UpdateOptions{})
	require.NoError(t, err)

	err = r.configureSentinelPod(context.Background(), redisClient, inst, pod, "10.0.0.20", "")

	require.ErrorContains(t, err, `is missing the CA key "ca.crt"`)
	assert.Len(t, redisClient.connections, 1, "no connection is made with the CA-less secret")
}

func withoutCA(secret *corev1.Secret) *corev1.Secret {
	delete(secret.Data, "ca.crt")
	return secret
}

// testCA signs the TLS secrets used by these tests, so that a reissue can be
// simulated by signing a second leaf with the same CA.
type testCA struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1000),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &testCA{cert: cert, key: key, certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

// secret returns a kubernetes.io/tls secret holding a leaf certificate, with
// both server and client auth like the Redis certificates it stands for.
func (ca *testCA) secret(t *testing.T, name, namespace string, serial int64) *corev1.Secret {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "redis"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  ca.certPEM,
			"tls.crt": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			"tls.key": pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		},
	}
}

// secretLeafDER returns the DER of the certificate currently stored in the secret.
func secretLeafDER(t *testing.T, r *Reconciler, name string) []byte {
	t.Helper()
	secret, err := r.K8sClient.CoreV1().Secrets("default").Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	block, _ := pem.Decode(secret.Data["tls.crt"])
	require.NotNil(t, block)
	return block.Bytes
}

type fakeSentinelRedisClient struct {
	connections []*redis.ConnectionInfo
	svc         *fakeSentinelRedisService
}

func (f *fakeSentinelRedisClient) Connect(info *redis.ConnectionInfo) redis.Service {
	f.connections = append(f.connections, info)
	return f.svc
}

type fakeSentinelRedisService struct {
	slaves          int
	sentinels       int
	monitoredMaster *redis.ConnectionInfo
	resets          int
}

func (f *fakeSentinelRedisService) IsMaster(context.Context) (bool, error) { return false, nil }

func (f *fakeSentinelRedisService) GetKeyCount(context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeSentinelRedisService) GetAttachedReplicaCount(context.Context) (int, error) {
	return 0, nil
}

func (f *fakeSentinelRedisService) SentinelMonitor(_ context.Context, master *redis.ConnectionInfo, _, _ string) error {
	f.monitoredMaster = master
	return nil
}

func (f *fakeSentinelRedisService) SentinelSet(context.Context, string, string, string) error {
	return nil
}

func (f *fakeSentinelRedisService) SentinelReset(context.Context, string) error {
	f.resets++
	return nil
}

func (f *fakeSentinelRedisService) GetInfoSentinel(context.Context) (*redis.InfoSentinelResult, error) {
	return &redis.InfoSentinelResult{
		Masters: []redis.SentinelMasterInfo{
			{Name: masterGroupName, Slaves: f.slaves, Sentinels: f.sentinels},
		},
	}, nil
}

func (f *fakeSentinelRedisService) GetClusterInfo(context.Context) (*redis.ClusterStatus, error) {
	return &redis.ClusterStatus{}, nil
}

// TestNewSentinelStatefulSetMinReadySeconds guards spec.sentinel.minReadySeconds,
// which the embedded Sentinel inherits from KubernetesConfig, so that a rollout
// waits for each Sentinel to stay ready before replacing the next one.
func TestNewSentinelStatefulSetMinReadySeconds(t *testing.T) {
	tests := []struct {
		name            string
		minReadySeconds *int32
		want            int32
	}{
		{name: "unset", minReadySeconds: nil, want: 0},
		{name: "set", minReadySeconds: ptr.To(int32(30)), want: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := &rrvb2.RedisReplication{
				ObjectMeta: metav1.ObjectMeta{Name: "example-replication", Namespace: "default"},
				Spec: rrvb2.RedisReplicationSpec{
					Sentinel: &rrvb2.Sentinel{
						KubernetesConfig: commonapi.KubernetesConfig{MinReadySeconds: tt.minReadySeconds},
						Size:             3,
					},
				},
			}

			sts := newSentinelStatefulSet(rr, rr.SentinelHLService())

			assert.Equal(t, tt.want, sts.Spec.MinReadySeconds)
		})
	}
}
