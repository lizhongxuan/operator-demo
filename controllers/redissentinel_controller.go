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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	redisv1 "redis-sentinel/api/v1"
	corev1 "k8s.io/api/core/v1"
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
	if err := r.Client.Get(ctx, req.NamespacedName, instance) ;err != nil {
		reqLogger.Info("RedisSentinel Get", "error",err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}



	sts := &appsv1.StatefulSet{}
	if err := r.Client.Get(ctx,req.NamespacedName,sts);err != nil {
		reqLogger.Error(err,"sts Get")
		if errors.IsNotFound(err) {  // 没找到,创建新的
			reqLogger.Info("sts create")
			statefulSet := &appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: req.Name,
					Namespace: req.Namespace,
					Annotations: map[string]string{
						"foo-create": instance.Spec.Foo,
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &instance.Spec.Size,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":req.Name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":req.Name,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: req.Name,
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			if err := r.Client.Create(ctx,statefulSet);err != nil {
				reqLogger.Error(err,"sts Create error")
				return ctrl.Result{}, nil
			}
		}
	}

	reqLogger.Info("sts update")
	sts.Spec.Replicas = &instance.Spec.Size
	sts.Annotations = map[string]string{
		"foo-update": instance.Spec.Foo,
	}
	if err := r.Client.Update(ctx, sts);err != nil {
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
