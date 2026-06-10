package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DiscoveryConfig defines how agents are discovered.
type DiscoveryConfig struct {
	// +kubebuilder:validation:Enum=Cluster;Namespace
	// Scope is the discovery scope: "Cluster" or "Namespace".
	Scope string `json:"scope,omitempty"`

	// LabelSelector filters agents by label.
	LabelSelector string `json:"labelSelector,omitempty"`

	// Namespaces lists specific namespaces to watch when Scope is "Namespace".
	Namespaces []string `json:"namespaces,omitempty"`
}

// RegistrationConfig defines registration policies.
type RegistrationConfig struct {
	// RequireApproval requires manual approval for new agents.
	RequireApproval bool `json:"requireApproval,omitempty"`

	// RequireHealthCheck requires agent URL to be reachable before marking Ready.
	RequireHealthCheck bool `json:"requireHealthCheck,omitempty"`

	// RequireCardMatch requires spec to match the fetched agent card.
	RequireCardMatch bool `json:"requireCardMatch,omitempty"`
}

// HealthCheckDefaults defines cluster-wide health check defaults.
type HealthCheckDefaults struct {
	// +kubebuilder:validation:Minimum=1
	// IntervalSeconds is the default health check interval.
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`

	// TimeoutSeconds is the default health check timeout.
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// APIServerConfig defines the registry API server configuration.
type APIServerConfig struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// Port is the API server port.
	Port int32 `json:"port,omitempty"`

	// BindAddress is the API server bind address.
	BindAddress string `json:"bindAddress,omitempty"`

	// TLSCertRef references a TLS certificate secret.
	TLSCertRef *corev1.LocalObjectReference `json:"tlsCertRef,omitempty"`
}

// A2ARegistrySpec defines the desired state of A2ARegistry.
type A2ARegistrySpec struct {
	// Discovery configures agent discovery.
	Discovery DiscoveryConfig `json:"discovery,omitempty"`

	// Registration configures registration policies.
	Registration RegistrationConfig `json:"registration,omitempty"`

	// HealthCheck provides cluster-wide health check defaults.
	HealthCheck HealthCheckDefaults `json:"healthCheck,omitempty"`

	// APIServer configures the registry API server.
	APIServer APIServerConfig `json:"apiServer,omitempty"`
}

// A2ARegistryStatus defines the observed state of A2ARegistry.
type A2ARegistryStatus struct {
	// Phase is the current phase of the registry.
	Phase string `json:"phase,omitempty"`

	// AgentCount is the total number of registered agents.
	AgentCount int32 `json:"agentCount,omitempty"`

	// HealthyAgents is the number of currently healthy agents.
	HealthyAgents int32 `json:"healthyAgents,omitempty"`

	// Conditions represent the current state of the registry.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=a2aregistries,scope=Cluster,shortName=a2areg
// +kubebuilder:printcolumn:name="Scope",type="string",JSONPath=".spec.discovery.scope"
// +kubebuilder:printcolumn:name="Agents",type="integer",JSONPath=".status.agentCount"
// +kubebuilder:printcolumn:name="Healthy",type="integer",JSONPath=".status.healthyAgents"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A2ARegistry represents the A2A registry configuration.
type A2ARegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   A2ARegistrySpec   `json:"spec,omitempty"`
	Status A2ARegistryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// A2ARegistryList contains a list of A2ARegistry.
type A2ARegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []A2ARegistry `json:"items"`
}

// DeepCopy methods for A2ARegistry

func (in *A2ARegistry) DeepCopy() *A2ARegistry {
	if in == nil {
		return nil
	}
	out := new(A2ARegistry)
	in.DeepCopyInto(out)
	return out
}

func (in *A2ARegistry) DeepCopyInto(out *A2ARegistry) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *A2ARegistry) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy methods for A2ARegistryList

func (in *A2ARegistryList) DeepCopy() *A2ARegistryList {
	if in == nil {
		return nil
	}
	out := new(A2ARegistryList)
	in.DeepCopyInto(out)
	return out
}

func (in *A2ARegistryList) DeepCopyInto(out *A2ARegistryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]A2ARegistry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *A2ARegistryList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy methods for A2ARegistrySpec

func (in *A2ARegistrySpec) DeepCopy() *A2ARegistrySpec {
	if in == nil {
		return nil
	}
	out := new(A2ARegistrySpec)
	in.DeepCopyInto(out)
	return out
}

func (in *A2ARegistrySpec) DeepCopyInto(out *A2ARegistrySpec) {
	*out = *in
	in.Discovery.DeepCopyInto(&out.Discovery)
	out.Registration = in.Registration
	out.HealthCheck = in.HealthCheck
	in.APIServer.DeepCopyInto(&out.APIServer)
}

// DeepCopy methods for A2ARegistryStatus

func (in *A2ARegistryStatus) DeepCopy() *A2ARegistryStatus {
	if in == nil {
		return nil
	}
	out := new(A2ARegistryStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *A2ARegistryStatus) DeepCopyInto(out *A2ARegistryStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy methods for DiscoveryConfig

func (in *DiscoveryConfig) DeepCopy() *DiscoveryConfig {
	if in == nil {
		return nil
	}
	out := new(DiscoveryConfig)
	in.DeepCopyInto(out)
	return out
}

func (in *DiscoveryConfig) DeepCopyInto(out *DiscoveryConfig) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy methods for APIServerConfig

func (in *APIServerConfig) DeepCopy() *APIServerConfig {
	if in == nil {
		return nil
	}
	out := new(APIServerConfig)
	in.DeepCopyInto(out)
	return out
}

func (in *APIServerConfig) DeepCopyInto(out *APIServerConfig) {
	*out = *in
	if in.TLSCertRef != nil {
		in, out := &in.TLSCertRef, &out.TLSCertRef
		*out = new(corev1.LocalObjectReference)
		**out = **in
	}
}

// DeepCopy methods for RegistrationConfig

func (in *RegistrationConfig) DeepCopy() *RegistrationConfig {
	if in == nil {
		return nil
	}
	out := new(RegistrationConfig)
	*out = *in
	return out
}

func (in *RegistrationConfig) DeepCopyInto(out *RegistrationConfig) {
	*out = *in
}

// DeepCopy methods for HealthCheckDefaults

func (in *HealthCheckDefaults) DeepCopy() *HealthCheckDefaults {
	if in == nil {
		return nil
	}
	out := new(HealthCheckDefaults)
	*out = *in
	return out
}

func (in *HealthCheckDefaults) DeepCopyInto(out *HealthCheckDefaults) {
	*out = *in
}

func init() {
	SchemeBuilder.Register(&A2ARegistry{}, &A2ARegistryList{})
}
