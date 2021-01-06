/*


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
	"fmt"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	redisv1 "redis-sentinel/api/v1"
)


const ReconcileTime = 60 * time.Second

var (
	controllerFlagSet *pflag.FlagSet
	// maxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run. Defaults to 4.
	maxConcurrentReconciles int
	// reconcileTime is the delay between reconciliations. Defaults to 60s.
	reconcileTime int

	)


func init() {
	controllerFlagSet = pflag.NewFlagSet("controller", pflag.ExitOnError)
	controllerFlagSet.IntVar(&maxConcurrentReconciles, "ctr-maxconcurrent", 4, "the maximum number of concurrent Reconciles which can be run. Defaults to 4.")
	controllerFlagSet.IntVar(&reconcileTime, "ctr-reconciletime", 60, "")
}


// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	handler *RedisSentinelHandler
}

// +kubebuilder:rbac:groups=redis.kdcloud.io,resources=redissentinels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.kdcloud.io,resources=redissentinels/status,verbs=get;update;patch

func (r *RedisSentinelReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("redissentinel", req.NamespacedName)
	reqLogger.Info("begin Reconcile")

	// your logic here
	instance := &redisv1.RedisSentinel{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("RedisSentinel delete", "error", err)
			instance.Namespace = req.NamespacedName.Namespace
			instance.Name = req.NamespacedName.Name
			r.handler.MetaCache.Del(instance)
		}
		reqLogger.Info("Get RedisSentinel", "error", err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	reqLogger.Info(fmt.Sprintf("RedisSentinel Spec:\n %+v", instance))
	if err := r.handler.Do(instance); err != nil {
		if err.Error() == needRequeueMsg {
			return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
		}
		reqLogger.Error(err, "Reconcile handler")
		return reconcile.Result{}, err
	}
	if err := r.handler.rcChecker.CheckSentinelReadyReplicas(instance); err != nil {
		reqLogger.Info(err.Error())
		return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
	}
	reqLogger.Info("end Reconcile ,requeue after 60 second")
	return reconcile.Result{RequeueAfter: time.Duration(reconcileTime) * time.Second}, nil
}

func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1.RedisSentinel{}).
		Complete(r)
}
