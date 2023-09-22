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
var redislog = logf.Log.WithName("redis-resource")

var (
	redisCreateChecks = []func(*Redis) field.ErrorList{
		redisCheckCommonKubernetesConfig,
		redisCheckCommonRedisExporter,
	}
	redisUpdateChecks = []func(old, curr *Redis) field.ErrorList{}
)

func (r *Redis) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&RedisWebhook{}).
		Complete()
}

type RedisWebhook struct{}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/validate-redis-redis-opstreelabs-in-v1beta2-redis,mutating=false,failurePolicy=ignore,groups=redis.redis.opstreelabs.in,resources=rediss,verbs=create;update;delete,versions=v1beta2,name=redis-validation-v1beta2.redis.redis.opstreelabs.in,sideEffects=None,admissionReviewVersions=v1;v1beta1,matchPolicy=Exact

var _ webhook.CustomValidator = &RedisWebhook{}

func (r *RedisWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	redislog.Info("validate create", "name", obj.(*Redis).Name)
	return r.validate(ctx, nil, obj)
}

func (r *RedisWebhook) ValidateUpdate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	redislog.Info("validate update", "name", curr.(*Redis).Name)
	return r.validate(ctx, old, curr)
}

func (r *RedisWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	redislog.Info("validate delete", "name", obj.(*Redis).Name)
	return nil
}

func (r *RedisWebhook) validate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	var errors field.ErrorList

	if old != nil {
		oldRedis := old.(*Redis)
		for _, check := range redisUpdateChecks {
			errors = append(errors, check(oldRedis, curr.(*Redis))...)
		}
	}

	newRedis := curr.(*Redis)
	for _, check := range redisCreateChecks {
		errors = append(errors, check(newRedis)...)
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("Redis").GroupKind(), newRedis.Name, errors)
}

func redisCheckCommonKubernetesConfig(rc *Redis) field.ErrorList {
	return common.CheckCommonKubernetesConfig(rc.Spec.KubernetesConfig.KubernetesConfig)
}

func redisCheckCommonRedisExporter(rc *Redis) field.ErrorList {
	if rc.Spec.RedisExporter == nil {
		return nil
	}
	return common.CheckCommonRedisExporter(rc.Spec.RedisExporter.RedisExporter)
}
