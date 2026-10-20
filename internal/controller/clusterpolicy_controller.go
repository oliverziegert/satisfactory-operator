/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
)

// ClusterPolicyReconciler reconciles a ClusterPolicy object
type ClusterPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=clusterpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=clusterpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=clusterpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=satisfactoryservers,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile reconciles a ClusterPolicy resource against the actual cluster state.
func (r *ClusterPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling ClusterPolicy", "name", req.Name)

	// 1. Fetch the triggering ClusterPolicy (may be any policy if triggered by a SatisfactoryServer watch).
	// We always reconcile all ClusterPolicies to determine the active one.
	allPolicies := &satisfactoryv1alpha1.ClusterPolicyList{}
	if err := r.List(ctx, allPolicies); err != nil {
		return ctrl.Result{}, fmt.Errorf("list ClusterPolicies: %w", err)
	}

	if len(allPolicies.Items) == 0 {
		return ctrl.Result{}, nil
	}

	// 2. Determine active policy (oldest by creation timestamp).
	activePolicy, inactivePolicies := findActivePolicy(allPolicies.Items)

	// 3. Mark inactive policies with Ready=False.
	for i := range inactivePolicies {
		p := inactivePolicies[i]
		conditions.SetClusterPolicy(p, satisfactoryv1alpha1.ClusterPolicyConditionTypeReady,
			metav1.ConditionFalse, "MultiplePoliciesDetected",
			fmt.Sprintf("Multiple ClusterPolicy resources detected; active policy is %q", activePolicy.Name))
		conditions.SetClusterPolicy(p, satisfactoryv1alpha1.ClusterPolicyConditionTypeEnforced,
			metav1.ConditionFalse, "MultiplePoliciesDetected",
			"Policy not enforced: not the active ClusterPolicy")
		now := metav1.Now()
		p.Status.LastAppliedTime = &now
		if err := r.Status().Update(ctx, p); err != nil && !apierrors.IsConflict(err) {
			log.Error(err, "update inactive ClusterPolicy status", "name", p.Name)
		}
	}

	// 4. Only reconcile the active policy if it matches the requested resource,
	// OR if the trigger came from a SatisfactoryServer (req.Namespace != "").
	// Since ClusterPolicy is cluster-scoped, req.Namespace is always "".
	// We always proceed with the active policy reconciliation.
	cp := activePolicy

	// 5. List all SatisfactoryServer resources across all namespaces.
	serverList := &satisfactoryv1alpha1.SatisfactoryServerList{}
	if err := r.List(ctx, serverList); err != nil {
		return ctrl.Result{}, fmt.Errorf("list SatisfactoryServers: %w", err)
	}

	// 6. Apply defaults to servers with zero-valued fields.
	for i := range serverList.Items {
		original := serverList.Items[i].DeepCopy()
		modified := serverList.Items[i].DeepCopy()
		if applyDefaults(modified, cp) {
			patch := client.MergeFrom(original)
			if err := r.Patch(ctx, modified, patch); err != nil && !apierrors.IsConflict(err) {
				log.Error(err, "patch SatisfactoryServer defaults", "name", original.Name, "namespace", original.Namespace)
			}
		}
	}

	// 7. Validate constraints and collect violations.
	var violations []string

	// 7a. Image allowlist.
	if len(cp.Spec.AllowedImageVersions) > 0 {
		for i := range serverList.Items {
			server := &serverList.Items[i]
			if !validateImageAllowlist(server, cp.Spec.AllowedImageVersions) {
				msg := fmt.Sprintf("SatisfactoryServer %s/%s uses disallowed image %q", server.Namespace, server.Name, server.Spec.Image)
				violations = append(violations, msg)
				r.Recorder.Eventf(server, cp, corev1.EventTypeWarning, "DisallowedImage", "Validate",
					"Image %q is not in the ClusterPolicy allowedImageVersions list", server.Spec.Image)
			}
		}
	}

	// 7b. Max servers per namespace.
	if cp.Spec.MaxServersPerNamespace != nil && *cp.Spec.MaxServersPerNamespace > 0 {
		nsCounts := countServersPerNamespace(serverList.Items)
		limit := *cp.Spec.MaxServersPerNamespace
		for ns, count := range nsCounts {
			if int32(count) > limit {
				msg := fmt.Sprintf("namespace %q has %d SatisfactoryServer resources (limit: %d)", ns, count, limit)
				violations = append(violations, msg)
				// Emit events on each server in the over-limit namespace.
				for i := range serverList.Items {
					server := &serverList.Items[i]
					if server.Namespace == ns {
						r.Recorder.Eventf(server, cp, corev1.EventTypeWarning, "NamespaceLimitExceeded", "Validate",
							"Namespace %q exceeds maxServersPerNamespace limit (%d/%d)", ns, count, limit)
					}
				}
			}
		}
	}

	// 8. Set Enforced condition.
	if len(violations) == 0 {
		conditions.SetClusterPolicy(cp, satisfactoryv1alpha1.ClusterPolicyConditionTypeEnforced,
			metav1.ConditionTrue, "AllServersCompliant", "All governed servers comply with the policy")
	} else {
		conditions.SetClusterPolicy(cp, satisfactoryv1alpha1.ClusterPolicyConditionTypeEnforced,
			metav1.ConditionFalse, "PolicyViolation",
			fmt.Sprintf("Policy violations detected: %s", strings.Join(violations, "; ")))
	}

	// 9. Set Ready condition.
	conditions.SetClusterPolicy(cp, satisfactoryv1alpha1.ClusterPolicyConditionTypeReady,
		metav1.ConditionTrue, "Reconciled", "ClusterPolicy reconciled successfully")

	// 10. Populate status fields.
	cp.Status.ServersGoverned = int32(len(serverList.Items))
	now := metav1.Now()
	cp.Status.LastAppliedTime = &now

	if err := r.Status().Update(ctx, cp); err != nil {
		return ctrl.Result{}, fmt.Errorf("update ClusterPolicy status: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.ClusterPolicy{}).
		Watches(
			&satisfactoryv1alpha1.SatisfactoryServer{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				// Re-reconcile all ClusterPolicies when any SatisfactoryServer changes.
				policyList := &satisfactoryv1alpha1.ClusterPolicyList{}
				if err := mgr.GetClient().List(ctx, policyList); err != nil {
					return nil
				}
				var reqs []ctrl.Request
				for _, p := range policyList.Items {
					reqs = append(reqs, ctrl.Request{
						NamespacedName: client.ObjectKeyFromObject(&p),
					})
				}
				return reqs
			}),
		).
		Named("clusterpolicy").
		Complete(r)
}
