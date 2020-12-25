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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	redisv1 "redis-sentinel/api/v1"
)

// RedisSentinelReconciler reconciles a RedisSentinel object
type RedisSentinelReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=redis.kdcloud.io,resources=redissentinels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.kdcloud.io,resources=redissentinels/status,verbs=get;update;patch

func (r *RedisSentinelReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("redissentinel", req.NamespacedName)
	reqLogger.Info("begin Reconcile")

	// your logic here
	instance := &redisv1.RedisSentinel{}

	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		reqLogger.Info("sts Get", "error",err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	statefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name,
			Namespace: req.Namespace,
			Annotations: map[string]string{
				"foo": instance.Spec.Foo,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &instance.Spec.Size,
		},
	}

	err = r.Client.Update(ctx, statefulSet)
	if err != nil {
		reqLogger.Info("sts Update","error", err)
		return reconcile.Result{}, err
	}
	reqLogger.Info("end Reconcile")
	return ctrl.Result{RequeueAfter: 50 * time.Second}, nil
}


func (r *RedisSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1.RedisSentinel{}).
		Complete(r)
}
