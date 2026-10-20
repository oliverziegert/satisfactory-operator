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
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
)

// saveGameServerRefIndex indexes SaveGames by spec.serverRef for the SatisfactoryServer watch.
const saveGameServerRefIndex = "spec.serverRef"

// saveGameRetryInterval is how soon to retry when the server is missing, unreachable or a
// one-shot save failed.
const saveGameRetryInterval = 30 * time.Second

// SaveGameReconciler reconciles a SaveGame object
type SaveGameReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// GameAPIURL overrides the game server API URL; tests point it at an httptest server.
	GameAPIURL func(*satisfactoryv1alpha1.SatisfactoryServer) string
}

// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=savegames,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=savegames/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=savegames/finalizers,verbs=update
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=satisfactoryservers,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile resolves the referenced server, triggers a save when one is due, enforces
// retention and records the result in status.
func (r *SaveGameReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sg := &satisfactoryv1alpha1.SaveGame{}
	if err := r.Get(ctx, req.NamespacedName, sg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get SaveGame: %w", err)
	}

	// Set BackupScheduled condition based on whether a valid schedule is configured.
	var sched cron.Schedule
	if sg.Spec.Schedule != "" {
		var err error
		if sched, err = cron.ParseStandard(sg.Spec.Schedule); err != nil {
			msg := fmt.Sprintf("Invalid schedule %q: %v", sg.Spec.Schedule, err)
			conditions.SetSaveGame(sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled,
				metav1.ConditionFalse, "InvalidSchedule", msg)
			// No requeue: only a spec change can fix this.
			return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseFailed, metav1.ConditionFalse,
				"InvalidSchedule", msg, ctrl.Result{})
		}
		conditions.SetSaveGame(sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled,
			metav1.ConditionTrue, "ScheduleActive",
			fmt.Sprintf("Backup schedule active: %s", sg.Spec.Schedule))
	} else {
		conditions.SetSaveGame(sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled,
			metav1.ConditionFalse, "NoSchedule", "No backup schedule configured")
		sg.Status.NextBackupTime = nil
	}

	c, err := r.resolveServerClient(ctx, sg.Spec.ServerRef, sg.Namespace)
	if err != nil {
		var he *authHelperError
		if !errors.As(err, &he) {
			return ctrl.Result{}, err
		}
		phase := satisfactoryv1alpha1.SaveGamePhasePending
		if he.Reason == "ServerNotFound" {
			phase = satisfactoryv1alpha1.SaveGamePhaseFailed
		}
		return r.updateStatus(ctx, sg, phase, metav1.ConditionFalse, he.Reason, he.Message,
			ctrl.Result{RequeueAfter: saveGameRetryInterval})
	}

	now := time.Now()
	// After a failure, retry at the next cron tick (or soon, for a one-shot save).
	retry := ctrl.Result{RequeueAfter: saveGameRetryInterval}
	if sched != nil {
		next := sched.Next(now)
		sg.Status.NextBackupTime = &metav1.Time{Time: next}
		retry = ctrl.Result{RequeueAfter: next.Sub(now)}
	}

	due := backupDue(sched, sg.Status.LastBackupTime, now)
	if due {
		name, err := triggerSave(ctx, c, sg.Spec.SaveName, now)
		if err != nil {
			log.Error(err, "save failed")
			return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseFailed, metav1.ConditionFalse,
				"SaveFailed", err.Error(), retry)
		}
		log.Info("Saved game", "saveName", name)
		sg.Status.LastBackupTime = &metav1.Time{Time: now}
	}

	saves, err := enumerateSessionSaves(ctx, c, sg.Spec.SaveName)
	if err != nil {
		return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseFailed, metav1.ConditionFalse,
			"EnumerateFailed", err.Error(), retry)
	}
	if due {
		if saves, err = enforceRetention(ctx, c, saves, sg.Spec.RetentionPolicy, now); err != nil {
			return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseFailed, metav1.ConditionFalse,
				"RetentionFailed", err.Error(), retry)
		}
	}
	sg.Status.RetainedSaveCount = int32(len(saves))

	if sched == nil {
		return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseCompleted, metav1.ConditionTrue,
			"Reconciled", "One-shot save completed", ctrl.Result{})
	}
	next := sched.Next(sg.Status.LastBackupTime.Time)
	sg.Status.NextBackupTime = &metav1.Time{Time: next}
	return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseActive, metav1.ConditionTrue,
		"Reconciled", "SaveGame reconciled successfully", ctrl.Result{RequeueAfter: next.Sub(now)})
}

// updateStatus sets Phase and the Ready condition, writes status and returns res.
func (r *SaveGameReconciler) updateStatus(
	ctx context.Context,
	sg *satisfactoryv1alpha1.SaveGame,
	phase satisfactoryv1alpha1.SaveGamePhase,
	status metav1.ConditionStatus,
	reason, message string,
	res ctrl.Result,
) (ctrl.Result, error) {
	sg.Status.Phase = phase
	conditions.SetSaveGame(sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, status, reason, message)
	if err := r.Status().Update(ctx, sg); err != nil {
		return ctrl.Result{}, fmt.Errorf("update SaveGame status: %w", err)
	}
	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SaveGameReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &satisfactoryv1alpha1.SaveGame{},
		saveGameServerRefIndex, indexSaveGameServerRef); err != nil {
		return fmt.Errorf("index SaveGame %s: %w", saveGameServerRefIndex, err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.SaveGame{}).
		Watches(&satisfactoryv1alpha1.SatisfactoryServer{}, handler.EnqueueRequestsFromMapFunc(r.saveGamesForServer)).
		Named("savegame").
		Complete(r)
}

func indexSaveGameServerRef(o client.Object) []string {
	return []string{o.(*satisfactoryv1alpha1.SaveGame).Spec.ServerRef}
}

// saveGamesForServer maps a SatisfactoryServer to the SaveGames that reference it.
func (r *SaveGameReconciler) saveGamesForServer(ctx context.Context, obj client.Object) []reconcile.Request {
	list := &satisfactoryv1alpha1.SaveGameList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace()),
		client.MatchingFields{saveGameServerRefIndex: obj.GetName()}); err != nil {
		logf.FromContext(ctx).Error(err, "list SaveGames for server", "server", obj.GetName())
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for _, sg := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&sg)})
	}
	return reqs
}
