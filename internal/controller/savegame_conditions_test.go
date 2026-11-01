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
	"slices"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/backup"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := satisfactoryv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return s
}

func reconcileSaveGame(t *testing.T, r *SaveGameReconciler, name string) (reconcile.Result, *satisfactoryv1alpha1.SaveGame) {
	t.Helper()
	key := types.NamespacedName{Name: name, Namespace: "default"}
	res, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	updated := &satisfactoryv1alpha1.SaveGame{}
	if err := r.Get(context.Background(), key, updated); err != nil {
		t.Fatalf("get updated: %v", err)
	}
	return res, updated
}

func newSaveGame(name, schedule string) *satisfactoryv1alpha1.SaveGame {
	return &satisfactoryv1alpha1.SaveGame{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID(name + "-uid")},
		Spec:       satisfactoryv1alpha1.SaveGameSpec{ServerRef: "srv", SaveName: "save1", Schedule: schedule},
	}
}

// ownedJobs returns the backup Jobs in the fake cluster.
func ownedJobs(t *testing.T, r *SaveGameReconciler) []batchv1.Job {
	t.Helper()
	jobs := &batchv1.JobList{}
	if err := r.List(context.Background(), jobs); err != nil {
		t.Fatal(err)
	}
	return jobs.Items
}

// finishJob marks job finished and creates its pod with the given termination message,
// as the Job controller and kubelet would.
func finishJob(t *testing.T, r *SaveGameReconciler, job *batchv1.Job, succeeded bool, message string) {
	t.Helper()
	ctx := context.Background()
	condType := batchv1.JobFailed
	if succeeded {
		condType = batchv1.JobComplete
	}
	job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{Type: condType, Status: corev1.ConditionTrue})
	if err := r.Status().Update(ctx, job); err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-pod", Namespace: job.Namespace,
			Labels: map[string]string{batchv1.JobNameLabel: job.Name}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: backupContainerName,
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Message: message}}}}},
	}
	if err := r.Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
}

func resultJSON(t *testing.T, res backup.Result) string {
	t.Helper()
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSaveGameReconcile_OneShot(t *testing.T) {
	api := &fakeGameAPI{}
	server, secret := reachableServer()
	r := newSaveGameReconciler(t, api.start(t), server, secret, newSaveGame("sg1", ""))

	res, sg := reconcileSaveGame(t, r, "sg1")
	jobs := ownedJobs(t, r)
	if len(jobs) != 1 || len(api.saved) != 0 || res.RequeueAfter != 0 {
		t.Fatalf("expected one Job and no inline save, got %d Jobs, saves %v, requeue %v", len(jobs), api.saved, res.RequeueAfter)
	}
	checkSaveGameCondition(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionUnknown, "BackupRunning")

	// No second Job while the first is running.
	reconcileSaveGame(t, r, "sg1")
	if n := len(ownedJobs(t, r)); n != 1 {
		t.Fatalf("expected no second Job while running, got %d", n)
	}

	finishJob(t, r, &jobs[0], true, resultJSON(t, backup.Result{SaveName: "save1_x", Saved: true, Size: 10,
		Destinations: []backup.DestinationResult{{Name: "server", OK: true, Retained: 2}}}))
	res, sg = reconcileSaveGame(t, r, "sg1")
	if sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseCompleted || res.RequeueAfter != 0 {
		t.Fatalf("expected Completed, got phase %s, requeue %v", sg.Status.Phase, res.RequeueAfter)
	}
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionTrue)
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled, metav1.ConditionFalse)
	if sg.Status.LastBackupTime == nil || sg.Status.RetainedSaveCount != 2 || *sg.Status.BackupSize != 10 ||
		sg.Status.NextBackupTime != nil || len(sg.Status.Destinations) != 1 {
		t.Errorf("unexpected status %+v", sg.Status)
	}

	// A completed one-shot never creates another Job.
	reconcileSaveGame(t, r, "sg1")
	if n := len(ownedJobs(t, r)); n != 1 {
		t.Errorf("expected no new Job, got %d", n)
	}
}

func TestSaveGameReconcile_ScheduleFirstBackup(t *testing.T) {
	server, secret := reachableServer()
	r := newSaveGameReconciler(t, (&fakeGameAPI{}).start(t), server, secret, newSaveGame("sg2", "0 * * * *"))

	_, sg := reconcileSaveGame(t, r, "sg2")
	if n := len(ownedJobs(t, r)); n != 1 {
		t.Fatalf("expected one Job, got %d", n)
	}
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled, metav1.ConditionTrue)
	if sg.Status.NextBackupTime == nil || time.Until(sg.Status.NextBackupTime.Time) > time.Hour {
		t.Errorf("expected next backup within the hour, got %v", sg.Status.NextBackupTime)
	}
}

func TestSaveGameReconcile_NotDue(t *testing.T) {
	api := &fakeGameAPI{}
	server, secret := reachableServer()
	sg := newSaveGame("sg3", "0 * * * *")
	// Last backup on the current hour's tick, so the next one is the following hour.
	lastTick := time.Now().Truncate(time.Hour)
	sg.Status.LastBackupTime = &metav1.Time{Time: lastTick}
	r := newSaveGameReconciler(t, api.start(t), server, secret, sg)

	res, sg := reconcileSaveGame(t, r, "sg3")
	if len(ownedJobs(t, r)) != 0 || sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseActive {
		t.Fatalf("expected no Job and Active, got phase %s", sg.Status.Phase)
	}
	if want := time.Until(lastTick.Add(time.Hour)); (res.RequeueAfter - want).Abs() > time.Second {
		t.Errorf("expected RequeueAfter ~%v, got %v", want, res.RequeueAfter)
	}
}

func TestSaveGameReconcile_FailedRuns(t *testing.T) {
	tests := []struct {
		name       string
		succeeded  bool
		message    string
		wantReason string
	}{
		{name: "save failed", message: `{"saveName":"save1_x","saved":false,"error":"save_failed",` +
			`"destinations":[{"name":"server","ok":false,"error":"skipped"}]}`, wantReason: "SaveFailed"},
		{name: "destination failed", message: `{"saveName":"save1_x","saved":true,"size":1,` +
			`"destinations":[{"name":"server","ok":false,"error":"boom"}]}`, wantReason: "DestinationFailed"},
		{name: "crashed without result", message: "panic: oops", wantReason: "BackupJobFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, secret := reachableServer()
			r := newSaveGameReconciler(t, (&fakeGameAPI{}).start(t), server, secret, newSaveGame("sg4", "0 * * * *"))
			reconcileSaveGame(t, r, "sg4")
			jobs := ownedJobs(t, r)
			finishJob(t, r, &jobs[0], tt.succeeded, tt.message)

			res, sg := reconcileSaveGame(t, r, "sg4")
			if sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseFailed || res.RequeueAfter <= 0 || res.RequeueAfter > time.Hour {
				t.Fatalf("expected Failed with requeue at the next tick, got phase %s, requeue %v", sg.Status.Phase, res.RequeueAfter)
			}
			checkSaveGameCondition(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionFalse, tt.wantReason)
			if n := len(ownedJobs(t, r)); n != 1 {
				t.Errorf("a failed run must not start another Job before the next tick, got %d Jobs", n)
			}
		})
	}
}

func TestSaveGameReconcile_BackupImageNotConfigured(t *testing.T) {
	server, secret := reachableServer()
	r := newSaveGameReconciler(t, (&fakeGameAPI{}).start(t), server, secret, newSaveGame("sg7", ""))
	r.BackupImage = ""

	_, sg := reconcileSaveGame(t, r, "sg7")
	if n := len(ownedJobs(t, r)); n != 0 {
		t.Fatalf("expected no Job, got %d", n)
	}
	checkSaveGameCondition(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionFalse, "BackupImageNotConfigured")
}

func TestSaveGameReconcile_ServerProblems(t *testing.T) {
	tests := []struct {
		name       string
		objs       func() []client.Object
		wantPhase  satisfactoryv1alpha1.SaveGamePhase
		wantReason string
	}{
		{name: "server not found", objs: func() []client.Object { return nil },
			wantPhase: satisfactoryv1alpha1.SaveGamePhaseFailed, wantReason: "ServerNotFound"},
		{name: "server not reachable", objs: func() []client.Object { return []client.Object{newTestServer("srv")} },
			wantPhase: satisfactoryv1alpha1.SaveGamePhasePending, wantReason: "ServerNotReachable"},
		{name: "no admin secret ref", objs: func() []client.Object {
			server, _ := reachableServer()
			server.Spec.AdminPasswordSecretRef = nil
			return []client.Object{server}
		}, wantPhase: satisfactoryv1alpha1.SaveGamePhasePending, wantReason: "ServerNotAuthenticated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeGameAPI{}
			r := newSaveGameReconciler(t, api.start(t), append(tt.objs(), newSaveGame("sg5", ""))...)

			res, sg := reconcileSaveGame(t, r, "sg5")
			if sg.Status.Phase != tt.wantPhase || res.RequeueAfter != saveGameRetryInterval || len(ownedJobs(t, r)) != 0 {
				t.Fatalf("got phase %s, requeue %v", sg.Status.Phase, res.RequeueAfter)
			}
			checkSaveGameCondition(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionFalse, tt.wantReason)
		})
	}
}

func TestSaveGameReconcile_InvalidSchedule(t *testing.T) {
	r := newSaveGameReconciler(t, (&fakeGameAPI{}).start(t), newSaveGame("sg6", "not a cron"))

	res, sg := reconcileSaveGame(t, r, "sg6")
	if sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseFailed || res.RequeueAfter != 0 {
		t.Fatalf("expected Failed without requeue, got phase %s, requeue %v", sg.Status.Phase, res.RequeueAfter)
	}
	checkSaveGameCondition(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled, metav1.ConditionFalse, "InvalidSchedule")
}

func TestSaveGamesForServer(t *testing.T) {
	s := newTestScheme(t)
	other := newSaveGame("sg-other", "")
	other.Spec.ServerRef = "other"
	c := clientfake.NewClientBuilder().WithScheme(s).
		WithObjects(newSaveGame("sg-a", ""), newSaveGame("sg-b", ""), other).
		WithIndex(&satisfactoryv1alpha1.SaveGame{}, saveGameServerRefIndex, indexSaveGameServerRef).Build()
	r := &SaveGameReconciler{Client: c, Scheme: s}

	reqs := r.saveGamesForServer(context.Background(), newTestServer("srv"))
	names := make([]string, 0, len(reqs))
	for _, req := range reqs {
		names = append(names, req.Name)
	}
	slices.Sort(names)
	if want := []string{"sg-a", "sg-b"}; !slices.Equal(names, want) {
		t.Errorf("expected %v, got %v", want, names)
	}
}

func checkSaveGameCondition(t *testing.T, sg *satisfactoryv1alpha1.SaveGame, condType string, want metav1.ConditionStatus, reason string) {
	t.Helper()
	checkSaveGameConditionStatus(t, sg, condType, want)
	if c := meta.FindStatusCondition(sg.Status.Conditions, condType); c != nil && c.Reason != reason {
		t.Errorf("SaveGame condition %s: expected reason %s, got %s", condType, reason, c.Reason)
	}
}

func checkSaveGameConditionStatus(t *testing.T, sg *satisfactoryv1alpha1.SaveGame, condType string, want metav1.ConditionStatus) {
	t.Helper()
	for _, c := range sg.Status.Conditions {
		if c.Type == condType {
			if c.Status != want {
				t.Errorf("SaveGame condition %s: expected %s, got %s", condType, want, c.Status)
			}
			return
		}
	}
	t.Errorf("SaveGame condition %s not found", condType)
}
