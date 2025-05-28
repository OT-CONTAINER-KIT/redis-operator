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
	"time"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	intctrlutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/controllerutil"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/k8sutils"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// Reconciler reconciles a Redis object
type Reconciler struct {
	client.Client
	K8sClient kubernetes.Interface
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &rvb2.Redis{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		return intctrlutil.RequeueWithErrorChecking(ctx, err, "failed to get redis instance")
	}
	if instance.ObjectMeta.GetDeletionTimestamp() != nil {
		if err = k8sutils.HandleRedisFinalizer(ctx, r.Client, instance); err != nil {
			return intctrlutil.RequeueWithError(ctx, err, "failed to handle redis finalizer")
		}
		return intctrlutil.Reconciled()
	}
	if common.IsSkipReconcile(ctx, instance) {
		return intctrlutil.Reconciled()
	}
	if err = k8sutils.AddFinalizer(ctx, instance, k8sutils.RedisFinalizer, r.Client); err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "failed to add finalizer")
	}
	err = k8sutils.CreateStandaloneRedis(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "failed to create redis")
	}
	err = k8sutils.CreateStandaloneService(ctx, instance, r.K8sClient)
	if err != nil {
		return intctrlutil.RequeueWithError(ctx, err, "failed to create service")
	}
	return intctrlutil.RequeueAfter(ctx, time.Second*10, "requeue after 10 seconds")
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rvb2.Redis{}).
		WithOptions(opts).
		Complete(r)
}
