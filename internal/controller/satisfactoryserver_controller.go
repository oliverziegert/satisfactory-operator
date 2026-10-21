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
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

const satisfactoryServerFinalizer = "satisfactory.oliverziegert.github.com/finalizer"

// probeResult holds the structured outcome of a game server API probe.
type probeResult struct {
	Reachable         bool
	Claimed           bool
	ActiveSessionName string
	NumPlayers        int32
	ServerVersion     string
}

// SatisfactoryServerReconciler reconciles a SatisfactoryServer object
type SatisfactoryServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=satisfactoryservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=satisfactoryservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=satisfactoryservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile reconciles a SatisfactoryServer resource against the actual cluster state.
func (r *SatisfactoryServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the SatisfactoryServer resource.
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get SatisfactoryServer: %w", err)
	}

	// 2. Deletion path.
	if !server.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(server, satisfactoryServerFinalizer) {
			if server.Spec.RetainPVCOnDelete != nil && !*server.Spec.RetainPVCOnDelete {
				pvcName := fmt.Sprintf("config-%s-0", server.Name)
				pvc := &corev1.PersistentVolumeClaim{}
				if err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: server.Namespace}, pvc); err == nil {
					log.Info("Deleting PVC", "name", pvcName)
					if err := r.Delete(ctx, pvc); client.IgnoreNotFound(err) != nil {
						return ctrl.Result{}, fmt.Errorf("delete PVC %s: %w", pvcName, err)
					}
				}
			}
			controllerutil.RemoveFinalizer(server, satisfactoryServerFinalizer)
			if err := r.Update(ctx, server); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// 3. Add finalizer if missing.
	if !controllerutil.ContainsFinalizer(server, satisfactoryServerFinalizer) {
		controllerutil.AddFinalizer(server, satisfactoryServerFinalizer)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// 4. Reconcile StatefulSet.
	if err := r.reconcileStatefulSet(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	// 5. Reconcile headless Service.
	if err := r.reconcileHeadlessService(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	// 6. Reconcile external Service.
	if err := r.reconcileExternalService(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	// 7. Fetch current StatefulSet status.
	ss := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, ss); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get StatefulSet for status: %w", err)
	}

	// 8. Set Progressing condition.
	r.setProgressingCondition(server, ss)

	// 9. Probe game server API if StatefulSet has ready replicas.
	var probe *probeResult
	if ss.Status.ReadyReplicas > 0 {
		baseURL := gameAPIBaseURL(server)
		c := satisfactory.NewClient(baseURL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // self-signed cert expected

		// Authenticate if credentials are configured.
		if server.Spec.AdminPasswordSecretRef != nil {
			r.authenticateClient(ctx, server, c)
		}

		probe = r.probeGameServer(ctx, c)

		// Attempt auto-claiming if the server is reachable and unclaimed.
		if autoClaimEnabled(server, probe) {
			adminPassword, pwErr := r.getAdminPassword(ctx, server)
			if pwErr != nil {
				log.Error(pwErr, "failed to get admin password for auto-claiming")
			} else if claimErr := r.claimServer(ctx, server, baseURL, adminPassword); claimErr != nil {
				log.Error(claimErr, "auto-claiming failed")
			} else {
				// Claim succeeded: emit event and requeue so next probe picks up claimed state.
				r.Recorder.Eventf(server, nil, corev1.EventTypeNormal, "ServerClaimed", "Claim",
					"Server %q successfully claimed", getServerName(server))
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
		}
	}

	// 10. Set game-specific conditions.
	r.setGameConditions(server, ss, probe)

	// 11. Update Phase.
	r.updatePhaseFromConditions(server)

	// 12. Single status update.
	if err := r.Status().Update(ctx, server); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}

	// 13. Short requeue when server is not yet reachable.
	if !conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable) &&
		ss.Status.ReadyReplicas > 0 {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// gameAPIBaseURL returns the in-cluster HTTPS API URL of the server's single pod.
func gameAPIBaseURL(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	gamePort := server.Spec.GamePort
	if gamePort == 0 {
		gamePort = 7777
	}
	host := fmt.Sprintf("%s-0.%s-headless.%s.svc.cluster.local", server.Name, server.Name, server.Namespace)
	return fmt.Sprintf("https://%s:%d", host, gamePort)
}

// setProgressingCondition sets the Progressing condition based on StatefulSet rollout status.
func (r *SatisfactoryServerReconciler) setProgressingCondition(server *satisfactoryv1alpha1.SatisfactoryServer, ss *appsv1.StatefulSet) {
	if ss.Name == "" {
		// StatefulSet not yet created.
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeProgressing,
			metav1.ConditionTrue, "StatefulSetPending", "StatefulSet not yet created")
		return
	}
	desired := int32(1)
	if ss.Spec.Replicas != nil {
		desired = *ss.Spec.Replicas
	}
	if ss.Status.UpdatedReplicas < desired || ss.Status.ReadyReplicas < desired {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeProgressing,
			metav1.ConditionTrue, "RolloutInProgress",
			fmt.Sprintf("updated=%d ready=%d desired=%d", ss.Status.UpdatedReplicas, ss.Status.ReadyReplicas, desired))
	} else {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeProgressing,
			metav1.ConditionFalse, "RolloutComplete", "StatefulSet rollout complete")
	}
}

// probeGameServer calls the game server API using the provided client and returns a
// structured probeResult. It handles success, auth errors (server reachable but not
// integrated), and connection failures.
func (r *SatisfactoryServerReconciler) probeGameServer(ctx context.Context, c *satisfactory.Client) *probeResult {
	resp, err := c.QueryServerState(ctx)
	if err != nil {
		// Distinguish auth errors (server up but not integrated) from connectivity failures.
		var apiErr *satisfactory.APIError
		if errors.As(err, &apiErr) {
			// Auth error → server is reachable but not yet authenticated/claimed for full API access.
			return &probeResult{
				Reachable: true,
				Claimed:   isClaimed(ctx, c),
			}
		}
		// Connection failure / timeout → server not reachable.
		return &probeResult{
			Reachable: false,
		}
	}

	state := resp.ServerGameState
	return &probeResult{
		Reachable:         true,
		Claimed:           isClaimed(ctx, c),
		ActiveSessionName: state.ActiveSessionName,
		NumPlayers:        int32(state.NumConnectedPlayers),
	}
}

// isClaimed reports whether the server has been claimed. Only an unclaimed server grants
// InitialAdmin without a password; the game state carries no claim flag.
func isClaimed(ctx context.Context, c *satisfactory.Client) bool {
	_, err := c.PasswordlessLogin(ctx, satisfactory.PrivilegeLevelInitialAdmin)
	var apiErr *satisfactory.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode == "passwordless_login_not_possible"
}

// setGameConditions updates all game-specific and aggregate conditions from the probe result.
func (r *SatisfactoryServerReconciler) setGameConditions(
	server *satisfactoryv1alpha1.SatisfactoryServer,
	ss *appsv1.StatefulSet,
	probe *probeResult,
) {
	ssReady := ss.Name != "" && ss.Status.ReadyReplicas > 0

	if probe == nil {
		// StatefulSet not ready yet — no probe attempted.
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable,
			metav1.ConditionFalse, "StatefulSetNotReady", "Waiting for StatefulSet to have ready replicas")
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerClaimed,
			metav1.ConditionFalse, "StatefulSetNotReady", "Waiting for StatefulSet to have ready replicas")
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeWorldLoaded,
			metav1.ConditionFalse, "StatefulSetNotReady", "Waiting for StatefulSet to have ready replicas")
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded,
			metav1.ConditionFalse, "StatefulSetNotReady", "StatefulSet is not yet ready")
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady,
			metav1.ConditionFalse, "StatefulSetNotReady", "Waiting for StatefulSet to have ready replicas")
		// Clear Authenticated — no probe was attempted this cycle.
		conditions.Remove(&server.Status.Conditions, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated)
		return
	}

	// ServerReachable
	if probe.Reachable {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable,
			metav1.ConditionTrue, "APIResponding", "Game server HTTPS API is responding")
	} else {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable,
			metav1.ConditionFalse, "APIUnreachable", "Game server HTTPS API is not reachable")
	}

	// ServerClaimed
	if probe.Claimed {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerClaimed,
			metav1.ConditionTrue, "ServerClaimed", "Server has been claimed")
	} else {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerClaimed,
			metav1.ConditionFalse, "ServerUnclaimed", "Server has not been claimed yet")
	}

	// WorldLoaded
	if probe.ActiveSessionName != "" {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeWorldLoaded,
			metav1.ConditionTrue, "SessionActive", fmt.Sprintf("Active session: %s", probe.ActiveSessionName))
	} else {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeWorldLoaded,
			metav1.ConditionFalse, "NoActiveSession", "No active game session loaded")
	}

	// Degraded: StatefulSet ready but API unreachable.
	if ssReady && !probe.Reachable {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded,
			metav1.ConditionTrue, "APIUnreachable", "StatefulSet is ready but game server API is not reachable")
	} else {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded,
			metav1.ConditionFalse, "NotDegraded", "Game server API is reachable")
	}

	// Populate status fields from probe.
	server.Status.ConnectedPlayers = probe.NumPlayers
	if probe.ServerVersion != "" {
		server.Status.ServerVersion = probe.ServerVersion
	}

	// Aggregate Ready: StatefulSet ready AND ServerReachable AND WorldLoaded.
	if ssReady &&
		conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable) &&
		conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeWorldLoaded) {
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady,
			metav1.ConditionTrue, "ServerReady", "Server is ready and a world is loaded")
	} else {
		msg := buildNotReadyMessage(server, ssReady, probe)
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady,
			metav1.ConditionFalse, "ServerNotReady", msg)
	}
}

// buildNotReadyMessage composes a human-friendly message for a False Ready condition.
func buildNotReadyMessage(server *satisfactoryv1alpha1.SatisfactoryServer, ssReady bool, probe *probeResult) string {
	if !ssReady {
		return "StatefulSet is not ready"
	}
	if !probe.Reachable {
		return "Game server API is not reachable"
	}
	if !conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeWorldLoaded) {
		return "No active game session loaded"
	}
	return "Server is not ready"
}

// updatePhaseFromConditions derives the Phase field from conditions.
func (r *SatisfactoryServerReconciler) updatePhaseFromConditions(server *satisfactoryv1alpha1.SatisfactoryServer) {
	switch {
	case conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded):
		server.Status.Phase = satisfactoryv1alpha1.SatisfactoryServerPhaseError
	case conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeProgressing):
		server.Status.Phase = satisfactoryv1alpha1.SatisfactoryServerPhaseUpdating
	case conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady):
		server.Status.Phase = satisfactoryv1alpha1.SatisfactoryServerPhaseRunning
	default:
		server.Status.Phase = satisfactoryv1alpha1.SatisfactoryServerPhasePending
	}
}

func (r *SatisfactoryServerReconciler) reconcileStatefulSet(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) error {
	desired := buildStatefulSet(server)
	existing := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("create StatefulSet: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get StatefulSet: %w", err)
	}
	// Update the pod template spec to pick up spec changes (e.g. image, env vars).
	existing.Spec.Template = desired.Spec.Template
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("update StatefulSet: %w", err)
	}
	return nil
}

func (r *SatisfactoryServerReconciler) reconcileHeadlessService(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) error {
	desired := buildHeadlessService(server)
	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: server.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("create headless Service: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("get headless Service: %w", err)
	}
	return nil
}

func (r *SatisfactoryServerReconciler) reconcileExternalService(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) error {
	desired := buildExternalService(server)
	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: server.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("create external Service: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get external Service: %w", err)
	}
	// Update the service type if it changed.
	if existing.Spec.Type != desired.Spec.Type {
		existing.Spec.Type = desired.Spec.Type
		if err := r.Update(ctx, existing); err != nil {
			return fmt.Errorf("update external Service type: %w", err)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SatisfactoryServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.SatisfactoryServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Named("satisfactoryserver").
		Complete(r)
}
