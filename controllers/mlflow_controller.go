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
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	//	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cloudmlv1beta1 "github.com/paipaoso/mlflow/api/v1beta1"
)

// MlflowReconciler reconciles a Mlflow object
type MlflowReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cloudml.xiaomi.com,resources=mlflows,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloudml.xiaomi.com,resources=mlflows/status,verbs=get;update;patch

func (r *MlflowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := r.Log.WithValues("mlflow", req.NamespacedName)
	var Mlflow_image = "cr.d.xiaomi.net/cloud-ml/cloudml-metrics:v1"

	instance := &cloudmlv1beta1.Mlflow{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	command := "mlflow server -h 0.0.0.0"
	if instance.Spec.Source != "" {
		command = command + " --backend-store-uri " + instance.Spec.Source
	}
	if instance.Spec.ArtifactRoot != "" {
		command = command + " --default-artifact-root " + instance.Spec.ArtifactRoot
	}

	commands := []string{"bash", "-c", command}

	podtemplete := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      instance.Labels,
			Annotations: instance.ObjectMeta.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "mlflow",
					Image:           Mlflow_image,
					ImagePullPolicy: "Always",
					Command:         commands,
					Env:             instance.Spec.Env,
					Resources:       instance.Spec.Resources,
					VolumeMounts:    instance.Spec.VolumeMounts,
				},
			},
			Volumes:  instance.Spec.Volumes,
			Affinity: instance.Spec.Affinity,
		},
	}
	Labelselectors := &metav1.LabelSelector{
		MatchLabels: instance.Labels,
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Template: podtemplete,
			Selector: Labelselectors,
		},
	}

	if err := controllerutil.SetControllerReference(instance, deploy, r.Scheme); err != nil {
		log.Error(err, "Reconcile Set controller ref deployment", "deploy", instance)
		return ctrl.Result{}, err
	}
	founddeploy := &appsv1.Deployment{}
	err = r.Get(ctx, req.NamespacedName, founddeploy)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Deployment", "namespace", deploy.Namespace, "deployment", deploy.Name)
		err := r.Create(ctx, deploy)
		if err != nil {
			log.Error(err, "Creating deploy", "namespace", deploy.Namespace, "deploy", deploy.Name)
			return ctrl.Result{}, err
		}

	} else if err != nil {
		log.Error(err, "Creating deploy", "namespace", deploy.Namespace, "deploy", deploy.Name)
		r.Recorder.Event(instance, "Error", "FailedCreateDeploy", fmt.Sprintf("Failed Creat Deploy %s", instance.Name))
		return ctrl.Result{}, err
	}
	found := &cloudmlv1beta1.Mlflow{}
	err = r.Get(ctx, req.NamespacedName, found)
	if err != nil {
		// Error reading the object - requeue the request.
		log.Error(err, "reconcile error when fetch mlflow")
		return ctrl.Result{}, err
	}
	if len(founddeploy.Status.Conditions) != 0 {
		found.Status.Condition = founddeploy.Status.Conditions
	}

	uri := instance.Name + "-" + instance.Annotations["ingress-postfix"]
	if found.URI != uri {
		found.URI = uri
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "reconcile error when update status")
			return ctrl.Result{}, err
		}
	}
	r.Recorder.Event(instance, "Normal", "SuccessfulCreatedeployment", fmt.Sprintf("Created deployment %s", instance.Name))
	pods := &corev1.PodList{}
	label := []string{}
	for key, value := range founddeploy.ObjectMeta.Labels {
		label = append(label, key+"="+value)
	}
	labelSelector, _ := labels.Parse(strings.Join(label, ","))
	listOpts := &client.ListOptions{LabelSelector: labelSelector}
	err = r.List(ctx, pods, listOpts)
	if err != nil {
		log.Error(err, "List pods", "namespace", found.Namespace, "deployment name", found.Name)
		// NOTE(xychu): maybe we could not reconcile err when list pods err,
		//              since it's only needed for debug
		// return reconcile.Result{}, err
	}
	// podInfos := []cloudmlv1beta1.PodInfo{}

	pod := pods.Items[0]
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:     "metrics",
					Protocol: corev1.ProtocolTCP,
					Port:     int32(5000),
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(5000),
					},
				},
			},
			Selector: pod.Labels,
		},
	}
	if err := ctrl.SetControllerReference(instance, service, r.Scheme); err != nil {
		log.Info("error when setcontrollerreference for service", "service name", instance.Name)
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, service); err != nil {
		// log.Info("unable to create ingress for instance", "pod", ingress, "error", err.Code)
		if errors.IsAlreadyExists(err) {
			log.Info("service already exists", "service", instance.Name)
		} else {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(instance, "Normal", "SuccessfulCreateService", fmt.Sprintf("Created Service %s", instance.Name))
	}

	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: fmt.Sprintf("%s%s", instance.Name, instance.Annotations["ingress-postfix"]),
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
										ServiceName: pod.Name,
										ServicePort: intstr.FromInt(5000),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(instance, ingress, r.Scheme); err != nil {
		log.Info("error when setcontrollerreference for ingress", "ingress name", instance.Name)
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, ingress); err != nil {
		// log.Info("unable to create ingress for instance", "pod", ingress, "error", err.Code)
		if errors.IsAlreadyExists(err) {
			log.Info("ingress already exists", "ingress", deploy.Name)
		} else {
			return ctrl.Result{}, err
		}
	}
	r.Recorder.Event(instance, "Normal", "SuccessfulCreateIngress", fmt.Sprintf("Created ingress %s for mlflow service", ingress.Name))
	log.Info("ingress", "successfully create ingress", ingress.Name)

	return ctrl.Result{}, nil
}

func (r *MlflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cloudmlv1beta1.Mlflow{}).
		Complete(r)
}
