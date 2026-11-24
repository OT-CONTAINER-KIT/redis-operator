package k8sutils

import (
	"context"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
)

func Test_getRedisPassword(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *k8sClientFake.Clientset
		namespace   string
		secretName  string
		secretKey   string
		expected    string
		expectedErr bool
	}{
		{
			name: "successful retrieval",
			setup: func() *k8sClientFake.Clientset {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"password": []byte("secret-password"),
					},
				}
				client := k8sClientFake.NewSimpleClientset(secret.DeepCopyObject())
				return client
			},
			namespace:   "default",
			secretName:  "redis-secret",
			secretKey:   "password",
			expected:    "secret-password",
			expectedErr: false,
		},
		{
			name: "secret not found",
			setup: func() *k8sClientFake.Clientset {
				client := k8sClientFake.NewSimpleClientset()
				return client
			},
			namespace:   "default",
			secretName:  "non-existent",
			secretKey:   "password",
			expected:    "",
			expectedErr: true,
		},
		{
			name: "secret exists but key is missing",
			setup: func() *k8sClientFake.Clientset {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"anotherKey": []byte("some-value"),
					},
				}
				client := k8sClientFake.NewSimpleClientset(secret.DeepCopyObject())
				return client
			},
			namespace:   "default",
			secretName:  "redis-secret",
			secretKey:   "missingKey",
			expected:    "",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup()

			got, err := getRedisPassword(context.TODO(), client, tt.namespace, tt.secretName, tt.secretKey)

			if tt.expectedErr {
				require.Error(t, err, "Expected an error but didn't get one")
			} else {
				require.NoError(t, err, "Expected no error but got one")
				assert.Equal(t, tt.expected, got, "Expected and actual values do not match")
			}
		})
	}
}

func Test_GetRedisTLSConfig(t *testing.T) {
	tests := []struct {
		name         string
		setup        func() *k8sClientFake.Clientset
		redisCluster *rcvb2.RedisCluster
		redisInfo    RedisDetails
		expectTLS    bool
	}{
		{
			name: "TLS enabled and successful configuration",
			setup: func() *k8sClientFake.Clientset {
				tlsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-tls-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"ca.crt":  helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "ca.crt")),
						"tls.crt": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.crt")),
						"tls.key": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.key")),
					},
				}
				client := k8sClientFake.NewSimpleClientset(tlsSecret)
				return client
			},
			redisCluster: &rcvb2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: rcvb2.RedisClusterSpec{
					TLS: &common.TLSConfig{
						CaCertFile:  "ca.crt",
						CertKeyFile: "tls.crt",
						KeyFile:     "tls.key",
						Secret: corev1.SecretVolumeSource{
							SecretName: "redis-tls-secret",
						},
					},
				},
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectTLS: true,
		},
		{
			name: "TLS enabled with no CA key and implicit CA config",
			setup: func() *k8sClientFake.Clientset {
				tlsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-tls-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"tls.crt": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.crt")),
						"tls.key": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.key")),
					},
				}
				client := k8sClientFake.NewSimpleClientset(tlsSecret)
				return client
			},
			redisCluster: &rcvb2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: rcvb2.RedisClusterSpec{
					TLS: &common.TLSConfig{
						CertKeyFile: "tls.crt",
						KeyFile:     "tls.key",
						Secret: corev1.SecretVolumeSource{
							SecretName: "redis-tls-secret",
						},
					},
				},
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectTLS: true,
		},
		{
			name: "TLS enabled but secret not found",
			setup: func() *k8sClientFake.Clientset {
				client := k8sClientFake.NewSimpleClientset()
				return client
			},
			redisCluster: &rcvb2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: rcvb2.RedisClusterSpec{
					TLS: &common.TLSConfig{
						CaCertFile:  "ca.crt",
						CertKeyFile: "tls.crt",
						KeyFile:     "tls.key",
						Secret: corev1.SecretVolumeSource{
							SecretName: "redis-tls-secret",
						},
					},
				},
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectTLS: false,
		},
		{
			name: "TLS enabled with explicit CA configured but missing from secret",
			setup: func() *k8sClientFake.Clientset {
				tlsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-tls-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						// ca.crt is intentionally absent, but CaCertFile is explicitly set
						"tls.crt": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.crt")),
						"tls.key": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.key")),
					},
				}
				client := k8sClientFake.NewSimpleClientset(tlsSecret)
				return client
			},
			redisCluster: &rcvb2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: rcvb2.RedisClusterSpec{
					TLS: &common.TLSConfig{
						CaCertFile:  "ca.crt", // explicitly set but missing from secret → misconfiguration
						CertKeyFile: "tls.crt",
						KeyFile:     "tls.key",
						Secret: corev1.SecretVolumeSource{
							SecretName: "redis-tls-secret",
						},
					},
				},
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectTLS: false, // should fail fast as misconfiguration
		},
		{
			name: "TLS enabled with no CA in secret and no explicit CA config uses system trust store",
			setup: func() *k8sClientFake.Clientset {
				tlsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-tls-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"tls.crt": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.crt")),
						"tls.key": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "tls.key")),
					},
				}
				client := k8sClientFake.NewSimpleClientset(tlsSecret)
				return client
			},
			redisCluster: &rcvb2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: rcvb2.RedisClusterSpec{
					TLS: &common.TLSConfig{
						// CaCertFile intentionally not set → should use system trust store
						CertKeyFile: "tls.crt",
						KeyFile:     "tls.key",
						Secret: corev1.SecretVolumeSource{
							SecretName: "redis-tls-secret",
						},
					},
				},
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectTLS: true, // no explicit CA → system trust store fallback
		},
		{
			name: "TLS enabled but incomplete secret",
			setup: func() *k8sClientFake.Clientset {
				tlsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-tls-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"ca.crt": helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", "ca.crt")),
						// Missing tls.crt and tls.key
					},
				}
				client := k8sClientFake.NewSimpleClientset(tlsSecret)
				return client
			},
			redisCluster: &rcvb2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: rcvb2.RedisClusterSpec{
					TLS: &common.TLSConfig{
						CaCertFile:  "ca.crt",
						CertKeyFile: "tls.crt",
						KeyFile:     "tls.key",
						Secret: corev1.SecretVolumeSource{
							SecretName: "redis-tls-secret",
						},
					},
				},
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectTLS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup()

			tlsConfig := GetRedisTLSConfig(context.TODO(), client, tt.redisCluster.Namespace, tt.redisCluster.Spec.TLS)

			if tt.expectTLS {
				require.NotNil(t, tlsConfig, "Expected TLS configuration but got nil")
				require.NotEmpty(t, tlsConfig.Certificates, "TLS Certificates should not be empty")
				// Whether the CA comes from the secret or from the system trust
				// store fallback, the operator dials pods by IP/pod DNS that do
				// not match the server certificate SNI. The config must therefore
				// skip Go's default hostname verification and verify the chain
				// itself, otherwise the connection would fail. This is the
				// connection semantics the fallback path must preserve.
				require.NotNil(t, tlsConfig.RootCAs, "Root CAs should not be nil")
				require.True(t, tlsConfig.InsecureSkipVerify, "InsecureSkipVerify should be true so the default hostname check is skipped")
				require.NotNil(t, tlsConfig.VerifyPeerCertificate, "VerifyPeerCertificate must be set to verify the chain without hostname checks")
			} else {
				assert.Nil(t, tlsConfig, "Expected no TLS configuration but got one")
			}
		})
	}
}

func Test_GetSentinelTLSConfig(t *testing.T) {
	testdata := func(name string) []byte {
		return helperReadFile(filepath.Join("..", "..", "tests", "testdata", "secrets", name))
	}
	secretWith := func(keys ...string) *corev1.Secret {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-tls-secret", Namespace: "default"},
			Data:       map[string][]byte{},
		}
		for _, key := range keys {
			secret.Data[key] = testdata(key)
		}
		return secret
	}
	implicitCA := &common.TLSConfig{Secret: corev1.SecretVolumeSource{SecretName: "redis-tls-secret"}}

	tests := []struct {
		name      string
		tlsConfig *common.TLSConfig
		secret    *corev1.Secret
		wantTLS   bool
		wantErr   string
	}{
		{
			name: "TLS not configured",
		},
		{
			name:      "CA from the secret",
			tlsConfig: implicitCA,
			secret:    secretWith("ca.crt", "tls.crt", "tls.key"),
			wantTLS:   true,
		},
		{
			// GetRedisTLSConfig falls back to the system trust store here;
			// the Sentinel path must not.
			name:      "no CA in the secret and no explicit CA key",
			tlsConfig: implicitCA,
			secret:    secretWith("tls.crt", "tls.key"),
			wantErr:   `TLS secret default/redis-tls-secret is missing the CA key "ca.crt"`,
		},
		{
			name: "explicit CA key missing from the secret",
			tlsConfig: &common.TLSConfig{
				CaCertFile: "root.pem",
				Secret:     corev1.SecretVolumeSource{SecretName: "redis-tls-secret"},
			},
			secret:  secretWith("ca.crt", "tls.crt", "tls.key"),
			wantErr: `is missing the CA key "root.pem"`,
		},
		{
			name:      "secret not found",
			tlsConfig: implicitCA,
			wantErr:   "get TLS secret default/redis-tls-secret",
		},
		{
			name:      "no secret name",
			tlsConfig: &common.TLSConfig{},
			wantErr:   "TLS secret name is not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := k8sClientFake.NewSimpleClientset()
			if tt.secret != nil {
				client = k8sClientFake.NewSimpleClientset(tt.secret)
			}

			tlsConfig, err := GetSentinelTLSConfig(context.Background(), client, "default", tt.tlsConfig)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, tlsConfig)
				return
			}
			require.NoError(t, err)
			if !tt.wantTLS {
				assert.Nil(t, tlsConfig)
				return
			}
			require.NotNil(t, tlsConfig)
			require.NotEmpty(t, tlsConfig.Certificates)
			require.NotNil(t, tlsConfig.VerifyPeerCertificate)
			// The pool holds exactly the secret's CA, not the system roots.
			wantPool := x509.NewCertPool()
			require.True(t, wantPool.AppendCertsFromPEM(testdata("ca.crt")))
			assert.True(t, wantPool.Equal(tlsConfig.RootCAs), "RootCAs must be the secret's CA only")
		})
	}
}

func helperReadFile(filename string) []byte {
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return data
}
