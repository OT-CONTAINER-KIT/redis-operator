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
var redissentinellog = logf.Log.WithName("redissentinel-resource")

var (
	redisSentinelCreateChecks = []func(*RedisSentinel) field.ErrorList{
		redisSentinelCheckCommonKubernetesConfig,
		redisSentinelCheckCommonRedisExporter,
	}
	redisSentinelUpdateChecks = []func(old, curr *RedisSentinel) field.ErrorList{}
)

func (r *RedisSentinel) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&RedisSentinelWebhook{}).
		Complete()
}

type RedisSentinelWebhook struct{}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/validate-redis-redis-opstreelabs-in-v1beta2-redissentinel,mutating=false,failurePolicy=ignore,groups=redis.redis.opstreelabs.in,resources=redissentinels,verbs=create;update;delete,versions=v1beta2,name=redissentinel-validation-v1beta2.redis.redis.opstreelabs.in,sideEffects=None,admissionReviewVersions=v1;v1beta1,matchPolicy=Exact

var _ webhook.CustomValidator = &RedisSentinelWebhook{}

func (r *RedisSentinelWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	redissentinellog.Info("validate create", "name", obj.(*RedisSentinel).Name)
	return r.validate(ctx, nil, obj)
}

func (r *RedisSentinelWebhook) ValidateUpdate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	redissentinellog.Info("validate update", "name", curr.(*RedisSentinel).Name)
	return r.validate(ctx, old, curr)
}

func (r *RedisSentinelWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	redissentinellog.Info("validate delete", "name", obj.(*RedisSentinel).Name)
	return nil
}

func (r *RedisSentinelWebhook) validate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	var errors field.ErrorList

	if old != nil {
		oldRedisSentinel := old.(*RedisSentinel)
		for _, check := range redisSentinelUpdateChecks {
			errors = append(errors, check(oldRedisSentinel, curr.(*RedisSentinel))...)
		}
	}

	newRedisSentinel := curr.(*RedisSentinel)
	for _, check := range redisSentinelCreateChecks {
		errors = append(errors, check(newRedisSentinel)...)
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("RedisSentinel").GroupKind(), newRedisSentinel.Name, errors)
}

func redisSentinelCheckCommonKubernetesConfig(rc *RedisSentinel) field.ErrorList {
	return common.CheckCommonKubernetesConfig(rc.Spec.KubernetesConfig.KubernetesConfig)
}

func redisSentinelCheckCommonRedisExporter(rc *RedisSentinel) field.ErrorList {
	if rc.Spec.RedisExporter == nil {
		return nil
	}
	return common.CheckCommonRedisExporter(rc.Spec.RedisExporter.RedisExporter)
}
