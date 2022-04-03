package pkg

import (
	"context"
	v13 "k8s.io/api/core/v1"
	v1beta12 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/networking/v1beta1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	networkV1beta1 "k8s.io/client-go/listers/networking/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"reflect"
	"time"
)

type controller struct {
	client        *kubernetes.Clientset
	serviceListen corev1.ServiceLister
	ingressListen networkV1beta1.IngressLister
	queue         workqueue.RateLimitingInterface
}

func (c *controller) Run(stopCh <-chan struct{}) {
	for i := 0; i < 5; i++ {
		go wait.Until(c.worker, time.Minute, stopCh)
	}
	<-stopCh
}

func (c *controller) addService(obj interface{}) {
	key, err := cache.MetaNamespaceIndexFunc(obj)
	if err != nil {
		runtime.HandleError(err)
	}
	c.queue.Add(key)
}

func (c *controller) updateService(oldObj interface{}, newObj interface{}) {
	if !reflect.DeepEqual(oldObj, newObj) {
		key, err := cache.MetaNamespaceIndexFunc(newObj)
		if err != nil {
			runtime.HandleError(err)
		}
		c.queue.Add(key)
	}
}

func (c *controller) deleteIngress(obj interface{}) {
	ingress := obj.(*v1beta12.Ingress)
	ownerReference := v12.GetControllerOf(ingress)

	if ownerReference == nil {
		return
	}
	if ownerReference.Kind != "Service" {
		return
	}

	c.queue.Add(ingress.Namespace + "/" + ingress.Name)
}

func (c *controller) worker() {
	for c.processNextItem() {
	}
}

func (c *controller) processNextItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(item)

	key := item.(string)

	err := c.syncService(key)
	if err != nil {
		return false
	}
	return true
}

func (c *controller) syncService(key string) error {
	namespaceKey, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	service, err := c.serviceListen.Services(namespaceKey).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, ok := service.GetAnnotations()["ingress/http"]
	ingress, err := c.ingressListen.Ingresses(namespaceKey).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if ok && errors.IsNotFound(err) {
		//create ingress
		ig := c.constructIngress(service)
		_, err := c.client.NetworkingV1beta1().Ingresses(namespaceKey).Create(context.TODO(), ig, v12.CreateOptions{})
		if err != nil {
			return err
		}
	} else if !ok && ingress != nil {
		//delete ingress
		err := c.client.NetworkingV1beta1().Ingresses(namespaceKey).Delete(context.TODO(), name, v12.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) constructIngress(service *v13.Service) *v1beta12.Ingress {
	pt := v1beta12.PathTypeImplementationSpecific
	ingress := &v1beta12.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1beta1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
		},
		Spec: v1beta12.IngressSpec{
			IngressClassName: pointer.String("nginx"),
			Rules: []v1beta12.IngressRule{
				{
					Host: "gin.codepy.net",
					IngressRuleValue: v1beta12.IngressRuleValue{
						HTTP: &v1beta12.HTTPIngressRuleValue{
							Paths: []v1beta12.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pt,
									Backend: v1beta12.IngressBackend{
										ServiceName: service.Name,
										ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return ingress
}

func NewController(client *kubernetes.Clientset, serviceInformer v1.ServiceInformer, ingressInformer v1beta1.IngressInformer) *controller {
	c := controller{
		client:        client,
		serviceListen: serviceInformer.Lister(),
		ingressListen: ingressInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressManager"),
	}
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addService,
		UpdateFunc: c.updateService,
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deleteIngress,
	})

	return &c
}
