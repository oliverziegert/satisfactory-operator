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

	// Destinations are where each backup is written. Empty means [{name: server, server: {}}],
	// which keeps the save on the game server only.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=10
	// +kubebuilder:validation:XValidation:rule="self.filter(d, has(d.server)).size() <= 1",message="at most one server destination is allowed"
	Destinations []SaveGameDestination `json:"destinations,omitempty"`
}

// SaveGameDestination is a named backup destination. Exactly one type field must be set.
// +kubebuilder:validation:XValidation:rule="[has(self.server), has(self.pvc), has(self.s3)].filter(x, x).size() == 1",message="exactly one destination type must be set"
// +kubebuilder:validation:XValidation:rule="!(has(self.pvc) || has(self.s3)) || size(self.name) <= 58",message="pvc and s3 destination names must be at most 58 characters (the Job volume is dest-<name>)"
type SaveGameDestination struct {
	// Name identifies the destination in status. It must be a DNS label.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// RetentionPolicy overrides spec.retentionPolicy for this destination.
	// +optional
	RetentionPolicy *SaveGameRetentionPolicy `json:"retentionPolicy,omitempty"`

	// Server keeps the save on the game server.
	// +optional
	Server *ServerDestination `json:"server,omitempty"`

	// PVC writes saves to an existing PersistentVolumeClaim in the SaveGame's namespace.
	// +optional
	PVC *PVCDestination `json:"pvc,omitempty"`

	// S3 uploads saves to an existing bucket on AWS S3 or an S3-compatible store.
	// +optional
	S3 *S3Destination `json:"s3,omitempty"`
}

// ServerDestination keeps saves on the game server's own volume. It has no settings.
type ServerDestination struct{}

// PVCDestination writes saves to a user-owned PersistentVolumeClaim. The operator never
// creates, modifies or deletes the claim.
type PVCDestination struct {
	// ClaimName is the name of the PersistentVolumeClaim in the SaveGame's namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ClaimName string `json:"claimName"`

	// Path is an optional subdirectory on the claim. It must be relative and must not
	// contain ".." segments. It is created if missing.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('/')",message="path must be relative"
	// +kubebuilder:validation:XValidation:rule="!self.split('/').exists(s, s == '..')",message="path must not contain '..' segments"
	Path string `json:"path,omitempty"`
}

// S3Destination uploads saves as <prefix>/<name>.sav objects. The bucket must exist.
type S3Destination struct {
	// Bucket is the name of an existing bucket.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Bucket string `json:"bucket"`

	// Prefix is an optional key prefix. Empty means the bucket root.
	// +optional
	// +kubebuilder:validation:MaxLength=512
	Prefix string `json:"prefix,omitempty"`

	// Endpoint is the URL of an S3-compatible store. Empty means AWS S3.
	// +optional
	// +kubebuilder:validation:Format=uri
	// +kubebuilder:validation:MaxLength=2048
	Endpoint string `json:"endpoint,omitempty"`

	// Region is the bucket's region. Defaults to us-east-1.
	// +optional
	// +kubebuilder:validation:MaxLength=64
	Region string `json:"region,omitempty"`

	// ForcePathStyle uses path-style URLs (endpoint/bucket/key), which most S3-compatible
	// stores need.
	// +optional
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`

	// CredentialsSecretRef names a Secret in the SaveGame's namespace with the keys
	// AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY. Without it the SDK's default credential
	// chain is used.
	// +optional
	CredentialsSecretRef *S3CredentialsSecretRef `json:"credentialsSecretRef,omitempty"`
}

// S3CredentialsSecretRef references a Secret in the SaveGame's namespace.
type S3CredentialsSecretRef struct {
	// Name is the Secret's name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// SaveGameDestinationStatus is the outcome of the latest backup run for one destination.
type SaveGameDestinationStatus struct {
	// Name is the destination name from spec.destinations.
	Name string `json:"name"`

	// LastSuccessTime is the time of the latest run that wrote to this destination.
	// +optional
	LastSuccessTime *metav1.Time `json:"lastSuccessTime,omitempty"`

	// LastSize is the size in bytes of the save written by the latest successful run.
	// +optional
	LastSize *int64 `json:"lastSize,omitempty"`

	// RetainedCount is the number of saves this destination holds for the session.
	// +optional
	RetainedCount int32 `json:"retainedCount,omitempty"`

	// LastError is the error of the latest run, empty if it succeeded.
	// +optional
	LastError string `json:"lastError,omitempty"`
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

	// Destinations holds the per-destination results of the latest backup run.
	// +optional
	// +listType=map
	// +listMapKey=name
	Destinations []SaveGameDestinationStatus `json:"destinations,omitempty"`
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
