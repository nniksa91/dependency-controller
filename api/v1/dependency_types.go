// Package v1 contains API Schema definitions for the core v1 API group
// +kubebuilder:object:generate=true
// +groupName=core.example.com
package v1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DependencySpec defines the desired state of Dependency
type DependencySpec struct {
	// Dependency is the name of the deployment that the Dependent resource relies on
	Dependency string `json:"dependency"`

	// DependencyType is the type of the dependency resource (e.g., Deployment)
	DependencyType string `json:"dependencyType"`

	// Dependent is the name of the deployment or job that depends on the Dependency resource
	Dependent string `json:"dependent"`

	// DependentType is the type of the dependent resource (e.g., Deployment, Job)
	DependentType string `json:"dependentType"`
}

// DependencyStatus defines the observed state of Dependency
type DependencyStatus struct {
	// DependencyReady indicates if the Dependency deployment is fully ready
	DependencyReady bool `json:"dependencyReady"`

	// DependentScaledDown indicates if the Dependent deployment is scaled down (for Deployment)
	DependentScaledDown bool `json:"dependentScaledDown"`

	// DependentJobCreated indicates if the Dependent Job has been created (for Job)
	DependentJobCreated bool `json:"dependentJobCreated"`

	// LastKnownJobSpec stores the last known Job spec before it was deleted
	LastKnownJobSpec *batchv1.JobSpec `json:"lastKnownJobSpec,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Dependency is the Schema for the dependencies API
type Dependency struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

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
