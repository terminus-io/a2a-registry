package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// A2AAgentPhase represents the lifecycle phase of an A2A agent.
type A2AAgentPhase string

const (
	A2AAgentPhasePending     A2AAgentPhase = "Pending"
	A2AAgentPhaseReady       A2AAgentPhase = "Ready"
	A2AAgentPhaseError       A2AAgentPhase = "Error"
	A2AAgentPhaseUnreachable A2AAgentPhase = "Unreachable"
)

// A2AAgentHealth represents the health status of an A2A agent.
type A2AAgentHealth string

const (
	A2AAgentHealthHealthy   A2AAgentHealth = "Healthy"
	A2AAgentHealthUnhealthy A2AAgentHealth = "Unhealthy"
	A2AAgentHealthUnknown   A2AAgentHealth = "Unknown"
)

// A2AAgentCapabilities describes the capabilities of the agent.
type A2AAgentCapabilities struct {
	Streaming         bool `json:"streaming,omitempty"`
	PushNotifications bool `json:"pushNotifications,omitempty"`
}

// A2AAgentSkillSpec defines a skill that an agent can perform.
type A2AAgentSkillSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id"`
	// +kubebuilder:validation:Required
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// A2AAuthentication defines authentication requirements for the agent.
type A2AAuthentication struct {
	Schemes   []string                     `json:"schemes"`
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

// A2AProviderInfo describes the provider of the agent.
type A2AProviderInfo struct {
	Organization string `json:"organization,omitempty"`
	URL          string `json:"url,omitempty"`
}

// HealthCheckConfig defines health check parameters for an agent.
type HealthCheckConfig struct {
	// +kubebuilder:validation:Minimum=10
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`
	TimeoutSeconds  int32 `json:"timeoutSeconds,omitempty"`
	// +kubebuilder:validation:Minimum=1
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

// A2AAgentSpec defines the desired state of A2AAgent.
type A2AAgentSpec struct {
	// +kubebuilder:validation:Required
	// Name is the display name of the agent.
	Name string `json:"name"`

	// Description is a human-readable description of the agent.
	Description string `json:"description,omitempty"`

	// Version is the version of the agent.
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Required
	// URL is the base endpoint URL of the agent.
	URL string `json:"url"`

	// Capabilities describes the agent's capabilities.
	Capabilities A2AAgentCapabilities `json:"capabilities,omitempty"`

	// Skills lists the skills that the agent can perform.
	Skills []A2AAgentSkillSpec `json:"skills,omitempty"`

	// Authentication defines how to authenticate with the agent.
	Authentication *A2AAuthentication `json:"authentication,omitempty"`

	// DefaultInputModes are the default input content types.
	DefaultInputModes []string `json:"defaultInputModes,omitempty"`

	// DefaultOutputModes are the default output content types.
	DefaultOutputModes []string `json:"defaultOutputModes,omitempty"`

	// Provider describes the agent's provider.
	Provider *A2AProviderInfo `json:"provider,omitempty"`

	// ProtocolVersion is the A2A protocol version.
	ProtocolVersion string `json:"protocolVersion,omitempty"`

	// Tags are discovery tags for searching and filtering.
	Tags []string `json:"tags,omitempty"`

	// HealthCheck configures health checking for this agent.
	HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty"`

	// Enabled indicates whether this agent is active in the registry.
	Enabled bool `json:"enabled"`
}

// A2AAgentStatus defines the observed state of A2AAgent.
type A2AAgentStatus struct {
	// Phase is the current lifecycle phase of the agent.
	Phase A2AAgentPhase `json:"phase,omitempty"`

	// Health is the current health status of the agent.
	Health A2AAgentHealth `json:"health,omitempty"`

	// Message is a human-readable status message.
	Message string `json:"message,omitempty"`

	// LastHeartbeat is the timestamp of the last successful health check.
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// AgentCardHash is the SHA256 hash of the fetched agent card.
	AgentCardHash string `json:"agentCardHash,omitempty"`

	// ObservedGeneration is the generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConsecutiveFailures tracks the number of consecutive failed health checks.
	ConsecutiveFailures int32 `json:"consecutiveFailures,omitempty"`

	// RegisteredAt is the timestamp when the agent was first registered.
	RegisteredAt *metav1.Time `json:"registeredAt,omitempty"`

	// Conditions represent the current state of the agent.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=a2aagents,scope=Namespaced,shortName=a2aa
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Health",type="string",JSONPath=".status.health"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.url"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A2AAgent represents a registered A2A agent.
type A2AAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   A2AAgentSpec   `json:"spec,omitempty"`
	Status A2AAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// A2AAgentList contains a list of A2AAgent.
type A2AAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []A2AAgent `json:"items"`
}

// DeepCopy methods for A2AAgent

func (in *A2AAgent) DeepCopy() *A2AAgent {
	if in == nil {
		return nil
	}
	out := new(A2AAgent)
	in.DeepCopyInto(out)
	return out
}

func (in *A2AAgent) DeepCopyInto(out *A2AAgent) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *A2AAgent) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy methods for A2AAgentList

func (in *A2AAgentList) DeepCopy() *A2AAgentList {
	if in == nil {
		return nil
	}
	out := new(A2AAgentList)
	in.DeepCopyInto(out)
	return out
}

func (in *A2AAgentList) DeepCopyInto(out *A2AAgentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]A2AAgent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *A2AAgentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy methods for A2AAgentSpec

func (in *A2AAgentSpec) DeepCopy() *A2AAgentSpec {
	if in == nil {
		return nil
	}
	out := new(A2AAgentSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *A2AAgentSpec) DeepCopyInto(out *A2AAgentSpec) {
	*out = *in
	if in.Skills != nil {
		in, out := &in.Skills, &out.Skills
		*out = make([]A2AAgentSkillSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Authentication != nil {
		in, out := &in.Authentication, &out.Authentication
		*out = new(A2AAuthentication)
		(*in).DeepCopyInto(*out)
	}
	if in.DefaultInputModes != nil {
		in, out := &in.DefaultInputModes, &out.DefaultInputModes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DefaultOutputModes != nil {
		in, out := &in.DefaultOutputModes, &out.DefaultOutputModes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Provider != nil {
		in, out := &in.Provider, &out.Provider
		*out = new(A2AProviderInfo)
		**out = **in
	}
	if in.Tags != nil {
		in, out := &in.Tags, &out.Tags
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.HealthCheck != nil {
		in, out := &in.HealthCheck, &out.HealthCheck
		*out = new(HealthCheckConfig)
		**out = **in
	}
}

// DeepCopy methods for A2AAgentStatus

func (in *A2AAgentStatus) DeepCopy() *A2AAgentStatus {
	if in == nil {
		return nil
	}
	out := new(A2AAgentStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *A2AAgentStatus) DeepCopyInto(out *A2AAgentStatus) {
	*out = *in
	if in.LastHeartbeat != nil {
		in, out := &in.LastHeartbeat, &out.LastHeartbeat
		*out = new(metav1.Time)
		**out = **in
	}
	if in.RegisteredAt != nil {
		in, out := &in.RegisteredAt, &out.RegisteredAt
		*out = new(metav1.Time)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy methods for A2AAgentCapabilities

func (in *A2AAgentCapabilities) DeepCopy() *A2AAgentCapabilities {
	if in == nil {
		return nil
	}
	out := new(A2AAgentCapabilities)
	*out = *in
	return out
}

func (in *A2AAgentCapabilities) DeepCopyInto(out *A2AAgentCapabilities) {
	*out = *in
}

// DeepCopy methods for A2AAgentSkillSpec

func (in *A2AAgentSkillSpec) DeepCopy() *A2AAgentSkillSpec {
	if in == nil {
		return nil
	}
	out := new(A2AAgentSkillSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *A2AAgentSkillSpec) DeepCopyInto(out *A2AAgentSkillSpec) {
	*out = *in
	if in.Tags != nil {
		in, out := &in.Tags, &out.Tags
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Examples != nil {
		in, out := &in.Examples, &out.Examples
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy methods for A2AAuthentication

func (in *A2AAuthentication) DeepCopy() *A2AAuthentication {
	if in == nil {
		return nil
	}
	out := new(A2AAuthentication)
	in.DeepCopyInto(out)
	return out
}

func (in *A2AAuthentication) DeepCopyInto(out *A2AAuthentication) {
	*out = *in
	if in.Schemes != nil {
		in, out := &in.Schemes, &out.Schemes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(corev1.LocalObjectReference)
		**out = **in
	}
}

// DeepCopy methods for A2AProviderInfo

func (in *A2AProviderInfo) DeepCopy() *A2AProviderInfo {
	if in == nil {
		return nil
	}
	out := new(A2AProviderInfo)
	*out = *in
	return out
}

func (in *A2AProviderInfo) DeepCopyInto(out *A2AProviderInfo) {
	*out = *in
}

// DeepCopy methods for HealthCheckConfig

func (in *HealthCheckConfig) DeepCopy() *HealthCheckConfig {
	if in == nil {
		return nil
	}
	out := new(HealthCheckConfig)
	*out = *in
	return out
}

func (in *HealthCheckConfig) DeepCopyInto(out *HealthCheckConfig) {
	*out = *in
}

func init() {
	SchemeBuilder.Register(&A2AAgent{}, &A2AAgentList{})
}
