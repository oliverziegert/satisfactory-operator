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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

// configVolumeName names the /config volume, its mount and the StatefulSet PVC template.
const configVolumeName = "config"

// intstrFromInt32 converts an int32 to an intstr.IntOrString.
func intstrFromInt32(v int32) intstr.IntOrString {
	return intstr.FromInt32(v)
}

// buildEnvVars maps all SatisfactoryServerSpec fields to the environment
// variables expected by the robtme/satisfactory-server container image.
func buildEnvVars(spec satisfactoryv1alpha1.SatisfactoryServerSpec) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		// 2.2 Server config fields
		{Name: "MAXPLAYERS", Value: fmt.Sprintf("%d", int32OrDefault(spec.MaxPlayers, 4))},
		{Name: "AUTOSAVENUM", Value: fmt.Sprintf("%d", spec.AutosaveNum)},
		{Name: "MAXTICKRATE", Value: fmt.Sprintf("%d", int32OrDefault(spec.MaxTickRate, 30))},
		{Name: "MAXOBJECTS", Value: fmt.Sprintf("%d", spec.MaxObjects)},
		{Name: "TIMEOUT", Value: fmt.Sprintf("%d", spec.Timeout)},

		// 2.3 Boolean config fields
		{Name: "SERVERSTREAMING", Value: boolToString(spec.ServerStreaming, true)},
		{Name: "DISABLESEASONALEVENTS", Value: boolToString(spec.DisableSeasonalEvents, false)},
		{Name: "DEBUG", Value: boolToString(spec.Debug, false)},
		{Name: "LOG", Value: boolToString(spec.Log, false)},

		// 2.4 Network fields
		{Name: "SERVERGAMEPORT", Value: fmt.Sprintf("%d", spec.GamePort)},
		{Name: "SERVERMESSAGINGPORT", Value: fmt.Sprintf("%d", spec.MessagingPort)},
		{Name: "MULTIHOME", Value: spec.Multihome},

		// 2.5 Update control fields
		{Name: "SKIPUPDATE", Value: boolToString(spec.SkipUpdate, true)},
		{Name: "STEAMBETA", Value: boolToString(spec.SteamBeta, false)},

		// 2.6 User identity fields
		{Name: "PUID", Value: fmt.Sprintf("%d", ptrInt64OrDefault(spec.RunAsUser, 1000))},
		{Name: "PGID", Value: fmt.Sprintf("%d", ptrInt64OrDefault(spec.RunAsGroup, 1000))},
	}

	// 2.5 STEAMBETAKEY is only set when provided
	if spec.SteamBetaPassword != nil && *spec.SteamBetaPassword != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "STEAMBETAKEY",
			Value: *spec.SteamBetaPassword,
		})
	}

	return envVars
}

// buildContainerPorts generates port definitions for the Satisfactory server container.
// The game uses gamePort on both TCP (HTTPS API) and UDP (game traffic), plus
// messagingPort on TCP (reliable messaging).
func buildContainerPorts(spec satisfactoryv1alpha1.SatisfactoryServerSpec) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "game-tcp", ContainerPort: spec.GamePort, Protocol: corev1.ProtocolTCP},
		{Name: "game-udp", ContainerPort: spec.GamePort, Protocol: corev1.ProtocolUDP},
		{Name: "messaging", ContainerPort: spec.MessagingPort, Protocol: corev1.ProtocolTCP},
	}
}

// buildVolumeMount returns the VolumeMount for the /config directory.
func buildVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      configVolumeName,
		MountPath: "/config",
	}
}

// buildPVCSpec returns the PersistentVolumeClaimSpec for the server's /config volume.
func buildPVCSpec(spec satisfactoryv1alpha1.SatisfactoryServerSpec) corev1.PersistentVolumeClaimSpec {
	storageSize := spec.Storage.Size
	if storageSize.IsZero() {
		storageSize = resource.MustParse("15Gi")
	}

	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: storageSize,
			},
		},
	}
	if spec.Storage.StorageClassName != nil {
		pvcSpec.StorageClassName = spec.Storage.StorageClassName
	}
	return pvcSpec
}

// buildReadinessProbe returns the readiness probe for the Satisfactory server container.
// If ReadinessProbeOverride is set on the spec, it is returned as-is. Otherwise the default
// probe runs the image's healthcheck.sh (POST HealthCheck on the HTTPS API), gated by the startup probe.
func buildReadinessProbe(spec satisfactoryv1alpha1.SatisfactoryServerSpec) *corev1.Probe {
	if spec.ReadinessProbeOverride != nil {
		return spec.ReadinessProbeOverride
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			// The API only answers POST, so an httpGet probe always gets 404.
			Exec: &corev1.ExecAction{Command: []string{"/home/steam/healthcheck.sh"}},
		},
		TimeoutSeconds:   5,
		PeriodSeconds:    10,
		FailureThreshold: 3,
	}
}

// buildLivenessProbe returns the liveness probe for the Satisfactory server container.
// If LivenessProbeOverride is set on the spec, it is returned as-is. Otherwise the default
// probe is a TCP socket check on the game port, gated by the startup probe.
func buildLivenessProbe(spec satisfactoryv1alpha1.SatisfactoryServerSpec) *corev1.Probe {
	if spec.LivenessProbeOverride != nil {
		return spec.LivenessProbeOverride
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstrFromInt32(spec.GamePort),
			},
		},
		TimeoutSeconds:   5,
		PeriodSeconds:    30,
		FailureThreshold: 10,
	}
}

// buildStartupProbe returns the startup probe for the Satisfactory server container.
// It holds off liveness and readiness until the game port opens, which on first boot
// waits for the image to download the full game (8GB+) even when SkipUpdate is set.
func buildStartupProbe(spec satisfactoryv1alpha1.SatisfactoryServerSpec) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstrFromInt32(spec.GamePort),
			},
		},
		PeriodSeconds:    10,
		FailureThreshold: 360, // 60 minutes
	}
}

// buildPodSpec assembles the complete PodSpec for a SatisfactoryServer instance.
func buildPodSpec(spec satisfactoryv1alpha1.SatisfactoryServerSpec) corev1.PodSpec {
	resources := spec.Resources
	if resources.Requests == nil && resources.Limits == nil {
		resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		}
	}

	image := spec.Image
	if image == "" {
		image = "ghcr.io/robtme/satisfactory-server:v1.9.10"
	}

	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "satisfactory",
				Image:           image,
				Env:             buildEnvVars(spec),
				Ports:           buildContainerPorts(spec),
				VolumeMounts:    []corev1.VolumeMount{buildVolumeMount()},
				Resources:       resources,
				ImagePullPolicy: corev1.PullIfNotPresent,
				ReadinessProbe:  buildReadinessProbe(spec),
				LivenessProbe:   buildLivenessProbe(spec),
				StartupProbe:    buildStartupProbe(spec),
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: configVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: configVolumeName,
					},
				},
			},
		},
	}
}

// boolToString converts a *bool pointer to the string "true"/"false",
// using defaultVal when the pointer is nil.
func boolToString(b *bool, defaultVal bool) string {
	if b == nil {
		return fmt.Sprintf("%t", defaultVal)
	}
	return fmt.Sprintf("%t", *b)
}

// int32OrDefault returns v, or defaultVal when v is unset (zero).
func int32OrDefault(v, defaultVal int32) int32 {
	if v == 0 {
		return defaultVal
	}
	return v
}

// ptrInt64OrDefault returns the value of a *int64 pointer, or defaultVal when nil.
func ptrInt64OrDefault(p *int64, defaultVal int64) int64 {
	if p == nil {
		return defaultVal
	}
	return *p
}

// serverLabels returns the standard label set applied to all resources owned by a SatisfactoryServer.
func serverLabels(name string) map[string]string {
	return map[string]string{
		"app": name,
		"satisfactory.oliverziegert.github.com/server": name,
	}
}

// buildStatefulSet constructs the StatefulSet for a SatisfactoryServer instance.
func buildStatefulSet(server *satisfactoryv1alpha1.SatisfactoryServer) *appsv1.StatefulSet {
	labels := serverLabels(server.Name)
	replicas := int32(1)
	pvcSpec := buildPVCSpec(server.Spec)

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(server, satisfactoryv1alpha1.GroupVersion.WithKind("SatisfactoryServer")),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: server.Name + "-headless",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       buildPodSpec(server.Spec),
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: configVolumeName},
					Spec:       pvcSpec,
				},
			},
		},
	}
	// Remove the inline PVC volume added by buildPodSpec — the StatefulSet
	// volumeClaimTemplate provides it instead.
	ss.Spec.Template.Spec.Volumes = nil
	return ss
}

// containerPortsToServicePorts converts ContainerPort definitions to ServicePort definitions.
func containerPortsToServicePorts(ports []corev1.ContainerPort) []corev1.ServicePort {
	svcPorts := make([]corev1.ServicePort, len(ports))
	for i, p := range ports {
		svcPorts[i] = corev1.ServicePort{
			Name:     p.Name,
			Protocol: p.Protocol,
			Port:     p.ContainerPort,
		}
	}
	return svcPorts
}

// buildHeadlessService constructs the governing headless Service for a SatisfactoryServer StatefulSet.
func buildHeadlessService(server *satisfactoryv1alpha1.SatisfactoryServer) *corev1.Service {
	labels := serverLabels(server.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name + "-headless",
			Namespace: server.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(server, satisfactoryv1alpha1.GroupVersion.WithKind("SatisfactoryServer")),
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports:     containerPortsToServicePorts(buildContainerPorts(server.Spec)),
		},
	}
}

// buildExternalService constructs the player-facing Service for a SatisfactoryServer.
func buildExternalService(server *satisfactoryv1alpha1.SatisfactoryServer) *corev1.Service {
	labels := serverLabels(server.Name)
	svcType := server.Spec.ServiceType
	if svcType == "" {
		svcType = corev1.ServiceTypeLoadBalancer
	}
	ports := containerPortsToServicePorts(buildContainerPorts(server.Spec))
	if svcType == corev1.ServiceTypeNodePort {
		// Game clients connect to the ports the server advertises, so every layer must use the same number.
		for i := range ports {
			ports[i].NodePort = ports[i].Port
		}
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(server, satisfactoryv1alpha1.GroupVersion.WithKind("SatisfactoryServer")),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: labels,
			Ports:    ports,
		},
	}
}

// servicePortsMatch reports whether existing has the operator-owned fields of desired.
// API-server-defaulted fields (TargetPort, an allocated NodePort) are ignored.
func servicePortsMatch(existing, desired []corev1.ServicePort) bool {
	if len(existing) != len(desired) {
		return false
	}
	for i, d := range desired {
		e := existing[i]
		if e.Name != d.Name || e.Protocol != d.Protocol || e.Port != d.Port {
			return false
		}
		if d.NodePort != 0 && e.NodePort != d.NodePort {
			return false
		}
	}
	return true
}
