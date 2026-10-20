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
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// saveDateTimeLayout is the format of SaveHeader.SaveDateTime (e.g. "2024.09.28-04.00.32").
const saveDateTimeLayout = "2006.01.02-15.04.05"

// resolveServerClient fetches the referenced SatisfactoryServer and returns a client
// authenticated with its admin credentials. Failures that map to a Ready condition reason
// (ServerNotFound, ServerNotReachable, ServerNotAuthenticated) are returned as *authHelperError.
func (r *SaveGameReconciler) resolveServerClient(ctx context.Context, serverRef, namespace string) (*satisfactory.Client, error) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: serverRef, Namespace: namespace}, server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &authHelperError{
				Reason:  "ServerNotFound",
				Message: fmt.Sprintf("SatisfactoryServer %q not found in namespace %q", serverRef, namespace),
			}
		}
		return nil, fmt.Errorf("get SatisfactoryServer %s: %w", serverRef, err)
	}
	if !conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable) {
		return nil, &authHelperError{
			Reason:  "ServerNotReachable",
			Message: fmt.Sprintf("SatisfactoryServer %q is not reachable", serverRef),
		}
	}
	if server.Spec.AdminPasswordSecretRef == nil {
		return nil, &authHelperError{
			Reason:  "ServerNotAuthenticated",
			Message: fmt.Sprintf("SatisfactoryServer %q has no adminPasswordSecretRef", serverRef),
		}
	}

	baseURL := gameAPIBaseURL(server)
	if r.GameAPIURL != nil {
		baseURL = r.GameAPIURL(server)
	}
	c := satisfactory.NewClient(baseURL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // self-signed cert expected

	// Reuse the server controller's token cache + login flow; it records the outcome
	// as the Authenticated condition on this in-memory copy (never persisted here).
	sr := &SatisfactoryServerReconciler{Client: r.Client, Scheme: r.Scheme}
	sr.authenticateClient(ctx, server, c)
	if !conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated) {
		msg := "authentication failed"
		if cond := meta.FindStatusCondition(server.Status.Conditions, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated); cond != nil {
			msg = cond.Message
		}
		return nil, &authHelperError{Reason: "ServerNotAuthenticated", Message: msg}
	}
	return c, nil
}

// triggerSave saves the running session as "<saveName>_<UTC timestamp>" and returns that name.
// The timestamp suffix is needed because the API overwrites saves with the same name,
// which would leave nothing for retention to manage.
func triggerSave(ctx context.Context, c *satisfactory.Client, saveName string, now time.Time) (string, error) {
	name := saveName + "_" + now.UTC().Format("20060102-150405")
	if err := c.SaveGame(ctx, name); err != nil {
		return "", fmt.Errorf("save game %s: %w", name, err)
	}
	return name, nil
}

// enumerateSessionSaves returns all saves named saveName or saveName_*, oldest first.
func enumerateSessionSaves(ctx context.Context, c *satisfactory.Client, saveName string) ([]satisfactory.SaveHeader, error) {
	resp, err := c.EnumerateSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("enumerate sessions: %w", err)
	}
	var saves []satisfactory.SaveHeader
	for _, session := range resp.Sessions {
		for _, h := range session.SaveHeaders {
			if h.SaveName == saveName || strings.HasPrefix(h.SaveName, saveName+"_") {
				saves = append(saves, h)
			}
		}
	}
	// The layout is zero-padded and big-endian, so string order is chronological.
	sort.Slice(saves, func(i, j int) bool { return saves[i].SaveDateTime < saves[j].SaveDateTime })
	return saves, nil
}

// enforceRetention deletes the oldest saves beyond MaxCount and any save older than MaxAge.
// saves must be sorted oldest first. Returns the saves that remain.
func enforceRetention(
	ctx context.Context,
	c *satisfactory.Client,
	saves []satisfactory.SaveHeader,
	policy *satisfactoryv1alpha1.SaveGameRetentionPolicy,
	now time.Time,
) ([]satisfactory.SaveHeader, error) {
	if policy == nil {
		return saves, nil
	}
	excess := 0
	if policy.MaxCount != nil {
		excess = max(0, len(saves)-int(*policy.MaxCount))
	}
	var kept []satisfactory.SaveHeader
	for i, s := range saves {
		expired := false
		if policy.MaxAge != nil {
			// Unparseable timestamps are kept: never delete what we cannot date.
			t, err := time.Parse(saveDateTimeLayout, s.SaveDateTime)
			expired = err == nil && now.Sub(t) > policy.MaxAge.Duration
		}
		if i < excess || expired {
			if err := c.DeleteSaveFile(ctx, s.SaveName); err != nil {
				return nil, fmt.Errorf("delete save %s: %w", s.SaveName, err)
			}
			continue
		}
		kept = append(kept, s)
	}
	return kept, nil
}

// backupDue reports whether a backup should run now. A nil schedule means a single
// one-shot backup; either way the first backup runs as soon as possible.
func backupDue(sched cron.Schedule, last *metav1.Time, now time.Time) bool {
	if last == nil {
		return true
	}
	return sched != nil && !now.Before(sched.Next(last.Time))
}
