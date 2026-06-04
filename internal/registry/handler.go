package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"k8s.io/apimachinery/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/metrics"
)

// Handler holds the HTTP handlers for the registry API.
type Handler struct {
	client client.Client
	config *DiscoveryConfig
}

// NewHandler creates a new Handler.
func NewHandler(client client.Client, config *DiscoveryConfig) *Handler {
	return &Handler{
		client: client,
		config: config,
	}
}

// WellKnown returns the registry's own Agent Card.
func (h *Handler) WellKnown(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	registryURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	card := &a2a.AgentCard{
		Name:        "A2A Registry",
		Description: "Kubernetes-native A2A Agent Registry — discover and register A2A agents",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(registryURL, a2a.TransportProtocolJSONRPC),
		},
		Capabilities: a2a.AgentCapabilities{
			Streaming: false,
		},
		Skills: []a2a.AgentSkill{
			{
				ID:          "discovery",
				Name:        "Agent Discovery",
				Description: "Discover registered A2A agents in the cluster.",
			},
			{
				ID:          "search",
				Name:        "Agent Search",
				Description: "Search agents by skills, tags, and capabilities.",
			},
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text", "application/json"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

// RegistryEntry is a lightweight view of a registered agent.
type RegistryEntry struct {
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url"`
	Version     string       `json:"version,omitempty"`
	Health      string       `json:"health"`
	Phase       string       `json:"phase"`
	Tags        []string     `json:"tags,omitempty"`
	Skills      []SkillEntry `json:"skills,omitempty"`
	Namespace   string       `json:"namespace"`
}

// SkillEntry is a lightweight view of an agent skill.
type SkillEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// labelSelectorFromConfig returns a list option for the configured label selector, or nil.
func labelSelectorFromConfig(config *DiscoveryConfig) client.ListOption {
	if config != nil && config.LabelSelector != "" {
		sel, err := labels.Parse(config.LabelSelector)
		if err == nil {
			return client.MatchingLabelsSelector{Selector: sel}
		}
	}
	return nil
}

// ListAgents lists all registered agents with optional filtering.
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	agents := &a2aiov1.A2AAgentList{}

	// Determine listing scope
	listOpts := []client.ListOption{}
	namespaceFilter := query.Get("namespace")

	if namespaceFilter != "" {
		listOpts = append(listOpts, client.InNamespace(namespaceFilter))
	} else if h.config != nil && h.config.Scope == "Namespace" && len(h.config.Namespaces) > 0 {
		// Only list from configured namespaces; for simplicity list all and filter below
	}

	// Apply label selector from discovery config
	if opt := labelSelectorFromConfig(h.config); opt != nil {
		listOpts = append(listOpts, opt)
	}

	if err := h.client.List(ctx, agents, listOpts...); err != nil {
		http.Error(w, fmt.Sprintf("Failed to list agents: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response entries
	entries := make([]RegistryEntry, 0)
	tagFilter := query.Get("tags")
	skillFilter := query.Get("skill")

	for _, agent := range agents.Items {
		// Skip disabled agents unless requested
		if !agent.Spec.Enabled {
			continue
		}

		// Filter by namespace scope
		if h.config != nil && h.config.Scope == "Namespace" && len(h.config.Namespaces) > 0 {
			found := false
			for _, ns := range h.config.Namespaces {
				if agent.Namespace == ns {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by tags
		if tagFilter != "" {
			filterTags := strings.Split(tagFilter, ",")
			if !hasAnyTag(agent.Spec.Tags, filterTags) {
				continue
			}
		}

		// Filter by skill ID
		if skillFilter != "" {
			if !hasSkill(agent.Spec.Skills, skillFilter) {
				continue
			}
		}

		entry := agentToEntry(agent)
		entries = append(entries, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// GetAgent returns a single agent by K8s resource name.
func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract name from URL path: /api/v1/agents/{name}
	name := extractNameFromPath(r.URL.Path, "/api/v1/agents/")
	if name == "" {
		http.Error(w, "Agent name is required.", http.StatusBadRequest)
		return
	}

	// Determine namespace from query or default
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	agent := &a2aiov1.A2AAgent{}
	if err := h.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, agent); err != nil {
		http.Error(w, fmt.Sprintf("Agent not found: %v", err), http.StatusNotFound)
		return
	}

	entry := agentToEntry(*agent)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// GetAgentCard returns the agent's A2A Agent Card constructed from the CR spec.
func (h *Handler) GetAgentCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := extractNameFromPath(r.URL.Path, "/api/v1/agents/")
	// Remove trailing "/card" if present
	name = strings.TrimSuffix(name, "/card")
	if name == "" {
		http.Error(w, "Agent name is required.", http.StatusBadRequest)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	agent := &a2aiov1.A2AAgent{}
	if err := h.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, agent); err != nil {
		http.Error(w, fmt.Sprintf("Agent not found: %v", err), http.StatusNotFound)
		return
	}

	// Build A2A AgentCard from CR spec
	card := agentToCard(agent)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

// RegisterRequest is the payload for the agent registration endpoint.
type RegisterRequest struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description,omitempty"`
	Version         string                 `json:"version,omitempty"`
	URL             string                 `json:"url"`
	Skills          []SkillEntry           `json:"skills,omitempty"`
	Tags            []string               `json:"tags,omitempty"`
	Streaming       bool                   `json:"streaming,omitempty"`
	PushNotifications bool                 `json:"pushNotifications,omitempty"`
	ProtocolVersion string                 `json:"protocolVersion,omitempty"`
}

// RegisterAgent handles agent registration via HTTP POST.
func (h *Handler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.URL == "" {
		http.Error(w, "name and url are required", http.StatusBadRequest)
		return
	}

	// Generate a K8s-safe name from the agent name
	k8sName := generateK8sName(req.Name)
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	// Check for URL conflict
	existing := &a2aiov1.A2AAgentList{}
	if err := h.client.List(ctx, existing); err == nil {
		for _, a := range existing.Items {
			if a.Spec.URL == req.URL {
				http.Error(w, fmt.Sprintf("URL %q is already registered by agent %q", req.URL, a.Name), http.StatusConflict)
				return
			}
		}
	}

	skills := make([]a2aiov1.A2AAgentSkillSpec, 0, len(req.Skills))
	for _, s := range req.Skills {
		skills = append(skills, a2aiov1.A2AAgentSkillSpec{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
		})
	}

	agent := &a2aiov1.A2AAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: namespace,
		},
		Spec: a2aiov1.A2AAgentSpec{
			Name:            req.Name,
			Description:     req.Description,
			Version:         req.Version,
			URL:             req.URL,
			Skills:          skills,
			Tags:            req.Tags,
			ProtocolVersion: req.ProtocolVersion,
			Enabled:         true,
			Capabilities: a2aiov1.A2AAgentCapabilities{
				Streaming:         req.Streaming,
				PushNotifications: req.PushNotifications,
			},
		},
	}

	if err := h.client.Create(ctx, agent); err != nil {
		http.Error(w, fmt.Sprintf("Failed to register agent: %v", err), http.StatusInternalServerError)
		return
	}

	metrics.RegistrationsTotal.Inc()

	entry := agentToEntry(*agent)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

// DeregisterAgent handles agent deregistration via HTTP DELETE.
func (h *Handler) DeregisterAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := extractNameFromPath(r.URL.Path, "/api/v1/agents/")
	if name == "" {
		http.Error(w, "Agent name is required.", http.StatusBadRequest)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	agent := &a2aiov1.A2AAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := h.client.Delete(ctx, agent); err != nil {
		http.Error(w, fmt.Sprintf("Failed to deregister agent: %v", err), http.StatusInternalServerError)
		return
	}

	metrics.DeregistrationsTotal.Inc()
	w.WriteHeader(http.StatusNoContent)
}

// generateK8sName converts a display name into a Kubernetes-safe resource name.
func generateK8sName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	// Remove characters that aren't alphanumeric, dash, or dot
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.' {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "agent"
	}
	return string(result)
}

// Search searches agents by various criteria.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	agents := &a2aiov1.A2AAgentList{}
	listOpts := []client.ListOption{}

	// Apply label selector from discovery config
	if opt := labelSelectorFromConfig(h.config); opt != nil {
		listOpts = append(listOpts, opt)
	}

	if err := h.client.List(ctx, agents, listOpts...); err != nil {
		http.Error(w, fmt.Sprintf("Failed to list agents: %v", err), http.StatusInternalServerError)
		return
	}

	q := strings.ToLower(query.Get("q"))
	tag := strings.ToLower(query.Get("tag"))
	skill := strings.ToLower(query.Get("skill"))
	capability := strings.ToLower(query.Get("capability"))

	entries := make([]RegistryEntry, 0)
	for _, agent := range agents.Items {
		if !agent.Spec.Enabled {
			continue
		}

		// Free-text search
		if q != "" {
			if !strings.Contains(strings.ToLower(agent.Spec.Name), q) &&
				!strings.Contains(strings.ToLower(agent.Spec.Description), q) {
				continue
			}
		}

		// Tag filter
		if tag != "" && !hasTag(agent.Spec.Tags, tag) {
			continue
		}

		// Skill filter
		if skill != "" && !hasSkill(agent.Spec.Skills, skill) {
			continue
		}

		// Capability filter
		if capability != "" {
			switch capability {
			case "streaming":
				if !agent.Spec.Capabilities.Streaming {
					continue
				}
			case "pushnotifications":
				if !agent.Spec.Capabilities.PushNotifications {
					continue
				}
			}
		}

		entries = append(entries, agentToEntry(agent))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// agentToEntry converts an A2AAgent CR to a RegistryEntry for API responses.
func agentToEntry(agent a2aiov1.A2AAgent) RegistryEntry {
	skills := make([]SkillEntry, 0, len(agent.Spec.Skills))
	for _, s := range agent.Spec.Skills {
		skills = append(skills, SkillEntry{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
		})
	}

	health := string(agent.Status.Health)
	if health == "" {
		health = "Unknown"
	}

	phase := string(agent.Status.Phase)
	if phase == "" {
		phase = "Pending"
	}

	return RegistryEntry{
		Name:        agent.Name,
		DisplayName: agent.Spec.Name,
		Description: agent.Spec.Description,
		URL:         agent.Spec.URL,
		Version:     agent.Spec.Version,
		Health:      health,
		Phase:       phase,
		Tags:        agent.Spec.Tags,
		Skills:      skills,
		Namespace:   agent.Namespace,
	}
}

// agentToCard converts an A2AAgent CR to an A2A AgentCard.
func agentToCard(agent *a2aiov1.A2AAgent) *a2a.AgentCard {
	// Build skills
	skills := make([]a2a.AgentSkill, 0, len(agent.Spec.Skills))
	for _, s := range agent.Spec.Skills {
		skills = append(skills, a2a.AgentSkill{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
			Examples:    s.Examples,
		})
	}

	// Build supported interfaces
	supportedInterfaces := []*a2a.AgentInterface{
		a2a.NewAgentInterface(agent.Spec.URL, a2a.TransportProtocolJSONRPC),
	}

	return &a2a.AgentCard{
		Name:                agent.Spec.Name,
		Description:         agent.Spec.Description,
		Version:             agent.Spec.Version,
		SupportedInterfaces: supportedInterfaces,
		Capabilities: a2a.AgentCapabilities{
			Streaming:         agent.Spec.Capabilities.Streaming,
			PushNotifications: agent.Spec.Capabilities.PushNotifications,
		},
		Skills:             skills,
		DefaultInputModes:  agent.Spec.DefaultInputModes,
		DefaultOutputModes: agent.Spec.DefaultOutputModes,
	}
}

// extractNameFromPath extracts the resource name from a URL path.
func extractNameFromPath(path, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

// hasAnyTag checks if any of the filter tags are present in the agent's tags.
func hasAnyTag(agentTags []string, filterTags []string) bool {
	for _, ft := range filterTags {
		for _, at := range agentTags {
			if strings.EqualFold(strings.TrimSpace(at), strings.TrimSpace(ft)) {
				return true
			}
		}
	}
	return false
}

// hasTag checks if a specific tag exists on the agent.
func hasTag(agentTags []string, tag string) bool {
	for _, t := range agentTags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// hasSkill checks if an agent has a specific skill by ID.
func hasSkill(skills []a2aiov1.A2AAgentSkillSpec, skillID string) bool {
	for _, s := range skills {
		if strings.EqualFold(s.ID, skillID) {
			return true
		}
	}
	return false
}
