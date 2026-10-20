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

// Package conditions provides helpers for setting, removing, and inspecting
// Kubernetes status conditions on the operator's CRDs.
package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

// Set adds or updates a condition in the supplied conditions slice.
// ObservedGeneration is stamped with the provided generation.
func Set(conditions *[]metav1.Condition, observedGeneration int64, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: observedGeneration,
		Reason:             reason,
		Message:            message,
	})
}

// Remove deletes a condition of the given type from the slice.
func Remove(conditions *[]metav1.Condition, condType string) {
	meta.RemoveStatusCondition(conditions, condType)
}

// IsTrue returns true when the named condition exists and its status is True.
func IsTrue(conditions []metav1.Condition, condType string) bool {
	cond := meta.FindStatusCondition(conditions, condType)
	return cond != nil && cond.Status == metav1.ConditionTrue
}

// --- Per-CRD typed wrappers ---

// SetSatisfactoryServer sets a condition on a SatisfactoryServer, stamping
// ObservedGeneration from server.Generation.
func SetSatisfactoryServer(server *satisfactoryv1alpha1.SatisfactoryServer, condType string, status metav1.ConditionStatus, reason, message string) {
	Set(&server.Status.Conditions, server.Generation, condType, status, reason, message)
}

// IsTrueSatisfactoryServer returns true when the named condition on a
// SatisfactoryServer is True.
func IsTrueSatisfactoryServer(server *satisfactoryv1alpha1.SatisfactoryServer, condType string) bool {
	return IsTrue(server.Status.Conditions, condType)
}

// SetSaveGame sets a condition on a SaveGame, stamping ObservedGeneration
// from sg.Generation.
func SetSaveGame(sg *satisfactoryv1alpha1.SaveGame, condType string, status metav1.ConditionStatus, reason, message string) {
	Set(&sg.Status.Conditions, sg.Generation, condType, status, reason, message)
}

// IsTrueSaveGame returns true when the named condition on a SaveGame is True.
func IsTrueSaveGame(sg *satisfactoryv1alpha1.SaveGame, condType string) bool {
	return IsTrue(sg.Status.Conditions, condType)
}

// SetClusterPolicy sets a condition on a ClusterPolicy, stamping
// ObservedGeneration from cp.Generation.
func SetClusterPolicy(cp *satisfactoryv1alpha1.ClusterPolicy, condType string, status metav1.ConditionStatus, reason, message string) {
	Set(&cp.Status.Conditions, cp.Generation, condType, status, reason, message)
}

// IsTrueClusterPolicy returns true when the named condition on a ClusterPolicy
// is True.
func IsTrueClusterPolicy(cp *satisfactoryv1alpha1.ClusterPolicy, condType string) bool {
	return IsTrue(cp.Status.Conditions, condType)
}
