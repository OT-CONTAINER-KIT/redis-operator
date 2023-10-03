/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	"context"
	common "github.com/OT-CONTAINER-KIT/redis-operator/api"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var redisclusterlog = logf.Log.WithName("rediscluster-resource")

var (
	redisClusterCreateChecks = []func(*RedisCluster) field.ErrorList{
		redisClusterCheckCommonKubernetesConfig,
		redisClusterCheckCommonRedisExporter,
	}
	redisClusterUpdateChecks = []func(old, curr *RedisCluster) field.ErrorList{}
)

func (r *RedisCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&RedisClusterWebhook{}).
		Complete()
}

type RedisClusterWebhook struct{}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/validate-redis-redis-opstreelabs-in-v1beta2-rediscluster,mutating=false,failurePolicy=ignore,groups=redis.redis.opstreelabs.in,resources=redisclusters,verbs=create;update;delete,versions=v1beta2,name=rediscluster-validation-v1beta2.redis.redis.opstreelabs.in,sideEffects=None,admissionReviewVersions=v1;v1beta1,matchPolicy=Exact

var _ webhook.CustomValidator = &RedisClusterWebhook{}

func (r *RedisClusterWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	redisclusterlog.Info("validate create", "name", obj.(*RedisCluster).Name)
	return r.validate(ctx, nil, obj)
}

func (r *RedisClusterWebhook) ValidateUpdate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	redisclusterlog.Info("validate update", "name", curr.(*RedisCluster).Name)
	return r.validate(ctx, old, curr)
}

func (r *RedisClusterWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	redisclusterlog.Info("validate delete", "name", obj.(*RedisCluster).Name)
	return nil
}

func (r *RedisClusterWebhook) validate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	var errors field.ErrorList

	if old != nil {
		oldRedisCluster := old.(*RedisCluster)
		for _, check := range redisClusterUpdateChecks {
			errors = append(errors, check(oldRedisCluster, curr.(*RedisCluster))...)
		}
	}

	newRedisCluster := curr.(*RedisCluster)
	for _, check := range redisClusterCreateChecks {
		errors = append(errors, check(newRedisCluster)...)
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("RedisCluster").GroupKind(), newRedisCluster.Name, errors)
}

func redisClusterCheckCommonKubernetesConfig(rc *RedisCluster) field.ErrorList {
	return common.CheckCommonKubernetesConfig(rc.Spec.KubernetesConfig.KubernetesConfig)
}

func redisClusterCheckCommonRedisExporter(rc *RedisCluster) field.ErrorList {
	if rc.Spec.RedisExporter == nil {
		return nil
	}
	return common.CheckCommonRedisExporter(rc.Spec.RedisExporter.RedisExporter)
}
