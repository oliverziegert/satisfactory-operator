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
	"errors"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// getServerName returns the in-game server name from the spec, falling back to the CR name.
func getServerName(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	if server.Spec.ServerName != "" {
		return server.Spec.ServerName
	}
	return server.Name
}

// autoClaimEnabled returns true if the reconciler should attempt to auto-claim the server.
// All of the following must hold: server is reachable, server is unclaimed, credentials are
// configured, and autoClaimDisabled is not true.
func autoClaimEnabled(server *satisfactoryv1alpha1.SatisfactoryServer, probe *probeResult) bool {
	if probe == nil || !probe.Reachable || probe.Claimed {
		return false
	}
	if server.Spec.AdminPasswordSecretRef == nil {
		return false
	}
	if server.Spec.AutoClaimDisabled != nil && *server.Spec.AutoClaimDisabled {
		return false
	}
	return true
}

// claimServer claims an unclaimed Satisfactory Dedicated Server.
//
// Flow:
//  1. Call PasswordlessLogin with InitialAdmin to get a temporary token.
//  2. Call ClaimServer with the configured server name and admin password.
//  3. Cache the returned auth token in the <server-name>-auth-token Secret.
//
// Returns a non-nil error if claiming failed. In the "already claimed" case (APIError from
// PasswordlessLogin or ClaimServer) the error is logged and returned so the caller proceeds
// with normal reconciliation without requeueing.
func (r *SatisfactoryServerReconciler) claimServer(
	ctx context.Context,
	server *satisfactoryv1alpha1.SatisfactoryServer,
	baseURL string,
	adminPassword string,
) error {
	log := logf.FromContext(ctx)

	// Create a fresh unauthenticated client for the claiming flow.
	c := satisfactory.NewClient(baseURL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // self-signed cert expected

	// Step 1: PasswordlessLogin to obtain an InitialAdmin token.
	loginResp, err := c.PasswordlessLogin(ctx, satisfactory.PrivilegeLevelInitialAdmin)
	if err != nil {
		var apiErr *satisfactory.APIError
		if errors.As(err, &apiErr) {
			log.Info("PasswordlessLogin failed during claiming — server may already be claimed", "error", err)
		}
		return fmt.Errorf("passwordless login for claiming: %w", err)
	}
	c.SetAuthToken(loginResp.AuthenticationToken)

	// Step 2: Claim the server.
	claimResp, err := c.ClaimServer(ctx, getServerName(server), adminPassword)
	if err != nil {
		var apiErr *satisfactory.APIError
		if errors.As(err, &apiErr) {
			log.Info("ClaimServer failed — server may already be claimed", "error", err)
		}
		return fmt.Errorf("claim server: %w", err)
	}

	// Step 3: Cache the returned auth token.
	if cacheErr := r.cacheAuthToken(ctx, server, claimResp.AuthenticationToken); cacheErr != nil {
		log.Error(cacheErr, "failed to cache auth token after claiming")
		// Non-fatal — claiming succeeded, token caching is best-effort.
	}

	return nil
}
