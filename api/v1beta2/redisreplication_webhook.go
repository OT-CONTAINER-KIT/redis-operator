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
)

// log is for logging in this package.
var redisreplicationlog = logf.Log.WithName("redisreplication-resource")

var (
	redisReplicationCreateChecks = []func(*RedisReplication) field.ErrorList{
		redisReplicationCheckCommonKubernetesConfig,
		redisReplicationCheckCommonRedisExporter,
	}
	redisReplicationUpdateChecks = []func(old, curr *RedisReplication) field.ErrorList{}
)

func (r *RedisReplication) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&RedisReplicationWebhook{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/validate-redis-redis-opstreelabs-in-v1beta2-redisreplication,mutating=false,failurePolicy=ignore,groups=redis.redis.opstreelabs.in,resources=redisreplications,verbs=create;update;delete,versions=v1beta2,name=redisreplication-validation-v1beta2.redis.redis.opstreelabs.in,sideEffects=None,admissionReviewVersions=v1;v1beta1,matchPolicy=Exact

type RedisReplicationWebhook struct{}

func (r *RedisReplicationWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	redisreplicationlog.Info("validate create", "name", obj.(*RedisReplication).Name)
	return r.validate(ctx, nil, obj)
}

func (r *RedisReplicationWebhook) ValidateUpdate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	redisreplicationlog.Info("validate update", "name", curr.(*RedisReplication).Name)
	return r.validate(ctx, old, curr)
}

func (r *RedisReplicationWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	redisreplicationlog.Info("validate delete", "name", obj.(*RedisReplication).Name)
	return nil
}

func (r *RedisReplicationWebhook) validate(ctx context.Context, old runtime.Object, curr runtime.Object) error {
	var errors field.ErrorList

	if old != nil {
		oldRedisReplication := old.(*RedisReplication)
		for _, check := range redisReplicationUpdateChecks {
			errors = append(errors, check(oldRedisReplication, curr.(*RedisReplication))...)
		}
	}

	newRedisReplication := curr.(*RedisReplication)
	for _, check := range redisReplicationCreateChecks {
		errors = append(errors, check(newRedisReplication)...)
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("RedisReplication").GroupKind(), newRedisReplication.Name, errors)
}

func redisReplicationCheckCommonKubernetesConfig(rc *RedisReplication) field.ErrorList {
	return common.CheckCommonKubernetesConfig(rc.Spec.KubernetesConfig.KubernetesConfig)
}

func redisReplicationCheckCommonRedisExporter(rc *RedisReplication) field.ErrorList {
	if rc.Spec.RedisExporter == nil {
		return nil
	}
	return common.CheckCommonRedisExporter(rc.Spec.RedisExporter.RedisExporter)
}
