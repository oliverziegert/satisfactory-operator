/*
Copyright 2022 Oliver Ziegert <paperless@pc-ziegert.dev>.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FactoryServerSpec defines the desired state of FactoryServer
type FactoryServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Size is the size of the FactoryServer StatefulSet
	//+optional
	//+kubebuilder:default=1
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=1
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Size int32 `json:"size,omitempty"`

	// Image
	//+optional
	//+kubebuilder:default="wolveix/satisfactory-server:latest"
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Image string `json:"image,omitempty"`

	// StorageClassName is the size of the FactoryServer StatefulSet
	//+optional
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	StorageClass string `json:"storageClass,omitempty"`

	// Autopause
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Autopause bool `json:"autopause,omitempty"`

	// AutosaveSpec
	//+optional
	//+kubebuilder:default={}
	Autosave AutosaveSpec `json:"autosave,omitempty"`

	// CrashReport
	//+optional
	//+kubebuilder:default=false
	CrashReport bool `json:"crashReport,omitempty"`

	// Debug
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Debug bool `json:"debug,omitempty"`

	// DisableSeasonalEvents
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	DisableSeasonalEvents bool `json:"disableSeasonalEvents,omitempty"`

	// Maxplayers
	//+optional
	//+kubebuilder:default=4
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Maxplayers uint64 `json:"maxplayers,omitempty"`

	// Networkquality
	//+optional
	//+kubebuilder:default=3
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Networkquality uint64 `json:"networkquality,omitempty"`

	// Ports
	//+optional
	//+kubebuilder:default={}
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Ports *PortSpec `json:"ports,omitempty"`

	// SkipUpdate
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	SkipUpdate bool `json:"skipUpdate,omitempty"`

	// Beta
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Beta bool `json:"steamBeta,omitempty"`

	// Service
	//+optional
	//+kubebuilder:default={}
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Service *ServiceSpec `json:"service,omitempty"`
}

type AutosaveSpec struct {

	// Interval
	//+optional
	//+kubebuilder:default=300
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Interval uint64 `json:"interval,omitempty"`

	// Num
	//+optional
	//+kubebuilder:default=5
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Num uint64 `json:"num,omitempty"`

	// OnDisconnect
	//+optional
	//+kubebuilder:default=true
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	OnDisconnect bool `json:"onDisconnect,omitempty"`
}

type PortSpec struct {

	// Beacon
	//+optional
	//+kubebuilder:default=15000
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Beacon int32 `json:"beacon,omitempty"`

	// Game
	//+optional
	//+kubebuilder:default=7777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Game int32 `json:"game,omitempty"`

	// Query
	//+optional
	//+kubebuilder:default=15777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Query int32 `json:"query,omitempty"`
}

type ServiceSpec struct {

	// Type
	//+optional
	//+kubebuilder:default="ClusterIP"
	//+kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Type corev1.ServiceType `json:"type,omitempty"`

	// LoadBalancerClass
	//+optional
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	LoadBalancerClass string `json:"loadBalancerClass,omitempty"`

	// LoadBalancerSourceRanges
	//+optional
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`

	// NodePorts
	//+optional
	//+kubebuilder:default={}
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	NodePorts NodePortsSpec `json:"nodePorts"`
}

type NodePortsSpec struct {

	// Beacon
	//+optional
	//+kubebuilder:default=15000
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Beacon int32 `json:"beacon,omitempty"`

	// Game
	//+optional
	//+kubebuilder:default=7777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Game int32 `json:"game,omitempty"`

	// Query
	//+optional
	//+kubebuilder:default=15777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Query int32 `json:"query,omitempty"`
}

// FactoryServerStatus defines the observed state of FactoryServer
type FactoryServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Represents the observations of a FactoryServer's current state.
	// FactoryServer.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// FactoryServer.status.conditions.status are one of True, False, Unknown.
	// FactoryServer.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// FactoryServer.status.conditions.Message is a human-readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Conditions stors the status conditions of the FactoryServer instances
	//+operator--sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// Nodes are the names of the factoryServer pods
	Nodes []string `json:"nodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+operator--sdk:csv:customresourcedefinitions:resources={{StatefulSet,v1,factoryServer-sts}}

// FactoryServer is the Schema for the factoryServers API
type FactoryServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FactoryServerSpec   `json:"spec,omitempty"`
	Status FactoryServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FactoryServerList contains a list of FactoryServer
type FactoryServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FactoryServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FactoryServer{}, &FactoryServerList{})
}
