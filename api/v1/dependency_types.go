// Package v1 contains API Schema definitions for the core v1 API group
// +kubebuilder:object:generate=true
// +groupName=core.example.com
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Compose-style dependency conditions (Docker Compose depends_on).
const (
	ConditionServiceStarted   = "serviceStarted"
	ConditionServiceHealthy   = "serviceHealthy"
	ConditionServiceCompleted = "serviceCompleted"
)

// ObjectRef identifies a namespaced Kubernetes object by GVK + name.
// The object is resolved in the Dependency CR's namespace.
type ObjectRef struct {
	// APIVersion is the group/version of the referent (e.g. apps/v1, v1).
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the referent (e.g. Deployment, StatefulSet, Pod).
	Kind string `json:"kind"`

	// Name is the name of the referent.
	Name string `json:"name"`
}

// ReadyWhen evaluates custom-resource readiness via JSONPath when no Ready condition exists.
type ReadyWhen struct {
	// JSONPath is a JSONPath expression evaluated against the dependency object (e.g. "{.status.phase}").
	JSONPath string `json:"jsonPath"`

	// Value is the expected string form of the JSONPath result.
	Value string `json:"value"`
}

// DependencySpec defines the desired state of Dependency.
type DependencySpec struct {
	// Dependency is the object that must satisfy Condition before the dependent may run.
	Dependency ObjectRef `json:"dependency"`

	// Dependent is the object gated until Dependency is ready.
	// Scalable kinds (Deployment, StatefulSet, ReplicaSet) are scaled to 0 / restored.
	// Other kinds are not mutated; status reports DependentNotScalable.
	Dependent ObjectRef `json:"dependent"`

	// Condition is the Compose-style readiness check applied to Dependency.
	// serviceStarted | serviceHealthy | serviceCompleted
	// +kubebuilder:default=serviceHealthy
	// +kubebuilder:validation:Enum=serviceStarted;serviceHealthy;serviceCompleted
	// +optional
	Condition string `json:"condition,omitempty"`

	// ReadyWhen optionally overrides CR readiness with a JSONPath match.
	// Used when Dependency is a custom resource without a standard Ready condition.
	// +optional
	ReadyWhen *ReadyWhen `json:"readyWhen,omitempty"`

	// DesiredReplicas, when set, is the replica count used when scaling a scalable
	// dependent back up. When unset, the controller restores the annotation value, or 1.
	// +optional
	DesiredReplicas *int32 `json:"desiredReplicas,omitempty"`
}

// DependencyStatus defines the observed state of Dependency.
type DependencyStatus struct {
	// DependencyReady indicates if the Dependency object satisfies the Condition.
	DependencyReady bool `json:"dependencyReady"`

	// DependentScaledDown indicates if a scalable Dependent is currently at 0 replicas.
	DependentScaledDown bool `json:"dependentScaledDown"`

	// Condition is the effective Compose condition used for evaluation.
	// +optional
	Condition string `json:"condition,omitempty"`

	// Reason is a short CamelCase reason for the current state.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable status detail.
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the .metadata.generation last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.dependencyReady`
// +kubebuilder:printcolumn:name="ScaledDown",type=boolean,JSONPath=`.status.dependentScaledDown`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Dependency is the Schema for the dependencies API
type Dependency struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DependencySpec   `json:"spec,omitempty"`
	Status DependencyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DependencyList contains a list of Dependency
type DependencyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dependency `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dependency{}, &DependencyList{})
}
