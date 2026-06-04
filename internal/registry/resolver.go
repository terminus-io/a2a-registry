package registry

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/terminus-io/a2a-registry/internal/metrics"
)

// ResolveResult holds the result of an agent card resolution.
type ResolveResult struct {
	Card    *a2a.AgentCard
	Hash    string
	Latency time.Duration
}

// AuthConfig holds authentication configuration for fetching agent cards.
type AuthConfig struct {
	Schemes    []string
	SecretData map[string][]byte
}

// AgentCardResolver fetches A2A Agent Cards from agent endpoints.
type AgentCardResolver struct {
	httpClient *http.Client
}

// NewAgentCardResolver creates a new resolver with the given timeout.
func NewAgentCardResolver(timeout time.Duration) *AgentCardResolver {
	return &AgentCardResolver{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// wellKnownPaths lists A2A Agent Card paths in order of preference.
// A2A spec v0.3+ uses "agent-card.json"; earlier drafts used "agent.json".
var wellKnownPaths = []string{
	"/.well-known/agent-card.json",
	"/.well-known/agent.json",
}

// FetchCard fetches the Agent Card from the agent's well-known endpoint without authentication.
// It tries each known well-known path until one succeeds.
func (r *AgentCardResolver) FetchCard(ctx context.Context, agentURL string) (*ResolveResult, error) {
	return r.fetchCard(ctx, agentURL, nil)
}

// FetchCardWithAuth fetches the Agent Card with authentication headers.
func (r *AgentCardResolver) FetchCardWithAuth(ctx context.Context, agentURL string, auth *AuthConfig) (*ResolveResult, error) {
	var headers map[string]string
	if auth != nil {
		headers = buildAuthHeaders(auth)
	}
	return r.fetchCard(ctx, agentURL, headers)
}

func (r *AgentCardResolver) fetchCard(ctx context.Context, agentURL string, headers map[string]string) (*ResolveResult, error) {
	baseURL := strings.TrimRight(agentURL, "/")

	var lastErr error
	for _, path := range wellKnownPaths {
		url := baseURL + path
		result, err := r.tryFetch(ctx, url, headers)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	metrics.AgentCardFetchErrorsTotal.Inc()
	return nil, fmt.Errorf("failed to fetch agent card from %s (tried %v): %w", agentURL, wellKnownPaths, lastErr)
}

func (r *AgentCardResolver) tryFetch(ctx context.Context, url string, headers map[string]string) (*ResolveResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}

	var card a2a.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("failed to decode agent card from %s: %w", url, err)
	}

	// Compute hash of the agent card for change detection
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent card for hashing: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(cardJSON))

	return &ResolveResult{
		Card:    &card,
		Hash:    hash,
		Latency: time.Since(start),
	}, nil
}

// buildAuthHeaders constructs HTTP headers from the auth config.
func buildAuthHeaders(auth *AuthConfig) map[string]string {
	headers := make(map[string]string)
	for _, scheme := range auth.Schemes {
		switch strings.ToLower(scheme) {
		case "bearer":
			if token, ok := auth.SecretData["token"]; ok {
				headers["Authorization"] = "Bearer " + string(token)
			}
		case "basic":
			username := auth.SecretData["username"]
			password := auth.SecretData["password"]
			if len(username) > 0 {
				auth := base64.StdEncoding.EncodeToString([]byte(string(username) + ":" + string(password)))
				headers["Authorization"] = "Basic " + auth
			}
		}
	}
	return headers
}
