/*
Copyright 2022 Lance Yuan.

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
	"github.com/go-logr/logr"
	devopsv1 "k8s-crd-demo/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName string = "caddy-controller"
)

var logger logr.Logger

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=devops.codepy.net,resources=apps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=devops.codepy.net,resources=apps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=devops.codepy.net,resources=apps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the App object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger = log.FromContext(ctx)

	instance := &devopsv1.App{}
	// TODO(user): your logic here
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("get App not found!!!!!")
			return ctrl.Result{}, nil
		}
		logger.Info("get App error!!!!!")
		return reconcile.Result{}, err
	}
	deployment := &appsv1.Deployment{}
	svc := &corev1.Service{}
	reqNamespaceName := req.NamespacedName
	reqNamespaceName.Name = controllerName
	if err := r.Client.Get(ctx, reqNamespaceName, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		deployment = NewDeployment(instance)
		if err := r.Client.Create(ctx, deployment); err != nil {
			return ctrl.Result{}, err
		}
	}
	if err := r.Client.Get(ctx, reqNamespaceName, svc); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		svc = NewService(instance)
		if err := r.Client.Create(ctx, svc); err != nil {
			return ctrl.Result{}, err
		}
	}
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("start delete ingress....")
		ingress := &networkingv1.Ingress{}
		if err := r.Client.Get(ctx, req.NamespacedName, ingress); err != nil {
			if errors.IsNotFound(err) {
				instance.ObjectMeta.Finalizers = []string{}
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
				return reconcile.Result{}, nil
			}
			return ctrl.Result{}, err
		} else {
			if err := r.Client.Delete(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			} else {
				instance.ObjectMeta.Finalizers = []string{}
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
		return reconcile.Result{}, nil
	}
	ingress := &networkingv1.Ingress{}
	if err := r.Client.Get(ctx, req.NamespacedName, ingress); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		logger.Info("add caddy route !!!!!")
		if err := AddCaddyRoute(instance); err != nil {
			logger.Info("add caddy route error !!!!!")
			return ctrl.Result{}, err
		}
		ingress = NewIngress(instance)
		logger.Info("create ingress !!!!!")
		if err := r.Client.Create(ctx, ingress); err != nil {
			return ctrl.Result{}, err
		} else {
			logger.Info("update App finalizer !!!!!")
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, instance.Name)
			if err := r.Client.Update(ctx, instance); err != nil {
				logger.Info("update finalizer err !!!!!")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	} else {
		logger.Info("get Ingress exist !!!!!")
		newIngress := NewIngress(instance)
		if !reflect.DeepEqual(newIngress.Spec, ingress.Spec) {
			logger.Info("update Ingress !!!!!")
			if err := r.Client.Update(ctx, newIngress); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	//if err := controllerutil.SetControllerReference(instance, ingress, r.Scheme); err != nil {
	//	logger.Info("sync ingress error....")
	//	return ctrl.Result{}, err
	//}
}
func (r *AppReconciler) DeleteIngress(event event.DeleteEvent, limiter workqueue.RateLimitingInterface) {
	name := event.Object.GetName()
	namespace := event.Object.GetNamespace()
	instance := &devopsv1.App{}
	reqNamespaceName := types.NamespacedName{Name: name, Namespace: namespace}
	if err := r.Get(context.TODO(), reqNamespaceName, instance); err != nil {
		logger.Info(err.Error())
	} else {
		if err := r.Delete(context.TODO(), instance); err != nil {
			logger.Info(err.Error())
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devopsv1.App{}).
		Watches(&source.Kind{
			Type: &networkingv1.Ingress{}},
			handler.Funcs{DeleteFunc: r.DeleteIngress}).
		Complete(r)
}
