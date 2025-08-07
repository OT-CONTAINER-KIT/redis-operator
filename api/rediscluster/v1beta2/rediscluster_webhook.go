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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	webhookPath = "/validate-redis-redis-opstreelabs-in-v1beta2-rediscluster"
)

// log is for logging in this package.
var redisclusterlog = logf.Log.WithName("rediscluster-v1beta2-validation")

// +kubebuilder:webhook:path=/validate-redis-redis-opstreelabs-in-v1beta2-rediscluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=redis.redis.opstreelabs.in,resources=redisclusters,verbs=create;update,versions=v1beta2,name=validate-rediscluster.redis.opstreelabs.in,admissionReviewVersions=v1

// SetupWebhookWithManager will setup the manager
func (r *RedisCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Validator = &RedisCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *RedisCluster) ValidateCreate() (admission.Warnings, error) {
	redisclusterlog.Info("validate create", "name", r.Name)

	return r.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *RedisCluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	redisclusterlog.Info("validate update", "name", r.Name)

	return r.validate(old.(*RedisCluster))
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *RedisCluster) ValidateDelete() (admission.Warnings, error) {
	redisclusterlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

// validate validates the Redis Cluster CR
func (r *RedisCluster) validate(_ *RedisCluster) (admission.Warnings, error) {
	var errors field.ErrorList
	var warnings admission.Warnings

	if r.Spec.ClusterSize == nil {
		return warnings, nil
	}

	// Check if the Size is at least 3 for proper cluster operation
	if *r.Spec.ClusterSize < 3 {
		errors = append(errors, field.Invalid(
			field.NewPath("spec").Child("clusterSize"),
			*r.Spec.ClusterSize,
			"Redis cluster must have at least 3 shards",
		))
	}

	if len(errors) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "redis.redis.opstreelabs.in", Kind: "RedisCluster"},
		r.Name,
		errors,
	)
}

func (r *RedisCluster) WebhookPath() string {
	return webhookPath
}
