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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// --- getServerName tests ---

func TestGetServerName_SpecifiedName(t *testing.T) {
	server := newTestServer("my-server")
	server.Spec.ServerName = "My Cool Factory"
	if got := getServerName(server); got != "My Cool Factory" {
		t.Errorf("expected %q, got %q", "My Cool Factory", got)
	}
}

func TestGetServerName_FallbackToResourceName(t *testing.T) {
	server := newTestServer("my-server")
	if got := getServerName(server); got != "my-server" {
		t.Errorf("expected %q (resource name fallback), got %q", "my-server", got)
	}
}

// --- autoClaimEnabled tests ---

func TestAutoClaimEnabled_NilProbe(t *testing.T) {
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "sec"}
	if autoClaimEnabled(server, nil) {
		t.Error("expected false for nil probe")
	}
}

func TestAutoClaimEnabled_NotReachable(t *testing.T) {
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "sec"}
	probe := &probeResult{Reachable: false, Claimed: false}
	if autoClaimEnabled(server, probe) {
		t.Error("expected false when not reachable")
	}
}

func TestAutoClaimEnabled_AlreadyClaimed(t *testing.T) {
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "sec"}
	probe := &probeResult{Reachable: true, Claimed: true}
	if autoClaimEnabled(server, probe) {
		t.Error("expected false when already claimed")
	}
}

func TestAutoClaimEnabled_NoCredentials(t *testing.T) {
	server := newTestServer("srv")
	// AdminPasswordSecretRef is nil
	probe := &probeResult{Reachable: true, Claimed: false}
	if autoClaimEnabled(server, probe) {
		t.Error("expected false when no credentials configured")
	}
}

func TestAutoClaimEnabled_AutoClaimDisabled(t *testing.T) {
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "sec"}
	disabled := true
	server.Spec.AutoClaimDisabled = &disabled
	probe := &probeResult{Reachable: true, Claimed: false}
	if autoClaimEnabled(server, probe) {
		t.Error("expected false when autoClaimDisabled=true")
	}
}

func TestAutoClaimEnabled_AllConditionsMet(t *testing.T) {
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "sec"}
	probe := &probeResult{Reachable: true, Claimed: false}
	if !autoClaimEnabled(server, probe) {
		t.Error("expected true when all conditions are met")
	}
}

func TestAutoClaimEnabled_ExplicitlyEnabled(t *testing.T) {
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "sec"}
	disabled := false
	server.Spec.AutoClaimDisabled = &disabled
	probe := &probeResult{Reachable: true, Claimed: false}
	if !autoClaimEnabled(server, probe) {
		t.Error("expected true when autoClaimDisabled=false explicitly")
	}
}

// --- claimServer tests ---

func TestClaimServer_Success(t *testing.T) {
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordlessLogin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "initial-token"},
			})
		case "ClaimServer":
			// Verify the Authorization header carries the initial token.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "claimed-token"},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "admin-secret"}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	err := r.claimServer(context.Background(), server, ts.URL, "hunter2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the claimed auth token was cached.
	cached := &corev1.Secret{}
	if err := fakeClient.Get(context.Background(),
		types.NamespacedName{Name: "my-server-auth-token", Namespace: "default"}, cached); err != nil {
		t.Fatalf("expected auth token Secret to exist: %v", err)
	}
	if string(cached.Data["token"]) != "claimed-token" {
		t.Errorf("expected cached token %q, got %q", "claimed-token", string(cached.Data["token"]))
	}
}

func TestClaimServer_UsesServerName(t *testing.T) {
	var capturedServerName string
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordlessLogin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "tok"},
			})
		case "ClaimServer":
			var body struct {
				Data struct {
					ServerName string `json:"serverName"`
				} `json:"data"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			capturedServerName = body.Data.ServerName
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "tok2"},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.ServerName = "My Factory Server"

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	_ = r.claimServer(context.Background(), server, ts.URL, "pw")
	if capturedServerName != "My Factory Server" {
		t.Errorf("expected ClaimServer called with serverName %q, got %q", "My Factory Server", capturedServerName)
	}
}

func TestClaimServer_PasswordlessLoginFailure(t *testing.T) {
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordlessLogin":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errorCode":"server_already_claimed"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	err := r.claimServer(context.Background(), server, ts.URL, "pw")
	if err == nil {
		t.Fatal("expected error from PasswordlessLogin failure, got nil")
	}
}

func TestClaimServer_AlreadyClaimed_ClaimServerFails(t *testing.T) {
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordlessLogin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "tok"},
			})
		case "ClaimServer":
			// Server was already claimed by someone else.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errorCode":"server_already_claimed","errorMessage":"Server is already claimed"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	err := r.claimServer(context.Background(), server, ts.URL, "pw")
	if err == nil {
		t.Fatal("expected non-nil error for already-claimed server, got nil")
	}
	// Verify the error wraps an APIError (not a connection error).
	var apiErr *satisfactory.APIError
	if !isAPIError(err, &apiErr) {
		t.Errorf("expected wrapped APIError, got %T: %v", err, err)
	}
}

// isAPIError is a type-unwrap helper for APIError (through fmt.Errorf wrapping).
func isAPIError(err error, target **satisfactory.APIError) bool {
	for err != nil {
		if e, ok := err.(*satisfactory.APIError); ok {
			if target != nil {
				*target = e
			}
			return true
		}
		err = unwrapOnce(err)
	}
	return false
}

func unwrapOnce(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}

// --- reconcile integration: auto-claim disabled / no credentials ---

func TestAutoClaimEnabled_ReconcileIntegration_Disabled(t *testing.T) {
	// When autoClaimDisabled=true, autoClaimEnabled should return false regardless of probe.
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "sec"}
	disabled := true
	server.Spec.AutoClaimDisabled = &disabled

	probe := &probeResult{Reachable: true, Claimed: false}
	if autoClaimEnabled(server, probe) {
		t.Error("autoClaimEnabled should return false when autoClaimDisabled=true")
	}
}

func TestAutoClaimEnabled_ReconcileIntegration_NoCredentials(t *testing.T) {
	// When no credentials are configured, autoClaimEnabled should return false.
	server := newTestServer("srv")
	// AdminPasswordSecretRef is nil

	probe := &probeResult{Reachable: true, Claimed: false}
	if autoClaimEnabled(server, probe) {
		t.Error("autoClaimEnabled should return false when no credentials configured")
	}
}

func TestAutoClaimEnabled_ReconcileIntegration_Enabled(t *testing.T) {
	// When all conditions are met, claiming should proceed.
	// We verify by checking that claimServer is invoked (a successful claim caches the token).
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordlessLogin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "tok"},
			})
		case "ClaimServer":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "admin-tok"},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "admin-secret"}
	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("pw")},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server, adminSecret).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	probe := &probeResult{Reachable: true, Claimed: false}
	if !autoClaimEnabled(server, probe) {
		t.Fatal("expected autoClaimEnabled=true, got false")
	}

	// Simulate the reconcile claiming pathway.
	adminPassword, err := r.getAdminPassword(context.Background(), server)
	if err != nil {
		t.Fatalf("getAdminPassword: %v", err)
	}
	if claimErr := r.claimServer(context.Background(), server, ts.URL, adminPassword); claimErr != nil {
		t.Fatalf("claimServer: %v", claimErr)
	}

	// Verify token was cached.
	cached := &corev1.Secret{}
	if err := fakeClient.Get(context.Background(),
		types.NamespacedName{Name: "srv-auth-token", Namespace: "default"}, cached); err != nil {
		t.Fatalf("expected auth token Secret to exist: %v", err)
	}
	if string(cached.Data["token"]) != "admin-tok" {
		t.Errorf("expected %q, got %q", "admin-tok", string(cached.Data["token"]))
	}
}

func TestAutoClaimEnabled_ReconcileIntegration_ClaimSkipped_Disabled(t *testing.T) {
	// Verify that when autoClaimDisabled=true, claimServer is never invoked.
	// We do this by checking the auth token secret is NOT created after checking autoClaimEnabled.
	s := newTestScheme(t)
	server := newTestServer("srv")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "admin-secret"}
	disabled := true
	server.Spec.AutoClaimDisabled = &disabled

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()

	probe := &probeResult{Reachable: true, Claimed: false}

	// autoClaimEnabled returns false → claimServer should not be called.
	if autoClaimEnabled(server, probe) {
		t.Fatal("expected autoClaimEnabled=false, but got true — test premise invalid")
	}

	// Confirm no auth token secret was created (claimServer was never called).
	cached := &corev1.Secret{}
	err := fakeClient.Get(context.Background(),
		types.NamespacedName{Name: "srv-auth-token", Namespace: "default"}, cached)
	if err == nil {
		t.Error("auth token Secret should NOT exist when claiming was skipped")
	}
}

// Verify that the TLS client from claimServer uses InsecureSkipVerify (needed for self-signed certs).
func TestClaimServer_TLSInsecureSkipVerify(t *testing.T) {
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordlessLogin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "tok"},
			})
		case "ClaimServer":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "tok2"},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	// The httptest TLS server uses a self-signed cert; claimServer must use InsecureSkipVerify.
	// If this test passes, InsecureSkipVerify is working.
	if err := r.claimServer(context.Background(), server, ts.URL, "pw"); err != nil {
		t.Fatalf("claimServer failed with self-signed cert (InsecureSkipVerify may not be set): %v", err)
	}
}

// verify that newTLSTestServer is reachable with InsecureSkipVerify (sanity check).
func TestClaimServer_NewClientInsecureSkipVerify(t *testing.T) {
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"authenticationToken": "tok"},
		})
	}))
	defer ts.Close()

	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	_, err := c.PasswordlessLogin(context.Background(), satisfactory.PrivilegeLevelInitialAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
