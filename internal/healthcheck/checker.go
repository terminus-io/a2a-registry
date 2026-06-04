package healthcheck

import (
	"context"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

// Result holds the result of a health check.
type Result struct {
	Healthy  bool
	Latency  time.Duration
	CardHash string
	Card     *a2a.AgentCard
	Error    string
}

// Checker performs health checks on A2A agent endpoints.
type Checker struct {
	resolver *registry.AgentCardResolver
}

// NewChecker creates a new health checker.
func NewChecker(resolver *registry.AgentCardResolver) *Checker {
	return &Checker{
		resolver: resolver,
	}
}

// Check performs a health check by fetching the agent card from the agent's URL.
func (c *Checker) Check(ctx context.Context, agent *a2aiov1.A2AAgent) *Result {
	return c.CheckWithAuth(ctx, agent, nil)
}

// CheckWithAuth performs a health check with optional authentication.
func (c *Checker) CheckWithAuth(ctx context.Context, agent *a2aiov1.A2AAgent, auth *registry.AuthConfig) *Result {
	result := &Result{
		Healthy: false,
	}

	resolveResult, err := c.resolver.FetchCardWithAuth(ctx, agent.Spec.URL, auth)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Healthy = true
	result.Latency = resolveResult.Latency
	result.CardHash = resolveResult.Hash
	result.Card = resolveResult.Card

	return result
}
