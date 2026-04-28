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

package redis

import (
	"context"
	"reflect"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	RedisFinalizer = "redisFinalizer"
)

// Reconciler reconciles a Redis object
type Reconciler struct {
	client.Client
	k8sutils.StatefulSet
	K8sClient kubernetes.Interface
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &rvb2.Redis{}

	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueECheck(ctx, err, "failed to get redis instance")
	}
	if instance.GetDeletionTimestamp() != nil {
		if err = k8sutils.HandleRedisFinalizer(ctx, r.Client, instance, RedisFinalizer); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to handle redis finalizer")
		}
		return intctrlutil.Reconciled()
	}
	if common.ShouldSkipReconcile(ctx, instance) {
		return intctrlutil.Reconciled()
	}
	if err = k8sutils.AddFinalizer(ctx, instance, RedisFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to add finalizer")
	}

	// Mark as Initializing until the StatefulSet is ready.
	if !r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name) {
		if err = r.updateStatus(ctx, instance, rvb2.RedisStatus{
			State:  rvb2.RedisInitializing,
			Reason: rvb2.InitializingRedisReason,
		}); err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to update redis status")
		}
	}

	err = k8sutils.CreateStandaloneRedis(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to create redis")
	}
	err = k8sutils.CreateStandaloneService(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to create service")
	}

	// Update status to Ready or Failed based on StatefulSet readiness.
	var newStatus rvb2.RedisStatus
	if r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name) {
		newStatus = rvb2.RedisStatus{State: rvb2.RedisReady, Reason: rvb2.ReadyRedisReason}
	} else {
		newStatus = rvb2.RedisStatus{State: rvb2.RedisInitializing, Reason: rvb2.InitializingRedisReason}
	}
	if err = r.updateStatus(ctx, instance, newStatus); err != nil {
		return intctrlutil.RequeueE(ctx, err, "failed to update redis status")
	}

	return intctrlutil.Reconciled()
}

func (r *Reconciler) updateStatus(ctx context.Context, redis *rvb2.Redis, status rvb2.RedisStatus) error {
	if reflect.DeepEqual(redis.Status, status) {
		return nil
	}
	copy := redis.DeepCopy()
	copy.Status = status
	err := common.UpdateStatus(ctx, r.Client, copy)
	if err != nil && apierrors.IsConflict(err) {
		log.FromContext(ctx).Info("conflict detected, reloading instance and retrying status update")
		if err := r.Get(ctx, client.ObjectKey{Namespace: redis.Namespace, Name: redis.Name}, redis); err != nil {
			return err
		}
		copy = redis.DeepCopy()
		copy.Status = status
		return common.UpdateStatus(ctx, r.Client, copy)
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
//
// Unlike RedisCluster, RedisReplication, and RedisSentinel controllers, the Redis standalone
// controller does not require periodic requeue. Those controllers need timed requeues to
// continuously monitor cluster topology, replication health, slot distribution, and sentinel
// readiness — state that can change independently of Kubernetes resource events. The standalone
// controller only creates a StatefulSet and a Service with no ongoing distributed state to poll,
// so a timed requeue is unnecessary.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rvb2.Redis{}).
		WithOptions(opts).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
