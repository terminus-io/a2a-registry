package controllers

// RegistryEntry is a lightweight summary of a registered agent for the API.
type RegistryEntry struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	URL         string   `json:"url"`
	Version     string   `json:"version,omitempty"`
	Health      string   `json:"health"`
	Tags        []string `json:"tags,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	Namespace   string   `json:"namespace"`
}

// RegistryConfig holds the current registry configuration.
type RegistryConfig struct {
	DiscoveryScope     string
	LabelSelector      string
	Namespaces         []string
	RequireHealthCheck bool
	RequireCardMatch   bool
	RequireApproval    bool
}
