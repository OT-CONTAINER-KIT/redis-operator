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
	"strconv"
	"time"

	"redis-operator/k8sutils"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Redis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Opstree Redis controller")
	instance := &redisv1beta1.Redis{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(instance, instance, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	found := &appsv1.StatefulSet{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		if instance.Spec.GlobalConfig.Password != nil && instance.Spec.GlobalConfig.ExistingPasswordSecret == nil {
			k8sutils.CreateRedisSecret(instance)
		}
		if instance.Spec.Mode == "cluster" {
			k8sutils.CreateRedisMaster(instance)
			k8sutils.CreateMasterService(instance)
			k8sutils.CreateMasterHeadlessService(instance)
			k8sutils.CreateRedisSlave(instance)
			k8sutils.CreateSlaveService(instance)
			k8sutils.CreateSlaveHeadlessService(instance)
			redisMasterInfo, err := k8sutils.GenerateK8sClient().AppsV1().StatefulSets(instance.Namespace).Get(context.TODO(), instance.ObjectMeta.Name+"-master", metav1.GetOptions{})
			if err != nil {
				return ctrl.Result{}, err
			}
			redisSlaveInfo, err := k8sutils.GenerateK8sClient().AppsV1().StatefulSets(instance.Namespace).Get(context.TODO(), instance.ObjectMeta.Name+"-slave", metav1.GetOptions{})
			if err != nil {
				return ctrl.Result{}, err
			}
			if int(redisMasterInfo.Status.ReadyReplicas) != int(*instance.Spec.Size) && int(redisSlaveInfo.Status.ReadyReplicas) != int(*instance.Spec.Size) {
				reqLogger.Info("Redis master and slave nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisMasterInfo.Status.ReadyReplicas)))
				return ctrl.Result{RequeueAfter: time.Second * 120}, nil
			}
			reqLogger.Info("Creating redis cluster by executing cluster creation command", "Ready.Replicas", strconv.Itoa(int(redisMasterInfo.Status.ReadyReplicas)))
			if k8sutils.CheckRedisNodeCount(instance) != int(*instance.Spec.Size)*2 {
				k8sutils.ExecuteRedisClusterCommand(instance)
				k8sutils.ExecuteRedisReplicationCommand(instance)
			} else {
				reqLogger.Info("Redis master count is desired")
				if k8sutils.CheckRedisClusterState(instance) >= int(*instance.Spec.Size)*2-1 {
					k8sutils.ExecuteFailoverOperation(instance)
				}
				return ctrl.Result{RequeueAfter: time.Second * 120}, nil
			}
		} else if instance.Spec.Mode == "standalone" {
			k8sutils.CreateRedisStandalone(instance)
			k8sutils.CreateStandaloneService(instance)
			k8sutils.CreateStandaloneHeadlessService(instance)
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	reqLogger.Info("Will reconcile in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta1.Redis{}).
		Complete(r)
}
