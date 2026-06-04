package registry

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/terminus-io/a2a-registry/internal/metrics"
)

// DiscoveryConfig holds the current discovery configuration.
type DiscoveryConfig struct {
	Scope         string
	LabelSelector string
	Namespaces    []string
}

// Server serves the registry API over HTTP.
// It implements manager.Runnable so it can be managed by controller-runtime.
type Server struct {
	client          client.Client
	addr            string
	httpServer      *http.Server
	handler         *Handler
	configMu        sync.RWMutex
	discoveryConfig DiscoveryConfig
}

// NewServer creates a new registry API server.
// addr is a host:port string (e.g., "0.0.0.0:8082" or ":8082").
func NewServer(client client.Client, addr string) *Server {
	config := DiscoveryConfig{
		Scope: "Cluster",
	}

	// Validate and normalize the address
	if _, _, err := net.SplitHostPort(addr); err != nil {
		// Try to recover: if addr looks like ":NNNN", use it as-is with default host
		addr = normalizeAddr(addr)
	}

	s := &Server{
		client:          client,
		addr:            addr,
		discoveryConfig: config,
	}

	s.handler = NewHandler(client, &s.discoveryConfig)
	return s
}

// normalizeAddr ensures a valid host:port string, defaulting host to "0.0.0.0"
// and port to 8082 when parsing fails.
func normalizeAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		if host == "" {
			host = "0.0.0.0"
		}
		return net.JoinHostPort(host, port)
	}

	// addr might be a bare port like "8082" or ":8082"
	if len(addr) > 1 && addr[0] == ':' {
		if _, err := strconv.Atoi(addr[1:]); err == nil {
			return net.JoinHostPort("0.0.0.0", addr[1:])
		}
	}
	if _, err := strconv.Atoi(addr); err == nil {
		return net.JoinHostPort("0.0.0.0", addr)
	}

	// Fallback
	return "0.0.0.0:8082"
}

// UpdateConfig updates the discovery configuration at runtime.
// Called by the A2ARegistry controller when the registry config changes.
func (s *Server) UpdateConfig(cfg DiscoveryConfig) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	s.discoveryConfig = cfg
}

// GetConfig returns the current discovery configuration.
func (s *Server) GetConfig() DiscoveryConfig {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.discoveryConfig
}

// measureDuration wraps an http.HandlerFunc with duration tracking.
func measureDuration(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler(w, r)
		metrics.APIRequestDuration.WithLabelValues(endpoint, r.Method).Observe(time.Since(start).Seconds())
	}
}

// Start starts the HTTP server. Implements manager.Runnable.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Well-known endpoints — the registry's own Agent Card (both A2A spec versions)
	mux.HandleFunc("/.well-known/agent-card.json", measureDuration("agent-card", s.handler.WellKnown))
	mux.HandleFunc("/.well-known/agent.json", measureDuration("agent-card", s.handler.WellKnown))

	// Agent listing, registration, and retrieval
	mux.HandleFunc("/api/v1/agents", measureDuration("agents", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// POST /api/v1/agents — register a new agent
		if r.Method == http.MethodPost && (path == "/api/v1/agents" || path == "/api/v1/agents/") {
			s.handler.RegisterAgent(w, r)
			return
		}

		if path == "/api/v1/agents" || path == "/api/v1/agents/" {
			// GET /api/v1/agents — list
			s.handler.ListAgents(w, r)
			return
		}

		// /api/v1/agents/{name} or /api/v1/agents/{name}/card
		if len(path) > len("/api/v1/agents/") {
			subPath := path[len("/api/v1/agents/"):]

			// DELETE /api/v1/agents/{name} — deregister
			if r.Method == http.MethodDelete && !strings.Contains(subPath, "/") {
				s.handler.DeregisterAgent(w, r)
				return
			}

			if len(subPath) > 5 && subPath[len(subPath)-5:] == "/card" {
				s.handler.GetAgentCard(w, r)
				return
			}
			s.handler.GetAgent(w, r)
			return
		}

		s.handler.ListAgents(w, r)
	}))

	// Search endpoint
	mux.HandleFunc("/api/v1/search", measureDuration("search", s.handler.Search))

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	ctrl.Log.WithName("registry-api").Info("Starting A2A Registry API server", "addr", s.addr)

	// Serve in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		// Manager asked us to stop
		ctrl.Log.WithName("registry-api").Info("Shutting down registry API server")
		return s.httpServer.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// NeedLeaderElection returns false — the API server should run on all replicas,
// not just the leader, so agents can always reach the registry.
func (s *Server) NeedLeaderElection() bool {
	return false
}
