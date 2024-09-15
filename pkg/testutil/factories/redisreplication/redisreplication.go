package redisreplication

import (
	"github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type customFieldOption func(replication *v1beta2.RedisReplication)

func WithAnnotations(annotations map[string]string) customFieldOption {
	return func(rc *v1beta2.RedisReplication) {
		rc.ObjectMeta.Annotations = annotations
	}
}

func WithIgnoredKeys(keys []string) customFieldOption {
	return func(rc *v1beta2.RedisReplication) {
		rc.Spec.KubernetesConfig.IgnoreAnnotations = keys
	}
}

func New(name string, options ...customFieldOption) *v1beta2.RedisReplication {
	size := int32(3)
	rr := &v1beta2.RedisReplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redis.redis.opstreelabs.in/v1beta2",
			Kind:       "RedisReplication",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1beta2.RedisReplicationSpec{
			Size:    &size,
			Storage: &v1beta2.Storage{},
		},
	}
	for _, option := range options {
		option(rr)
	}
	return rr
}
