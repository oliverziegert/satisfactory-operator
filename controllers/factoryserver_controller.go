/*
Copyright 2022 Oliver Ziegert <paperless@pc-ziegert.dev>.

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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	satisfactoryv1alpha1 "git.pc-ziegert.de/operators/satisfactory/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const factoryServerFinalizer = "satisfactory.pc-ziegert.de/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailableFactoryServer represents the status of the Deployment reconciliation
	typeAvailableFactoryServer = "Available"
	// typeDegradedFactoryServer represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedFactoryServer = "Degraded"
)

// FactoryServerReconciler reconciles a FactoryServer object
type FactoryServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// The following markers are used to generate the rules permissions (RBAC) on config/rbac using controller-gen
// when the command <make manifests> is executed.
// To know more about markers see: https://book.kubebuilder.io/reference/markers.html

//+kubebuilder:rbac:groups=satisfactory.pc-ziegert.de,resources=factoryservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=satisfactory.pc-ziegert.de,resources=factoryservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=satisfactory.pc-ziegert.de,resources=factoryservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods;services,verbs=get;list;watch;create;update;patch;delete;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// It is essential for the controller's reconciliation loop to be idempotent. By following the Operator
// pattern you will create Controllers which provide a reconcile function
// responsible for synchronizing resources until the desired state is reached on the cluster.
// Breaking this recommendation goes against the design principles of controller-runtime.
// and may lead to unforeseen consequences such as resources becoming stuck and requiring manual intervention.
// For further info:
// - About Operator Pattern: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
// - About Controllers: https://kubernetes.io/docs/concepts/architecture/controller/
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *FactoryServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the FactoryServer instance
	// The purpose is check if the Custom Resource for the Kind FactoryServer
	// is applied on the cluster if not we return nil to stop the reconciliation
	factoryServer := &satisfactoryv1alpha1.FactoryServer{}
	err := r.Get(ctx, req.NamespacedName, factoryServer)
	if err != nil {
		if errors.IsNotFound(err) {
			// If the custom resource is not stsFound then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("FactoryServer resource not deploy. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get FactoryServer")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if factoryServer.Status.Conditions == nil || len(factoryServer.Status.Conditions) == 0 {
		meta.SetStatusCondition(&factoryServer.Status.Conditions, metav1.Condition{
			Type:    typeAvailableFactoryServer,
			Status:  metav1.ConditionUnknown,
			Reason:  "Reconciling",
			Message: "Starting reconciliation",
		})
		if err = r.Status().Update(ctx, factoryServer); err != nil {
			log.Error(err, "Failed to update FactoryServer status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the factoryServer Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, factoryServer); err != nil {
			log.Error(err, "Failed to re-fetch factoryServer")
			return ctrl.Result{}, err
		}
	}

	// Let`s add a finalizer. Then, we can define some operations which schould
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(factoryServer, factoryServerFinalizer) {
		log.Info("Adding finalizer for FactoryServer")
		if ok := controllerutil.AddFinalizer(factoryServer, factoryServerFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, err
		}

		if err = r.Update(ctx, factoryServer); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the FactoryServer instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set
	isFactoryServerMarkedToBeDeleted := factoryServer.GetDeletionTimestamp() != nil
	if isFactoryServerMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(factoryServer, factoryServerFinalizer) {
			log.Info("Performing Finalizer Operations for FactoryServer before delete CR")

			// Let's add here a status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&factoryServer.Status.Conditions, metav1.Condition{
				Type:    typeDegradedFactoryServer,
				Status:  metav1.ConditionUnknown,
				Reason:  "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", factoryServer.Name),
			})

			if err := r.Status().Update(ctx, factoryServer); err != nil {
				log.Error(err, "Failed to update FactoryServer status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom custom resource.
			r.doFinalizerOperationsForFactoryServer(factoryServer)

			// TODO(user): If you add operations to the doFinalizerOperationsForMemcached method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the memcached Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, factoryServer); err != nil {
				log.Error(err, "Failed to re-fetch FactoryServer")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&factoryServer.Status.Conditions, metav1.Condition{
				Type:   typeDegradedFactoryServer,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", factoryServer.Name),
			})

			if err := r.Status().Update(ctx, factoryServer); err != nil {
				log.Error(err, "Failed to update FactoryServer status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for FactoryServer after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(factoryServer, factoryServerFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for FactoryServer")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, factoryServer); err != nil {
				log.Error(err, "Failed to remove finalizer for FactoryServer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if the StatefulSet already exists, if not create a new one
	stsFound := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: factoryServer.Name, Namespace: factoryServer.Namespace}, stsFound)
	if err != nil && errors.IsNotFound(err) {
		// Define a new StatefulSet
		sts, err := r.statefulsetForFactoryServer(factoryServer)
		if err != nil {
			log.Error(err, "Failed to define new StatefulSet resource for FactoryServer")

			// The following implementation will update the status
			meta.SetStatusCondition(&factoryServer.Status.Conditions, metav1.Condition{
				Type:   typeAvailableFactoryServer,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create StatefulSet for the custom resource (%s): (%s)", factoryServer.Name, err),
			})

			if err := r.Status().Update(ctx, factoryServer); err != nil {
				log.Error(err, "Failed to update FactoryServer status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
		if err = r.Create(ctx, sts); err != nil {
			log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			return ctrl.Result{}, err
		}
		// StatefulSet created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 30}, nil
	} else if err != nil {
		log.Error(err, "Failed to get StatefulSet")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Check if the service already exists, if not create a new one
	svcFound := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: factoryServer.Name, Namespace: factoryServer.Namespace}, svcFound)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		svc, err := r.serviceForFactoryServer(factoryServer)
		if err != nil {
			log.Error(err, "Failed to define new Service resource for FactoryServer")

			// The following implementation will update the status
			meta.SetStatusCondition(&factoryServer.Status.Conditions, metav1.Condition{
				Type:   typeAvailableFactoryServer,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Service for the custom resource (%s): (%s)", factoryServer.Name, err),
			})

			if err := r.Status().Update(ctx, factoryServer); err != nil {
				log.Error(err, "Failed to update FactoryServer status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// The CRD API is defining that the FactoryServer type, have a FactoryServerSpec.Size field
	// to set the quantity of StatefulSet instances is the desired state on the cluster.
	// Therefore, the following code will ensure the StatefulSet size is the same as defined
	// via the Size spec of the Custom Resource which we are reconciling.
	size := factoryServer.Spec.Size
	if *stsFound.Spec.Replicas != size {
		// Increment FactoryServerDeploymentSizeUndesiredCountTotal metric by 1
		// monitoring.FactoryServerDeploymentSizeUndesiredCountTotal.Inc()
		stsFound.Spec.Replicas = &size
		if err = r.Update(ctx, stsFound); err != nil {
			log.Error(err, "Failed to update StatefulSet",
				"StatefulSet.Namespace", stsFound.Namespace, "StatefulSet.Name", stsFound.Name)

			// Re-fetch the factoryServer Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, factoryServer); err != nil {
				log.Error(err, "Failed to re-fetch FactoryServer")
				return ctrl.Result{}, err
			}

			// The following implementation will update the status
			meta.SetStatusCondition(&factoryServer.Status.Conditions, metav1.Condition{
				Type:   typeAvailableFactoryServer,
				Status: metav1.ConditionFalse, Reason: "Resizing",
				Message: fmt.Sprintf("Failed to update the size for the custom resource (%s): (%s)", factoryServer.Name, err),
			})

			if err := r.Status().Update(ctx, factoryServer); err != nil {
				log.Error(err, "Failed to update FactoryServer status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		// Now, that we update the size we want to requeue the reconciliation
		// so that we can ensure that we have the latest state of the resource before
		// update. Also, it will help ensure the desired state on the cluster
		return ctrl.Result{Requeue: true}, nil
	}

	// The following implementation will update the status
	meta.SetStatusCondition(&factoryServer.Status.Conditions, metav1.Condition{
		Type:   typeAvailableFactoryServer,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) with %d replicas created successfully", factoryServer.Name, size),
	})

	if err := r.Status().Update(ctx, factoryServer); err != nil {
		log.Error(err, "Failed to update FactoryServer status")
		return ctrl.Result{}, err
	}

	// Update the FactoryServer status with the pod names
	// List the pods for this factoryServer's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(factoryServer.Namespace),
		client.MatchingLabels(labelsForFactoryServer(factoryServer)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "FactoryServer.Namespace", factoryServer.Namespace, "FactoryServer.Name", factoryServer.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, factoryServer.Status.Nodes) {
		factoryServer.Status.Nodes = podNames
		err := r.Status().Update(ctx, factoryServer)
		if err != nil {
			log.Error(err, "Failed to update FactoryServer status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *FactoryServerReconciler) doFinalizerOperationsForFactoryServer(cr *satisfactoryv1alpha1.FactoryServer) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))
}

// statefulsetForFactoryServer returns a factoryServer Deployment object
func (r *FactoryServerReconciler) statefulsetForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) (*appsv1.StatefulSet, error) {
	ls := labelsForFactoryServer(f)
	replicas := f.Spec.Size

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.Name,
			Namespace: f.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:    &[]int64{0}[0],
						RunAsGroup:   &[]int64{0}[0],
						RunAsNonRoot: &[]bool{false}[0],
						FSGroup:      &[]int64{0}[0],
					},
					Containers: []corev1.Container{{
						Name:  "factory-server",
						Image: "wolveix/satisfactory-server:v1.2.5",
						Ports: []corev1.ContainerPort{
							{
								Name:          "query",
								ContainerPort: f.Spec.Ports.Query,
								Protocol:      "UDP",
							},
							{
								Name:          "beacon",
								ContainerPort: f.Spec.Ports.Beacon,
								Protocol:      "UDP",
							},
							{
								Name:          "game",
								ContainerPort: f.Spec.Ports.Game,
								Protocol:      "UDP",
							},
						},
						Env: []corev1.EnvVar{
							{
								Name:  "AUTOPAUSE",
								Value: strconv.FormatBool(f.Spec.Autopause),
							},
							{
								Name:  "AUTOSAVEINTERVAL",
								Value: strconv.FormatUint(f.Spec.Autosave.Interval, 10),
							},
							{
								Name:  "AUTOSAVENUM",
								Value: strconv.FormatUint(f.Spec.Autosave.Num, 10),
							},
							{
								Name:  "AUTOSAVEONDISCONNECT",
								Value: strconv.FormatBool(f.Spec.Autosave.OnDisconnect),
							},
							{
								Name:  "CRASHREPORT",
								Value: strconv.FormatBool(f.Spec.CrashReport),
							},
							{
								Name:  "DEBUG",
								Value: strconv.FormatBool(f.Spec.Debug),
							},
							{
								Name:  "DISABLESEASONALEVENTS",
								Value: strconv.FormatBool(f.Spec.DisableSeasonalEvents),
							},
							{
								Name:  "MAXPLAYERS",
								Value: strconv.FormatUint(f.Spec.Maxplayers, 10),
							},
							{
								Name:  "NETWORKQUALITY",
								Value: strconv.FormatUint(f.Spec.Networkquality, 10),
							},
							{
								Name:  "SERVERBEACONPORT",
								Value: strconv.FormatInt(int64(f.Spec.Ports.Beacon), 10),
							},
							{
								Name:  "SERVERGAMEPORT",
								Value: strconv.FormatInt(int64(f.Spec.Ports.Game), 10),
							},
							{
								Name:  "SERVERQUERYPORT",
								Value: strconv.FormatInt(int64(f.Spec.Ports.Query), 10),
							},
							{
								Name:  "SKIPUPDATE",
								Value: strconv.FormatBool(f.Spec.SkipUpdate),
							},
							{
								Name:  "STEAMBETA",
								Value: strconv.FormatBool(f.Spec.Beta),
							},
						},
						Resources: corev1.ResourceRequirements{},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      fmt.Sprintf("%s-%s", f.Name, "data"),
								MountPath: "/config",
							},
						},
						LivenessProbe:  nil,
						ReadinessProbe: nil,
						StartupProbe:   nil,
						SecurityContext: &corev1.SecurityContext{
							//Capabilities: &corev1.Capabilities{
							//	Add:  []corev1.Capability{"CAP_CHOWN", "CAP_SETGID", "CAP_SETUID"},
							//	Drop: []corev1.Capability{"ALL"},
							//},
							Privileged:               &[]bool{false}[0],
							RunAsUser:                &[]int64{0}[0],
							RunAsGroup:               &[]int64{0}[0],
							RunAsNonRoot:             &[]bool{false}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", f.Name, "data"),
					Namespace: f.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("50Gi"),
						},
					},
				},
			}},
		},
	}

	if len(f.Spec.StorageClass) > 0 {
		sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = &f.Spec.StorageClass
	}

	// Set FactoryServer instance as the owner and controller
	if err := ctrl.SetControllerReference(f, sts, r.Scheme); err != nil {
		return nil, err
	}
	return sts, nil
}

// serviceForFactoryServer returns a factoryServer Service object
func (r *FactoryServerReconciler) serviceForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) (*corev1.Service, error) {
	ls := labelsForFactoryServer(f)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.Name,
			Namespace: f.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "query",
					Protocol:   "UDP",
					Port:       f.Spec.Ports.Query,
					TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "query"},
				},
				{
					Name:       "beacon",
					Protocol:   "UDP",
					Port:       f.Spec.Ports.Beacon,
					TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "beacon"},
				},
				{
					Name:       "game",
					Protocol:   "UDP",
					Port:       f.Spec.Ports.Game,
					TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "game"},
				},
			},
			Selector: ls,
			Type:     f.Spec.Service.Type,
		},
	}

	switch f.Spec.Service.Type {
	case corev1.ServiceTypeLoadBalancer:
		svc.Spec.LoadBalancerClass = &f.Spec.Service.LoadBalancerClass
		svc.Spec.LoadBalancerSourceRanges = f.Spec.Service.LoadBalancerSourceRanges
	case corev1.ServiceTypeNodePort:
		for _, port := range svc.Spec.Ports {
			switch port.Name {
			case "query":
				port.NodePort = f.Spec.Service.NodePorts.Query
			case "beacon":
				port.NodePort = f.Spec.Service.NodePorts.Beacon
			case "game":
				port.NodePort = f.Spec.Service.NodePorts.Game
			}
		}
	}

	// Set FactoryServer instance as the owner and controller
	if err := ctrl.SetControllerReference(f, svc, r.Scheme); err != nil {
		return nil, err
	}
	return svc, nil
}

// labelsForFactoryServer returns the labels for selecting the resources
// belonging to the given factoryServer CR name.
func labelsForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) map[string]string {
	var imageTag string
	image := imageForFactoryServer(f)
	imageTag = strings.Split(image, ":")[1]

	return map[string]string{"app.kubernetes.io/name": "FactoryServer",
		"app.kubernetes.io/instance":   f.Name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "factoryServer-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// imageForMemcached gets the Operand image which is managed by this controller
// from the MEMCACHED_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) string {
	var imageEnvVar = "FACTORYSERVER_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return image
	}
	return f.Spec.Image
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *FactoryServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.FactoryServer{}).
		Owns(&appsv1.StatefulSet{}).
		// Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
