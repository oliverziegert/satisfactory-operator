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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/backup"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
)

// saveGameServerRefIndex indexes SaveGames by spec.serverRef for the SatisfactoryServer watch.
const saveGameServerRefIndex = "spec.serverRef"

// saveGameRetryInterval is how soon to retry when the server is missing, unreachable or a
// one-shot save failed.
const saveGameRetryInterval = 30 * time.Second

// Backup Job constants.
const (
	backupTimeAnnotation = "satisfactory.oliverziegert.github.com/backup-time"
	backupContainerName  = "backup"
)

// Ready reasons for server problems; they clear once the server is back.
const (
	reasonServerNotFound         = "ServerNotFound"
	reasonServerNotReachable     = "ServerNotReachable"
	reasonServerNotAuthenticated = "ServerNotAuthenticated"
)

var serverProblemReasons = []string{reasonServerNotFound, reasonServerNotReachable, reasonServerNotAuthenticated}

// SaveGameReconciler reconciles a SaveGame object
type SaveGameReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// BackupImage is the operator image the backup Jobs run (RELATED_IMAGE_BACKUP).
	BackupImage string

	// GameAPIURL overrides the game server API URL; tests point it at an httptest server.
	GameAPIURL func(*satisfactoryv1alpha1.SatisfactoryServer) string
}

// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=savegames,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=savegames/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=savegames/finalizers,verbs=update
// +kubebuilder:rbac:groups=satisfactory.oliverziegert.github.com,resources=satisfactoryservers,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete

// Reconcile records the result of the latest backup Job and creates the next Job when a
// backup is due. Saving and retention run inside the Job.
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
			return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseFailed,
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

	job, err := r.latestBackupJob(ctx, sg)
	if err != nil {
		return ctrl.Result{}, err
	}
	if job != nil {
		finished, succeeded := jobFinished(job)
		if !finished {
			// One run at a time; the Job watch re-triggers us when it finishes.
			return ctrl.Result{}, r.writeStatus(ctx, sg)
		}
		r.recordJobResult(ctx, sg, job, succeeded)
	}

	now := time.Now()
	next, ok := nextBackup(sched, sg.Status, jobRunTime(job), now)
	if !ok || now.Before(next) {
		if _, err := r.reachableServer(ctx, sg.Spec.ServerRef, sg.Namespace); err != nil {
			return r.serverProblem(ctx, sg, err)
		}
		if ready := meta.FindStatusCondition(sg.Status.Conditions, satisfactoryv1alpha1.SaveGameConditionTypeReady); ready == nil ||
			slices.Contains(serverProblemReasons, ready.Reason) {
			phase := satisfactoryv1alpha1.SaveGamePhaseActive
			if sched == nil {
				phase = satisfactoryv1alpha1.SaveGamePhaseCompleted
			}
			r.setReady(sg, phase, metav1.ConditionTrue, "Reconciled", "SaveGame reconciled successfully")
		}
		res := ctrl.Result{}
		if ok {
			res.RequeueAfter = next.Sub(now)
			if sched != nil {
				sg.Status.NextBackupTime = &metav1.Time{Time: next}
			}
		}
		return res, r.writeStatus(ctx, sg)
	}

	if r.BackupImage == "" {
		// Only a manager restart with RELATED_IMAGE_BACKUP set can fix this.
		return r.updateStatus(ctx, sg, satisfactoryv1alpha1.SaveGamePhaseFailed,
			"BackupImageNotConfigured", "RELATED_IMAGE_BACKUP is not set on the operator", ctrl.Result{})
	}
	server, err := r.authenticatedServer(ctx, sg.Spec.ServerRef, sg.Namespace)
	if err != nil {
		return r.serverProblem(ctx, sg, err)
	}

	at := now.UTC().Truncate(time.Second)
	saveName := sg.Spec.SaveName + "_" + at.Format("20060102-150405")
	job, err = buildBackupJob(sg, r.Scheme, r.BackupImage, server.Name, r.gameAPIURL(server), saveName, at)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, job); err != nil {
		return ctrl.Result{}, fmt.Errorf("create backup Job: %w", err)
	}
	log.Info("Created backup Job", "job", job.Name, "saveName", saveName)
	if sched != nil {
		sg.Status.NextBackupTime = &metav1.Time{Time: sched.Next(at)}
	}
	if sg.Status.Phase == "" {
		r.setReady(sg, satisfactoryv1alpha1.SaveGamePhasePending, metav1.ConditionUnknown,
			"BackupRunning", "First backup Job is running")
	}
	return ctrl.Result{}, r.writeStatus(ctx, sg)
}

// serverProblem records a server lookup or authentication failure and retries soon.
func (r *SaveGameReconciler) serverProblem(ctx context.Context, sg *satisfactoryv1alpha1.SaveGame, err error) (ctrl.Result, error) {
	var he *authHelperError
	if !errors.As(err, &he) {
		return ctrl.Result{}, err
	}
	phase := satisfactoryv1alpha1.SaveGamePhasePending
	if he.Reason == reasonServerNotFound {
		phase = satisfactoryv1alpha1.SaveGamePhaseFailed
	}
	return r.updateStatus(ctx, sg, phase, he.Reason, he.Message,
		ctrl.Result{RequeueAfter: saveGameRetryInterval})
}

// latestBackupJob returns the newest Job owned by sg, or nil.
func (r *SaveGameReconciler) latestBackupJob(ctx context.Context, sg *satisfactoryv1alpha1.SaveGame) (*batchv1.Job, error) {
	jobs := &batchv1.JobList{}
	if err := r.List(ctx, jobs, client.InNamespace(sg.Namespace)); err != nil {
		return nil, fmt.Errorf("list Jobs: %w", err)
	}
	var latest *batchv1.Job
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if !metav1.IsControlledBy(j, sg) {
			continue
		}
		if t := jobRunTime(j); latest == nil || (t != nil && t.After(*jobRunTime(latest))) {
			latest = j
		}
	}
	return latest, nil
}

// recordJobResult reads a finished Job's result from its pod's termination message into status.
func (r *SaveGameReconciler) recordJobResult(ctx context.Context, sg *satisfactoryv1alpha1.SaveGame, job *batchv1.Job, succeeded bool) {
	at := jobRunTime(job)
	if at == nil {
		return
	}
	if succeeded && sg.Status.LastBackupTime != nil && sg.Status.LastBackupTime.Equal(&metav1.Time{Time: *at}) {
		return // already recorded
	}
	msg, err := r.terminationMessage(ctx, job)
	var res backup.Result
	if err != nil || json.Unmarshal([]byte(msg), &res) != nil || res.SaveName == "" {
		if err != nil {
			msg = err.Error()
		}
		if jm := jobFailureMessage(job); jm != "" {
			msg = jm + ": " + msg
		}
		r.setReady(sg, satisfactoryv1alpha1.SaveGamePhaseFailed, metav1.ConditionFalse, "BackupJobFailed",
			truncateMessage("Backup Job "+job.Name+" failed: "+msg))
		return
	}
	phase, status, reason, message := applyBackupResult(sg, res, *at)
	r.setReady(sg, phase, status, reason, message)
}

// terminationMessage returns the backup container's termination message from the Job's pod.
// Pods are not cached, so this reads from the API server.
func (r *SaveGameReconciler) terminationMessage(ctx context.Context, job *batchv1.Job) (string, error) {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(job.Namespace),
		client.MatchingLabels{batchv1.JobNameLabel: job.Name}); err != nil {
		return "", fmt.Errorf("list pods of Job %s: %w", job.Name, err)
	}
	for _, p := range pods.Items {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name == backupContainerName && cs.State.Terminated != nil {
				return cs.State.Terminated.Message, nil
			}
		}
	}
	return "", fmt.Errorf("no terminated pod found for Job %s", job.Name)
}

// truncateMessage keeps a condition message short; termination messages can hold log tails.
func truncateMessage(s string) string {
	const maxLen = 1024
	if len(s) <= maxLen {
		return s
	}
	return strings.ToValidUTF8(s[:maxLen], "")
}

// setReady sets Phase and the Ready condition.
func (r *SaveGameReconciler) setReady(
	sg *satisfactoryv1alpha1.SaveGame,
	phase satisfactoryv1alpha1.SaveGamePhase,
	status metav1.ConditionStatus,
	reason, message string,
) {
	sg.Status.Phase = phase
	conditions.SetSaveGame(sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, status, reason, message)
}

func (r *SaveGameReconciler) writeStatus(ctx context.Context, sg *satisfactoryv1alpha1.SaveGame) error {
	if err := r.Status().Update(ctx, sg); err != nil {
		return fmt.Errorf("update SaveGame status: %w", err)
	}
	return nil
}

// updateStatus sets Phase and Ready=False, writes status and returns res.
func (r *SaveGameReconciler) updateStatus(
	ctx context.Context,
	sg *satisfactoryv1alpha1.SaveGame,
	phase satisfactoryv1alpha1.SaveGamePhase,
	reason, message string,
	res ctrl.Result,
) (ctrl.Result, error) {
	r.setReady(sg, phase, metav1.ConditionFalse, reason, message)
	return res, r.writeStatus(ctx, sg)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SaveGameReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &satisfactoryv1alpha1.SaveGame{},
		saveGameServerRefIndex, indexSaveGameServerRef); err != nil {
		return fmt.Errorf("index SaveGame %s: %w", saveGameServerRefIndex, err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&satisfactoryv1alpha1.SaveGame{}).
		Owns(&batchv1.Job{}).
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
