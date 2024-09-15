package rediscluster

import (
	"github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type customFieldOption func(*v1beta2.RedisCluster)

func DisablePersistence() customFieldOption {
	return func(rc *v1beta2.RedisCluster) {
		rc.Spec.PersistenceEnabled = new(bool)
	}
}

func WithAnnotations(annotations map[string]string) customFieldOption {
	return func(rc *v1beta2.RedisCluster) {
		rc.ObjectMeta.Annotations = annotations
	}
}

func WithIgnoredKeys(keys []string) customFieldOption {
	return func(rc *v1beta2.RedisCluster) {
		rc.Spec.KubernetesConfig.IgnoreAnnotations = keys
	}
}

func New(name string, options ...customFieldOption) *v1beta2.RedisCluster {
	size := int32(3)
	rc := &v1beta2.RedisCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redis.redis.opstreelabs.in/v1beta2",
			Kind:       "RedisCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1beta2.RedisClusterSpec{
			Size:    &size,
			Storage: &v1beta2.ClusterStorage{},
		},
	}
	for _, option := range options {
		option(rc)
	}
	return rc
}
