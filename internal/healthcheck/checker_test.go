package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

func TestCheck_HealthyAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"test","description":"A test agent"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := registry.NewAgentCardResolver(10 * time.Second)
	checker := NewChecker(resolver)

	agent := &a2aiov1.A2AAgent{Spec: a2aiov1.A2AAgentSpec{URL: server.URL}}
	result := checker.Check(context.Background(), agent)

	if !result.Healthy {
		t.Errorf("expected healthy, got unhealthy: %s", result.Error)
	}
	if result.CardHash == "" {
		t.Error("expected non-empty card hash")
	}
	if result.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestCheck_UnreachableAgent(t *testing.T) {
	resolver := registry.NewAgentCardResolver(2 * time.Second)
	checker := NewChecker(resolver)

	agent := &a2aiov1.A2AAgent{
		Spec: a2aiov1.A2AAgentSpec{URL: "http://127.0.0.1:19999"},
	}
	result := checker.Check(context.Background(), agent)

	if result.Healthy {
		t.Error("expected unhealthy for unreachable agent")
	}
	if result.Error == "" {
		t.Error("expected error message for unreachable agent")
	}
}

func TestCheck_Non200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	resolver := registry.NewAgentCardResolver(10 * time.Second)
	checker := NewChecker(resolver)

	agent := &a2aiov1.A2AAgent{Spec: a2aiov1.A2AAgentSpec{URL: server.URL}}
	result := checker.Check(context.Background(), agent)

	if result.Healthy {
		t.Error("expected unhealthy for non-200 response")
	}
}

func TestCheck_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`not valid json`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := registry.NewAgentCardResolver(10 * time.Second)
	checker := NewChecker(resolver)

	agent := &a2aiov1.A2AAgent{Spec: a2aiov1.A2AAgentSpec{URL: server.URL}}
	result := checker.Check(context.Background(), agent)

	if result.Healthy {
		t.Error("expected unhealthy for invalid JSON")
	}
}

func TestCheck_FallbackToAgentJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"legacy","description":"Legacy agent"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := registry.NewAgentCardResolver(10 * time.Second)
	checker := NewChecker(resolver)

	agent := &a2aiov1.A2AAgent{Spec: a2aiov1.A2AAgentSpec{URL: server.URL}}
	result := checker.Check(context.Background(), agent)

	if !result.Healthy {
		t.Errorf("expected healthy (fallback to agent.json), got unhealthy: %s", result.Error)
	}
}

func TestCheck_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	resolver := registry.NewAgentCardResolver(10 * time.Second)
	checker := NewChecker(resolver)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	agent := &a2aiov1.A2AAgent{Spec: a2aiov1.A2AAgentSpec{URL: server.URL}}
	result := checker.Check(ctx, agent)

	if result.Healthy {
		t.Error("expected unhealthy for cancelled context")
	}
}

func TestCheck_ReturnsCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"test-card","description":"card desc"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := registry.NewAgentCardResolver(10 * time.Second)
	checker := NewChecker(resolver)

	agent := &a2aiov1.A2AAgent{Spec: a2aiov1.A2AAgentSpec{URL: server.URL}}
	result := checker.Check(context.Background(), agent)

	if !result.Healthy {
		t.Fatalf("expected healthy, got: %s", result.Error)
	}
	if result.Card == nil {
		t.Fatal("expected Card to be non-nil")
	}
	if result.Card.Name != "test-card" {
		t.Errorf("expected card name 'test-card', got '%s'", result.Card.Name)
	}
}

func TestCheckWithAuth_PassesAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mytoken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"auth-agent"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := registry.NewAgentCardResolver(10 * time.Second)
	checker := NewChecker(resolver)

	agent := &a2aiov1.A2AAgent{Spec: a2aiov1.A2AAgentSpec{URL: server.URL}}
	auth := &registry.AuthConfig{
		Schemes:    []string{"bearer"},
		SecretData: map[string][]byte{"token": []byte("mytoken")},
	}
	result := checker.CheckWithAuth(context.Background(), agent, auth)

	if !result.Healthy {
		t.Fatalf("expected healthy with auth, got: %s", result.Error)
	}
	if result.Card == nil || result.Card.Name != "auth-agent" {
		t.Errorf("expected card name 'auth-agent', got %v", result.Card)
	}
}
