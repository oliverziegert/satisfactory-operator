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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

func TestClusterPolicyConditions_Ready(t *testing.T) {
	s := newScheme(t)
	cp := &satisfactoryv1alpha1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cp1"},
	}
	c := clientfake.NewClientBuilder().WithScheme(s).WithObjects(cp).WithStatusSubresource(cp).Build()
	rec := &ClusterPolicyReconciler{Client: c, Scheme: s}

	_, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "cp1"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	updated := &satisfactoryv1alpha1.ClusterPolicy{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "cp1"}, updated); err != nil {
		t.Fatalf("get updated: %v", err)
	}

	checkClusterPolicyConditionStatus(t, updated, satisfactoryv1alpha1.ClusterPolicyConditionTypeReady, metav1.ConditionTrue)
	checkClusterPolicyConditionStatus(t, updated, satisfactoryv1alpha1.ClusterPolicyConditionTypeEnforced, metav1.ConditionTrue)
}

func checkClusterPolicyConditionStatus(t *testing.T, cp *satisfactoryv1alpha1.ClusterPolicy, condType string, want metav1.ConditionStatus) {
	t.Helper()
	for _, c := range cp.Status.Conditions {
		if c.Type == condType {
			if c.Status != want {
				t.Errorf("ClusterPolicy condition %s: expected %s, got %s", condType, want, c.Status)
			}
			return
		}
	}
	t.Errorf("ClusterPolicy condition %s not found", condType)
}
