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

// FactoryServerSpec defines the desired state of FactoryServe
type FactoryServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Size is the size of the FactoryServer StatefulSet
	//+optional
	//+kubebuilder:default=1
	//+kubebuilder:validation:Minimum=0
	//+kubebuilder:validation:Maximum=1
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Size int32 `json:"size,omitempty" operator:"StatefulSet.Spec.Replicas"`

	// Image
	//+optional
	//+kubebuilder:default="wolveix/satisfactory-server:latest"
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Image string `json:"image,omitempty" operator:"StatefulSet.Spec.Template.Spec.Containers[factory-server].Image"`

	// StorageClassName is the size of the FactoryServer StatefulSet
	//+optional
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	StorageClass string `json:"storageClass,omitempty" operator:"StatefulSet.Spec.VolumeClaimTemplates[ObjectMeta:Name=factory-server-pvc].Spec.StorageClassName"`

	// StorageClassName is the size of the FactoryServer StatefulSet
	//+optional
	//+kubebuilder:default="50Gi"
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	StorageRequests string `json:"storageRequests,omitempty" operator:"StatefulSet.Spec.VolumeClaimTemplates[ObjectMeta:Name=factory-server-pvc].Spec.Resources.Requests{storage}"`

	// Autopause
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Autopause bool `json:"autopause,omitempty" operator:"ConfigMap.Data{AUTOPAUSE}"`

	// Interval
	//+optional
	//+kubebuilder:default=300
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	AutosaveInterval uint64 `json:"autosaveInterval,omitempty" operator:"ConfigMap.Data{AUTOSAVEINTERVAL}"`

	// Num
	//+optional
	//+kubebuilder:default=5
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	AutosaveNum uint64 `json:"autosaveNum,omitempty" operator:"ConfigMap.Data{AUTOSAVENUM}"`

	// OnDisconnect
	//+optional
	//+kubebuilder:default=true
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	AutosaveOnDisconnect bool `json:"autosaveOnDisconnect,omitempty" operator:"ConfigMap.Data{AUTOSAVEONDISCONNECT}"`

	// CrashReport
	//+optional
	//+kubebuilder:default=false
	CrashReport bool `json:"crashReport,omitempty" operator:"ConfigMap.Data{CRASHREPORT}"`

	// Debug
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Debug bool `json:"debug,omitempty" operator:"ConfigMap.Data{DEBUG}"`

	// DisableSeasonalEvents
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	DisableSeasonalEvents bool `json:"disableSeasonalEvents,omitempty" operator:"ConfigMap.Data{DISABLESEASONALEVENTS}"`

	// Maxplayers
	//+optional
	//+kubebuilder:default=4
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Maxplayers uint64 `json:"maxplayers,omitempty" operator:"ConfigMap.Data{MAXPLAYERS}"`

	// Networkquality
	//+optional
	//+kubebuilder:default=3
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Networkquality uint64 `json:"networkquality,omitempty" operator:"ConfigMap.Data{NETWORKQUALITY}"`

	// Beacon
	//+optional
	//+kubebuilder:default=15000
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	PortBeacon int32 `json:"portBeacon,omitempty" operator:"ConfigMap.Data{SERVERBEACONPORT};Service.Spec.Ports[beacon].Port;StatefulSet.Spec.Template.Spec.Containers[factory-server].Ports[beacon].ContainerPort"`

	// Game
	//+optional
	//+kubebuilder:default=7777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	PortGame int32 `json:"portGame,omitempty" operator:"ConfigMap.Data{SERVERGAMEPORT};Service.Spec.Ports[game].Port;StatefulSet.Spec.Template.Spec.Containers[factory-server].Ports[game].ContainerPort"`

	// Query
	//+optional
	//+kubebuilder:default=15777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	PortQuery int32 `json:"portQuery,omitempty" operator:"ConfigMap.Data{SERVERQUERYPORT};Service.Spec.Ports[query].Port;StatefulSet.Spec.Template.Spec.Containers[factory-server].Ports[query].ContainerPort"`

	// SkipUpdate
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	SkipUpdate bool `json:"skipUpdate,omitempty" operator:"ConfigMap.Data{SKIPUPDATE}"`

	// Beta
	//+optional
	//+kubebuilder:default=false
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	Beta bool `json:"beta,omitempty" operator:"ConfigMap.Data{STEAMBETA}"`

	// Type
	//+optional
	//+kubebuilder:default="ClusterIP"
	//+kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	ServiceType corev1.ServiceType `json:"serviceType,omitempty" operator:"Service.Spec.Type"`

	// LoadBalancerClass
	//+optional
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	ServiceLoadBalancerClass string `json:"serviceLoadBalancerClass,omitempty" operator:"Service.Spec.LoadBalancerClass"`

	// LoadBalancerSourceRanges
	//+optional
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	ServiceLoadBalancerSourceRanges []string `json:"serviceLoadBalancerSourceRanges,omitempty" operator:"Service.Spec.LoadBalancerSourceRanges"`

	// Beacon
	//+optional
	//+kubebuilder:default=15000
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	ServiceNodePortBeacon int32 `json:"serviceNodePortBeacon,omitempty" operator:"Service.Spec.Ports[beacon].NodePort"`

	// Game
	//+optional
	//+kubebuilder:default=7777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	ServiceNodePortGame int32 `json:"serviceNodePortGame,omitempty" operator:"Service.Spec.Ports[game].NodePort"`

	// Query
	//+optional
	//+kubebuilder:default=15777
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=65535
	//+operator--sdk:csv:customresourcedefinitions:type=spec
	ServiceNodePortQuery int32 `json:"serviceNodePortQuery,omitempty" operator:"Service.Spec.Ports[query].NodePort"`
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
	//+operator--sdk:csv:customresourcedefinitions:type=status
	Nodes []string `json:"nodes,omitempty"`
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
