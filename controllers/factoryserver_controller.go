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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/record"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
)

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
	result := &ctrl.Result{}
	err := error(nil)

	// Fetch the FactoryServer instance
	// The purpose is check if the Custom Resource for the Kind FactoryServer
	// is applied on the cluster if not we return nil to stop the reconciliation
	factoryServer := &satisfactoryv1alpha1.FactoryServer{}
	if err := r.Get(ctx, req.NamespacedName, factoryServer); err != nil {
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
		if err := r.setStatusCondition(
			ctx,
			factoryServer,
			typeAvailableFactoryServer,
			metav1.ConditionUnknown,
			"Reconciling",
			"Starting reconciliation",
		); err != nil {
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

		if err := r.Update(ctx, factoryServer); err != nil {
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
			if err := r.setStatusCondition(
				ctx,
				factoryServer,
				typeDegradedFactoryServer,
				metav1.ConditionUnknown,
				"Finalizing",
				fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", factoryServer.Name),
			); err != nil {
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

			if err := r.setStatusCondition(
				ctx,
				factoryServer,
				typeDegradedFactoryServer,
				metav1.ConditionTrue,
				"Finalizing",
				fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", factoryServer.Name),
			); err != nil {
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

	if result, err = r.ensureConfigMap(ctx, req, factoryServer, r.configMapForFactoryServer(factoryServer)); result != nil {
		return *result, err
	}

	if result, err = r.ensureStatefulSet(ctx, req, factoryServer, r.statefulsetForFactoryServer(factoryServer)); result != nil {
		return *result, err
	}

	if result, err = r.ensureService(ctx, req, factoryServer, r.serviceForFactoryServer(factoryServer)); result != nil {
		return *result, err
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

// statefulsetForFactoryServer returns a factoryServer StatefulSet object
func (r *FactoryServerReconciler) statefulsetForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) *appsv1.StatefulSet {
	ls := labelsForFactoryServer(f)
	replicas := f.Spec.Size

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      factoryServerStatefulSetName(f),
			Namespace: f.Namespace,
			Labels:    ls,
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
						Name:  factoryServerServiceName(f),
						Image: imageForFactoryServer(f),
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
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: factoryServerConfigMapName(f),
									},
									Optional: &[]bool{false}[0],
								},
							},
						},
						Resources: corev1.ResourceRequirements{},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      factoryServerPersistentVolumeClaimName(f),
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
					Name:      factoryServerPersistentVolumeClaimName(f),
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

	sts.Labels[factoryServerLastAppliedHash] = asSha256(sts)

	// Set FactoryServer instance as the owner and controller
	ctrl.SetControllerReference(f, sts, r.Scheme)
	return sts
}

// serviceForFactoryServer returns a factoryServer Service object
func (r *FactoryServerReconciler) serviceForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) *corev1.Service {
	ls := labelsForFactoryServer(f)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      factoryServerServiceName(f),
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
		if f.Spec.Service.LoadBalancerClass != "" {
			svc.Spec.LoadBalancerClass = &f.Spec.Service.LoadBalancerClass
		}
		if len(f.Spec.Service.LoadBalancerSourceRanges) > 0 {
			svc.Spec.LoadBalancerSourceRanges = f.Spec.Service.LoadBalancerSourceRanges
		}
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

	svc.Labels[factoryServerLastAppliedHash] = asSha256(svc)

	// Set FactoryServer instance as the owner and controller
	ctrl.SetControllerReference(f, svc, r.Scheme)
	return svc
}

// serviceForFactoryServer returns a factoryServer Service object
func (r *FactoryServerReconciler) configMapForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) *corev1.ConfigMap {
	ls := labelsForFactoryServer(f)

	data := make(map[string]string)
	data["AUTOPAUSE"] = strconv.FormatBool(f.Spec.Autopause)
	data["AUTOSAVEINTERVAL"] = strconv.FormatUint(f.Spec.Autosave.Interval, 10)
	data["AUTOSAVENUM"] = strconv.FormatUint(f.Spec.Autosave.Num, 10)
	data["AUTOSAVEONDISCONNECT"] = strconv.FormatBool(f.Spec.Autosave.OnDisconnect)
	data["CRASHREPORT"] = strconv.FormatBool(f.Spec.CrashReport)
	data["DEBUG"] = strconv.FormatBool(f.Spec.Debug)
	data["DISABLESEASONALEVENTS"] = strconv.FormatBool(f.Spec.DisableSeasonalEvents)
	data["MAXPLAYERS"] = strconv.FormatUint(f.Spec.Maxplayers, 10)
	data["NETWORKQUALITY"] = strconv.FormatUint(f.Spec.Networkquality, 10)
	data["SERVERBEACONPORT"] = strconv.FormatInt(int64(f.Spec.Ports.Beacon), 10)
	data["SERVERGAMEPORT"] = strconv.FormatInt(int64(f.Spec.Ports.Game), 10)
	data["SERVERQUERYPORT"] = strconv.FormatInt(int64(f.Spec.Ports.Query), 10)
	data["SKIPUPDATE"] = strconv.FormatBool(f.Spec.SkipUpdate)
	data["STEAMBETA"] = strconv.FormatBool(f.Spec.Beta)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      factoryServerConfigMapName(f),
			Namespace: f.Namespace,
			Labels:    ls,
		},
		Data: data,
	}

	cm.Labels[factoryServerLastAppliedHash] = asSha256(cm)

	// Set FactoryServer instance as the owner and controller
	ctrl.SetControllerReference(f, cm, r.Scheme)
	return cm
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

// imageForFactoryServer gets the Operand image which is managed by this controller
// from the FACTORYSERVER_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForFactoryServer(f *satisfactoryv1alpha1.FactoryServer) string {
	var imageEnvVar = "FACTORYSERVER_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if found {
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

func factoryServerStatefulSetName(f *satisfactoryv1alpha1.FactoryServer) string {
	return f.Name + "-server"
}

func factoryServerPersistentVolumeClaimName(f *satisfactoryv1alpha1.FactoryServer) string {
	return f.Name + "-pvc"
}

func factoryServerServiceName(f *satisfactoryv1alpha1.FactoryServer) string {
	return f.Name + "-svc"
}

func factoryServerConfigMapName(f *satisfactoryv1alpha1.FactoryServer) string {
	return f.Name + "-cm"
}

func (r *FactoryServerReconciler) ensureConfigMap(ctx context.Context, req reconcile.Request, f *satisfactoryv1alpha1.FactoryServer, c *corev1.ConfigMap) (*ctrl.Result, error) {
	log := log.FromContext(ctx)
	needsUpdate := false
	found := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", c.Namespace, "ConfigMap.Name", c.Name)
		if err = r.Client.Create(ctx, c); err != nil {
			// Creation failed
			log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", c.Namespace, "ConfigMap.Name", c.Name)

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
		log.Error(err, "Failed to get ConfigMap")
		return &ctrl.Result{}, err
	}

	if c.Labels[factoryServerLastAppliedHash] != getLastAppliedHash(found.Labels) {
		needsUpdate = true
	}

	if needsUpdate {
		if err = r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update ConfigMap", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
			if err := r.setStatusCondition(
				ctx,
				f,
				typeAvailableFactoryServer,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to update ConfigMap for the custom resource (%s): (%s)", f.Name, err),
			); err != nil {
				return &ctrl.Result{}, err
			}
			return &ctrl.Result{}, err
		}

		log.Info("Updating ConfigMap", "ConfigMap.Namespace", c.Namespace, "ConfigMap.Name", c.Name)
		if err := r.setStatusCondition(
			ctx,
			f,
			typeAvailableFactoryServer,
			metav1.ConditionTrue,
			"Reconciling",
			fmt.Sprintf("ConfigMap for custom resource (%s) updated successfully", f.Name),
		); err != nil {
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{}, err
	}
	return nil, nil
}

func (r *FactoryServerReconciler) ensureService(ctx context.Context, req reconcile.Request, f *satisfactoryv1alpha1.FactoryServer, s *corev1.Service) (*ctrl.Result, error) {
	log := log.FromContext(ctx)
	needsUpdate := false

	// Check if the service already exists, if not create a new one
	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		log.Info("Creating a new Service", "Service.Namespace", s.Namespace, "Service.Name", s.Name)
		if err = r.Create(ctx, s); err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", s.Namespace, "Service.Name", s.Name)
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
		log.Error(err, "Failed to get Service")
		return &ctrl.Result{}, err
	}

	if s.Labels[factoryServerLastAppliedHash] != getLastAppliedHash(found.Labels) {
		needsUpdate = true
		found.Labels = s.Labels
		found.Spec.Selector = s.Spec.Selector
		found.Spec.Type = s.Spec.Type
		found.Spec.Ports = s.Spec.Ports
	}

	if needsUpdate {
		if err = r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update Service", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
			if err := r.setStatusCondition(
				ctx,
				f,
				typeAvailableFactoryServer,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to update Service for the custom resource (%s): (%s)", f.Name, err),
			); err != nil {
				return &ctrl.Result{}, err
			}
			return &ctrl.Result{}, err
		}

		log.Info("Updating Service", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
		if err := r.setStatusCondition(
			ctx,
			f,
			typeAvailableFactoryServer,
			metav1.ConditionTrue,
			"Reconciling",
			fmt.Sprintf("Service for custom resource (%s) updated successfully", f.Name),
		); err != nil {
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{}, err
	}
	return nil, nil
}

func (r *FactoryServerReconciler) ensureStatefulSet(ctx context.Context, req reconcile.Request, f *satisfactoryv1alpha1.FactoryServer, s *appsv1.StatefulSet) (*ctrl.Result, error) {
	log := log.FromContext(ctx)
	needsUpdate := false

	// Check if the StatefulSet already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new StatefulSet
		log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", s.Namespace, "StatefulSet.Name", s.Name)
		if err = r.Create(ctx, s); err != nil {
			log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", s.Namespace, "StatefulSet.Name", s.Name)
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
		// Error that isn't due to the service not existing
		log.Error(err, "Failed to get StatefulSet")
		return &ctrl.Result{}, err
	}

	if s.Labels[factoryServerLastAppliedHash] != getLastAppliedHash(found.Labels) {
		needsUpdate = true
		found.Labels = s.Labels
		found.Spec.Replicas = s.Spec.Replicas
		found.Spec.Template.Spec = s.Spec.Template.Spec
		found.Spec.UpdateStrategy = s.Spec.UpdateStrategy
		found.Spec.PersistentVolumeClaimRetentionPolicy = s.Spec.PersistentVolumeClaimRetentionPolicy
		found.Spec.MinReadySeconds = s.Spec.MinReadySeconds
	}

	if needsUpdate {
		if err = r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update StatefulSet", "StatefulSet.Namespace", found.Namespace, "StatefulSet.Name", found.Name)
			if err := r.setStatusCondition(
				ctx,
				f,
				typeAvailableFactoryServer,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to update StatefulSet for the custom resource (%s): (%s)", f.Name, err),
			); err != nil {
				return &ctrl.Result{}, err
			}
			return &ctrl.Result{}, err
		}

		log.Info("Updating StatefulSet", "StatefulSet.Namespace", found.Namespace, "StatefulSet.Name", found.Name)
		if err := r.setStatusCondition(
			ctx,
			f,
			typeAvailableFactoryServer,
			metav1.ConditionTrue,
			"Reconciling",
			fmt.Sprintf("StatefulSet for custom resource (%s) updated successfully", f.Name),
		); err != nil {
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{}, err
	}
	return nil, nil
}

func asSha256(o interface{}) string {
	m, _ := json.Marshal(o)
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", m)))
	return fmt.Sprintf("%x", h.Sum(nil))[:63]
}

func getLastAppliedHash(labels map[string]string) string {
	if val, ok := labels[factoryServerLastAppliedHash]; ok {
		return val
	}
	return ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *FactoryServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.FactoryServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
