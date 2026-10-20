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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

func int32Ptr(i int32) *int32 { return &i }

// --- applyDefaults tests ---

func TestApplyDefaults_ResourceLimits(t *testing.T) {
	policy := &satisfactoryv1alpha1.ClusterPolicy{
		Spec: satisfactoryv1alpha1.ClusterPolicySpec{
			DefaultResourceLimits: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("8Gi"),
				},
			},
		},
	}

	t.Run("defaults applied when server has no resources", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{}
		changed := applyDefaults(server, policy)
		if !changed {
			t.Fatal("expected changed=true")
		}
		if server.Spec.Resources.Requests == nil {
			t.Error("expected Resources.Requests to be set")
		}
	})

	t.Run("defaults not applied when server has explicit requests", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
			},
		}
		changed := applyDefaults(server, policy)
		if changed {
			t.Error("expected changed=false when server has explicit requests")
		}
		if server.Spec.Resources.Requests[corev1.ResourceMemory] != resource.MustParse("2Gi") {
			t.Error("expected original requests to be preserved")
		}
	})

	t.Run("defaults not applied when server has explicit limits", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("16Gi"),
					},
				},
			},
		}
		changed := applyDefaults(server, policy)
		if changed {
			t.Error("expected changed=false when server has explicit limits")
		}
	})
}

func TestApplyDefaults_StorageClassName(t *testing.T) {
	policy := &satisfactoryv1alpha1.ClusterPolicy{
		Spec: satisfactoryv1alpha1.ClusterPolicySpec{
			DefaultStorageClassName: stringPtr("fast-ssd"),
		},
	}

	t.Run("storage class applied when not set", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{}
		changed := applyDefaults(server, policy)
		if !changed {
			t.Fatal("expected changed=true")
		}
		if server.Spec.Storage.StorageClassName == nil || *server.Spec.Storage.StorageClassName != "fast-ssd" {
			t.Errorf("expected storageClassName=fast-ssd, got %v", server.Spec.Storage.StorageClassName)
		}
	})

	t.Run("storage class not overwritten when explicitly set", func(t *testing.T) {
		sc := "existing-class"
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				Storage: satisfactoryv1alpha1.SatisfactoryServerStorage{
					StorageClassName: &sc,
				},
			},
		}
		changed := applyDefaults(server, policy)
		if changed {
			t.Error("expected changed=false")
		}
		if *server.Spec.Storage.StorageClassName != "existing-class" {
			t.Error("expected existing storage class to be preserved")
		}
	})
}

func TestApplyDefaults_ServerConfig(t *testing.T) {
	policy := &satisfactoryv1alpha1.ClusterPolicy{
		Spec: satisfactoryv1alpha1.ClusterPolicySpec{
			DefaultServerConfig: &satisfactoryv1alpha1.ClusterPolicyServerDefaults{
				MaxPlayers:  int32Ptr(8),
				MaxTickRate: int32Ptr(60),
				SkipUpdate:  boolPtr(false),
			},
		},
	}

	t.Run("maxPlayers default applied at zero", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{}
		changed := applyDefaults(server, policy)
		if !changed {
			t.Fatal("expected changed=true")
		}
		if server.Spec.MaxPlayers != 8 {
			t.Errorf("expected maxPlayers=8, got %d", server.Spec.MaxPlayers)
		}
	})

	t.Run("maxPlayers not overwritten when explicitly set", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{MaxPlayers: 16},
		}
		applyDefaults(server, policy)
		if server.Spec.MaxPlayers != 16 {
			t.Errorf("expected maxPlayers=16, got %d", server.Spec.MaxPlayers)
		}
	})

	t.Run("skipUpdate default applied when nil", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{}
		applyDefaults(server, policy)
		if server.Spec.SkipUpdate == nil || *server.Spec.SkipUpdate != false {
			t.Error("expected skipUpdate to be set to false")
		}
	})

	t.Run("skipUpdate not overwritten when explicitly set", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{SkipUpdate: boolPtr(true)},
		}
		applyDefaults(server, policy)
		if !*server.Spec.SkipUpdate {
			t.Error("expected skipUpdate=true to be preserved")
		}
	})
}

func TestApplyDefaults_NoDefaultsConfigured(t *testing.T) {
	policy := &satisfactoryv1alpha1.ClusterPolicy{}
	server := &satisfactoryv1alpha1.SatisfactoryServer{
		Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{MaxPlayers: 4},
	}
	changed := applyDefaults(server, policy)
	if changed {
		t.Error("expected changed=false when no defaults are configured")
	}
}

// --- validateImageAllowlist tests ---

func TestValidateImageAllowlist(t *testing.T) {
	allowedImages := []string{
		"ghcr.io/robtme/satisfactory-server:v1.9.10",
		"ghcr.io/robtme/satisfactory-server:v1.9.9",
	}

	t.Run("image in allowlist is compliant", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{Image: "ghcr.io/robtme/satisfactory-server:v1.9.10"},
		}
		if !validateImageAllowlist(server, allowedImages) {
			t.Error("expected compliant=true")
		}
	})

	t.Run("image not in allowlist is non-compliant", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{Image: "ghcr.io/robtme/satisfactory-server:v1.0.0"},
		}
		if validateImageAllowlist(server, allowedImages) {
			t.Error("expected compliant=false")
		}
	})

	t.Run("different registry spelling is non-compliant", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{Image: "docker.io/robtme/satisfactory-server:v1.9.10"},
		}
		if validateImageAllowlist(server, []string{"robtme/satisfactory-server:v1.9.10"}) {
			t.Error("expected compliant=false")
		}
	})

	t.Run("same repository with different tag is non-compliant", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{Image: "ghcr.io/robtme/satisfactory-server:latest"},
		}
		if validateImageAllowlist(server, []string{"ghcr.io/robtme/satisfactory-server:v1.9.10"}) {
			t.Error("expected compliant=false")
		}
	})

	t.Run("empty allowlist allows any image", func(t *testing.T) {
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{Image: "anything:latest"},
		}
		if !validateImageAllowlist(server, nil) {
			t.Error("expected compliant=true for empty allowlist")
		}
		if !validateImageAllowlist(server, []string{}) {
			t.Error("expected compliant=true for empty allowlist")
		}
	})
}

// --- countServersPerNamespace tests ---

func TestCountServersPerNamespace(t *testing.T) {
	servers := []satisfactoryv1alpha1.SatisfactoryServer{
		{ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "ns-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "s2", Namespace: "ns-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "s3", Namespace: "ns-b"}},
	}

	counts := countServersPerNamespace(servers)
	if counts["ns-a"] != 2 {
		t.Errorf("expected ns-a=2, got %d", counts["ns-a"])
	}
	if counts["ns-b"] != 1 {
		t.Errorf("expected ns-b=1, got %d", counts["ns-b"])
	}

	t.Run("namespace limit enforcement", func(t *testing.T) {
		limit := int32(1)
		for ns, count := range counts {
			if int32(count) > limit {
				if ns != "ns-a" {
					t.Errorf("expected ns-a to exceed limit, got %s", ns)
				}
			}
		}
	})

	t.Run("empty list returns empty map", func(t *testing.T) {
		empty := countServersPerNamespace(nil)
		if len(empty) != 0 {
			t.Errorf("expected empty map, got %v", empty)
		}
	})
}

// --- findActivePolicy tests ---

func TestFindActivePolicy_SinglePolicy(t *testing.T) {
	policies := []satisfactoryv1alpha1.ClusterPolicy{
		{ObjectMeta: metav1.ObjectMeta{Name: "only-policy", CreationTimestamp: metav1.Now()}},
	}
	active, inactive := findActivePolicy(policies)
	if active == nil {
		t.Fatal("expected non-nil active policy")
	}
	if active.Name != "only-policy" {
		t.Errorf("expected active=only-policy, got %s", active.Name)
	}
	if len(inactive) != 0 {
		t.Errorf("expected 0 inactive, got %d", len(inactive))
	}
}

func TestFindActivePolicy_MultiplePolicies(t *testing.T) {
	older := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	newer := metav1.NewTime(time.Now())

	policies := []satisfactoryv1alpha1.ClusterPolicy{
		{ObjectMeta: metav1.ObjectMeta{Name: "newer-policy", CreationTimestamp: newer}},
		{ObjectMeta: metav1.ObjectMeta{Name: "older-policy", CreationTimestamp: older}},
	}
	active, inactive := findActivePolicy(policies)
	if active == nil {
		t.Fatal("expected non-nil active policy")
	}
	if active.Name != "older-policy" {
		t.Errorf("expected active=older-policy, got %s", active.Name)
	}
	if len(inactive) != 1 || inactive[0].Name != "newer-policy" {
		t.Errorf("expected inactive=[newer-policy], got %v", inactive)
	}
}

func TestFindActivePolicy_Empty(t *testing.T) {
	active, inactive := findActivePolicy(nil)
	if active != nil {
		t.Error("expected nil active for empty list")
	}
	if inactive != nil {
		t.Error("expected nil inactive for empty list")
	}
}
