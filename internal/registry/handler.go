package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"sigs.k8s.io/controller-runtime/pkg/client"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
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

// Search searches agents by various criteria.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	agents := &a2aiov1.A2AAgentList{}
	if err := h.client.List(ctx, agents); err != nil {
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
