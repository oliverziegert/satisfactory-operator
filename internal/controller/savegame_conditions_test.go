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
	"slices"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       satisfactoryv1alpha1.SaveGameSpec{ServerRef: "srv", SaveName: "save1", Schedule: schedule},
	}
}

func TestSaveGameReconcile_OneShot(t *testing.T) {
	api := &fakeGameAPI{}
	server, secret := reachableServer()
	r := newSaveGameReconciler(t, api.start(t), server, secret, newSaveGame("sg1", ""))

	res, sg := reconcileSaveGame(t, r, "sg1")
	if sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseCompleted || len(api.saved) != 1 || res.RequeueAfter != 0 {
		t.Fatalf("expected one save and Completed, got phase %s, saves %v, requeue %v", sg.Status.Phase, api.saved, res.RequeueAfter)
	}
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionTrue)
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled, metav1.ConditionFalse)
	if sg.Status.LastBackupTime == nil || sg.Status.RetainedSaveCount != 1 || sg.Status.NextBackupTime != nil {
		t.Errorf("unexpected status %+v", sg.Status)
	}

	// A second reconcile must not save again.
	_, sg = reconcileSaveGame(t, r, "sg1")
	if len(api.saved) != 1 || sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseCompleted {
		t.Errorf("expected no new save, got %v (phase %s)", api.saved, sg.Status.Phase)
	}
}

func TestSaveGameReconcile_ScheduleFirstBackup(t *testing.T) {
	api := &fakeGameAPI{}
	server, secret := reachableServer()
	sg := newSaveGame("sg2", "0 * * * *")
	sg.Spec.RetentionPolicy = &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](1)}
	api.saves = []satisfactory.SaveHeader{{SaveName: "save1_old", SaveDateTime: "2020.01.01-00.00.00"}}
	r := newSaveGameReconciler(t, api.start(t), server, secret, sg)

	res, sg := reconcileSaveGame(t, r, "sg2")
	if sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseActive || len(api.saved) != 1 {
		t.Fatalf("expected a save and Active, got phase %s, saves %v", sg.Status.Phase, api.saved)
	}
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionTrue)
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled, metav1.ConditionTrue)
	if !slices.Equal(api.deleted, []string{"save1_old"}) || sg.Status.RetainedSaveCount != 1 {
		t.Errorf("expected retention to delete save1_old, deleted %v, retained %d", api.deleted, sg.Status.RetainedSaveCount)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter > time.Hour || sg.Status.NextBackupTime == nil {
		t.Errorf("expected requeue at next hourly tick, got %v (next %v)", res.RequeueAfter, sg.Status.NextBackupTime)
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
	if len(api.saved) != 0 || sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseActive {
		t.Fatalf("expected no save and Active, got saves %v, phase %s", api.saved, sg.Status.Phase)
	}
	if want := time.Until(lastTick.Add(time.Hour)); (res.RequeueAfter - want).Abs() > time.Second {
		t.Errorf("expected RequeueAfter ~%v, got %v", want, res.RequeueAfter)
	}
}

func TestSaveGameReconcile_SaveFailed(t *testing.T) {
	api := &fakeGameAPI{failSave: true}
	server, secret := reachableServer()
	r := newSaveGameReconciler(t, api.start(t), server, secret, newSaveGame("sg4", "0 * * * *"))

	res, sg := reconcileSaveGame(t, r, "sg4")
	if sg.Status.Phase != satisfactoryv1alpha1.SaveGamePhaseFailed || res.RequeueAfter <= 0 {
		t.Fatalf("expected Failed with requeue, got phase %s, requeue %v", sg.Status.Phase, res.RequeueAfter)
	}
	checkSaveGameCondition(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionFalse, "SaveFailed")
	checkSaveGameConditionStatus(t, sg, satisfactoryv1alpha1.SaveGameConditionTypeBackupScheduled, metav1.ConditionTrue)
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
			if sg.Status.Phase != tt.wantPhase || res.RequeueAfter != saveGameRetryInterval || len(api.saved) != 0 {
				t.Fatalf("got phase %s, requeue %v, saves %v", sg.Status.Phase, res.RequeueAfter, api.saved)
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
