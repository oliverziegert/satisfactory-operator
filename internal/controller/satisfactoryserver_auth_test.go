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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// newTestScheme returns a scheme with the required types registered.
func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("add k8s scheme: %v", err)
	}
	if err := satisfactoryv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add satisfactory scheme: %v", err)
	}
	return s
}

// newServer returns a minimal SatisfactoryServer for testing.
func newTestServer(name string) *satisfactoryv1alpha1.SatisfactoryServer {
	return &satisfactoryv1alpha1.SatisfactoryServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       "test-uid-1234",
		},
	}
}

// --- getAdminPassword tests ---

func TestGetAdminPassword_Success(t *testing.T) {
	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "my-secret"}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("hunter2")},
	}

	r := &SatisfactoryServerReconciler{
		Client: fake.NewClientBuilder().WithScheme(s).WithObjects(secret).Build(),
		Scheme: s,
	}

	pw, err := r.getAdminPassword(context.Background(), server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pw != "hunter2" {
		t.Errorf("expected password %q, got %q", "hunter2", pw)
	}
}

func TestGetAdminPassword_SecretMissing(t *testing.T) {
	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "missing-secret"}

	r := &SatisfactoryServerReconciler{
		Client: fake.NewClientBuilder().WithScheme(s).Build(),
		Scheme: s,
	}

	_, err := r.getAdminPassword(context.Background(), server)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var authErr *authHelperError
	if !isAuthHelperError(err, &authErr) || authErr.Reason != "SecretNotFound" {
		t.Errorf("expected SecretNotFound authHelperError, got %v", err)
	}
}

func TestGetAdminPassword_KeyMissing(t *testing.T) {
	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "my-secret"}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"other-key": []byte("value")},
	}

	r := &SatisfactoryServerReconciler{
		Client: fake.NewClientBuilder().WithScheme(s).WithObjects(secret).Build(),
		Scheme: s,
	}

	_, err := r.getAdminPassword(context.Background(), server)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var authErr *authHelperError
	if !isAuthHelperError(err, &authErr) || authErr.Reason != "SecretKeyMissing" {
		t.Errorf("expected SecretKeyMissing authHelperError, got %v", err)
	}
}

// --- getCachedAuthToken and cacheAuthToken tests ---

func TestGetCachedAuthToken_NotFound(t *testing.T) {
	s := newTestScheme(t)
	server := newTestServer("my-server")

	r := &SatisfactoryServerReconciler{
		Client: fake.NewClientBuilder().WithScheme(s).Build(),
		Scheme: s,
	}

	token, err := r.getCachedAuthToken(context.Background(), server)
	if err != nil {
		t.Fatalf("expected nil error for missing cache, got %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token for missing cache, got %q", token)
	}
}

func TestCacheAuthToken_Create(t *testing.T) {
	s := newTestScheme(t)
	server := newTestServer("my-server")

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	if err := r.cacheAuthToken(context.Background(), server, "my-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the Secret was created with the token.
	token, err := r.getCachedAuthToken(context.Background(), server)
	if err != nil {
		t.Fatalf("unexpected error reading cached token: %v", err)
	}
	if token != "my-token" {
		t.Errorf("expected token %q, got %q", "my-token", token)
	}
}

func TestCacheAuthToken_Update(t *testing.T) {
	s := newTestScheme(t)
	server := newTestServer("my-server")

	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server-auth-token", Namespace: "default"},
		Data:       map[string][]byte{"token": []byte("old-token")},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server, existing).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	if err := r.cacheAuthToken(context.Background(), server, "new-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	token, err := r.getCachedAuthToken(context.Background(), server)
	if err != nil {
		t.Fatalf("unexpected error reading cached token: %v", err)
	}
	if token != "new-token" {
		t.Errorf("expected updated token %q, got %q", "new-token", token)
	}
}

func TestCacheAuthToken_OwnerReference(t *testing.T) {
	s := newTestScheme(t)
	server := newTestServer("my-server")

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}

	if err := r.cacheAuthToken(context.Background(), server, "tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	created := &corev1.Secret{}
	if err := fakeClient.Get(context.Background(),
		types.NamespacedName{Name: "my-server-auth-token", Namespace: "default"}, created); err != nil {
		t.Fatalf("get created secret: %v", err)
	}
	if len(created.OwnerReferences) == 0 {
		t.Error("expected owner reference to be set on auth token Secret")
	}
	if created.OwnerReferences[0].Name != "my-server" {
		t.Errorf("expected owner name %q, got %q", "my-server", created.OwnerReferences[0].Name)
	}
}

// --- authenticateClient tests ---

func TestAuthenticateClient_SuccessfulLogin(t *testing.T) {
	// Test server returns a valid token on PasswordLogin.
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "VerifyAuthenticationToken":
			// No cached token yet — this shouldn't be called, but handle gracefully.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errorCode":"token_invalid"}`))
		case "PasswordLogin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "fresh-token"},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "admin-secret"}
	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("secret")},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server, adminSecret).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}
	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec

	r.authenticateClient(context.Background(), server, c)

	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated)
}

func TestAuthenticateClient_CachedTokenValid(t *testing.T) {
	// Test server accepts the cached token as valid.
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("function") {
		case "VerifyAuthenticationToken":
			// Return 204 — token is valid.
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "admin-secret"}

	cachedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server-auth-token", Namespace: "default"},
		Data:       map[string][]byte{"token": []byte("cached-token")},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server, cachedSecret).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}
	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec

	r.authenticateClient(context.Background(), server, c)

	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated)
}

func TestAuthenticateClient_CachedTokenExpired(t *testing.T) {
	// Cached token fails verification; re-authenticates successfully.
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "VerifyAuthenticationToken":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errorCode":"token_expired"}`))
		case "PasswordLogin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"authenticationToken": "new-token"},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "admin-secret"}

	cachedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server-auth-token", Namespace: "default"},
		Data:       map[string][]byte{"token": []byte("expired-token")},
	}
	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("pass")},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server, cachedSecret, adminSecret).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}
	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec

	r.authenticateClient(context.Background(), server, c)

	checkConditionTrue(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated)
}

func TestAuthenticateClient_LoginFailed(t *testing.T) {
	ts := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("function") {
		case "PasswordLogin":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errorCode":"wrong_password"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	s := newTestScheme(t)
	server := newTestServer("my-server")
	server.Spec.AdminPasswordSecretRef = &corev1.LocalObjectReference{Name: "admin-secret"}

	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("wrong")},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(server, adminSecret).Build()
	r := &SatisfactoryServerReconciler{Client: fakeClient, Scheme: s}
	c := satisfactory.NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec

	r.authenticateClient(context.Background(), server, c)

	checkConditionFalse(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated)
	checkConditionReason(t, server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated, "AuthenticationFailed")
}

// --- helpers ---

// isAuthHelperError is a type-assert helper used in tests.
func isAuthHelperError(err error, target **authHelperError) bool {
	if err == nil {
		return false
	}
	var e *authHelperError
	if e, _ = err.(*authHelperError); e != nil {
		*target = e
		return true
	}
	return false
}

// checkConditionReason fails the test if the named condition doesn't have the expected reason.
func checkConditionReason(t *testing.T, server *satisfactoryv1alpha1.SatisfactoryServer, condType, wantReason string) {
	t.Helper()
	for _, c := range server.Status.Conditions {
		if c.Type == condType {
			if c.Reason != wantReason {
				t.Errorf("condition %s: expected reason %q, got %q", condType, wantReason, c.Reason)
			}
			return
		}
	}
	t.Errorf("condition %s not found", condType)
}
