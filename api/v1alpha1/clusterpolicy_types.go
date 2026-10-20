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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterPolicy condition type constants.
const (
	// ClusterPolicyConditionTypeReady indicates the policy has been applied to all governed servers.
	ClusterPolicyConditionTypeReady = "Ready"
	// ClusterPolicyConditionTypeEnforced indicates all governed servers comply with the policy.
	ClusterPolicyConditionTypeEnforced = "Enforced"
)

// ClusterPolicyServerDefaults defines default server configuration values applied to new SatisfactoryServer resources.
type ClusterPolicyServerDefaults struct {
	// MaxPlayers is the default maximum number of players.
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxPlayers *int32 `json:"maxPlayers,omitempty"`

	// MaxTickRate is the default maximum tick rate.
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxTickRate *int32 `json:"maxTickRate,omitempty"`

	// SkipUpdate is the default value for skipping game updates on pod restart.
	// +optional
	SkipUpdate *bool `json:"skipUpdate,omitempty"`
}

// ClusterPolicySpec defines the desired state of ClusterPolicy.
type ClusterPolicySpec struct {
	// DefaultResourceLimits defines the default CPU and memory resource requirements
	// applied to SatisfactoryServer containers when not explicitly specified.
	// +optional
	DefaultResourceLimits *corev1.ResourceRequirements `json:"defaultResourceLimits,omitempty"`

	// DefaultStorageClassName is the default StorageClass name used for PVCs
	// when not specified in the SatisfactoryServer resource.
	// +optional
	DefaultStorageClassName *string `json:"defaultStorageClassName,omitempty"`

	// DefaultServerConfig provides default server configuration values
	// applied to new SatisfactoryServer resources.
	// +optional
	DefaultServerConfig *ClusterPolicyServerDefaults `json:"defaultServerConfig,omitempty"`

	// AllowedImageVersions is a list of permitted container image references for
	// SatisfactoryServer resources. If empty, any image is allowed.
	// +optional
	AllowedImageVersions []string `json:"allowedImageVersions,omitempty"`

	// MaxServersPerNamespace is the maximum number of SatisfactoryServer resources
	// allowed in a single namespace. 0 means no limit.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxServersPerNamespace *int32 `json:"maxServersPerNamespace,omitempty"`
}

// ClusterPolicyStatus defines the observed state of ClusterPolicy.
type ClusterPolicyStatus struct {
	// Conditions represent the latest available observations of the ClusterPolicy's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ServersGoverned is the total number of SatisfactoryServer resources currently
	// governed by this ClusterPolicy across all namespaces.
	// +optional
	ServersGoverned int32 `json:"serversGoverned,omitempty"`

	// LastAppliedTime is the timestamp when this ClusterPolicy was last applied.
	// +optional
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Servers",type="integer",JSONPath=".status.serversGoverned"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterPolicy is the Schema for the clusterpolicies API.
type ClusterPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterPolicySpec   `json:"spec,omitempty"`
	Status ClusterPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterPolicyList contains a list of ClusterPolicy.
type ClusterPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterPolicy{}, &ClusterPolicyList{})
}
