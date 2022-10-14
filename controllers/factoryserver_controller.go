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
	"crypto/sha256"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/record"
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

const (
	factoryServerFinalizer       = "satisfactory.pc-ziegert.de/finalizer"
	factoryServerLastAppliedHash = "satisfactory.pc-ziegert.de/last-applied-hash"

	// Definitions to manage status conditions
	// typeAvailableFactoryServer represents the status of the Deployment reconciliation
	typeAvailableFactoryServer = "Available"
	// typeDegradedFactoryServer represents the status used when the custom resource is deleted and the finalizer operations are must occur.
	typeDegradedFactoryServer = "Degraded"
)

// FactoryServerReconciler reconciles a FactoryServer object
type FactoryServerReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	typeFields []field
	log        *logr.Logger
}

// The following markers are used to generate the rules permissions (RBAC) on config/rbac using controller-gen
// when the command <make manifests> is executed.
// To know more about markers see: https://book.kubebuilder.io/reference/markers.html

//+kubebuilder:rbac:groups=satisfactory.pc-ziegert.de,resources=factoryservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=satisfactory.pc-ziegert.de,resources=factoryservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=satisfactory.pc-ziegert.de,resources=factoryservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods;services;configmaps,verbs=get;list;watch;create;update;patch;delete;

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
	r.log = &log
	result := ctrl.Result{}

	// Fetch the FactoryServer instance
	// The purpose is to check if the Custom Resource for the Kind FactoryServer
	// is applied on the cluster,1 if not we return nil to stop the reconciliation
	factoryServer := &satisfactoryv1alpha1.FactoryServer{}
	if err := r.Get(ctx, req.NamespacedName, factoryServer); err != nil {
		if errors.IsNotFound(err) {
			// If the custom resource is not stsFound then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			r.log.Info("FactoryServer resource not deploy. Ignoring since object must be deleted")
			result.Requeue = false
			return result, nil
		}
		// Error reading the object - requeue the request.
		r.log.Error(err, "Failed to get FactoryServer")
		result.Requeue = false
		return result, err
	}

	// Let's just set the status as Unknown when no status are available
	if factoryServer.Status.Conditions == nil || len(factoryServer.Status.Conditions) == 0 {
		if err := r.setStatusCondition(
			ctx,
			factoryServer,
			typeAvailableFactoryServer,
			metav1.ConditionUnknown,
			"Reconciling",
			"Starting reconciliation",
		); err != nil {
			result.Requeue = false
			return result, err
		}

		// Let's re-fetch the factoryServer Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, factoryServer); err != nil {
			r.log.Error(err, "Failed to re-fetch factoryServer")
			result.Requeue = false
			return result, err
		}
	}

	// check finalizer
	if result, err := r.addFinalizer(ctx, factoryServer); result != nil {
		return *result, err
	}

	// check if crd is marked as deleted
	if result, err := r.isDeleted(ctx, req, factoryServer); result != nil {
		return *result, err
	}

	// check ConfigMap
	if result, err := r.ensureConfigMapExists(ctx, factoryServer, r.getConfigMap(factoryServer)); result != nil {
		return *result, err
	}

	// check StatefulSet
	if result, err := r.ensureStatefulSetExists(ctx, factoryServer, r.getStatefulSet(factoryServer)); result != nil {
		return *result, err
	}

	// check Service
	if result, err := r.ensureServiceExists(ctx, factoryServer, r.getService(factoryServer)); result != nil {
		return *result, err
	}

	// check Spec
	if result, err := r.ensureSpec(ctx, factoryServer); result != nil {
		return *result, err
	}

	// Update the FactoryServer status with the pod names
	// List the pods for this factoryServer's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(factoryServer.Namespace),
		client.MatchingLabels(getLabels(factoryServer)),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "FactoryServer.Namespace", factoryServer.Namespace, "FactoryServer.Name", factoryServer.Name)
		result.Requeue = false
		return result, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, factoryServer.Status.Nodes) {
		factoryServer.Status.Nodes = podNames
		err := r.Status().Update(ctx, factoryServer)
		if err != nil {
			log.Error(err, "Failed to update FactoryServer status")
			result.Requeue = false
			return result, err
		}
	}
	return result, nil
}

func (r *FactoryServerReconciler) addFinalizer(ctx context.Context, f *satisfactoryv1alpha1.FactoryServer) (*ctrl.Result, error) {
	result := &ctrl.Result{}
	// Let`s add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(f, factoryServerFinalizer) {
		r.log.Info("Adding finalizer for FactoryServer")
		if ok := controllerutil.AddFinalizer(f, factoryServerFinalizer); !ok {
			err := fmt.Errorf("failed to add finalizer into the custom resource")
			r.log.Error(err, "Requeue current request in 3 seconds")
			result.Requeue = true
			result.RequeueAfter = time.Second * 3
			return result, err
		}

		if err := r.Update(ctx, f); err != nil {
			r.log.Error(err, "Failed to update custom resource to add finalizer")
			result.Requeue = false
			return result, err
		}
	}
	return nil, nil
}

func (r *FactoryServerReconciler) isDeleted(ctx context.Context, req ctrl.Request, f *satisfactoryv1alpha1.FactoryServer) (*ctrl.Result, error) {
	result := &ctrl.Result{}

	// Check if the FactoryServer instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set
	isFactoryServerMarkedToBeDeleted := f.GetDeletionTimestamp() != nil
	if isFactoryServerMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(f, factoryServerFinalizer) {
			r.log.Info("Performing Finalizer Operations for FactoryServer before delete CR")

			// Let's add here a status "Downgrade" to define that this resource begin its process to be terminated.
			if err := r.setStatusCondition(
				ctx,
				f,
				typeDegradedFactoryServer,
				metav1.ConditionUnknown,
				"Finalizing",
				fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", f.Name),
			); err != nil {
				result.Requeue = false
				return result, err
			}
			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperations(f)

			// TODO(user): If you add operations to the doFinalizerOperations method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the memcached Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, f); err != nil {
				r.log.Error(err, "Failed to re-fetch FactoryServer")
				result.Requeue = false
				return result, err
			}

			if err := r.setStatusCondition(
				ctx,
				f,
				typeDegradedFactoryServer,
				metav1.ConditionTrue,
				"Finalizing",
				fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", f.Name),
			); err != nil {
				result.Requeue = false
				return result, err
			}

			r.log.Info("Removing Finalizer for FactoryServer after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(f, factoryServerFinalizer); !ok {
				err := fmt.Errorf("failed to remove finalizer for FactoryServer")
				r.log.Error(err, "Requeue current request in 3 seconds")
				result.Requeue = true
				result.RequeueAfter = time.Second * 3
				return result, err
			}

			if err := r.Update(ctx, f); err != nil {
				r.log.Error(err, "Failed to remove finalizer for FactoryServer")
				result.Requeue = false
				return result, err
			}
		}
		return result, nil
	}
	return nil, nil
}

func (r *FactoryServerReconciler) setStatusCondition(ctx context.Context, f *satisfactoryv1alpha1.FactoryServer, t string, s metav1.ConditionStatus, re string, m string) error {
	log := log.FromContext(ctx)
	// The following implementation will update the status
	meta.SetStatusCondition(&f.Status.Conditions, metav1.Condition{
		Type:    t,
		Status:  s,
		Reason:  re,
		Message: m,
	})

	if err := r.Status().Update(ctx, f); err != nil {
		log.Error(err, "Failed to update FactoryServer status")
		return err
	}
	return nil
}

func (r *FactoryServerReconciler) doFinalizerOperations(cr *satisfactoryv1alpha1.FactoryServer) {
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

// ---------------------------------------------------- StatefulSet ----------------------------------------------------
// getStatefulSetName returns StatefulSet name for this FactoryServer
func getStatefulSetName(f *satisfactoryv1alpha1.FactoryServer) string {
	return f.Name + "-server"
}

// getPersistentVolumeClaimName returns PersistentVolumeClaim name for this FactoryServer
func getPersistentVolumeClaimName() string {
	return getContainerName() + "-pvc"
}

// getPersistentVolumeClaimName returns PersistentVolumeClaim name for this FactoryServer
func getContainerName() string {
	return "factory-server"
}

// getStatefulSet returns a factoryServer StatefulSet object
func (r *FactoryServerReconciler) getStatefulSet(f *satisfactoryv1alpha1.FactoryServer) *appsv1.StatefulSet {
	replicas := f.Spec.Size

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getStatefulSetName(f),
			Namespace: f.Namespace,
			Labels:    getLabels(f),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(f),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: getLabels(f),
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:    &[]int64{0}[0],
						RunAsGroup:   &[]int64{0}[0],
						RunAsNonRoot: &[]bool{false}[0],
						FSGroup:      &[]int64{0}[0],
					},
					Containers: []corev1.Container{{
						Name:  getContainerName(),
						Image: f.Spec.Image,
						Ports: []corev1.ContainerPort{
							{
								Name:          "query",
								ContainerPort: f.Spec.PortQuery,
								Protocol:      "UDP",
							},
							{
								Name:          "beacon",
								ContainerPort: f.Spec.PortBeacon,
								Protocol:      "UDP",
							},
							{
								Name:          "game",
								ContainerPort: f.Spec.PortGame,
								Protocol:      "UDP",
							},
						},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: getConfigMapName(f),
									},
									Optional: &[]bool{false}[0],
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      getPersistentVolumeClaimName(),
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
					Name:      getPersistentVolumeClaimName(),
					Namespace: f.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(f.Spec.StorageRequests),
						},
					},
				},
			}},
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		},
	}

	if len(f.Spec.StorageClass) > 0 {
		sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = &f.Spec.StorageClass
	}

	sts.ObjectMeta.Labels[factoryServerLastAppliedHash] = asSha256(sts)

	// Set FactoryServer instance as the owner and controller
	ctrl.SetControllerReference(f, sts, r.Scheme)
	return sts
}

// ensureConfigMapExists creates a StatefulSet if it does not exist
func (r *FactoryServerReconciler) ensureStatefulSetExists(ctx context.Context, f *satisfactoryv1alpha1.FactoryServer, s *appsv1.StatefulSet) (*ctrl.Result, error) {

	// Check if the StatefulSet already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new StatefulSet
		r.log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", s.Namespace, "StatefulSet.Name", s.Name)
		if err = r.Create(ctx, s); err != nil {
			r.log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", s.Namespace, "StatefulSet.Name", s.Name)
			if err := r.setStatusCondition(
				ctx,
				f,
				typeAvailableFactoryServer,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to create StatefulSet for the custom resource (%s): (%s)", f.Name, err),
			); err != nil {
				return &ctrl.Result{}, err
			}
			return &ctrl.Result{}, err
		}
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		if err := r.setStatusCondition(
			ctx,
			f,
			typeAvailableFactoryServer,
			metav1.ConditionTrue,
			"Reconciling",
			fmt.Sprintf("StatefulSet for custom resource (%s) created successfully", f.Name),
		); err != nil {
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{Requeue: true, RequeueAfter: time.Second * 30}, nil
	} else if err != nil {
		// Error that isn't due to the StatefulSet not existing
		r.log.Error(err, "Failed to get StatefulSet")
		return &ctrl.Result{}, err
	}
	return nil, nil
}

// ------------------------------------------------------ Service ------------------------------------------------------
// getServiceName returns Service name for this FactoryServer
func getServiceName(f *satisfactoryv1alpha1.FactoryServer) string {
	return f.Name + "-svc"
}

// getService returns a factoryServer Service object
func (r *FactoryServerReconciler) getService(f *satisfactoryv1alpha1.FactoryServer) *corev1.Service {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceName(f),
			Namespace: f.Namespace,
			Labels:    getLabels(f),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "query",
					Protocol:   "UDP",
					Port:       f.Spec.PortQuery,
					TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "query"},
				},
				{
					Name:       "beacon",
					Protocol:   "UDP",
					Port:       f.Spec.PortBeacon,
					TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "beacon"},
				},
				{
					Name:       "game",
					Protocol:   "UDP",
					Port:       f.Spec.PortGame,
					TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "game"},
				},
			},
			Selector: getLabels(f),
			Type:     f.Spec.ServiceType,
		},
	}

	switch f.Spec.ServiceType {
	case corev1.ServiceTypeLoadBalancer:
		if f.Spec.ServiceLoadBalancerClass != "" {
			svc.Spec.LoadBalancerClass = &f.Spec.ServiceLoadBalancerClass
		}
		if len(f.Spec.ServiceLoadBalancerSourceRanges) > 0 {
			svc.Spec.LoadBalancerSourceRanges = f.Spec.ServiceLoadBalancerSourceRanges
		}
	case corev1.ServiceTypeNodePort:
		for _, port := range svc.Spec.Ports {
			switch port.Name {
			case "query":
				port.NodePort = f.Spec.ServiceNodePortQuery
			case "beacon":
				port.NodePort = f.Spec.ServiceNodePortBeacon
			case "game":
				port.NodePort = f.Spec.ServiceNodePortGame
			}
		}
	}

	svc.ObjectMeta.Labels[factoryServerLastAppliedHash] = asSha256(svc)

	// Set FactoryServer instance as the owner and controller
	ctrl.SetControllerReference(f, svc, r.Scheme)
	return svc
}

// ensureConfigMapExists creates a Service if it does not exist
func (r *FactoryServerReconciler) ensureServiceExists(ctx context.Context, f *satisfactoryv1alpha1.FactoryServer, s *corev1.Service) (*ctrl.Result, error) {

	// Check if the Service already exists, if not create a new one
	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		r.log.Info("Creating a new Service", "Service.Namespace", s.Namespace, "Service.Name", s.Name)
		if err = r.Create(ctx, s); err != nil {
			r.log.Error(err, "Failed to create new Service", "Service.Namespace", s.Namespace, "Service.Name", s.Name)
			if err := r.setStatusCondition(
				ctx,
				f,
				typeAvailableFactoryServer,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to create Service for the custom resource (%s): (%s)", f.Name, err),
			); err != nil {
				return &ctrl.Result{}, err
			}
			return &ctrl.Result{}, err
		}
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		if err := r.setStatusCondition(
			ctx,
			f,
			typeAvailableFactoryServer,
			metav1.ConditionTrue,
			"Reconciling",
			fmt.Sprintf("Service for custom resource (%s) created successfully", f.Name),
		); err != nil {
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5}, nil
	} else if err != nil {
		// Error that isn't due to the service not existing
		r.log.Error(err, "Failed to get Service")
		return &ctrl.Result{}, err
	}
	return nil, nil
}

// ----------------------------------------------------- ConfigMap -----------------------------------------------------
// getConfigMapName returns ConfigMap name for this FactoryServer
func getConfigMapName(f *satisfactoryv1alpha1.FactoryServer) string {
	return f.Name + "-cm"
}

// getConfigMap returns a factoryServer ConfigMap object
func (r *FactoryServerReconciler) getConfigMap(f *satisfactoryv1alpha1.FactoryServer) *corev1.ConfigMap {
	data := make(map[string]string)
	data["AUTOPAUSE"] = strconv.FormatBool(f.Spec.Autopause)
	data["AUTOSAVEINTERVAL"] = strconv.FormatUint(f.Spec.AutosaveInterval, 10)
	data["AUTOSAVENUM"] = strconv.FormatUint(f.Spec.AutosaveNum, 10)
	data["AUTOSAVEONDISCONNECT"] = strconv.FormatBool(f.Spec.AutosaveOnDisconnect)
	data["CRASHREPORT"] = strconv.FormatBool(f.Spec.CrashReport)
	data["DEBUG"] = strconv.FormatBool(f.Spec.Debug)
	data["DISABLESEASONALEVENTS"] = strconv.FormatBool(f.Spec.DisableSeasonalEvents)
	data["MAXPLAYERS"] = strconv.FormatUint(f.Spec.Maxplayers, 10)
	data["NETWORKQUALITY"] = strconv.FormatUint(f.Spec.Networkquality, 10)
	data["SERVERBEACONPORT"] = strconv.FormatInt(int64(f.Spec.PortBeacon), 10)
	data["SERVERGAMEPORT"] = strconv.FormatInt(int64(f.Spec.PortGame), 10)
	data["SERVERQUERYPORT"] = strconv.FormatInt(int64(f.Spec.PortQuery), 10)
	data["SKIPUPDATE"] = strconv.FormatBool(f.Spec.SkipUpdate)
	data["STEAMBETA"] = strconv.FormatBool(f.Spec.Beta)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getConfigMapName(f),
			Namespace: f.Namespace,
			Labels:    getLabels(f),
		},
		Data: data,
	}

	cm.ObjectMeta.Labels[factoryServerLastAppliedHash] = asSha256(cm)

	// Set FactoryServer instance as the owner and controller
	ctrl.SetControllerReference(f, cm, r.Scheme)
	return cm
}

// ensureConfigMapExists creates a ConfigMap if it does not exist
func (r *FactoryServerReconciler) ensureConfigMapExists(ctx context.Context, f *satisfactoryv1alpha1.FactoryServer, c *corev1.ConfigMap) (*ctrl.Result, error) {

	// Check if the ConfigMap already exists, if not create a new one
	found := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		r.log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", c.Namespace, "ConfigMap.Name", c.Name)
		if err = r.Client.Create(ctx, c); err != nil {
			// Creation failed
			r.log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", c.Namespace, "ConfigMap.Name", c.Name)

			if err := r.setStatusCondition(
				ctx,
				f,
				typeAvailableFactoryServer,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to create ConfigMap for the custom resource (%s): (%s)", f.Name, err),
			); err != nil {
				return &ctrl.Result{}, err
			}
			return &ctrl.Result{}, err
		}

		if err := r.setStatusCondition(
			ctx,
			f,
			typeAvailableFactoryServer,
			metav1.ConditionTrue,
			"Reconciling",
			fmt.Sprintf("ConfigMap for custom resource (%s) created successfully", f.Name),
		); err != nil {
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{}, nil
	} else if err != nil {
		// Error that isn't due to the configMap not existing
		r.log.Error(err, "Failed to get ConfigMap")
		return &ctrl.Result{}, err
	}
	return nil, nil
}

// ------------------------------------------------------- Helper ------------------------------------------------------
// getLabels returns the labels for selecting the resources
// belonging to the given factoryServer CR name.
func getLabels(f *satisfactoryv1alpha1.FactoryServer) map[string]string {
	var imageTag string
	image := f.Spec.Image
	imageTag = strings.Split(image, ":")[1]

	return map[string]string{"app.kubernetes.io/name": "FactoryServer",
		"app.kubernetes.io/instance":   f.Name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "factoryServer-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

func asSha256(o interface{}) string {
	m, _ := json.Marshal(o)
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", m)))
	return fmt.Sprintf("%x", h.Sum(nil))[:63]
}

func (r *FactoryServerReconciler) ensureSpec(ctx context.Context, f *satisfactoryv1alpha1.FactoryServer) (*ctrl.Result, error) {
	result := &ctrl.Result{}
	// Source resources
	sCm := r.getConfigMap(f)
	sSts := r.getStatefulSet(f)
	sSvc := r.getService(f)

	// target resources
	tCm := &corev1.ConfigMap{}
	tSts := &appsv1.StatefulSet{}
	tSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: getConfigMapName(f), Namespace: f.Namespace}, tCm); err != nil {
		return &ctrl.Result{}, err
	}
	if err := r.Get(ctx, types.NamespacedName{Name: getStatefulSetName(f), Namespace: f.Namespace}, tSts); err != nil {
		return &ctrl.Result{}, err
	}
	if err := r.Get(ctx, types.NamespacedName{Name: getServiceName(f), Namespace: f.Namespace}, tSvc); err != nil {
		return &ctrl.Result{}, err
	}

	uCm := false
	uSts := false
	uSvc := false

	for _, field := range r.typeFields {
		for _, resource := range field.Resources {
			switch strings.ToLower(resource.Typ) {
			case "deployment", "deploy":
				continue
			case "daemonSet", "ds":
				continue
			case "cronjob", "cj":
				continue
			case "ingress", "ing":
				continue
			case "statefulset", "sts":
				if adjust(sSts, tSts, resource.Path) {
					r.log.Info(fmt.Sprintf("StatefulSet: %s does not match, will be updated", resource.Path))
					uSts = true
				}
			case "configmap", "cm":
				if adjust(sCm, tCm, resource.Path) {
					r.log.Info(fmt.Sprintf("ConfigMap: %s does not match, will be updated", resource.Path))
					uCm = true
				}
			case "service", "svc":
				if strings.Contains(resource.Path, "NodePort") && f.Spec.ServiceType != corev1.ServiceTypeNodePort {
					continue
				}
				if adjust(sSvc, tSvc, resource.Path) {
					r.log.Info(fmt.Sprintf("Service: %s does not match, will be updated", resource.Path))
					uSvc = true
				}
			case "persistentvolumeclaims", "pvc":
				continue
			case "serviceaccounts", "sa":
				continue
			default:
				continue
			}
		}
	}

	if uCm {
		r.log.Info("ConfigMap needs an update")
		res, err := r.updateResources(ctx, tCm)
		if err != nil {
			return res, err
		}
		if res.Requeue {
			result.Requeue = res.Requeue
			if res.RequeueAfter > result.RequeueAfter {
				result.RequeueAfter = res.RequeueAfter
			}
		}
		r.log.Info("ConfigMap updated successfully")
	}

	if uSts {
		r.log.Info("StatefulSet needs an update")
		res, err := r.updateResources(ctx, tSts)
		if err != nil {
			return res, err
		}
		if res.Requeue {
			result.Requeue = res.Requeue
			if res.RequeueAfter > result.RequeueAfter {
				result.RequeueAfter = res.RequeueAfter
			}
		}
		r.log.Info("StatefulSet updated successfully")
	}

	if uSvc {
		r.log.Info("Service needs an update")
		res, err := r.updateResources(ctx, tSvc)
		if err != nil {
			return res, err
		}
		if res.Requeue {
			result.Requeue = res.Requeue
			if res.RequeueAfter > result.RequeueAfter {
				result.RequeueAfter = res.RequeueAfter
			}
		}
		r.log.Info("Service updated successfully")
	}

	if uCm || uSts || uSvc {
		return result, nil
	}
	return nil, nil
}

func adjust(source interface{}, target interface{}, fieldPath string) bool {

	// if target is not pointer, then immediately return
	// modifying struct's field requires addressable object
	sAddrValue := reflect.ValueOf(source)
	tAddrValue := reflect.ValueOf(target)
	if sAddrValue.Kind() != reflect.Ptr || tAddrValue.Kind() != reflect.Ptr {
		panic("Fuuu!!!")
	}

	sValue := sAddrValue.Elem()
	tValue := tAddrValue.Elem()
	if !sValue.IsValid() || !tValue.IsValid() {
		panic("Fuuu!!!")
	}

	// If the field/struct is passed by pointer, then first dereference it to get the
	// underlying value (the pointer must not be pointing to a nil value).
	if sValue.Type().Kind() == reflect.Ptr && !sValue.IsNil() ||
		tValue.Type().Kind() == reflect.Ptr && !tValue.IsNil() {
		sValue = sValue.Elem()
		tValue = tValue.Elem()
		if !sValue.IsValid() || !tValue.IsValid() {
			panic("Fuuu!!!")
		}
	}

	// split filedPath into individual pieces
	// walk through given path
	fieldPathElements := strings.Split(fieldPath, ".")
	for indexFieldName, fieldName := range fieldPathElements {
		if sValue.Type().Kind() == reflect.Ptr && !sValue.IsNil() ||
			tValue.Type().Kind() == reflect.Ptr && !tValue.IsNil() {
			sValue = sValue.Elem()
			tValue = tValue.Elem()
			if !sValue.IsValid() || !tValue.IsValid() {
				panic("Fuuu!!!")
			}
		}

		// if given field is an array
		if strings.ContainsAny(fieldName, "[]") {
			idx := strings.Index(fieldName, "[")
			listElementName := fieldName[idx+1 : len(fieldName)-1]
			fieldName = fieldName[:idx]
			sValue = sValue.FieldByName(fieldName)
			tValue = tValue.FieldByName(fieldName)

			if strings.ContainsRune(listElementName, '=') {
				idx = strings.Index(listElementName, "=")
				keys := listElementName[:idx]
				value := listElementName[idx+1:]
				for i := 0; i < sValue.Len(); i++ {
					sValueIndex := sValue.Index(i)
					for _, key := range strings.Split(keys, ":") {
						sValueIndex = sValueIndex.FieldByName(key)
						if sValueIndex.Type().Kind() == reflect.Ptr && !sValueIndex.IsNil() {
							sValueIndex = sValueIndex.Elem()
						}
					}
					if sValueIndex.String() == value {
						sValue = sValue.Index(i)
						break
					}
				}
				for i := 0; i < tValue.Len(); i++ {
					tValueIndex := tValue.Index(i)
					for _, key := range strings.Split(keys, ":") {
						tValueIndex = tValueIndex.FieldByName(key)
						if tValueIndex.Type().Kind() == reflect.Ptr && !tValueIndex.IsNil() {
							tValueIndex = tValueIndex.Elem()
						}
					}
					if tValueIndex.String() == value {
						tValue = tValue.Index(i)
						break
					}
				}
				continue
			}

			//search for next element by name
			for i := 0; i < tValue.Len(); i++ {
				if tValue.Index(i).FieldByName("Name").String() == listElementName {
					tValue = tValue.Index(i)
					break
				}
			}
			for i := 0; i < sValue.Len(); i++ {
				if sValue.Index(i).FieldByName("Name").String() == listElementName {
					sValue = sValue.Index(i)
					break
				}
			}
			continue
		}

		// if given field is a map
		if strings.ContainsAny(fieldName, "{}") {
			idx := strings.Index(fieldName, "{")
			mapElementName := fieldName[idx+1 : len(fieldName)-1]
			fieldName = fieldName[:idx]
			sValue = sValue.FieldByName(fieldName)
			tValue = tValue.FieldByName(fieldName)

			//search for next element by name
			for _, key := range sValue.MapKeys() {
				keyPtr := key
				if keyPtr.Type().Kind() == reflect.Ptr && !keyPtr.IsNil() {
					keyPtr = keyPtr.Elem()
				}
				if keyPtr.String() == mapElementName {
					sValue = sValue.MapIndex(key)
					break
				}
			}
			for _, key := range tValue.MapKeys() {
				keyPtr := key
				if keyPtr.Type().Kind() == reflect.Ptr && !keyPtr.IsNil() {
					keyPtr = keyPtr.Elem()
				}
				if keyPtr.String() == mapElementName {
					tValueTemp := tValue.MapIndex(key)
					// Last Element in path -> set value directly
					if indexFieldName+1 == len(fieldPathElements) {
						if reflect.DeepEqual(sValue.Interface(), tValueTemp.Interface()) {
							return false
						}
						tValue.SetMapIndex(key, sValue)
						return true
					}
					tValue = tValueTemp
					break
				}
			}
			continue
		}

		sValue = sValue.FieldByName(fieldName)
		tValue = tValue.FieldByName(fieldName)
	}

	if sValue.Type().Kind() == reflect.Ptr && !sValue.IsNil() ||
		tValue.Type().Kind() == reflect.Ptr && !tValue.IsNil() {
		sValue = sValue.Elem()
		tValue = tValue.Elem()
		if !sValue.IsValid() || !tValue.IsValid() {
			panic("Fuuu!!!")
		}
	}

	if reflect.DeepEqual(sValue.Interface(), tValue.Interface()) {
		return false
	}
	//check if tValue is a correct Value
	if !tValue.IsValid() {
		panic("Fuuu!!!")
	}

	// check if we can set this value
	// ToDo: Fails if struct needs to be updated
	// Spec.VolumeClaimTemplates.Spec.Resources.Requests{storage}
	if !tValue.CanSet() {
		return false
	}

	// set actual value
	tValue.Set(sValue)
	return true
}

func (r *FactoryServerReconciler) updateResources(ctx context.Context, obj client.Object) (*ctrl.Result, error) {
	if err := r.Update(ctx, obj); err != nil {
		r.log.Error(err, "Failed to update Deployment",
			"Deployment.Namespace", obj.GetNamespace(), "Deployment.Name", obj.GetName())

		return &ctrl.Result{}, err
	}
	return &ctrl.Result{Requeue: true}, nil
}

// ------------------------------------------------------ Startup ------------------------------------------------------

// SetupWithManager sets up the controller with the Manager.
func (r *FactoryServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.typeFields = TypeFields(reflect.TypeOf(satisfactoryv1alpha1.FactoryServer{}))
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.FactoryServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
