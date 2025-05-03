package redis

import (
	"github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type customFieldOption func(*v1beta2.Redis)

func WithAnnotations(annotations map[string]string) customFieldOption {
	return func(rc *v1beta2.Redis) {
		rc.ObjectMeta.Annotations = annotations
	}
}

func WithIgnoredKeys(keys []string) customFieldOption {
	return func(rc *v1beta2.Redis) {
		rc.Spec.KubernetesConfig.IgnoreAnnotations = keys
	}
}

func New(name string, options ...customFieldOption) *v1beta2.Redis {
	rr := &v1beta2.Redis{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redisv1beta2/apiVersion",
			Kind:       "Redis",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
	for _, option := range options {
		option(rr)
	}
	return rr
}
