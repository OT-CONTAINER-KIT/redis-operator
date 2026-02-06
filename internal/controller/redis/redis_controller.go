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
	"time"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	if instance.Status.State == "" || instance.Status.State == rvb2.RedisFailed {
		requeue, err := r.updateStatus(ctx, instance, rvb2.RedisStatus{
			State:  rvb2.RedisInitializing,
			Reason: rvb2.InitializingReason,
		})
		if err != nil {
			return intctrlutil.RequeueE(ctx, err, "failed to update status to initializing")
		}
		if requeue {
			return intctrlutil.Requeue()
		}
	}

	err = k8sutils.CreateStandaloneRedis(ctx, instance, r.K8sClient)
	if err != nil {
		if _, statusErr := r.updateStatus(ctx, instance, rvb2.RedisStatus{
			State:  rvb2.RedisFailed,
			Reason: rvb2.FailedReason,
		}); statusErr != nil {
			return intctrlutil.RequeueE(ctx, statusErr, "failed to update status to failed")
		}
		return intctrlutil.RequeueE(ctx, err, "failed to create redis")
	}

	err = k8sutils.CreateStandaloneService(ctx, instance, r.K8sClient)
	if err != nil {
		if _, statusErr := r.updateStatus(ctx, instance, rvb2.RedisStatus{
			State:  rvb2.RedisFailed,
			Reason: rvb2.FailedReason,
		}); statusErr != nil {
			return intctrlutil.RequeueE(ctx, statusErr, "failed to update status to failed")
		}
		return intctrlutil.RequeueE(ctx, err, "failed to create service")
	}

	if r.IsStatefulSetReady(ctx, instance.Namespace, instance.Name) {
		if instance.Status.State != rvb2.RedisReady {
			requeue, err := r.updateStatus(ctx, instance, rvb2.RedisStatus{
				State:  rvb2.RedisReady,
				Reason: rvb2.ReadyReason,
			})
			if err != nil {
				return intctrlutil.RequeueE(ctx, err, "failed to update status to ready")
			}
			if requeue {
				return intctrlutil.Requeue()
			}
		}
		return intctrlutil.Reconciled()
	}

	return intctrlutil.RequeueAfter(ctx, time.Second*10, "StatefulSet not ready, requeue after 10 seconds")
}

func (r *Reconciler) updateStatus(ctx context.Context, instance *rvb2.Redis, status rvb2.RedisStatus) (requeue bool, err error) {
	if reflect.DeepEqual(instance.Status, status) {
		return false, nil
	}
	copy := instance.DeepCopy()
	copy.Spec = rvb2.RedisSpec{}
	copy.Status = status
	err = common.UpdateStatus(ctx, r.Client, copy)
	if err != nil && apierrors.IsConflict(err) {
		log.FromContext(ctx).Info("conflict detected, reloading instance and retrying status update")
		namespacedName := client.ObjectKey{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		}
		if err := r.Get(ctx, namespacedName, instance); err != nil {
			return true, err
		}
		copy = instance.DeepCopy()
		copy.Spec = rvb2.RedisSpec{}
		copy.Status = status
		return true, common.UpdateStatus(ctx, r.Client, copy)
	}
	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rvb2.Redis{}).
		WithOptions(opts).
		Complete(r)
}
