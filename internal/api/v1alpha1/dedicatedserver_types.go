package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *DedicatedServer) GetConditions() []metav1.Condition {
	return s.Status.Conditions
}

func (s *DedicatedServer) SetConditions(conditions []metav1.Condition) {
	s.Status.Conditions = conditions
}

// DedicatedServerSpec defines the desired state of DedicatedServer.
type DedicatedServerSpec struct {
	// +kubebuilder:validation:Required
	Steamapp corev1.ObjectReference `json:"steamapp"`
}

// DedicatedServerStatus defines the observed state of DedicatedServer.
type DedicatedServerStatus struct {
	// +kubebuilder:default="Pending"
	// +kubebuilder:validation:Enum=Pending;Ready;Failed;Paused
	Phase string `json:"phase"`
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +kubebuilder:validation:Optional
	IP string
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// DedicatedServer is the Schema for the dedicatedservers API.
type DedicatedServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DedicatedServerSpec   `json:"spec"`
	Status DedicatedServerStatus `json:"status"`
}

// +kubebuilder:object:root=true

// DedicatedServerList contains a list of DedicatedServer.
type DedicatedServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DedicatedServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DedicatedServer{}, &DedicatedServerList{})
}
