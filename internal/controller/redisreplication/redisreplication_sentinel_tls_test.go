package redisreplication

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// A standalone RedisSentinel with spec.TLS only accepts TLS connections: the
// RedisReplication controller must reach it the way the RedisSentinel
// controller does, otherwise it misses a running failover and undoes it.
func tlsSentinelReconciler(t *testing.T) *Reconciler {
	t.Helper()
	rs := newStandaloneSentinelForTest("redis-sentinel", "example-replication")
	rs.Spec.TLS = &commonapi.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "sentinel-tls"}}
	objects := append(standaloneSentinelObjects(rs), selfSignedTLSSecret(t, "default", "sentinel-tls"))
	return &Reconciler{
		Client:    ctrlClientWith(t, rs),
		K8sClient: fake.NewSimpleClientset(objects...),
	}
}

func assertSentinelConnectionsUseTLS(t *testing.T, connections []*redis.ConnectionInfo) {
	t.Helper()
	require.Len(t, connections, 3)
	for _, info := range connections {
		assert.NotNil(t, info.TLSConfig, "sentinel %s reached without TLS", info.Host)
		assert.True(t, strings.HasPrefix(info.Host, "redis-sentinel-sentinel-"), "TLS needs the pod DNS name, got %s", info.Host)
		assert.Equal(t, "26379", info.Port)
	}
}

func TestQuerySentinelFailoverInProgressReachesTLSSentinelsOverTLS(t *testing.T) {
	redisClient := &fakeSentinelInfoClient{}

	_, err := tlsSentinelReconciler(t).querySentinelFailoverInProgress(context.Background(), redisClient, newReplicationInstanceForTest())

	require.NoError(t, err)
	assertSentinelConnectionsUseTLS(t, redisClient.connections)
}

func TestQuerySentinelMonitoredMasterReachesTLSSentinelsOverTLS(t *testing.T) {
	redisClient := &fakeSentinelInfoClient{}

	_, err := tlsSentinelReconciler(t).querySentinelMonitoredMaster(context.Background(), redisClient, newReplicationInstanceForTest(), []string{"example-replication-0", "example-replication-2"})

	require.NoError(t, err)
	assertSentinelConnectionsUseTLS(t, redisClient.connections)
}

func selfSignedTLSSecret(t *testing.T, namespace, name string) *corev1.Secret {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "sentinel"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data: map[string][]byte{
			"ca.crt":  certPEM,
			"tls.crt": certPEM,
			"tls.key": pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		},
	}
}
