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
	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

// applyDefaults patches zero-valued fields of server with defaults from policy.
// Returns true if any field was changed.
func applyDefaults(server *satisfactoryv1alpha1.SatisfactoryServer, policy *satisfactoryv1alpha1.ClusterPolicy) bool {
	changed := false

	// Apply defaultResourceLimits when the server has no explicit resource requirements.
	if policy.Spec.DefaultResourceLimits != nil {
		if server.Spec.Resources.Requests == nil && server.Spec.Resources.Limits == nil {
			server.Spec.Resources = *policy.Spec.DefaultResourceLimits
			changed = true
		}
	}

	// Apply defaultStorageClassName when the server has no explicit storage class.
	if policy.Spec.DefaultStorageClassName != nil && server.Spec.Storage.StorageClassName == nil {
		sc := *policy.Spec.DefaultStorageClassName
		server.Spec.Storage.StorageClassName = &sc
		changed = true
	}

	// Apply defaultServerConfig fields where the server field is at its zero value.
	if policy.Spec.DefaultServerConfig != nil {
		cfg := policy.Spec.DefaultServerConfig

		if cfg.MaxPlayers != nil && server.Spec.MaxPlayers == 0 {
			server.Spec.MaxPlayers = *cfg.MaxPlayers
			changed = true
		}

		if cfg.MaxTickRate != nil && server.Spec.MaxTickRate == 0 {
			server.Spec.MaxTickRate = *cfg.MaxTickRate
			changed = true
		}

		if cfg.SkipUpdate != nil && server.Spec.SkipUpdate == nil {
			v := *cfg.SkipUpdate
			server.Spec.SkipUpdate = &v
			changed = true
		}
	}

	return changed
}

// validateImageAllowlist returns true if the server image is in the allowlist,
// or if the allowlist is empty (no restriction).
func validateImageAllowlist(server *satisfactoryv1alpha1.SatisfactoryServer, allowedImages []string) bool {
	if len(allowedImages) == 0 {
		return true
	}
	for _, img := range allowedImages {
		if server.Spec.Image == img {
			return true
		}
	}
	return false
}

// countServersPerNamespace groups SatisfactoryServer resources by namespace and
// returns a map of namespace → count.
func countServersPerNamespace(servers []satisfactoryv1alpha1.SatisfactoryServer) map[string]int {
	counts := make(map[string]int)
	for _, s := range servers {
		counts[s.Namespace]++
	}
	return counts
}

// findActivePolicy returns the oldest ClusterPolicy (by creation timestamp) as the
// active policy, and all others as inactive. If policies is empty, returns nil, nil.
func findActivePolicy(policies []satisfactoryv1alpha1.ClusterPolicy) (*satisfactoryv1alpha1.ClusterPolicy, []*satisfactoryv1alpha1.ClusterPolicy) {
	if len(policies) == 0 {
		return nil, nil
	}

	activeIdx := 0
	for i := 1; i < len(policies); i++ {
		if policies[i].CreationTimestamp.Before(&policies[activeIdx].CreationTimestamp) {
			activeIdx = i
		}
	}

	active := &policies[activeIdx]
	var inactive []*satisfactoryv1alpha1.ClusterPolicy
	for i := range policies {
		if i != activeIdx {
			inactive = append(inactive, &policies[i])
		}
	}
	return active, inactive
}
