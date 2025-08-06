package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/cryptutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Healer interface {
	SentinelMonitor(ctx context.Context, rs *rsvb2.RedisSentinel, master string) error
	SentinelReset(ctx context.Context, rs *rsvb2.RedisSentinel) error

	// UpdatePodRoleLabel connect to all redis pods and update pod role label `redis-role` to `master` or `slave` according to their role.
	UpdateRedisRoleLabel(ctx context.Context, ns string, labels map[string]string, secret *commonapi.ExistingPasswordSecret, tlsConfig *commonapi.TLSConfig) error
}

type healer struct {
	redis redis.Client
	k8s   kubernetes.Interface
}

func NewHealer(clientset kubernetes.Interface) Healer {
	return &healer{
		k8s:   clientset,
		redis: redis.NewClient(),
	}
}

func (h *healer) UpdateRedisRoleLabel(ctx context.Context, ns string, labels map[string]string, secret *commonapi.ExistingPasswordSecret, tlsConfig *commonapi.TLSConfig) error {
	selector := make([]string, 0, len(labels))
	for key, value := range labels {
		selector = append(selector, fmt.Sprintf("%s=%s", key, value))
	}
	pods, err := h.k8s.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(selector, ","),
	})
	if err != nil {
		return err
	}
	password, err := NewChecker(h.k8s).GetPassword(ctx, ns, secret)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		connInfo := createConnectionInfo(ctx, pod, password, tlsConfig, h.k8s, ns, "6379")
		isMaster, err := h.redis.Connect(connInfo).IsMaster(ctx)
		if err != nil {
			return err
		}
		role := common.RedisRoleLabelSlave
		if isMaster {
			role = common.RedisRoleLabelMaster
		}
		if oldRole := pod.Labels[common.RedisRoleLabelKey]; oldRole != role {
			patch := []byte(fmt.Sprintf(`[{"op": "add", "path": "/metadata/labels/%s", "value": "%s"}]`, common.RedisRoleLabelKey, role))
			rErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				_, err = h.k8s.CoreV1().Pods(ns).Patch(ctx, pod.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
				if err != nil {
					log.FromContext(ctx).Error(err, "failed to update pod role label", "pod", pod.Name, "oldRole", oldRole, "newRole", role)
					return err
				}
				return nil
			})
			if rErr != nil {
				return fmt.Errorf("failed to update pod role label: %w", rErr)
			}
			log.FromContext(ctx).Info("updated pod role label", "pod", pod.Name, "oldRole", oldRole, "newRole", role)
		}
	}
	return nil
}

// SentinelReset range all sentinel execute `sentinel reset *`
func (h *healer) SentinelReset(ctx context.Context, rs *rsvb2.RedisSentinel) error {
	pods, err := h.getSentinelPods(ctx, rs)
	if err != nil {
		return err
	}

	sentinelPass, err := NewChecker(h.k8s).GetPassword(ctx, rs.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		connInfo := createConnectionInfo(ctx, pod, sentinelPass, rs.Spec.TLS, h.k8s, rs.Namespace, "26379")

		err = h.redis.Connect(connInfo).SentinelReset(ctx, rs.Spec.RedisSentinelConfig.MasterGroupName)
		if err != nil {
			return err
		}
	}
	return nil
}

// SentinelMonitor range all sentinel execute `sentinel monitor`
func (h *healer) SentinelMonitor(ctx context.Context, rs *rsvb2.RedisSentinel, master string) error {
	pods, err := h.getSentinelPods(ctx, rs)
	if err != nil {
		return err
	}

	sentinelPass, err := NewChecker(h.k8s).GetPassword(ctx, rs.Namespace, rs.Spec.KubernetesConfig.ExistingPasswordSecret)
	if err != nil {
		return err
	}

	var masterPass string
	if rs.Spec.RedisSentinelConfig.RedisReplicationPassword != nil && rs.Spec.RedisSentinelConfig.RedisReplicationPassword.SecretKeyRef != nil {
		masterPass, err = NewChecker(h.k8s).GetPassword(ctx, rs.Namespace, &commonapi.ExistingPasswordSecret{
			Name: &rs.Spec.RedisSentinelConfig.RedisReplicationPassword.SecretKeyRef.Name,
			Key:  &rs.Spec.RedisSentinelConfig.RedisReplicationPassword.SecretKeyRef.Key,
		})
		if err != nil {
			return err
		}
	}

	for _, pod := range pods.Items {
		connInfo := createConnectionInfo(ctx, pod, sentinelPass, rs.Spec.TLS, h.k8s, rs.Namespace, "26379")

		masterConnInfo := &redis.ConnectionInfo{
			Host:     master,
			Port:     "6379",
			Password: masterPass,
		}
		err = h.redis.Connect(connInfo).SentinelMonitor(
			ctx,
			masterConnInfo,
			rs.Spec.RedisSentinelConfig.MasterGroupName,
			rs.Spec.RedisSentinelConfig.Quorum,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *healer) getSentinelPods(ctx context.Context, rs *rsvb2.RedisSentinel) (*v1.PodList, error) {
	sentinelSTS, err := h.k8s.AppsV1().StatefulSets(rs.Namespace).Get(ctx, rs.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var labels []string
	for k, v := range sentinelSTS.Spec.Selector.MatchLabels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	pods, err := h.k8s.CoreV1().Pods(rs.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(labels, ","),
	})
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// getRedisTLSConfig creates a TLS configuration for Redis connections
func getRedisTLSConfig(ctx context.Context, client kubernetes.Interface, namespace, tlsSecretName string) *tls.Config {
	// This is a wrapper to access the k8sutils internal function
	// We'll implement a simplified version here for now
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, tlsSecretName, metav1.GetOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed in getting TLS secret", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	tlsClientCert, certExists := secret.Data["tls.crt"]
	tlsClientKey, keyExists := secret.Data["tls.key"]
	tlsCACert, caExists := secret.Data["ca.crt"]

	if !certExists || !keyExists || !caExists {
		log.FromContext(ctx).Error(fmt.Errorf("TLS secret missing required keys"), "TLS secret is missing required keys", "secretName", tlsSecretName)
		return nil
	}

	cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to load TLS key pair", "secretName", tlsSecretName)
		return nil
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(tlsCACert)

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			_, _, err := cryptutil.VerifyCertificateExceptServerName(rawCerts, &tls.Config{RootCAs: caCertPool})
			return err
		},
	}
}

// createConnectionInfo creates a Redis connection info with TLS support
func createConnectionInfo(ctx context.Context, pod v1.Pod, password string, tlsConfig *commonapi.TLSConfig, k8sClient kubernetes.Interface, namespace, port string) *redis.ConnectionInfo {
	connInfo := &redis.ConnectionInfo{
		Host:     pod.Status.PodIP,
		Port:     port,
		Password: password,
	}

	// Configure TLS if enabled
	if tlsConfig != nil && tlsConfig.Secret.SecretName != "" {
		serviceName := common.GetHeadlessServiceNameFromPodName(pod.Name)
		connInfo.Host = fmt.Sprintf("%s.%s.%s.svc.cluster.local", pod.Name, serviceName, namespace)
		// Get TLS configuration
		tlsCfg := getRedisTLSConfig(ctx, k8sClient, namespace, tlsConfig.Secret.SecretName)
		connInfo.TLSConfig = tlsCfg
	}

	return connInfo
}
