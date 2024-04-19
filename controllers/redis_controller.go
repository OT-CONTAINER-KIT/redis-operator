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

package controllers

import (
	"context"
	"time"

	redisv1beta2 "github.com/teocns/redis-operator/api/v1beta2"
	"github.com/teocns/redis-operator/k8sutils"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	K8sClient  kubernetes.Interface
	Dk8sClient dynamic.Interface
	Log        logr.Logger
	Scheme     *runtime.Scheme
}

func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling opstree redis controller")
	instance := &redisv1beta2.Redis{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if _, found := instance.ObjectMeta.GetAnnotations()["redis.opstreelabs.in/skip-reconcile"]; found {
		reqLogger.Info("Found annotations redis.opstreelabs.in/skip-reconcile, so skipping reconcile")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
	if err = k8sutils.HandleRedisFinalizer(r.Client, r.K8sClient, r.Log, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err = k8sutils.AddFinalizer(instance, k8sutils.RedisFinalizer, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	err = k8sutils.CreateStandaloneRedis(instance, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateStandaloneService(instance, r.K8sClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	reqLogger.Info("Will reconcile redis operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta2.Redis{}).
		Complete(r)
}
