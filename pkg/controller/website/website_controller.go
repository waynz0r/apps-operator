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

package website

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	appsv1alpha1 "github.com/waynz0r/apps-operator/pkg/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	objectmatch "github.com/banzaicloud/k8s-objectmatcher"
)

var log = logf.Log.WithName("controller")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Website Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWebsite{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("website-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Website
	err = c.Watch(&source.Kind{Type: &appsv1alpha1.Website{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to a Deployment created by Website
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appsv1alpha1.Website{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to a Service created by Website
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appsv1alpha1.Website{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to an Ingress created by Website
	err = c.Watch(&source.Kind{Type: &extensions.Ingress{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appsv1alpha1.Website{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileWebsite{}

// ReconcileWebsite reconciles a Website object
type ReconcileWebsite struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Website object and makes changes based on the state read
// and what is in the Website.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.k8smeetup.io,resources=websites,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.k8smeetup.io,resources=websites/status,verbs=get;update;patch
func (r *ReconcileWebsite) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Website instance
	instance := &appsv1alpha1.Website{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Reconcile resources
	for _, res := range []runtime.Object{
		r.getDeployment(instance),
		r.getService(instance),
		r.getIngress(instance),
	} {
		err = r.reconcile(res)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileWebsite) reconcile(desired runtime.Object) error {
	var current = desired.DeepCopyObject()
	var desiredType = reflect.TypeOf(desired)

	key, err := client.ObjectKeyFromObject(current)
	if err != nil {
		return err
	}
	logKeys := []interface{}{"kind", desiredType, "namespace", key.Namespace, "name", key.Name}

	err = r.Client.Get(context.TODO(), key, current)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		log.Info("create resource", logKeys...)
		if err := r.Client.Create(context.TODO(), desired); err != nil {
			return err
		}
		log.Info("resource created")
		return nil
	}

	objectsEquals, err := objectmatch.New(log).Match(current, desired)
	if err != nil {
		log.Error(err, "could not match objects", logKeys...)
	} else if objectsEquals {
		log.V(1).Info("resource is in sync", logKeys...)
		return nil
	}

	switch desired.(type) {
	default:
		return errors.New("unexpected resource type")
	case *appsv1.Deployment:
		desired := desired.(*appsv1.Deployment)
		desired.ResourceVersion = current.(*appsv1.Deployment).ResourceVersion
	case *corev1.Service:
		desired := desired.(*corev1.Service)
		desired.ResourceVersion = current.(*corev1.Service).ResourceVersion
	case *extensions.Ingress:
		desired := desired.(*extensions.Ingress)
		desired.ResourceVersion = current.(*extensions.Ingress).ResourceVersion
	}

	log.Info("update resource", logKeys...)
	err = r.Client.Update(context.TODO(), desired)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileWebsite) getIngress(instance *appsv1alpha1.Website) runtime.Object {
	resource := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-ingress",
			Namespace: instance.Namespace,
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: "demo1.k8smeetup.io",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Backend: extensions.IngressBackend{
										ServiceName: instance.Name + "-service",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	controllerutil.SetControllerReference(instance, resource, r.scheme)

	return resource
}

func (r *ReconcileWebsite) getService(instance *appsv1alpha1.Website) runtime.Object {
	resource := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-service",
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(80),
				},
			},
			Selector: map[string]string{"deployment": instance.Name + "-deployment"},
		},
	}

	controllerutil.SetControllerReference(instance, resource, r.scheme)

	return resource
}

func (r *ReconcileWebsite) getDeployment(instance *appsv1alpha1.Website) runtime.Object {
	replicaCount := int32(1)

	resource := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-deployment",
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"deployment": instance.Name + "-deployment"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"deployment": instance.Name + "-deployment"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:                     "nginx",
							Image:                    "nginx",
							ImagePullPolicy:          corev1.PullAlways,
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
				},
			},
		},
	}

	controllerutil.SetControllerReference(instance, resource, r.scheme)

	return resource
}
