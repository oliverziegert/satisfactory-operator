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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SatisfactoryServerPhase represents the lifecycle phase of a SatisfactoryServer.
// +kubebuilder:validation:Enum=Pending;Running;Updating;Error
type SatisfactoryServerPhase string

const (
	SatisfactoryServerPhasePending  SatisfactoryServerPhase = "Pending"
	SatisfactoryServerPhaseRunning  SatisfactoryServerPhase = "Running"
	SatisfactoryServerPhaseUpdating SatisfactoryServerPhase = "Updating"
	SatisfactoryServerPhaseError    SatisfactoryServerPhase = "Error"
)

// SatisfactoryServer condition type constants.
const (
	// SatisfactoryServerConditionTypeReady indicates the server is fully operational:
	// StatefulSet ready AND game API reachable AND a world is loaded.
	SatisfactoryServerConditionTypeReady = "Ready"
	// SatisfactoryServerConditionTypeProgressing indicates a StatefulSet rollout is in progress.
	SatisfactoryServerConditionTypeProgressing = "Progressing"
	// SatisfactoryServerConditionTypeDegraded indicates the StatefulSet is ready but the
	// game server API is unreachable or returning errors.
	SatisfactoryServerConditionTypeDegraded = "Degraded"
	// SatisfactoryServerConditionTypeServerReachable indicates the game server HTTPS API responds.
	SatisfactoryServerConditionTypeServerReachable = "ServerReachable"
	// SatisfactoryServerConditionTypeServerClaimed indicates the server has been claimed
	// (is no longer in the initial unclaimed state).
	SatisfactoryServerConditionTypeServerClaimed = "ServerClaimed"
	// SatisfactoryServerConditionTypeWorldLoaded indicates an active game session is loaded.
	SatisfactoryServerConditionTypeWorldLoaded = "WorldLoaded"
	// SatisfactoryServerConditionTypeAuthenticated indicates whether the controller holds
	// a valid API authentication token for the game server.
	SatisfactoryServerConditionTypeAuthenticated = "Authenticated"
)

// SatisfactoryServerStorage defines persistent storage configuration for the server.
type SatisfactoryServerStorage struct {
	// Size is the requested storage capacity for the /config volume.
	// Defaults to 15Gi.
	// +kubebuilder:default="15Gi"
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClassName is the name of the StorageClass to use for the PVC.
	// If empty, the cluster default StorageClass is used.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// SatisfactoryServerSpec defines the desired state of SatisfactoryServer.
type SatisfactoryServerSpec struct {
	// Image is the container image for the Satisfactory Dedicated Server.
	// Defaults to ghcr.io/robtme/satisfactory-server:v1.9.10.
	// +kubebuilder:default="ghcr.io/robtme/satisfactory-server:v1.9.10"
	// +optional
	Image string `json:"image,omitempty"`

	// MaxPlayers is the maximum number of players allowed on the server.
	// Maps to the MAXPLAYERS environment variable.
	// +kubebuilder:default=4
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxPlayers int32 `json:"maxPlayers,omitempty"`

	// AutosaveNum is the number of rotating autosave files to keep.
	// Maps to the AUTOSAVENUM environment variable.
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +optional
	AutosaveNum int32 `json:"autosaveNum,omitempty"`

	// MaxTickRate is the maximum simulation tick rate for the server.
	// Maps to the MAXTICKRATE environment variable.
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxTickRate int32 `json:"maxTickRate,omitempty"`

	// MaxObjects sets the object limit for the server.
	// Maps to the MAXOBJECTS environment variable.
	// +kubebuilder:default=2162688
	// +optional
	MaxObjects int64 `json:"maxObjects,omitempty"`

	// Timeout is the client timeout in seconds.
	// Maps to the TIMEOUT environment variable.
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	// +optional
	Timeout int32 `json:"timeout,omitempty"`

	// ServerStreaming toggles whether the game utilizes asset streaming.
	// Maps to the SERVERSTREAMING environment variable.
	// +kubebuilder:default=true
	// +optional
	ServerStreaming *bool `json:"serverStreaming,omitempty"`

	// DisableSeasonalEvents disables seasonal in-game events (e.g. FICSMAS).
	// Maps to the DISABLESEASONALEVENTS environment variable.
	// +kubebuilder:default=false
	// +optional
	DisableSeasonalEvents *bool `json:"disableSeasonalEvents,omitempty"`

	// Debug enables debug logging for the server.
	// Maps to the DEBUG environment variable.
	// +kubebuilder:default=false
	// +optional
	Debug *bool `json:"debug,omitempty"`

	// Log disables Satisfactory log pruning when true.
	// Maps to the LOG environment variable.
	// +kubebuilder:default=false
	// +optional
	Log *bool `json:"log,omitempty"`

	// GamePort is the port used for game traffic (UDP) and the HTTPS API (TCP).
	// Maps to the SERVERGAMEPORT environment variable.
	// +kubebuilder:default=7777
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	GamePort int32 `json:"gamePort,omitempty"`

	// MessagingPort is the port used for reliable messaging traffic (TCP).
	// Maps to the SERVERMESSAGINGPORT environment variable.
	// +kubebuilder:default=8888
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	MessagingPort int32 `json:"messagingPort,omitempty"`

	// Multihome sets the server's listening interface.
	// Maps to the MULTIHOME environment variable.
	// +kubebuilder:default="::"
	// +optional
	Multihome string `json:"multihome,omitempty"`

	// SkipUpdate avoids updating the game on container start/restart.
	// Maps to the SKIPUPDATE environment variable.
	// Defaults to true in Kubernetes to prevent unexpected 8GB+ downloads on pod restart.
	// +kubebuilder:default=true
	// +optional
	SkipUpdate *bool `json:"skipUpdate,omitempty"`

	// SteamBeta enables the experimental branch of the game.
	// Maps to the STEAMBETA environment variable.
	// +kubebuilder:default=false
	// +optional
	SteamBeta *bool `json:"steamBeta,omitempty"`

	// SteamBetaPassword is the password for the Steam beta branch (if required).
	// Maps to the STEAMBETAKEY environment variable.
	// +optional
	SteamBetaPassword *string `json:"steamBetaPassword,omitempty"`

	// RunAsUser is the UID the server process runs as (PUID).
	// +kubebuilder:default=1000
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`

	// RunAsGroup is the GID the server process runs as (PGID).
	// +kubebuilder:default=1000
	// +optional
	RunAsGroup *int64 `json:"runAsGroup,omitempty"`

	// Storage defines the persistent volume configuration for the /config directory.
	// +optional
	Storage SatisfactoryServerStorage `json:"storage,omitempty"`

	// Resources defines CPU and memory resource requests and limits for the server container.
	// Defaults to 4Gi memory request and 8Gi memory limit.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ServiceType is the Kubernetes Service type used to expose the server externally.
	// +kubebuilder:default="LoadBalancer"
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// RetainPVCOnDelete controls whether the /config PVC is preserved when the
	// SatisfactoryServer resource is deleted. Defaults to true to prevent data loss.
	// +kubebuilder:default=true
	// +optional
	RetainPVCOnDelete *bool `json:"retainPVCOnDelete,omitempty"`

	// AdminPasswordSecretRef optionally references a Secret in the same namespace that
	// contains the admin password for the game server API under key "password".
	// When set, the controller authenticates with the game server API and caches the token
	// in a managed Secret named "<server-name>-auth-token".
	// +optional
	AdminPasswordSecretRef *corev1.LocalObjectReference `json:"adminPasswordSecretRef,omitempty"`

	// ServerName is the in-game display name set when the server is claimed.
	// Defaults to the Kubernetes resource name when not specified.
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// AutoClaimDisabled disables automatic server claiming when set to true.
	// Defaults to false (auto-claiming is enabled).
	// Use this when you want to claim the server manually or when migrating an existing server.
	// +kubebuilder:default=false
	// +optional
	AutoClaimDisabled *bool `json:"autoClaimDisabled,omitempty"`

	// ReadinessProbeOverride replaces the default readiness probe when set.
	// The default probe issues an HTTPS GET on /api/v1 with initialDelaySeconds: 30,
	// periodSeconds: 10, and failureThreshold: 3.
	// +optional
	ReadinessProbeOverride *corev1.Probe `json:"readinessProbeOverride,omitempty"`

	// LivenessProbeOverride replaces the default liveness probe when set.
	// The default probe uses a TCP socket check on the game port with
	// initialDelaySeconds: 120, periodSeconds: 30, and failureThreshold: 10.
	// +optional
	LivenessProbeOverride *corev1.Probe `json:"livenessProbeOverride,omitempty"`
}

// SatisfactoryServerStatus defines the observed state of SatisfactoryServer.
type SatisfactoryServerStatus struct {
	// Phase is the current lifecycle phase of the server.
	// +optional
	Phase SatisfactoryServerPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the server's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ServerVersion is the version of the Satisfactory Dedicated Server currently running.
	// +optional
	ServerVersion string `json:"serverVersion,omitempty"`

	// ConnectedPlayers is the number of players currently connected to the server.
	// +optional
	ConnectedPlayers int32 `json:"connectedPlayers,omitempty"`

	// LastSaveTime is the timestamp of the most recent save operation.
	// +optional
	LastSaveTime *metav1.Time `json:"lastSaveTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Players",type="integer",JSONPath=".status.connectedPlayers"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SatisfactoryServer is the Schema for the satisfactoryservers API.
type SatisfactoryServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SatisfactoryServerSpec   `json:"spec,omitempty"`
	Status SatisfactoryServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SatisfactoryServerList contains a list of SatisfactoryServer.
type SatisfactoryServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SatisfactoryServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SatisfactoryServer{}, &SatisfactoryServerList{})
}
