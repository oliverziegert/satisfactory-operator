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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// --- probeGameServer tests ---

// newTLSTestServer wraps httptest.NewTLSServer for convenience.
func newTLSTestServer(handler http.Handler) *httptest.Server {
	return httptest.NewTLSServer(handler)
}

// newProbeTestServer serves QueryServerState and answers PasswordlessLogin(InitialAdmin)
// the way a claimed or unclaimed server does.
func newProbeTestServer(claimed bool) *httptest.Server {
	return newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Function string `json:"function"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Function == "PasswordlessLogin" {
			if claimed {
				// The real server sends this error with HTTP 200.
				_, _ = w.Write([]byte(`{"errorCode":"passwordless_login_not_possible"}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"initial-admin"}}`))
			return
		}
		// A fresh claimed server: world loaded, but no autoload session and game not running.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"serverGameState": map[string]any{
					"activeSessionName":   "MyWorld",
					"numConnectedPlayers": 2,
					"playerLimit":         4,
					"autoLoadSessionName": "",
					"isGameRunning":       false,
				},
			},
		})
	}))
}

func TestProbeGameServer_Success(t *testing.T) {
	ts := newProbeTestServer(true)
	defer ts.Close()

	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	rec := &SatisfactoryServerReconciler{}
	result := rec.probeGameServer(context.Background(), c)
	if !result.Reachable {
		t.Error("expected Reachable=true")
	}
	if !result.Claimed {
		t.Error("expected Claimed=true (passwordless InitialAdmin login refused)")
	}
	if result.ActiveSessionName != "MyWorld" {
		t.Errorf("expected ActiveSessionName=MyWorld, got %s", result.ActiveSessionName)
	}
	if result.NumPlayers != 2 {
		t.Errorf("expected NumPlayers=2, got %d", result.NumPlayers)
	}
}

func TestProbeGameServer_Unclaimed(t *testing.T) {
	ts := newProbeTestServer(false)
	defer ts.Close()

	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	rec := &SatisfactoryServerReconciler{}
	if rec.probeGameServer(context.Background(), c).Claimed {
		t.Error("expected Claimed=false (passwordless InitialAdmin login granted)")
	}
}

func TestProbeGameServer_AuthError(t *testing.T) {
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorCode":"forbidden_error","errorMessage":"not authenticated"}`))
	}))
	defer ts.Close()

	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	rec := &SatisfactoryServerReconciler{}
	result := rec.probeGameServer(context.Background(), c)
	if !result.Reachable {
		t.Error("expected Reachable=true on auth error (server is up)")
	}
	if result.Claimed {
		t.Error("expected Claimed=false on auth error")
	}
}

func TestProbeGameServer_ConnectionFailure(t *testing.T) {
	// Point at a port that refuses connections.
	c := satisfactory.NewClient("https://127.0.0.1:1", &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	rec := &SatisfactoryServerReconciler{}
	result := rec.probeGameServer(context.Background(), c)
	if result.Reachable {
		t.Error("expected Reachable=false on connection failure")
	}
}

// --- setProgressingCondition tests ---

func TestSetProgressingCondition_NotCreated(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	ss := &appsv1.StatefulSet{} // empty — no Name set
	rec := &SatisfactoryServerReconciler{}
	rec.setProgressingCondition(server, ss)

	if len(server.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(server.Status.Conditions))
	}
	c := server.Status.Conditions[0]
	if c.Type != satisfactoryv1alpha1.SatisfactoryServerConditionTypeProgressing {
		t.Errorf("expected Progressing condition, got %s", c.Type)
	}
	if c.Status != metav1.ConditionTrue {
		t.Errorf("expected True (StatefulSet not created), got %s", c.Status)
	}
}

func TestSetProgressingCondition_Rollout(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{UpdatedReplicas: 0, ReadyReplicas: 0},
	}
	rec := &SatisfactoryServerReconciler{}
	rec.setProgressingCondition(server, ss)

	if server.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Error("expected Progressing=True during rollout")
	}
	if server.Status.Conditions[0].Reason != "RolloutInProgress" {
		t.Errorf("expected reason RolloutInProgress, got %s", server.Status.Conditions[0].Reason)
	}
}

func TestSetProgressingCondition_Complete(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{UpdatedReplicas: 1, ReadyReplicas: 1},
	}
	rec := &SatisfactoryServerReconciler{}
	rec.setProgressingCondition(server, ss)

	if server.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Error("expected Progressing=False when rollout complete")
	}
}

// --- setGameConditions tests ---

func TestSetGameConditions_NilProbe(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	ss := &appsv1.StatefulSet{}
	rec := &SatisfactoryServerReconciler{}
	rec.setGameConditions(server, ss, nil)

	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady)
	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable)
	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeWorldLoaded)
	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded)
}

func TestSetGameConditions_Reachable_WorldLoaded(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	probe := &probeResult{
		Reachable:         true,
		Claimed:           true,
		ActiveSessionName: "World1",
		NumPlayers:        3,
	}
	rec := &SatisfactoryServerReconciler{}
	rec.setGameConditions(server, ss, probe)

	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady)
	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable)
	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeWorldLoaded)
	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerClaimed)
	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded)

	if server.Status.ConnectedPlayers != 3 {
		t.Errorf("expected ConnectedPlayers=3, got %d", server.Status.ConnectedPlayers)
	}
}

func TestSetGameConditions_Degraded(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	probe := &probeResult{
		Reachable: false,
	}
	rec := &SatisfactoryServerReconciler{}
	rec.setGameConditions(server, ss, probe)

	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady)
	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable)
	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded)
}

// --- helpers ---

func checkConditionTrue(t *testing.T, server *satisfactoryv1alpha1.SatisfactoryServer, condType string) {
	t.Helper()
	for _, c := range server.Status.Conditions {
		if c.Type == condType {
			if c.Status != metav1.ConditionTrue {
				t.Errorf("condition %s: expected True, got %s", condType, c.Status)
			}
			return
		}
	}
	t.Errorf("condition %s not found", condType)
}

func checkConditionFalse(t *testing.T, server *satisfactoryv1alpha1.SatisfactoryServer, condType string) {
	t.Helper()
	for _, c := range server.Status.Conditions {
		if c.Type == condType {
			if c.Status != metav1.ConditionFalse {
				t.Errorf("condition %s: expected False, got %s", condType, c.Status)
			}
			return
		}
	}
	t.Errorf("condition %s not found", condType)
}
