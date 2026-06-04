package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchCard_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"test","description":"A test agent"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	result, err := resolver.FetchCard(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Card.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", result.Card.Name)
	}
	if result.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if result.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestFetchCard_FallbackToAgentJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"legacy","description":"Legacy agent"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	result, err := resolver.FetchCard(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Card.Name != "legacy" {
		t.Errorf("expected name 'legacy', got '%s'", result.Card.Name)
	}
}

func TestFetchCard_BothPathsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	_, err := resolver.FetchCard(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error when both paths fail")
	}
}

func TestFetchCard_Non200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	_, err := resolver.FetchCard(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestFetchCard_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`not valid`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	_, err := resolver.FetchCard(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchCard_UnreachableHost(t *testing.T) {
	resolver := NewAgentCardResolver(2 * time.Second)
	_, err := resolver.FetchCard(context.Background(), "http://127.0.0.1:19999")

	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestFetchCard_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolver.FetchCard(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchCard_HashConsistency(t *testing.T) {
	cardJSON := `{"name":"test","description":"A test agent"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cardJSON))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)

	result1, _ := resolver.FetchCard(context.Background(), server.URL)
	result2, _ := resolver.FetchCard(context.Background(), server.URL)

	if result1.Hash != result2.Hash {
		t.Error("expected consistent hash for identical card content")
	}
}

func TestFetchCardWithAuth_BearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"test"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	auth := &AuthConfig{
		Schemes:    []string{"bearer"},
		SecretData: map[string][]byte{"token": []byte("test-token")},
	}
	result, err := resolver.FetchCardWithAuth(context.Background(), server.URL, auth)
	if err != nil {
		t.Fatalf("unexpected error with bearer token: %v", err)
	}
	if result.Card.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", result.Card.Name)
	}
}

func TestFetchCardWithAuth_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic dXNlcjpwYXNz" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"test"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	auth := &AuthConfig{
		Schemes:    []string{"basic"},
		SecretData: map[string][]byte{"username": []byte("user"), "password": []byte("pass")},
	}
	result, err := resolver.FetchCardWithAuth(context.Background(), server.URL, auth)
	if err != nil {
		t.Fatalf("unexpected error with basic auth: %v", err)
	}
	if result.Card.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", result.Card.Name)
	}
}

func TestFetchCardWithAuth_NilAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"test"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewAgentCardResolver(10 * time.Second)
	_, err := resolver.FetchCardWithAuth(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil auth: %v", err)
	}
}

func TestBuildAuthHeaders_EmptySchemes(t *testing.T) {
	auth := &AuthConfig{
		Schemes:    []string{},
		SecretData: map[string][]byte{"token": []byte("xxx")},
	}
	headers := buildAuthHeaders(auth)
	if len(headers) != 0 {
		t.Errorf("expected empty headers, got %v", headers)
	}
}
