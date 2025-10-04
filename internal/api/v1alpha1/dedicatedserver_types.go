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

type DedicatedServerPort struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65536
	Port int32 `json:"port"`
	// Protocols for port.
	// +kubebuilder:default={"UDP"}
	Protocols []corev1.Protocol `json:"protocols,omitempty"`
}

// DedicatedServerSpec defines the desired state of DedicatedServer.
type DedicatedServerSpec struct {
	// +kubebuilder:validation:Required
	AppID int `json:"appID"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="public"
	Branch string `json:"branch"`
	// +kubebuilder:validation:Optional
	Ports []DedicatedServerPort `json:"ports,omitempty"`
	// +kubebuilder:validation:Optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +kubebuilder:validation:Optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// +kubebuilder:validation:Optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

const (
	PhasePending = "Pending"
	PhaseReady   = "Ready"
	PhaseFailed  = "Failed"
	PhasePaused  = "Paused"
)

// DedicatedServerStatus defines the observed state of DedicatedServer.
type DedicatedServerStatus struct {
	// +kubebuilder:default="Pending"
	// +kubebuilder:validation:Enum=Pending;Ready;Failed;Paused
	Phase string `json:"phase"`
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +kubebuilder:validation:Optional
	IP string `json:"ip,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.ip`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// DedicatedServer is the Schema for the dedicatedservers API.
type DedicatedServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec DedicatedServerSpec `json:"spec"`
	// +kubebuilder:validation:Optional
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
