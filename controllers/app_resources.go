package controllers

import (
	devopsv1 "k8s-crd-demo/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewDeployment(app *devopsv1.App) *appsv1.Deployment {
	selector := map[string]string{"app": controllerName}
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerName,
			Namespace: controllerNamespace,
			Labels: selector,
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Replicas: app.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllerName,
					Namespace: controllerNamespace,
					Labels: selector,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: controllerName,
							Image: app.Spec.Image,
							Env: app.Spec.Envs,
							Ports: []corev1.ContainerPort{
								{
									Name: "http",
									Protocol: corev1.ProtocolTCP,
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}
}

func NewService(app *devopsv1.App) *corev1.Service {
	selector := map[string]string{"app": controllerName}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerName,
			Namespace: controllerNamespace,
			Labels: selector,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: app.Spec.Ports,
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}