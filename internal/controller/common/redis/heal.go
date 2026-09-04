package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/envs"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/cryptutil"
	rediscli "github.com/redis/go-redis/v9"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Healer interface {
	SentinelMonitor(ctx context.Context, rs *rsvb2.RedisSentinel, master string) error
	// SentinelSet sets the config for a specific master
	// See: https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/#reconfiguring-sentinel-at-runtime
	SentinelSet(ctx context.Context, rs *rsvb2.RedisSentinel, master string) error
	SentinelReset(ctx context.Context, rs *rsvb2.RedisSentinel) error

	// UpdateRedisRoleLabel checks each Running and Ready pod and updates its `redis-role`
	// label to match the pod's real role.
	//
	// If a pod can't be reached, its label is normally left as is. The one exception is a pod
	// labeled as master that is unreachable while another pod already answers as master and
	// either has replicas attached or is masterPod, the pod the caller has positively identified
	// as the master (through Sentinel, say, after a failover that left it no replica to attach).
	// In that case the old master label is removed, because keeping it could point the master
	// service at a dead pod. If that removal fails, an error is returned so the caller retries.
	// Pass an empty masterPod when no master has been identified.
	UpdateRedisRoleLabel(ctx context.Context, ns string, labels map[string]string, secret *commonapi.ExistingPasswordSecret, tlsConfig *commonapi.TLSConfig, masterPod string) error
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

func (h *healer) UpdateRedisRoleLabel(ctx context.Context, ns string, labels map[string]string, secret *commonapi.ExistingPasswordSecret, tlsConfig *commonapi.TLSConfig, masterPod string) error {
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
	patchFunc := func(pod string, patchBs []byte) func() error {
		return func() error {
			_, err := h.k8s.
				CoreV1().
				Pods(ns).
				Patch(ctx, pod, types.JSONPatchType, patchBs, metav1.PatchOptions{})
			return err
		}
	}
	removeRoleLabelFunc := func(pod string) func() error {
		patchBs := []byte(fmt.Sprintf(`{"metadata":{"labels":{"%s":null}}}`, common.RedisRoleLabelKey))
		return func() error {
			_, err := h.k8s.
				CoreV1().
				Pods(ns).
				Patch(ctx, pod, types.MergePatchType, patchBs, metav1.PatchOptions{})
			return err
		}
	}

	var masterConfirmed bool
	var unreachableMasterPods []string
	for _, pod := range pods.Items {
		if !k8sutils.IsRedisPodProbeable(&pod) {
			continue
		}

		connInfo := createConnectionInfo(ctx, pod, password, tlsConfig, h.k8s, ns, "6379")
		redisService := h.redis.Connect(connInfo)
		isMaster, err := redisService.IsMaster(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			log.FromContext(ctx).Error(err, "failed to check redis role, skipping pod", "pod", pod.Name)
			// A probe failure doesn't prove the pod lost its master role. Only a pod
			// with a master label can cause split-brain, so collect unreachable pods
			// and remove their master labels only if another master with replicas
			// answers. This avoids removing labels due to operator-side issues like
			// authentication or network policy. Stale slave labels are left alone.
			if pod.Labels[common.RedisRoleLabelKey] == common.RedisRoleLabelMaster {
				if isConnectivityError(err) {
					unreachableMasterPods = append(unreachableMasterPods, pod.Name)
				} else {
					log.FromContext(ctx).Info("keeping master role label, the probe failed without evidence that the pod is unreachable",
						"pod", pod.Name,
					)
				}
			}
			continue
		}
		role := common.RedisRoleLabelSlave
		if isMaster {
			role = common.RedisRoleLabelMaster
			// The pod the caller identified as the master needs no further proof: a
			// survivor promoted by Sentinel after its only peer was lost has no replica
			// left to attach, yet it is the master and the lost peer's label is stale.
			if !masterConfirmed && pod.Name == masterPod {
				masterConfirmed = true
			}
			if !masterConfirmed {
				replicas, rErr := redisService.GetAttachedReplicaCount(ctx)
				switch {
				case rErr != nil:
					if ctxErr := ctx.Err(); ctxErr != nil {
						return ctxErr
					}
					log.FromContext(ctx).V(1).Info("failed to count attached replicas, not using the pod as evidence of a stale master",
						"pod", pod.Name,
						"error", rErr,
					)
				case replicas > 0:
					masterConfirmed = true
				}
			}
		}
		if oldRole := pod.Labels[common.RedisRoleLabelKey]; oldRole != role {
			patch := []byte(fmt.Sprintf(`[{"op": "add", "path": "/metadata/labels/%s", "value": "%s"}]`, common.RedisRoleLabelKey, role))
			rErr := retry.RetryOnConflict(retry.DefaultRetry, patchFunc(pod.Name, patch))
			if rErr != nil {
				return fmt.Errorf("failed to update pod role label: %w", rErr)
			}
			log.FromContext(ctx).Info("updated pod role label",
				"pod", pod.Name,
				"oldRole", oldRole,
				"newRole", role,
			)
		}
	}
	if len(unreachableMasterPods) == 0 {
		return nil
	}
	if !masterConfirmed {
		log.FromContext(ctx).Info("keeping master role label of unreachable pods, no other pod answered as the identified master or as a master with attached replicas",
			"pods", unreachableMasterPods,
			"identifiedMaster", masterPod,
		)
		return nil
	}
	var removeErrs []error
	for _, podName := range unreachableMasterPods {
		rErr := retry.RetryOnConflict(retry.DefaultRetry, removeRoleLabelFunc(podName))
		switch {
		case rErr == nil:
			log.FromContext(ctx).Info("removed stale master role label after probe failure",
				"pod", podName,
			)
		case apierrors.IsNotFound(rErr):
			log.FromContext(ctx).V(1).Info("pod was deleted before its stale master role label could be removed",
				"pod", podName,
			)
		default:
			removeErrs = append(removeErrs, fmt.Errorf("failed to remove stale master role label from pod %s: %w", podName, rErr))
		}
	}
	return errors.Join(removeErrs...)
}

// isConnectivityError reports whether err means the pod was unreachable.
// Redis server errors, TLS alerts, and cert failures prove the pod responded,
// so they return false. Unrecognized errors are treated as inconclusive.
func isConnectivityError(err error) bool {
	if err == nil {
		return false
	}
	var serverErr rediscli.Error
	if errors.As(err, &serverErr) {
		return false
	}
	var (
		certErr             *tls.CertificateVerificationError
		recordErr           tls.RecordHeaderError
		unknownAuthorityErr x509.UnknownAuthorityError
		hostnameErr         x509.HostnameError
		certInvalidErr      x509.CertificateInvalidError
	)
	if errors.As(err, &certErr) || errors.As(err, &recordErr) ||
		errors.As(err, &unknownAuthorityErr) || errors.As(err, &hostnameErr) ||
		errors.As(err, &certInvalidErr) {
		return false
	}
	// TLS alerts surface as OpError with Op "remote error"/"local error" and
	// mean the pod answered; any other Op (dial, read, write) is a real failure.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op != "remote error" && opErr.Op != "local error"
	}
	// Bare context errors belong to the reconcile, not the pod.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, io.EOF) ||
		errors.Is(err, os.ErrDeadlineExceeded)
}

func (h *healer) SentinelSet(ctx context.Context, rs *rsvb2.RedisSentinel, master string) error {
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

		for k, v := range map[string]string{
			"down-after-milliseconds": rs.Spec.RedisSentinelConfig.DownAfterMilliseconds,
			"parallel-syncs":          rs.Spec.RedisSentinelConfig.ParallelSyncs,
			"failover-timeout":        rs.Spec.RedisSentinelConfig.FailoverTimeout,
		} {
			if v == "" {
				continue
			}
			err = h.redis.Connect(connInfo).SentinelSet(ctx, rs.Spec.RedisSentinelConfig.MasterGroupName, k, v)
			if err != nil {
				return err
			}
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

func tlsKeyOrDefault(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

func getTLSSecretKeys(tlsConfig *commonapi.TLSConfig) (caFile, certFile, keyFile string) {
	caFile = "ca.crt"
	certFile = "tls.crt"
	keyFile = "tls.key"
	if tlsConfig == nil {
		return caFile, certFile, keyFile
	}
	caFile = tlsKeyOrDefault(tlsConfig.CaCertFile, caFile)
	certFile = tlsKeyOrDefault(tlsConfig.CertKeyFile, certFile)
	keyFile = tlsKeyOrDefault(tlsConfig.KeyFile, keyFile)
	return caFile, certFile, keyFile
}

// getRedisTLSConfig creates a TLS configuration for Redis connections
func getRedisTLSConfig(ctx context.Context, client kubernetes.Interface, namespace string, tlsConfig *commonapi.TLSConfig) *tls.Config {
	if tlsConfig == nil || tlsConfig.Secret.SecretName == "" {
		return nil
	}

	tlsSecretName := tlsConfig.Secret.SecretName
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, tlsSecretName, metav1.GetOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed in getting TLS secret", "secretName", tlsSecretName, "namespace", namespace)
		return nil
	}

	caFile, certFile, keyFile := getTLSSecretKeys(tlsConfig)
	tlsClientCert, certExists := secret.Data[certFile]
	tlsClientKey, keyExists := secret.Data[keyFile]
	tlsCACert, caExists := secret.Data[caFile]

	if !certExists || !keyExists {
		log.FromContext(ctx).Error(fmt.Errorf("TLS secret missing required cert/key"), "TLS secret is missing required cert/key", "secretName", tlsSecretName)
		return nil
	}

	cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to load TLS key pair", "secretName", tlsSecretName)
		return nil
	}

	if !caExists && tlsConfig.CaCertFile != "" {
		log.FromContext(ctx).Error(fmt.Errorf("configured TLS CA key file is missing in the secret"), "TLS secret is missing configured CA key", "secretName", tlsSecretName, "caKeyFile", tlsConfig.CaCertFile)
		return nil
	}

	if !caExists {
		log.FromContext(ctx).V(1).Info("CA certificate not found in TLS secret, using system trust store", "secretName", tlsSecretName)
		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to load system certificate pool", "secretName", tlsSecretName)
			return nil
		}
		return newTLSConfigVerifyingChainWithoutHostname(cert, systemCertPool)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(tlsCACert)

	return newTLSConfigVerifyingChainWithoutHostname(cert, caCertPool)
}

// newTLSConfigVerifyingChainWithoutHostname builds a client *tls.Config that
// trusts the supplied root pool and verifies the peer certificate chain WITHOUT
// checking the server name. The operator dials Redis pods by IP / pod DNS that
// does not match the server certificate SNI, so Go's default hostname
// verification is disabled (InsecureSkipVerify) and the chain is instead
// validated by VerifyPeerCertificate against the provided roots.
func newTLSConfigVerifyingChainWithoutHostname(cert tls.Certificate, rootCAs *x509.CertPool) *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            rootCAs,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // skips default verification; chain re-verified without hostname below
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			_, _, err := cryptutil.VerifyCertificateExceptServerName(rawCerts, &tls.Config{RootCAs: rootCAs})
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
		connInfo.Host = fmt.Sprintf("%s.%s.%s.svc.%s", pod.Name, serviceName, namespace, envs.GetServiceDNSDomain())
		// Get TLS configuration
		tlsCfg := getRedisTLSConfig(ctx, k8sClient, namespace, tlsConfig)
		connInfo.TLSConfig = tlsCfg
	}

	return connInfo
}
