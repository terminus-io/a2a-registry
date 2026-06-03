package registry

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
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
	port            int32
	bindAddress     string
	httpServer      *http.Server
	handler         *Handler
	configMu        sync.RWMutex
	discoveryConfig DiscoveryConfig
}

// NewServer creates a new registry API server.
func NewServer(client client.Client, port int32, bindAddress string) *Server {
	config := DiscoveryConfig{
		Scope: "Cluster",
	}

	s := &Server{
		client:          client,
		port:            port,
		bindAddress:     bindAddress,
		discoveryConfig: config,
	}

	s.handler = NewHandler(client, &s.discoveryConfig)
	return s
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

// Start starts the HTTP server. Implements manager.Runnable.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Well-known endpoints — the registry's own Agent Card (both A2A spec versions)
	mux.HandleFunc("/.well-known/agent-card.json", s.handler.WellKnown)
	mux.HandleFunc("/.well-known/agent.json", s.handler.WellKnown)

	// Agent listing and retrieval
	mux.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		// Route: /api/v1/agents or /api/v1/agents/{name} or /api/v1/agents/{name}/card
		path := r.URL.Path

		if path == "/api/v1/agents" || path == "/api/v1/agents/" {
			s.handler.ListAgents(w, r)
			return
		}

		// Check for /card suffix
		if len(path) > len("/api/v1/agents/") {
			subPath := path[len("/api/v1/agents/"):]
			if len(subPath) > 5 && subPath[len(subPath)-5:] == "/card" {
				s.handler.GetAgentCard(w, r)
				return
			}
			s.handler.GetAgent(w, r)
			return
		}

		s.handler.ListAgents(w, r)
	})

	// Search endpoint
	mux.HandleFunc("/api/v1/search", s.handler.Search)

	addr := fmt.Sprintf("%s:%d", s.bindAddress, s.port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger := fmt.Sprintf("[registry-api] Starting A2A Registry API server on %s", addr)
	fmt.Println(logger)

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
		fmt.Println("[registry-api] Shutting down registry API server...")
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
