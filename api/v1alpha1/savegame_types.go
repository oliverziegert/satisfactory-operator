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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SaveGamePhase represents the lifecycle phase of a SaveGame.
// +kubebuilder:validation:Enum=Pending;Active;Completed;Failed
type SaveGamePhase string

const (
	SaveGamePhasePending   SaveGamePhase = "Pending"
	SaveGamePhaseActive    SaveGamePhase = "Active"
	SaveGamePhaseCompleted SaveGamePhase = "Completed"
	SaveGamePhaseFailed    SaveGamePhase = "Failed"
)

// SaveGame condition type constants.
const (
	// SaveGameConditionTypeReady indicates the SaveGame resource is reconciled and operational.
	SaveGameConditionTypeReady = "Ready"
	// SaveGameConditionTypeBackupScheduled indicates a cron schedule is active and the next
	// backup is planned.
	SaveGameConditionTypeBackupScheduled = "BackupScheduled"
)

// SaveGameRetentionPolicy defines how many saves to keep and for how long.
type SaveGameRetentionPolicy struct {
	// MaxCount is the maximum number of save files to retain.
	// Older saves are deleted when this limit is exceeded.
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxCount *int32 `json:"maxCount,omitempty"`

	// MaxAge is the maximum age of save files to retain (e.g. "72h").
	// Save files older than this duration are deleted.
	// +optional
	MaxAge *metav1.Duration `json:"maxAge,omitempty"`
}

// SaveGameSpec defines the desired state of SaveGame.
type SaveGameSpec struct {
	// ServerRef is the name of the SatisfactoryServer resource this SaveGame belongs to.
	// Must be in the same namespace.
	// +kubebuilder:validation:Required
	ServerRef string `json:"serverRef"`

	// SaveName is the name of the save game file on the server.
	// The server may sanitize the name to satisfy filesystem restrictions.
	// +kubebuilder:validation:Required
	SaveName string `json:"saveName"`

	// RetentionPolicy defines how many save files to keep and for how long.
	// +optional
	RetentionPolicy *SaveGameRetentionPolicy `json:"retentionPolicy,omitempty"`

	// Schedule is a cron expression for automated backups (e.g. "0 * * * *" for hourly).
	// If empty, no automated backups are performed.
	// +optional
	Schedule string `json:"schedule,omitempty"`
}

// SaveGameStatus defines the observed state of SaveGame.
type SaveGameStatus struct {
	// Phase is the current lifecycle phase of the save game operation.
	// +optional
	Phase SaveGamePhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the SaveGame's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastBackupTime is the timestamp of the most recent successful backup.
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// BackupSize is the size of the most recent backup file in bytes.
	// +optional
	BackupSize *int64 `json:"backupSize,omitempty"`

	// NextBackupTime is when the next scheduled backup will occur.
	// +optional
	NextBackupTime *metav1.Time `json:"nextBackupTime,omitempty"`

	// RetainedSaveCount is the number of save files on the server belonging to this SaveGame.
	// +optional
	RetainedSaveCount int32 `json:"retainedSaveCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Server",type="string",JSONPath=".spec.serverRef"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Last Backup",type="date",JSONPath=".status.lastBackupTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SaveGame is the Schema for the savegames API.
type SaveGame struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SaveGameSpec   `json:"spec,omitempty"`
	Status SaveGameStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SaveGameList contains a list of SaveGame.
type SaveGameList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SaveGame `json:"items"`
}
