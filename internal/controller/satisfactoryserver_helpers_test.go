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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

// int64Ptr returns a pointer to the given int64 value.
func int64Ptr(i int64) *int64 { return &i }

// stringPtr returns a pointer to the given string value.
func stringPtr(s string) *string { return &s }

// findEnv searches envs for an entry with the given name and returns its value
// and whether it was found.
func findEnv(envs []corev1.EnvVar, name string) (string, bool) {
	for _, e := range envs {
		if e.Name == name {
			return e.Value, true
		}
	}
	return "", false
}

// defaultSpec returns a SatisfactoryServerSpec with all numeric/bool fields set
// to their kubebuilder-documented defaults. kubebuilder defaults only apply at
// admission time, so tests must set them explicitly.
func defaultSpec() satisfactoryv1alpha1.SatisfactoryServerSpec {
	return satisfactoryv1alpha1.SatisfactoryServerSpec{
		MaxPlayers:            4,
		AutosaveNum:           5,
		MaxTickRate:           30,
		MaxObjects:            2162688,
		Timeout:               30,
		ServerStreaming:       boolPtr(true),
		DisableSeasonalEvents: boolPtr(false),
		Debug:                 boolPtr(false),
		Log:                   boolPtr(false),
		GamePort:              7777,
		MessagingPort:         8888,
		Multihome:             "::",
		SkipUpdate:            boolPtr(true),
		SteamBeta:             boolPtr(false),
		RunAsUser:             int64Ptr(1000),
		RunAsGroup:            int64Ptr(1000),
	}
}

// ---------------------------------------------------------------------------
// TestBuildEnvVars
// ---------------------------------------------------------------------------

func TestBuildEnvVars_DefaultSpec(t *testing.T) {
	envs := buildEnvVars(defaultSpec())

	expectations := []struct {
		name  string
		value string
	}{
		{"MAXPLAYERS", "4"},
		{"AUTOSAVENUM", "5"},
		{"MAXTICKRATE", "30"},
		{"MAXOBJECTS", "2162688"},
		{"TIMEOUT", "30"},
		{"SERVERSTREAMING", "true"},
		{"DISABLESEASONALEVENTS", "false"},
		{"DEBUG", "false"},
		{"LOG", "false"},
		{"SERVERGAMEPORT", "7777"},
		{"SERVERMESSAGINGPORT", "8888"},
		{"MULTIHOME", "::"},
		{"SKIPUPDATE", "true"},
		{"STEAMBETA", "false"},
		{"PUID", "1000"},
		{"PGID", "1000"},
	}

	for _, tt := range expectations {
		t.Run(tt.name, func(t *testing.T) {
			got, found := findEnv(envs, tt.name)
			if !found {
				t.Errorf("env var %q not present", tt.name)
				return
			}
			if got != tt.value {
				t.Errorf("env var %q = %q, want %q", tt.name, got, tt.value)
			}
		})
	}
}

func TestBuildEnvVars_SteamBetaKeyAbsentWhenNil(t *testing.T) {
	spec := defaultSpec()
	spec.SteamBetaPassword = nil

	envs := buildEnvVars(spec)

	if _, found := findEnv(envs, "STEAMBETAKEY"); found {
		t.Error("STEAMBETAKEY should not be present when SteamBetaPassword is nil")
	}
}

func TestBuildEnvVars_SteamBetaKeyAbsentWhenEmpty(t *testing.T) {
	spec := defaultSpec()
	spec.SteamBetaPassword = stringPtr("")

	envs := buildEnvVars(spec)

	if _, found := findEnv(envs, "STEAMBETAKEY"); found {
		t.Error("STEAMBETAKEY should not be present when SteamBetaPassword is an empty string")
	}
}

func TestBuildEnvVars_SteamBetaKeyPresentWhenSet(t *testing.T) {
	spec := defaultSpec()
	spec.SteamBetaPassword = stringPtr("mykey")

	envs := buildEnvVars(spec)

	got, found := findEnv(envs, "STEAMBETAKEY")
	if !found {
		t.Fatal("STEAMBETAKEY should be present when SteamBetaPassword is set")
	}
	if got != "mykey" {
		t.Errorf("STEAMBETAKEY = %q, want %q", got, "mykey")
	}
}

func TestBuildEnvVars_CustomValues(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*satisfactoryv1alpha1.SatisfactoryServerSpec)
		envName  string
		envValue string
	}{
		{
			name:     "MaxPlayers custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.MaxPlayers = 10 },
			envName:  "MAXPLAYERS",
			envValue: "10",
		},
		{
			name:     "AutosaveNum custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.AutosaveNum = 3 },
			envName:  "AUTOSAVENUM",
			envValue: "3",
		},
		{
			name:     "MaxTickRate custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.MaxTickRate = 60 },
			envName:  "MAXTICKRATE",
			envValue: "60",
		},
		{
			name:     "MaxObjects custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.MaxObjects = 1000000 },
			envName:  "MAXOBJECTS",
			envValue: "1000000",
		},
		{
			name:     "Timeout custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.Timeout = 60 },
			envName:  "TIMEOUT",
			envValue: "60",
		},
		{
			name:     "Multihome custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.Multihome = "0.0.0.0" },
			envName:  "MULTIHOME",
			envValue: "0.0.0.0",
		},
		{
			name:     "GamePort custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.GamePort = 7778 },
			envName:  "SERVERGAMEPORT",
			envValue: "7778",
		},
		{
			name:     "MessagingPort custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.MessagingPort = 8889 },
			envName:  "SERVERMESSAGINGPORT",
			envValue: "8889",
		},
		{
			name:     "ServerStreaming false",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.ServerStreaming = boolPtr(false) },
			envName:  "SERVERSTREAMING",
			envValue: "false",
		},
		{
			name:     "DisableSeasonalEvents true",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.DisableSeasonalEvents = boolPtr(true) },
			envName:  "DISABLESEASONALEVENTS",
			envValue: "true",
		},
		{
			name:     "Debug true",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.Debug = boolPtr(true) },
			envName:  "DEBUG",
			envValue: "true",
		},
		{
			name:     "Log true",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.Log = boolPtr(true) },
			envName:  "LOG",
			envValue: "true",
		},
		{
			name:     "SkipUpdate false",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.SkipUpdate = boolPtr(false) },
			envName:  "SKIPUPDATE",
			envValue: "false",
		},
		{
			name:     "SteamBeta true",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.SteamBeta = boolPtr(true) },
			envName:  "STEAMBETA",
			envValue: "true",
		},
		{
			name:     "RunAsUser custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.RunAsUser = int64Ptr(2000) },
			envName:  "PUID",
			envValue: "2000",
		},
		{
			name:     "RunAsGroup custom",
			mutate:   func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.RunAsGroup = int64Ptr(2000) },
			envName:  "PGID",
			envValue: "2000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := defaultSpec()
			tt.mutate(&spec)
			envs := buildEnvVars(spec)

			got, found := findEnv(envs, tt.envName)
			if !found {
				t.Errorf("env var %q not present", tt.envName)
				return
			}
			if got != tt.envValue {
				t.Errorf("env var %q = %q, want %q", tt.envName, got, tt.envValue)
			}
		})
	}
}

// MaxPlayers and MaxTickRate have no API-server default, so unset values fall back here.
func TestBuildEnvVars_ZeroIntFieldsFallBackToDefaults(t *testing.T) {
	spec := defaultSpec()
	spec.MaxPlayers = 0
	spec.MaxTickRate = 0

	envs := buildEnvVars(spec)

	for name, want := range map[string]string{"MAXPLAYERS": "4", "MAXTICKRATE": "30"} {
		if got, _ := findEnv(envs, name); got != want {
			t.Errorf("env var %q = %q, want %q", name, got, want)
		}
	}
}

// When *bool fields are nil, boolToString falls back to its documented default.
func TestBuildEnvVars_NilBoolFieldsFallBackToDefaults(t *testing.T) {
	spec := defaultSpec()
	spec.ServerStreaming = nil       // default true
	spec.DisableSeasonalEvents = nil // default false
	spec.Debug = nil                 // default false
	spec.Log = nil                   // default false
	spec.SkipUpdate = nil            // default true
	spec.SteamBeta = nil             // default false

	envs := buildEnvVars(spec)

	nilBoolDefaults := []struct {
		name  string
		value string
	}{
		{"SERVERSTREAMING", "true"},
		{"DISABLESEASONALEVENTS", "false"},
		{"DEBUG", "false"},
		{"LOG", "false"},
		{"SKIPUPDATE", "true"},
		{"STEAMBETA", "false"},
	}

	for _, tt := range nilBoolDefaults {
		t.Run(tt.name, func(t *testing.T) {
			got, found := findEnv(envs, tt.name)
			if !found {
				t.Errorf("env var %q not present", tt.name)
				return
			}
			if got != tt.value {
				t.Errorf("env var %q = %q, want %q", tt.name, got, tt.value)
			}
		})
	}
}

// When RunAsUser/RunAsGroup are nil, ptrInt64OrDefault falls back to 1000.
func TestBuildEnvVars_NilRunAsFieldsFallBackTo1000(t *testing.T) {
	spec := defaultSpec()
	spec.RunAsUser = nil
	spec.RunAsGroup = nil

	envs := buildEnvVars(spec)

	for _, envName := range []string{"PUID", "PGID"} {
		t.Run(envName, func(t *testing.T) {
			got, found := findEnv(envs, envName)
			if !found {
				t.Errorf("env var %q not present", envName)
				return
			}
			if got != "1000" {
				t.Errorf("env var %q = %q, want %q", envName, got, "1000")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildContainerPorts
// ---------------------------------------------------------------------------

func TestBuildContainerPorts_DefaultPorts(t *testing.T) {
	spec := defaultSpec()
	ports := buildContainerPorts(spec)

	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}

	type wantPort struct {
		name     string
		port     int32
		protocol corev1.Protocol
	}

	wants := []wantPort{
		{"game-tcp", 7777, corev1.ProtocolTCP},
		{"game-udp", 7777, corev1.ProtocolUDP},
		{"messaging", 8888, corev1.ProtocolTCP},
	}

	for i, w := range wants {
		t.Run(w.name, func(t *testing.T) {
			p := ports[i]
			if p.Name != w.name {
				t.Errorf("ports[%d].Name = %q, want %q", i, p.Name, w.name)
			}
			if p.ContainerPort != w.port {
				t.Errorf("ports[%d].ContainerPort = %d, want %d", i, p.ContainerPort, w.port)
			}
			if p.Protocol != w.protocol {
				t.Errorf("ports[%d].Protocol = %q, want %q", i, p.Protocol, w.protocol)
			}
		})
	}
}

func TestBuildContainerPorts_CustomPorts(t *testing.T) {
	spec := defaultSpec()
	spec.GamePort = 7778
	spec.MessagingPort = 8889

	ports := buildContainerPorts(spec)

	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}

	tests := []struct {
		idx      int
		name     string
		wantPort int32
		protocol corev1.Protocol
	}{
		{0, "game-tcp", 7778, corev1.ProtocolTCP},
		{1, "game-udp", 7778, corev1.ProtocolUDP},
		{2, "messaging", 8889, corev1.ProtocolTCP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ports[tt.idx]
			if p.ContainerPort != tt.wantPort {
				t.Errorf("ports[%d].ContainerPort = %d, want %d", tt.idx, p.ContainerPort, tt.wantPort)
			}
			if p.Protocol != tt.protocol {
				t.Errorf("ports[%d].Protocol = %q, want %q", tt.idx, p.Protocol, tt.protocol)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildPodSpec
// ---------------------------------------------------------------------------

func TestBuildPodSpec_DefaultSpec(t *testing.T) {
	spec := defaultSpec()
	podSpec := buildPodSpec(spec)

	// Must have exactly one container.
	if len(podSpec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(podSpec.Containers))
	}
	c := podSpec.Containers[0]

	t.Run("container name is satisfactory", func(t *testing.T) {
		if c.Name != "satisfactory" {
			t.Errorf("container name = %q, want %q", c.Name, "satisfactory")
		}
	})

	t.Run("default image", func(t *testing.T) {
		const want = "ghcr.io/robtme/satisfactory-server:v1.9.10"
		if c.Image != want {
			t.Errorf("image = %q, want %q", c.Image, want)
		}
	})

	t.Run("env vars are set", func(t *testing.T) {
		if _, found := findEnv(c.Env, "MAXPLAYERS"); !found {
			t.Error("MAXPLAYERS env var not present in container")
		}
	})

	t.Run("three ports are defined", func(t *testing.T) {
		if len(c.Ports) != 3 {
			t.Errorf("expected 3 ports, got %d", len(c.Ports))
		}
	})

	t.Run("volume mount for /config is present", func(t *testing.T) {
		found := false
		for _, vm := range c.VolumeMounts {
			if vm.MountPath == "/config" {
				found = true
				break
			}
		}
		if !found {
			t.Error("/config volume mount not found in container")
		}
	})

	t.Run("default memory request 4Gi", func(t *testing.T) {
		req, ok := c.Resources.Requests[corev1.ResourceMemory]
		if !ok {
			t.Fatal("memory request not set")
		}
		want := resource.MustParse("4Gi")
		if req.Cmp(want) != 0 {
			t.Errorf("memory request = %s, want %s", req.String(), want.String())
		}
	})

	t.Run("default memory limit 8Gi", func(t *testing.T) {
		lim, ok := c.Resources.Limits[corev1.ResourceMemory]
		if !ok {
			t.Fatal("memory limit not set")
		}
		want := resource.MustParse("8Gi")
		if lim.Cmp(want) != 0 {
			t.Errorf("memory limit = %s, want %s", lim.String(), want.String())
		}
	})

	t.Run("volume named config with PVC source", func(t *testing.T) {
		found := false
		for _, v := range podSpec.Volumes {
			if v.Name == "config" && v.PersistentVolumeClaim != nil {
				found = true
				break
			}
		}
		if !found {
			t.Error("volume named 'config' with PVC source not found")
		}
	})

	t.Run("readiness probe is set", func(t *testing.T) {
		if c.ReadinessProbe == nil {
			t.Error("ReadinessProbe should be set on the container")
		}
	})

	t.Run("liveness probe is set", func(t *testing.T) {
		if c.LivenessProbe == nil {
			t.Error("LivenessProbe should be set on the container")
		}
	})

	t.Run("startup probe covers first-boot game download", func(t *testing.T) {
		p := c.StartupProbe
		if p == nil || p.TCPSocket == nil {
			t.Fatal("StartupProbe should be a TCPSocket probe")
		}
		if p.TCPSocket.Port.IntValue() != int(spec.GamePort) {
			t.Errorf("port = %d, want %d", p.TCPSocket.Port.IntValue(), spec.GamePort)
		}
		if budget := p.PeriodSeconds * p.FailureThreshold; budget < 3600 {
			t.Errorf("startup budget = %ds, want >= 3600s", budget)
		}
	})
}

func TestBuildPodSpec_CustomImage(t *testing.T) {
	spec := defaultSpec()
	spec.Image = "ghcr.io/robtme/satisfactory-server:latest"

	podSpec := buildPodSpec(spec)

	if len(podSpec.Containers) == 0 {
		t.Fatal("expected at least one container")
	}
	got := podSpec.Containers[0].Image
	want := "ghcr.io/robtme/satisfactory-server:latest"
	if got != want {
		t.Errorf("image = %q, want %q", got, want)
	}
}

func TestBuildPodSpec_DefaultImageWhenEmpty(t *testing.T) {
	spec := defaultSpec()
	spec.Image = "" // triggers the fallback inside buildPodSpec

	podSpec := buildPodSpec(spec)

	if len(podSpec.Containers) == 0 {
		t.Fatal("expected at least one container")
	}
	got := podSpec.Containers[0].Image
	want := "ghcr.io/robtme/satisfactory-server:v1.9.10"
	if got != want {
		t.Errorf("image = %q, want %q", got, want)
	}
}

func TestBuildPodSpec_CustomResources(t *testing.T) {
	spec := defaultSpec()
	customReq := resource.MustParse("6Gi")
	customLim := resource.MustParse("12Gi")
	spec.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: customReq,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: customLim,
		},
	}

	podSpec := buildPodSpec(spec)

	if len(podSpec.Containers) == 0 {
		t.Fatal("expected at least one container")
	}
	c := podSpec.Containers[0]

	t.Run("custom memory request is used", func(t *testing.T) {
		req, ok := c.Resources.Requests[corev1.ResourceMemory]
		if !ok {
			t.Fatal("memory request not set")
		}
		if req.Cmp(customReq) != 0 {
			t.Errorf("memory request = %s, want %s", req.String(), customReq.String())
		}
	})

	t.Run("custom memory limit is used", func(t *testing.T) {
		lim, ok := c.Resources.Limits[corev1.ResourceMemory]
		if !ok {
			t.Fatal("memory limit not set")
		}
		if lim.Cmp(customLim) != 0 {
			t.Errorf("memory limit = %s, want %s", lim.String(), customLim.String())
		}
	})
}

// When only Requests are set, the spec is non-nil so defaults are not injected.
func TestBuildPodSpec_OnlyRequestsSet_DefaultsNotInjected(t *testing.T) {
	spec := defaultSpec()
	customReq := resource.MustParse("2Gi")
	spec.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: customReq,
		},
		// Limits intentionally absent — caller provided Requests only.
	}

	podSpec := buildPodSpec(spec)

	if len(podSpec.Containers) == 0 {
		t.Fatal("expected at least one container")
	}
	c := podSpec.Containers[0]

	req, ok := c.Resources.Requests[corev1.ResourceMemory]
	if !ok {
		t.Fatal("memory request not set")
	}
	if req.Cmp(customReq) != 0 {
		t.Errorf("memory request = %s, want %s", req.String(), customReq.String())
	}

	// The 8Gi default limit must NOT have been injected.
	defaultLimit := resource.MustParse("8Gi")
	if lim, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
		if lim.Cmp(defaultLimit) == 0 {
			t.Error("default 8Gi limit should not be injected when caller set Requests")
		}
	}
}

// ---------------------------------------------------------------------------
// Shared fixture helpers for builder tests
// ---------------------------------------------------------------------------

// serverFixture returns a *SatisfactoryServer with the given name and namespace,
// using defaultSpec() plus ServiceType LoadBalancer. It is used by the
// buildStatefulSet, buildHeadlessService, and buildExternalService tests.
func serverFixture() *satisfactoryv1alpha1.SatisfactoryServer {
	spec := defaultSpec()
	spec.ServiceType = corev1.ServiceTypeLoadBalancer
	return &satisfactoryv1alpha1.SatisfactoryServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-server",
			Namespace: "default",
		},
		Spec: spec,
	}
}

// findServicePort searches ports for an entry with the given name, returning
// the port and whether it was found.
func findServicePort(ports []corev1.ServicePort, name string) (corev1.ServicePort, bool) {
	for _, p := range ports {
		if p.Name == name {
			return p, true
		}
	}
	return corev1.ServicePort{}, false
}

// assertOwnerRef verifies that refs contains exactly one entry whose Controller
// field is true.
func assertOwnerRef(t *testing.T, refs []metav1.OwnerReference) {
	t.Helper()
	if len(refs) != 1 {
		t.Fatalf("expected 1 OwnerReference, got %d", len(refs))
	}
	if refs[0].Controller == nil || !*refs[0].Controller {
		t.Error("OwnerReference.Controller should be true")
	}
}

// ---------------------------------------------------------------------------
// TestBuildStatefulSet
// ---------------------------------------------------------------------------

func TestBuildStatefulSet(t *testing.T) {
	server := serverFixture()
	ss := buildStatefulSet(server)

	t.Run("Replicas is 1", func(t *testing.T) {
		if ss.Spec.Replicas == nil {
			t.Fatal("Replicas is nil")
		}
		if *ss.Spec.Replicas != 1 {
			t.Errorf("Replicas = %d, want 1", *ss.Spec.Replicas)
		}
	})

	t.Run("ServiceName is my-server-headless", func(t *testing.T) {
		want := "my-server-headless"
		if ss.Spec.ServiceName != want {
			t.Errorf("ServiceName = %q, want %q", ss.Spec.ServiceName, want)
		}
	})

	t.Run("labels contain app and server keys", func(t *testing.T) {
		wantLabels := map[string]string{
			"app": "my-server",
			"satisfactory.oliverziegert.github.com/server": "my-server",
		}
		for k, v := range wantLabels {
			if got, ok := ss.Labels[k]; !ok || got != v {
				t.Errorf("label %q = %q, want %q", k, got, v)
			}
		}
	})

	t.Run("Selector.MatchLabels equal the server labels", func(t *testing.T) {
		if ss.Spec.Selector == nil {
			t.Fatal("Selector is nil")
		}
		wantLabels := map[string]string{
			"app": "my-server",
			"satisfactory.oliverziegert.github.com/server": "my-server",
		}
		for k, v := range wantLabels {
			if got, ok := ss.Spec.Selector.MatchLabels[k]; !ok || got != v {
				t.Errorf("MatchLabels[%q] = %q, want %q", k, got, v)
			}
		}
		if len(ss.Spec.Selector.MatchLabels) != len(wantLabels) {
			t.Errorf("MatchLabels has %d entries, want %d", len(ss.Spec.Selector.MatchLabels), len(wantLabels))
		}
	})

	t.Run("container name is satisfactory", func(t *testing.T) {
		if len(ss.Spec.Template.Spec.Containers) == 0 {
			t.Fatal("no containers in pod template")
		}
		if got := ss.Spec.Template.Spec.Containers[0].Name; got != "satisfactory" {
			t.Errorf("container name = %q, want %q", got, "satisfactory")
		}
	})

	t.Run("container image is the default satisfactory image", func(t *testing.T) {
		if len(ss.Spec.Template.Spec.Containers) == 0 {
			t.Fatal("no containers in pod template")
		}
		want := "ghcr.io/robtme/satisfactory-server:v1.9.10"
		if got := ss.Spec.Template.Spec.Containers[0].Image; got != want {
			t.Errorf("container image = %q, want %q", got, want)
		}
	})

	t.Run("VolumeClaimTemplates has exactly 1 entry named config", func(t *testing.T) {
		if len(ss.Spec.VolumeClaimTemplates) != 1 {
			t.Fatalf("VolumeClaimTemplates len = %d, want 1", len(ss.Spec.VolumeClaimTemplates))
		}
		if got := ss.Spec.VolumeClaimTemplates[0].Name; got != "config" {
			t.Errorf("VolumeClaimTemplates[0].Name = %q, want %q", got, "config")
		}
	})

	t.Run("pod template Volumes is nil (PVC volume removed for StatefulSet)", func(t *testing.T) {
		if ss.Spec.Template.Spec.Volumes != nil {
			t.Errorf("Volumes should be nil, got %v", ss.Spec.Template.Spec.Volumes)
		}
	})

	t.Run("OwnerReferences has exactly 1 entry with Controller=true", func(t *testing.T) {
		assertOwnerRef(t, ss.OwnerReferences)
	})

	t.Run("readiness probe propagates to pod template container", func(t *testing.T) {
		if len(ss.Spec.Template.Spec.Containers) == 0 {
			t.Fatal("no containers in pod template")
		}
		if ss.Spec.Template.Spec.Containers[0].ReadinessProbe == nil {
			t.Error("ReadinessProbe should be set on StatefulSet pod template container")
		}
	})

	t.Run("liveness probe propagates to pod template container", func(t *testing.T) {
		if len(ss.Spec.Template.Spec.Containers) == 0 {
			t.Fatal("no containers in pod template")
		}
		if ss.Spec.Template.Spec.Containers[0].LivenessProbe == nil {
			t.Error("LivenessProbe should be set on StatefulSet pod template container")
		}
	})
}

// ---------------------------------------------------------------------------
// TestBuildHeadlessService
// ---------------------------------------------------------------------------

func TestBuildHeadlessService(t *testing.T) {
	server := serverFixture()
	svc := buildHeadlessService(server)

	t.Run("Name is my-server-headless", func(t *testing.T) {
		want := "my-server-headless"
		if svc.Name != want {
			t.Errorf("Name = %q, want %q", svc.Name, want)
		}
	})

	t.Run("ClusterIP is None", func(t *testing.T) {
		if svc.Spec.ClusterIP != "None" {
			t.Errorf("ClusterIP = %q, want %q", svc.Spec.ClusterIP, "None")
		}
	})

	t.Run("Selector matches server labels", func(t *testing.T) {
		wantSelector := map[string]string{
			"app": "my-server",
			"satisfactory.oliverziegert.github.com/server": "my-server",
		}
		for k, v := range wantSelector {
			if got, ok := svc.Spec.Selector[k]; !ok || got != v {
				t.Errorf("Selector[%q] = %q, want %q", k, got, v)
			}
		}
		if len(svc.Spec.Selector) != len(wantSelector) {
			t.Errorf("Selector has %d entries, want %d", len(svc.Spec.Selector), len(wantSelector))
		}
	})

	t.Run("Ports has 3 entries", func(t *testing.T) {
		if len(svc.Spec.Ports) != 3 {
			t.Fatalf("Ports len = %d, want 3", len(svc.Spec.Ports))
		}
	})

	t.Run("port game-tcp is 7777 TCP", func(t *testing.T) {
		p, found := findServicePort(svc.Spec.Ports, "game-tcp")
		if !found {
			t.Fatal("port game-tcp not found")
		}
		if p.Port != 7777 {
			t.Errorf("game-tcp port = %d, want 7777", p.Port)
		}
		if p.Protocol != corev1.ProtocolTCP {
			t.Errorf("game-tcp protocol = %q, want TCP", p.Protocol)
		}
	})

	t.Run("port game-udp is 7777 UDP", func(t *testing.T) {
		p, found := findServicePort(svc.Spec.Ports, "game-udp")
		if !found {
			t.Fatal("port game-udp not found")
		}
		if p.Port != 7777 {
			t.Errorf("game-udp port = %d, want 7777", p.Port)
		}
		if p.Protocol != corev1.ProtocolUDP {
			t.Errorf("game-udp protocol = %q, want UDP", p.Protocol)
		}
	})

	t.Run("port messaging is 8888 TCP", func(t *testing.T) {
		p, found := findServicePort(svc.Spec.Ports, "messaging")
		if !found {
			t.Fatal("port messaging not found")
		}
		if p.Port != 8888 {
			t.Errorf("messaging port = %d, want 8888", p.Port)
		}
		if p.Protocol != corev1.ProtocolTCP {
			t.Errorf("messaging protocol = %q, want TCP", p.Protocol)
		}
	})

	t.Run("OwnerReferences has exactly 1 entry with Controller=true", func(t *testing.T) {
		assertOwnerRef(t, svc.OwnerReferences)
	})

	t.Run("NodePort server leaves node ports unset", func(t *testing.T) {
		server := serverFixture()
		server.Spec.ServiceType = corev1.ServiceTypeNodePort
		for _, p := range buildHeadlessService(server).Spec.Ports {
			if p.NodePort != 0 {
				t.Errorf("%s nodePort = %d, want 0", p.Name, p.NodePort)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestBuildExternalService
// ---------------------------------------------------------------------------

func TestBuildExternalService(t *testing.T) {
	t.Run("Name is my-server (not headless)", func(t *testing.T) {
		server := serverFixture()
		svc := buildExternalService(server)
		if svc.Name != "my-server" {
			t.Errorf("Name = %q, want %q", svc.Name, "my-server")
		}
	})

	t.Run("ServiceType LoadBalancer is preserved", func(t *testing.T) {
		server := serverFixture()
		server.Spec.ServiceType = corev1.ServiceTypeLoadBalancer
		svc := buildExternalService(server)
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			t.Errorf("Spec.Type = %q, want LoadBalancer", svc.Spec.Type)
		}
	})

	t.Run("ServiceType NodePort is preserved", func(t *testing.T) {
		server := serverFixture()
		server.Spec.ServiceType = corev1.ServiceTypeNodePort
		svc := buildExternalService(server)
		if svc.Spec.Type != corev1.ServiceTypeNodePort {
			t.Errorf("Spec.Type = %q, want NodePort", svc.Spec.Type)
		}
	})

	t.Run("empty ServiceType falls back to LoadBalancer", func(t *testing.T) {
		server := serverFixture()
		server.Spec.ServiceType = ""
		svc := buildExternalService(server)
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			t.Errorf("Spec.Type = %q, want LoadBalancer", svc.Spec.Type)
		}
	})

	t.Run("Ports has 3 entries", func(t *testing.T) {
		server := serverFixture()
		svc := buildExternalService(server)
		if len(svc.Spec.Ports) != 3 {
			t.Fatalf("Ports len = %d, want 3", len(svc.Spec.Ports))
		}
	})

	t.Run("port game-tcp is 7777 TCP", func(t *testing.T) {
		server := serverFixture()
		svc := buildExternalService(server)
		p, found := findServicePort(svc.Spec.Ports, "game-tcp")
		if !found {
			t.Fatal("port game-tcp not found")
		}
		if p.Port != 7777 {
			t.Errorf("game-tcp port = %d, want 7777", p.Port)
		}
		if p.Protocol != corev1.ProtocolTCP {
			t.Errorf("game-tcp protocol = %q, want TCP", p.Protocol)
		}
	})

	t.Run("port game-udp is 7777 UDP", func(t *testing.T) {
		server := serverFixture()
		svc := buildExternalService(server)
		p, found := findServicePort(svc.Spec.Ports, "game-udp")
		if !found {
			t.Fatal("port game-udp not found")
		}
		if p.Port != 7777 {
			t.Errorf("game-udp port = %d, want 7777", p.Port)
		}
		if p.Protocol != corev1.ProtocolUDP {
			t.Errorf("game-udp protocol = %q, want UDP", p.Protocol)
		}
	})

	t.Run("port messaging is 8888 TCP", func(t *testing.T) {
		server := serverFixture()
		svc := buildExternalService(server)
		p, found := findServicePort(svc.Spec.Ports, "messaging")
		if !found {
			t.Fatal("port messaging not found")
		}
		if p.Port != 8888 {
			t.Errorf("messaging port = %d, want 8888", p.Port)
		}
		if p.Protocol != corev1.ProtocolTCP {
			t.Errorf("messaging protocol = %q, want TCP", p.Protocol)
		}
	})

	t.Run("OwnerReferences has exactly 1 entry with Controller=true", func(t *testing.T) {
		server := serverFixture()
		svc := buildExternalService(server)
		assertOwnerRef(t, svc.OwnerReferences)
	})

	t.Run("NodePort pins nodePort to port", func(t *testing.T) {
		server := serverFixture()
		server.Spec.ServiceType = corev1.ServiceTypeNodePort
		server.Spec.GamePort = 30777
		server.Spec.MessagingPort = 30888
		want := map[string]int32{"game-tcp": 30777, "game-udp": 30777, "messaging": 30888}
		for _, p := range buildExternalService(server).Spec.Ports {
			if p.Port != want[p.Name] || p.NodePort != want[p.Name] {
				t.Errorf("%s port/nodePort = %d/%d, want %d/%d", p.Name, p.Port, p.NodePort, want[p.Name], want[p.Name])
			}
		}
	})

	t.Run("LoadBalancer leaves nodePort unset", func(t *testing.T) {
		server := serverFixture()
		server.Spec.ServiceType = corev1.ServiceTypeLoadBalancer
		for _, p := range buildExternalService(server).Spec.Ports {
			if p.NodePort != 0 {
				t.Errorf("%s nodePort = %d, want 0", p.Name, p.NodePort)
			}
		}
	})
}

func TestServicePortsMatch(t *testing.T) {
	desired := []corev1.ServicePort{
		{Name: "game-tcp", Protocol: corev1.ProtocolTCP, Port: 7777},
		{Name: "messaging", Protocol: corev1.ProtocolTCP, Port: 8888},
	}
	pinned := []corev1.ServicePort{{Name: "game-tcp", Protocol: corev1.ProtocolTCP, Port: 30777, NodePort: 30777}}

	tests := []struct {
		name              string
		existing, desired []corev1.ServicePort
		want              bool
	}{
		{"equal", desired, desired, true},
		{"changed port", []corev1.ServicePort{
			{Name: "game-tcp", Protocol: corev1.ProtocolTCP, Port: 7778},
			{Name: "messaging", Protocol: corev1.ProtocolTCP, Port: 8888},
		}, desired, false},
		{"allocated node port ignored", []corev1.ServicePort{
			{Name: "game-tcp", Protocol: corev1.ProtocolTCP, Port: 7777, NodePort: 31234, TargetPort: intstrFromInt32(7777)},
			{Name: "messaging", Protocol: corev1.ProtocolTCP, Port: 8888, NodePort: 31235, TargetPort: intstrFromInt32(8888)},
		}, desired, true},
		{"pinned node port differs", []corev1.ServicePort{
			{Name: "game-tcp", Protocol: corev1.ProtocolTCP, Port: 30777, NodePort: 31234},
		}, pinned, false},
		{"different length", desired[:1], desired, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := servicePortsMatch(tt.existing, tt.desired); got != tt.want {
				t.Errorf("servicePortsMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildReadinessProbe
// ---------------------------------------------------------------------------

func TestBuildReadinessProbe_Default(t *testing.T) {
	spec := defaultSpec()
	probe := buildReadinessProbe(spec)

	if probe == nil {
		t.Fatal("expected non-nil probe")
	}

	t.Run("execs the image healthcheck", func(t *testing.T) {
		if probe.Exec == nil || len(probe.Exec.Command) != 1 || probe.Exec.Command[0] != "/home/steam/healthcheck.sh" {
			t.Fatalf("Exec = %+v, want /home/steam/healthcheck.sh", probe.Exec)
		}
	})

	t.Run("timeoutSeconds is 5", func(t *testing.T) {
		if probe.TimeoutSeconds != 5 {
			t.Errorf("timeoutSeconds = %d, want 5", probe.TimeoutSeconds)
		}
	})

	t.Run("periodSeconds is 10", func(t *testing.T) {
		if probe.PeriodSeconds != 10 {
			t.Errorf("periodSeconds = %d, want 10", probe.PeriodSeconds)
		}
	})

	t.Run("failureThreshold is 3", func(t *testing.T) {
		if probe.FailureThreshold != 3 {
			t.Errorf("failureThreshold = %d, want 3", probe.FailureThreshold)
		}
	})
}

func TestBuildReadinessProbe_Override(t *testing.T) {
	spec := defaultSpec()
	custom := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstrFromInt32(8080),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		FailureThreshold:    1,
	}
	spec.ReadinessProbeOverride = custom
	probe := buildReadinessProbe(spec)

	if probe != custom {
		t.Error("expected override probe to be returned as-is")
	}
}

// ---------------------------------------------------------------------------
// TestBuildLivenessProbe
// ---------------------------------------------------------------------------

func TestBuildLivenessProbe_Default(t *testing.T) {
	spec := defaultSpec()
	probe := buildLivenessProbe(spec)

	if probe == nil {
		t.Fatal("expected non-nil probe")
	}

	t.Run("uses TCPSocket handler", func(t *testing.T) {
		if probe.TCPSocket == nil {
			t.Fatal("TCPSocket should be set for default liveness probe")
		}
	})

	t.Run("port is game port", func(t *testing.T) {
		if probe.TCPSocket.Port.IntValue() != int(spec.GamePort) {
			t.Errorf("port = %d, want %d", probe.TCPSocket.Port.IntValue(), spec.GamePort)
		}
	})

	t.Run("timeoutSeconds is 5", func(t *testing.T) {
		if probe.TimeoutSeconds != 5 {
			t.Errorf("timeoutSeconds = %d, want 5", probe.TimeoutSeconds)
		}
	})

	t.Run("periodSeconds is 30", func(t *testing.T) {
		if probe.PeriodSeconds != 30 {
			t.Errorf("periodSeconds = %d, want 30", probe.PeriodSeconds)
		}
	})

	t.Run("failureThreshold is 10", func(t *testing.T) {
		if probe.FailureThreshold != 10 {
			t.Errorf("failureThreshold = %d, want 10", probe.FailureThreshold)
		}
	})
}

func TestBuildLivenessProbe_CustomPort(t *testing.T) {
	spec := defaultSpec()
	spec.GamePort = 7778
	probe := buildLivenessProbe(spec)

	if probe.TCPSocket == nil {
		t.Fatal("TCPSocket should be set")
	}
	if probe.TCPSocket.Port.IntValue() != 7778 {
		t.Errorf("port = %d, want 7778", probe.TCPSocket.Port.IntValue())
	}
}

func TestBuildLivenessProbe_Override(t *testing.T) {
	spec := defaultSpec()
	custom := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstrFromInt32(9999),
			},
		},
		InitialDelaySeconds: 60,
		PeriodSeconds:       60,
		FailureThreshold:    5,
	}
	spec.LivenessProbeOverride = custom
	probe := buildLivenessProbe(spec)

	if probe != custom {
		t.Error("expected override probe to be returned as-is")
	}
}
