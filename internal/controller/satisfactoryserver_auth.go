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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// authHelperError is a structured error from authentication helpers that carries
// a condition Reason suitable for setting the Authenticated condition.
type authHelperError struct {
	Reason  string
	Message string
}

func (e *authHelperError) Error() string { return e.Message }

// authSecretNameFor returns the name of the managed auth token Secret for a server.
func authSecretNameFor(serverName string) string {
	return serverName + "-auth-token"
}

// getAdminPassword reads the admin password from the Secret referenced by
// server.Spec.AdminPasswordSecretRef. Returns the password string or an error.
// An *authHelperError is returned when the failure maps to a specific condition reason
// (SecretNotFound, SecretKeyMissing).
func (r *SatisfactoryServerReconciler) getAdminPassword(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) (string, error) {
	secretName := server.Spec.AdminPasswordSecretRef.Name
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: server.Namespace}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", &authHelperError{
				Reason:  "SecretNotFound",
				Message: fmt.Sprintf("Secret %q not found in namespace %q", secretName, server.Namespace),
			}
		}
		return "", fmt.Errorf("get secret %s: %w", secretName, err)
	}
	password, ok := secret.Data["password"]
	if !ok {
		return "", &authHelperError{
			Reason:  "SecretKeyMissing",
			Message: fmt.Sprintf("Secret %q does not contain key \"password\"", secretName),
		}
	}
	return string(password), nil
}

// getCachedAuthToken reads the cached auth token from the <server-name>-auth-token Secret.
// Returns an empty string (with nil error) when the Secret does not exist yet.
func (r *SatisfactoryServerReconciler) getCachedAuthToken(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer) (string, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: authSecretNameFor(server.Name), Namespace: server.Namespace}, secret)
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get auth token secret: %w", err)
	}
	return string(secret.Data["token"]), nil
}

// cacheAuthToken creates or updates the <server-name>-auth-token Secret with the given token.
// The Secret gets an owner reference pointing to the SatisfactoryServer so it is garbage
// collected when the server resource is deleted.
func (r *SatisfactoryServerReconciler) cacheAuthToken(ctx context.Context, server *satisfactoryv1alpha1.SatisfactoryServer, token string) error {
	secretName := authSecretNameFor(server.Name)
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: server.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: server.Namespace,
			},
			Data: map[string][]byte{"token": []byte(token)},
		}
		if err := controllerutil.SetControllerReference(server, newSecret, r.Scheme); err != nil {
			return fmt.Errorf("set owner reference on auth token secret: %w", err)
		}
		if err := r.Create(ctx, newSecret); err != nil {
			return fmt.Errorf("create auth token secret: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get auth token secret: %w", err)
	}
	existing.Data = map[string][]byte{"token": []byte(token)}
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("update auth token secret: %w", err)
	}
	return nil
}

// authenticateClient tries to authenticate the given client with the game server API.
//
// Flow:
//  1. If a cached token exists, verify it with an authenticated QueryServerState call — use it if valid.
//  2. If no valid cached token, read the admin password from adminPasswordSecretRef.
//  3. Call PasswordLogin; cache the resulting token.
//
// On success the Authenticated condition is set to True.
// On failure the condition is set to False with the appropriate reason, and the caller
// can still proceed with an unauthenticated probe.
func (r *SatisfactoryServerReconciler) authenticateClient(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
	c *satisfactory.Client,
) {
	log := logf.FromContext(ctx)

	// 1. Try cached token first.
	cachedToken, err := r.getCachedAuthToken(ctx, server)
	if err != nil {
		log.Error(err, "failed to read cached auth token, will re-authenticate")
	}
	if cachedToken != "" {
		c.SetAuthToken(cachedToken)
		// VerifyAuthenticationToken rejects valid tokens on v1.9.10 (#43); any authenticated call proves the token.
		_, verifyErr := c.QueryServerState(ctx)
		if verifyErr == nil {
			conditions.SetSatisfactoryServer(server,
				satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated,
				metav1.ConditionTrue, "TokenValid", "Cached authentication token is valid")
			return
		}
		// Cached token is invalid — clear it and fall through to re-authenticate.
		c.SetAuthToken("")
		log.Info("Cached auth token is invalid, re-authenticating", "error", verifyErr)
	}

	// 2. Read admin password from referenced Secret.
	password, err := r.getAdminPassword(ctx, server)
	if err != nil {
		var authErr *authHelperError
		if errors.As(err, &authErr) {
			conditions.SetSatisfactoryServer(server,
				satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated,
				metav1.ConditionFalse, authErr.Reason, authErr.Message)
		} else {
			conditions.SetSatisfactoryServer(server,
				satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated,
				metav1.ConditionFalse, "SecretNotFound",
				fmt.Sprintf("failed to read admin password: %v", err))
		}
		return // non-fatal — proceed with unauthenticated probe
	}

	// 3. Authenticate via PasswordLogin.
	resp, err := c.PasswordLogin(ctx, satisfactory.PrivilegeLevelAdministrator, password)
	if err != nil {
		conditions.SetSatisfactoryServer(server,
			satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated,
			metav1.ConditionFalse, "AuthenticationFailed",
			fmt.Sprintf("PasswordLogin failed: %v", err))
		return // non-fatal — proceed with unauthenticated probe
	}

	// 4. Set token on client and cache it.
	token := resp.AuthenticationToken
	c.SetAuthToken(token)
	if cacheErr := r.cacheAuthToken(ctx, server, token); cacheErr != nil {
		log.Error(cacheErr, "failed to cache auth token")
		// Non-fatal — we still have the token for this reconcile loop.
	}

	conditions.SetSatisfactoryServer(server,
		satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated,
		metav1.ConditionTrue, "TokenValid", "Successfully authenticated with game server API")
}
