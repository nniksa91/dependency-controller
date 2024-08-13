// Package v1 contains API Schema definitions for the core v1 API group
// +kubebuilder:object:generate=true
// +groupName=core.example.com
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DependencySpec defines the desired state of Dependency
type DependencySpec struct {
	// Dependency is the name of the deployment that the Dependent deployment relies on
	Dependency string `json:"dependency"`

	// Dependent is the name of the deployment that depends on the Dependency deployment
	Dependent string `json:"dependent"`
}

// DependencyStatus defines the observed state of Dependency
type DependencyStatus struct {
	// DependencyReady indicates if the Dependency deployment is fully ready
	DependencyReady bool `json:"dependencyReady"`

	// DependentScaledDown indicates if the Dependent deployment is scaled down
	DependentScaledDown bool `json:"dependentScaledDown"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Dependency is the Schema for the dependencies API
type Dependency struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"` // Embeds ObjectMeta to implement client.Object interface

	Spec   DependencySpec   `json:"spec,omitempty"`
	Status DependencyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DependencyList contains a list of Dependency
type DependencyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dependency `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dependency{}, &DependencyList{})
}
