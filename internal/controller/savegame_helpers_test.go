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
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/backup"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
)

// fakeGameAPI is a game server API covering login. It records SaveGame calls so tests can
// check that the reconciler never saves itself (the backup Job does).
type fakeGameAPI struct {
	saved     []string
	failLogin bool
}

func (f *fakeGameAPI) start(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewTLSServer(f.handler())
	t.Cleanup(ts.Close)
	return ts
}

func (f *fakeGameAPI) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Data struct {
				SaveName string `json:"saveName"`
			} `json:"data"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordLogin":
			if f.failLogin {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"errorCode":"wrong_password"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"authenticationToken": "tok"}})
		case "QueryServerState":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"serverGameState": map[string]any{}}})
		case "SaveGame":
			f.saved = append(f.saved, req.Data.SaveName)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

// reachableServer returns a SatisfactoryServer with ServerReachable=True and admin credentials,
// plus the password Secret it references.
func reachableServer() (*satisfactoryv1alpha1.SatisfactoryServer, *corev1.Secret) {
	const name = "srv"
	server := newTestServer(name)
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: name + "-admin"}
	conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable,
		metav1.ConditionTrue, "Reachable", "")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-admin", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("pass")},
	}
	return server, secret
}

func newSaveGameReconciler(t *testing.T, ts *httptest.Server, objs ...client.Object) *SaveGameReconciler {
	t.Helper()
	s := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).
		WithStatusSubresource(&satisfactoryv1alpha1.SaveGame{}).Build()
	return &SaveGameReconciler{
		Client:      c,
		Scheme:      s,
		BackupImage: "operator:test",
		GameAPIURL:  func(*satisfactoryv1alpha1.SatisfactoryServer) string { return ts.URL },
	}
}

func TestAuthenticatedServer(t *testing.T) {
	server, secret := reachableServer()
	unreachable := newTestServer("down")

	tests := []struct {
		name       string
		ref        string
		failLogin  bool
		wantReason string
	}{
		{name: "server found and authenticated", ref: "srv"},
		{name: "server not found", ref: "missing", wantReason: "ServerNotFound"},
		{name: "server not reachable", ref: "down", wantReason: "ServerNotReachable"},
		{name: "auth failure", ref: "srv", failLogin: true, wantReason: "ServerNotAuthenticated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := (&fakeGameAPI{failLogin: tt.failLogin}).start(t)
			r := newSaveGameReconciler(t, ts, server.DeepCopy(), unreachable.DeepCopy(), secret.DeepCopy())

			srv, err := r.authenticatedServer(context.Background(), tt.ref, "default")
			if tt.wantReason == "" {
				if err != nil || srv == nil {
					t.Fatalf("expected server, got err %v", err)
				}
				return
			}
			var he *authHelperError
			if !isAuthHelperError(err, &he) || he.Reason != tt.wantReason {
				t.Fatalf("expected reason %s, got %v", tt.wantReason, err)
			}
		})
	}
}

func TestNextBackup(t *testing.T) {
	hourly, err := cron.ParseStandard("0 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	daily, err := cron.ParseStandard("0 0 * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 19, 10, 30, 0, 0, time.UTC)
	at := func(h, m int) *metav1.Time { return &metav1.Time{Time: time.Date(2026, 10, 19, h, m, 0, 0, time.UTC)} }
	jobAt := at(10, 0).Time

	tests := []struct {
		name    string
		sched   cron.Schedule
		status  satisfactoryv1alpha1.SaveGameStatus
		lastRun *time.Time
		want    time.Time
		wantOK  bool
	}{
		{name: "scheduled, never ran: now", sched: hourly, want: now, wantOK: true},
		{name: "scheduled, last backup only", sched: hourly,
			status: satisfactoryv1alpha1.SaveGameStatus{LastBackupTime: at(9, 0)}, want: at(10, 0).Time, wantOK: true},
		{name: "scheduled, failed run waits for stored next tick", sched: hourly, lastRun: &jobAt,
			status: satisfactoryv1alpha1.SaveGameStatus{LastBackupTime: at(8, 0), NextBackupTime: at(11, 0)},
			want:   at(11, 0).Time, wantOK: true},
		{name: "schedule changed to run sooner", sched: hourly,
			status: satisfactoryv1alpha1.SaveGameStatus{NextBackupTime: &metav1.Time{Time: now.Add(24 * time.Hour)}},
			want:   at(11, 0).Time, wantOK: true},
		{name: "stored next tick of a slower schedule still runs once", sched: daily,
			status: satisfactoryv1alpha1.SaveGameStatus{NextBackupTime: at(11, 0)}, want: at(11, 0).Time, wantOK: true},
		{name: "one-shot, never ran: now", want: now, wantOK: true},
		{name: "one-shot, failed run retries later", lastRun: &jobAt, want: jobAt.Add(saveGameRetryInterval), wantOK: true},
		{name: "one-shot, done", status: satisfactoryv1alpha1.SaveGameStatus{LastBackupTime: at(9, 0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nextBackup(tt.sched, tt.status, tt.lastRun, now)
			if ok != tt.wantOK || !got.Equal(tt.want) {
				t.Errorf("nextBackup = %v, %v; want %v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestBuildBackupJob(t *testing.T) {
	sg := newSaveGame(strings.Repeat("a", 60), "")
	sg.Spec.RetentionPolicy = &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](5)}
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	job, err := buildBackupJob(sg, newTestScheme(t), "img:v1", "srv", "https://srv:7777", "save1_20261101-120000", at)
	if err != nil {
		t.Fatal(err)
	}
	if len(job.Name) > 63 || !strings.HasSuffix(job.Name, fmt.Sprintf("-%d", at.Unix())) {
		t.Errorf("bad name %q", job.Name)
	}
	if !metav1.IsControlledBy(job, sg) {
		t.Error("Job is not owned by the SaveGame")
	}
	if *job.Spec.BackoffLimit != 0 || *job.Spec.ActiveDeadlineSeconds != 1800 || *job.Spec.TTLSecondsAfterFinished != 3600 {
		t.Errorf("unexpected Job limits %+v", job.Spec)
	}
	pod := job.Spec.Template.Spec
	c := pod.Containers[0]
	if *pod.AutomountServiceAccountToken || !*pod.SecurityContext.RunAsNonRoot || pod.RestartPolicy != corev1.RestartPolicyNever {
		t.Error("pod must not mount a token, must run as non-root and never restart")
	}
	if !*c.SecurityContext.ReadOnlyRootFilesystem || *c.SecurityContext.AllowPrivilegeEscalation ||
		!slices.Equal(c.SecurityContext.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Errorf("container not hardened: %+v", c.SecurityContext)
	}
	if c.Image != "img:v1" || c.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError ||
		c.Resources.Limits.Memory().String() != "256Mi" || !c.Resources.Limits.Cpu().IsZero() {
		t.Errorf("unexpected container %+v", c)
	}
	wantArgs := []string{"backup", "--server-url=https://srv:7777", "--save-name=save1_20261101-120000",
		"--session-prefix=save1",
		`--config=[{"name":"server","retentionPolicy":{"maxCount":5},"server":{}}]`,
		"--token-file=/var/run/satisfactory/token"}
	if !slices.Equal(c.Args, wantArgs) {
		t.Errorf("args %v, want %v", c.Args, wantArgs)
	}
	vols := pod.Volumes
	if vols[0].EmptyDir.SizeLimit.String() != "2Gi" || vols[1].Secret.SecretName != "srv-auth-token" || !c.VolumeMounts[1].ReadOnly {
		t.Errorf("unexpected volumes %+v / mounts %+v", vols, c.VolumeMounts)
	}
	if *pod.SecurityContext.FSGroup != 65532 || *pod.SecurityContext.FSGroupChangePolicy != corev1.FSGroupChangeOnRootMismatch {
		t.Errorf("unexpected fsGroup %+v", pod.SecurityContext)
	}
	if len(vols) != 2 {
		t.Errorf("server-only Job has extra volumes %+v", vols)
	}
}

func TestBuildBackupJobPVCVolumes(t *testing.T) {
	sg := newSaveGame("sg", "")
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	sg.Spec.Destinations = []satisfactoryv1alpha1.SaveGameDestination{
		{Name: "local", Server: &satisfactoryv1alpha1.ServerDestination{}},
		{Name: "nas", PVC: &satisfactoryv1alpha1.PVCDestination{ClaimName: "backups", Path: "factory"}},
	}
	job, err := buildBackupJob(sg, newTestScheme(t), "img:v1", "srv", "https://srv:7777", "save1_20261101-120000", at)
	if err != nil {
		t.Fatal(err)
	}
	pod := job.Spec.Template.Spec
	vols, mounts := pod.Volumes, pod.Containers[0].VolumeMounts
	if len(vols) != 3 || vols[2].Name != "dest-nas" || vols[2].PersistentVolumeClaim.ClaimName != "backups" ||
		len(mounts) != 3 || mounts[2].Name != "dest-nas" || mounts[2].MountPath != "/backup/nas" || mounts[2].ReadOnly {
		t.Errorf("unexpected pvc volumes %+v / mounts %+v", vols, mounts)
	}
}

func TestBuildBackupJobS3Volumes(t *testing.T) {
	sg := newSaveGame("sg", "")
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	sg.Spec.Destinations = []satisfactoryv1alpha1.SaveGameDestination{
		{Name: "aws", S3: &satisfactoryv1alpha1.S3Destination{Bucket: "a",
			CredentialsSecretRef: &satisfactoryv1alpha1.S3CredentialsSecretRef{Name: "creds-a"}}},
		{Name: "rustfs", S3: &satisfactoryv1alpha1.S3Destination{Bucket: "b",
			CredentialsSecretRef: &satisfactoryv1alpha1.S3CredentialsSecretRef{Name: "creds-b"}}},
		{Name: "chain", S3: &satisfactoryv1alpha1.S3Destination{Bucket: "c"}},
	}
	job, err := buildBackupJob(sg, newTestScheme(t), "img:v1", "srv", "https://srv:7777", "save1_20261101-120000", at)
	if err != nil {
		t.Fatal(err)
	}
	pod := job.Spec.Template.Spec
	vols, mounts := pod.Volumes, pod.Containers[0].VolumeMounts
	if len(vols) != 4 || len(mounts) != 4 {
		t.Fatalf("want two s3 volumes, got %+v / %+v", vols, mounts)
	}
	for i, want := range []struct{ name, secret, path string }{
		{"dest-aws", "creds-a", "/var/run/backup/s3/aws"},
		{"dest-rustfs", "creds-b", "/var/run/backup/s3/rustfs"},
	} {
		v, m := vols[2+i], mounts[2+i]
		if v.Name != want.name || v.Secret.SecretName != want.secret || len(v.Secret.Items) != 2 ||
			m.Name != want.name || m.MountPath != want.path || !m.ReadOnly {
			t.Errorf("unexpected s3 volume %+v / mount %+v", v, m)
		}
	}
}

func TestApplyBackupResult(t *testing.T) {
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	earlier := metav1.Time{Time: at.Add(-time.Hour)}
	newSG := func() *satisfactoryv1alpha1.SaveGame {
		sg := newSaveGame("sg", "0 * * * *")
		sg.Spec.Destinations = []satisfactoryv1alpha1.SaveGameDestination{
			{Name: "local", Server: &satisfactoryv1alpha1.ServerDestination{}}, {Name: "offsite"},
		}
		sg.Status.Destinations = []satisfactoryv1alpha1.SaveGameDestinationStatus{
			{Name: "offsite", LastSuccessTime: &earlier}, {Name: "removed"},
		}
		return sg
	}

	t.Run("all succeed", func(t *testing.T) {
		sg := newSG()
		phase, status, reason, _ := applyBackupResult(sg, backup.Result{SaveName: "x", Saved: true, Size: 42,
			Destinations: []backup.DestinationResult{{Name: "local", OK: true, Retained: 3}, {Name: "offsite", OK: true, Retained: 7}}}, at)
		if phase != satisfactoryv1alpha1.SaveGamePhaseActive || status != metav1.ConditionTrue || reason != "Reconciled" {
			t.Fatalf("got %s %s %s", phase, status, reason)
		}
		d := sg.Status.Destinations
		if len(d) != 2 || !d[1].LastSuccessTime.Time.Equal(at) || *d[1].LastSize != 42 || sg.Status.RetainedSaveCount != 3 ||
			!sg.Status.LastBackupTime.Time.Equal(at) || *sg.Status.BackupSize != 42 {
			t.Errorf("unexpected status %+v", sg.Status)
		}
	})

	t.Run("one destination fails", func(t *testing.T) {
		sg := newSG()
		phase, status, reason, msg := applyBackupResult(sg, backup.Result{SaveName: "x", Saved: true, Size: 42,
			Destinations: []backup.DestinationResult{{Name: "local", OK: true, Retained: 3}, {Name: "offsite", Error: "boom"}}}, at)
		if phase != satisfactoryv1alpha1.SaveGamePhaseFailed || status != metav1.ConditionFalse || reason != "DestinationFailed" ||
			!strings.Contains(msg, "offsite: boom") {
			t.Fatalf("got %s %s %s %q", phase, status, reason, msg)
		}
		if off := sg.Status.Destinations[1]; off.LastError != "boom" || !off.LastSuccessTime.Equal(&earlier) {
			t.Errorf("unexpected offsite status %+v", off)
		}
	})

	t.Run("save failed", func(t *testing.T) {
		sg := newSG()
		phase, _, reason, msg := applyBackupResult(sg, backup.Result{SaveName: "x", Error: "save_failed",
			Destinations: []backup.DestinationResult{{Name: "local", Error: "skipped"}, {Name: "offsite", Error: "skipped"}}}, at)
		if phase != satisfactoryv1alpha1.SaveGamePhaseFailed || reason != "SaveFailed" || msg != "save_failed" ||
			sg.Status.LastBackupTime != nil {
			t.Fatalf("got %s %s %q, last backup %v", phase, reason, msg, sg.Status.LastBackupTime)
		}
	})
}
