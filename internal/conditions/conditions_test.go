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

package conditions_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
)

func TestSet(t *testing.T) {
	conds := []metav1.Condition{}
	conditions.Set(&conds, 1, "Ready", metav1.ConditionTrue, "AllGood", "everything is fine")

	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	c := conds[0]
	if c.Type != "Ready" {
		t.Errorf("expected type Ready, got %s", c.Type)
	}
	if c.Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %s", c.Status)
	}
	if c.Reason != "AllGood" {
		t.Errorf("expected reason AllGood, got %s", c.Reason)
	}
	if c.ObservedGeneration != 1 {
		t.Errorf("expected observedGeneration 1, got %d", c.ObservedGeneration)
	}
}

func TestSet_UpdatesExisting(t *testing.T) {
	conds := []metav1.Condition{}
	conditions.Set(&conds, 1, "Ready", metav1.ConditionFalse, "NotYet", "not ready")
	conditions.Set(&conds, 2, "Ready", metav1.ConditionTrue, "AllGood", "now ready")

	if len(conds) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(conds))
	}
	if conds[0].Status != metav1.ConditionTrue {
		t.Errorf("expected updated status True, got %s", conds[0].Status)
	}
	if conds[0].ObservedGeneration != 2 {
		t.Errorf("expected observedGeneration 2, got %d", conds[0].ObservedGeneration)
	}
}

func TestRemove(t *testing.T) {
	conds := []metav1.Condition{}
	conditions.Set(&conds, 1, "Ready", metav1.ConditionTrue, "AllGood", "fine")
	conditions.Set(&conds, 1, "Degraded", metav1.ConditionFalse, "NotDegraded", "fine")

	conditions.Remove(&conds, "Degraded")

	if len(conds) != 1 {
		t.Fatalf("expected 1 condition after remove, got %d", len(conds))
	}
	if conds[0].Type != "Ready" {
		t.Errorf("expected remaining condition Ready, got %s", conds[0].Type)
	}
}

func TestIsTrue(t *testing.T) {
	conds := []metav1.Condition{}
	conditions.Set(&conds, 1, "Ready", metav1.ConditionTrue, "AllGood", "fine")
	conditions.Set(&conds, 1, "Degraded", metav1.ConditionFalse, "NotDegraded", "fine")

	if !conditions.IsTrue(conds, "Ready") {
		t.Error("expected IsTrue=true for Ready=True")
	}
	if conditions.IsTrue(conds, "Degraded") {
		t.Error("expected IsTrue=false for Degraded=False")
	}
	if conditions.IsTrue(conds, "Unknown") {
		t.Error("expected IsTrue=false for missing condition")
	}
}

func TestSetSatisfactoryServer(t *testing.T) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	server.Generation = 3
	conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady, metav1.ConditionTrue, "AllGood", "running")

	if len(server.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(server.Status.Conditions))
	}
	if server.Status.Conditions[0].ObservedGeneration != 3 {
		t.Errorf("expected observedGeneration 3, got %d", server.Status.Conditions[0].ObservedGeneration)
	}
	if !conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady) {
		t.Error("expected IsTrueSatisfactoryServer=true")
	}
}

func TestSetSaveGame(t *testing.T) {
	sg := &satisfactoryv1alpha1.SaveGame{}
	sg.Generation = 2
	conditions.SetSaveGame(sg, satisfactoryv1alpha1.SaveGameConditionTypeReady, metav1.ConditionTrue, "Reconciled", "ok")

	if len(sg.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(sg.Status.Conditions))
	}
	if !conditions.IsTrueSaveGame(sg, satisfactoryv1alpha1.SaveGameConditionTypeReady) {
		t.Error("expected IsTrueSaveGame=true")
	}
}

func TestSetClusterPolicy(t *testing.T) {
	cp := &satisfactoryv1alpha1.ClusterPolicy{}
	cp.Generation = 1
	conditions.SetClusterPolicy(cp, satisfactoryv1alpha1.ClusterPolicyConditionTypeReady, metav1.ConditionTrue, "Reconciled", "ok")

	if len(cp.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(cp.Status.Conditions))
	}
	if !conditions.IsTrueClusterPolicy(cp, satisfactoryv1alpha1.ClusterPolicyConditionTypeReady) {
		t.Error("expected IsTrueClusterPolicy=true")
	}
}
