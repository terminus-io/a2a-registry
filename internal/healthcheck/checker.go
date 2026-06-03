package healthcheck

import (
	"context"
	"time"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

// Result holds the result of a health check.
type Result struct {
	Healthy  bool
	Latency  time.Duration
	CardHash string
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
	result := &Result{
		Healthy: false,
	}

	resolveResult, err := c.resolver.FetchCard(ctx, agent.Spec.URL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Healthy = true
	result.Latency = resolveResult.Latency
	result.CardHash = resolveResult.Hash

	return result
}
