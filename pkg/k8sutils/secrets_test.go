package k8sutils

import (
	"os"
	"path/filepath"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/go-logr/logr/testr"
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
			logger := testr.New(t)
			got, err := getRedisPassword(client, logger, tt.namespace, tt.secretName, tt.secretKey)

			if tt.expectedErr {
				require.Error(t, err, "Expected an error but didn't get one")
			} else {
				require.NoError(t, err, "Expected no error but got one")
				assert.Equal(t, tt.expected, got, "Expected and actual values do not match")
			}
		})
	}
}

func Test_getRedisTLSConfig(t *testing.T) {
	tests := []struct {
		name         string
		setup        func() *k8sClientFake.Clientset
		redisCluster *redisv1beta2.RedisCluster
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
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					TLS: &redisv1beta2.TLSConfig{
						TLSConfig: common.TLSConfig{
							CaKeyFile:   "ca.crt",
							CertKeyFile: "tls.crt",
							KeyFile:     "tls.key",
							Secret: corev1.SecretVolumeSource{
								SecretName: "redis-tls-secret",
							},
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
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					TLS: &redisv1beta2.TLSConfig{
						TLSConfig: common.TLSConfig{
							CaKeyFile:   "ca.crt",
							CertKeyFile: "tls.crt",
							KeyFile:     "tls.key",
							Secret: corev1.SecretVolumeSource{
								SecretName: "redis-tls-secret",
							},
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
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					TLS: &redisv1beta2.TLSConfig{
						TLSConfig: common.TLSConfig{
							CaKeyFile:   "ca.crt",
							CertKeyFile: "tls.crt",
							KeyFile:     "tls.key",
							Secret: corev1.SecretVolumeSource{
								SecretName: "redis-tls-secret",
							},
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
			logger := testr.New(t)
			tlsConfig := getRedisTLSConfig(client, logger, tt.redisCluster.Namespace, tt.redisCluster.Spec.TLS.Secret.SecretName, tt.redisInfo.PodName)

			if tt.expectTLS {
				require.NotNil(t, tlsConfig, "Expected TLS configuration but got nil")
				require.NotEmpty(t, tlsConfig.Certificates, "TLS Certificates should not be empty")
				require.NotNil(t, tlsConfig.RootCAs, "Root CAs should not be nil")
			} else {
				assert.Nil(t, tlsConfig, "Expected no TLS configuration but got one")
			}
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
