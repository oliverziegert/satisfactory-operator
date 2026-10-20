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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// fakeGameAPI is an in-memory game server API covering the save-related functions.
type fakeGameAPI struct {
	saves     []satisfactory.SaveHeader
	saved     []string
	deleted   []string
	failSave  bool
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
		case "VerifyAuthenticationToken":
			w.WriteHeader(http.StatusNoContent)
		case "SaveGame":
			if f.failSave {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errorCode":"save_failed"}`))
				return
			}
			f.saved = append(f.saved, req.Data.SaveName)
			f.saves = append(f.saves, satisfactory.SaveHeader{
				SaveName: req.Data.SaveName, SaveDateTime: time.Now().UTC().Format(saveDateTimeLayout),
			})
			w.WriteHeader(http.StatusNoContent)
		case "EnumerateSessions":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": satisfactory.EnumerateSessionsResponse{
				Sessions: []satisfactory.SessionSaveStruct{{SessionName: "world", SaveHeaders: f.saves}},
			}})
		case "DeleteSaveFile":
			f.deleted = append(f.deleted, req.Data.SaveName)
			f.saves = slices.DeleteFunc(f.saves, func(h satisfactory.SaveHeader) bool { return h.SaveName == req.Data.SaveName })
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
		Client:     c,
		Scheme:     s,
		GameAPIURL: func(*satisfactoryv1alpha1.SatisfactoryServer) string { return ts.URL },
	}
}

func testClient(ts *httptest.Server) *satisfactory.Client {
	return satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
}

func TestResolveServerClient(t *testing.T) {
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

			c, err := r.resolveServerClient(context.Background(), tt.ref, "default")
			if tt.wantReason == "" {
				if err != nil || c == nil {
					t.Fatalf("expected client, got err %v", err)
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

func TestTriggerSave(t *testing.T) {
	api := &fakeGameAPI{}
	ts := api.start(t)
	now := time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC)

	name, err := triggerSave(context.Background(), testClient(ts), "mysave", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "mysave_20261019-140000" || !slices.Equal(api.saved, []string{name}) {
		t.Errorf("unexpected save name %q, saved %v", name, api.saved)
	}

	api.failSave = true
	if _, err := triggerSave(context.Background(), testClient(ts), "mysave", now); err == nil {
		t.Error("expected error when SaveGame fails")
	}
}

func TestEnumerateSessionSaves(t *testing.T) {
	api := &fakeGameAPI{saves: []satisfactory.SaveHeader{
		{SaveName: "mysave_b", SaveDateTime: "2026.10.19-12.00.00"},
		{SaveName: "other_a", SaveDateTime: "2026.10.19-10.00.00"},
		{SaveName: "mysave", SaveDateTime: "2026.10.18-12.00.00"},
		{SaveName: "mysaveextra", SaveDateTime: "2026.10.17-12.00.00"},
	}}
	ts := api.start(t)

	saves, err := enumerateSessionSaves(context.Background(), testClient(ts), "mysave")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := make([]string, 0, len(saves))
	for _, s := range saves {
		names = append(names, s.SaveName)
	}
	if want := []string{"mysave", "mysave_b"}; !slices.Equal(names, want) {
		t.Errorf("expected %v (oldest first), got %v", want, names)
	}
}

func TestEnforceRetention(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	saveAt := func(name string, ago time.Duration) satisfactory.SaveHeader {
		return satisfactory.SaveHeader{SaveName: name, SaveDateTime: now.Add(-ago).Format(saveDateTimeLayout)}
	}
	saves := []satisfactory.SaveHeader{
		saveAt("s1", 100*time.Hour), saveAt("s2", 80*time.Hour), saveAt("s3", 10*time.Hour), saveAt("s4", time.Hour),
	}

	tests := []struct {
		name        string
		policy      *satisfactoryv1alpha1.SaveGameRetentionPolicy
		wantDeleted []string
	}{
		{name: "no policy"},
		{name: "maxCount", policy: &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxCount: ptr.To[int32](3)},
			wantDeleted: []string{"s1"}},
		{name: "maxAge", policy: &satisfactoryv1alpha1.SaveGameRetentionPolicy{MaxAge: &metav1.Duration{Duration: 72 * time.Hour}},
			wantDeleted: []string{"s1", "s2"}},
		{name: "both", policy: &satisfactoryv1alpha1.SaveGameRetentionPolicy{
			MaxCount: ptr.To[int32](1), MaxAge: &metav1.Duration{Duration: 72 * time.Hour}},
			wantDeleted: []string{"s1", "s2", "s3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeGameAPI{saves: slices.Clone(saves)}
			ts := api.start(t)

			kept, err := enforceRetention(context.Background(), testClient(ts), slices.Clone(saves), tt.policy, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(api.deleted, tt.wantDeleted) {
				t.Errorf("deleted %v, want %v", api.deleted, tt.wantDeleted)
			}
			if len(kept) != len(saves)-len(tt.wantDeleted) {
				t.Errorf("kept %d saves, want %d", len(kept), len(saves)-len(tt.wantDeleted))
			}
		})
	}
}

func TestBackupDue(t *testing.T) {
	hourly, err := cron.ParseStandard("0 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	last := &metav1.Time{Time: time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)}

	tests := []struct {
		name  string
		sched cron.Schedule
		last  *metav1.Time
		now   time.Time
		want  bool
	}{
		{name: "no prior backup triggers immediately", sched: hourly, now: last.Time, want: true},
		{name: "next tick in the future", sched: hourly, last: last, now: last.Add(30 * time.Minute), want: false},
		{name: "next tick reached", sched: hourly, last: last, now: last.Add(time.Hour), want: true},
		{name: "no schedule, never saved: one-shot", now: last.Time, want: true},
		{name: "no schedule, already saved", last: last, now: last.Add(100 * time.Hour), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backupDue(tt.sched, tt.last, tt.now); got != tt.want {
				t.Errorf("backupDue = %v, want %v", got, tt.want)
			}
		})
	}
}
