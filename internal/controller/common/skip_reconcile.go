package common

import (
	"context"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/monitoring"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	RedisClusterSkipReconcileAnnotation     = "rediscluster.opstreelabs.in/skip-reconcile"
	RedisSkipReconcileAnnotation            = "redis.opstreelabs.in/skip-reconcile"
	RedisReplicationSkipReconcileAnnotation = "redisreplication.opstreelabs.in/skip-reconcile"
	RedisSentinelSkipReconcileAnnotation    = "redissentinel.opstreelabs.in/skip-reconcile"
)

func ShouldSkipReconcile(ctx context.Context, obj metav1.Object) (skip bool) {
	defer func() {
		if skip {
			log.FromContext(ctx).Info("found skip reconcile annotation", "namespace", obj.GetNamespace(), "name", obj.GetName())
		}
	}()
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	switch obj.(type) {
	case *rcvb2.RedisCluster:
		monitoring.RedisClusterSkipReconcile.WithLabelValues(obj.GetNamespace(), obj.GetName()).Set(0)
		if value, found := annotations[RedisClusterSkipReconcileAnnotation]; found && value == "true" {
			monitoring.RedisClusterSkipReconcile.WithLabelValues(obj.GetNamespace(), obj.GetName()).Set(1)
			return true
		}
	case *rvb2.Redis:
		if value, found := annotations[RedisSkipReconcileAnnotation]; found && value == "true" {
			return true
		}
	case *rrvb2.RedisReplication:
		monitoring.RedisReplicationSkipReconcile.WithLabelValues(obj.GetNamespace(), obj.GetName()).Set(0)
		if value, found := annotations[RedisReplicationSkipReconcileAnnotation]; found && value == "true" {
			monitoring.RedisReplicationSkipReconcile.WithLabelValues(obj.GetNamespace(), obj.GetName()).Set(1)
			return true
		}
	case *rsvb2.RedisSentinel:
		if value, found := annotations[RedisSentinelSkipReconcileAnnotation]; found && value == "true" {
			return true
		}
	}
	return false
}
