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

type DiscoveryConfig struct {
	Scope         string
	LabelSelector string
	Namespaces    []string
}

type Server struct {
	client          client.Client
	addr            string
	httpServer      *http.Server
	handler         *Handler
	configMu        sync.RWMutex
	discoveryConfig DiscoveryConfig
}

func NewServer(client client.Client, addr string) *Server {
	config := DiscoveryConfig{Scope: "Cluster"}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = normalizeAddr(addr)
	}
	s := &Server{client: client, addr: addr, discoveryConfig: config}
	s.handler = NewHandler(client, &s.discoveryConfig)
	return s
}

func normalizeAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		if host == "" {
			host = "0.0.0.0"
		}
		return net.JoinHostPort(host, port)
	}
	if len(addr) > 1 && addr[0] == ':' {
		if _, err := strconv.Atoi(addr[1:]); err == nil {
			return net.JoinHostPort("0.0.0.0", addr[1:])
		}
	}
	if _, err := strconv.Atoi(addr); err == nil {
		return net.JoinHostPort("0.0.0.0", addr)
	}
	return "0.0.0.0:8082"
}

func (s *Server) UpdateConfig(cfg DiscoveryConfig) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	s.discoveryConfig = cfg
}

func (s *Server) GetConfig() DiscoveryConfig {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.discoveryConfig
}

func measureDuration(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler(w, r)
		metrics.APIRequestDuration.WithLabelValues(endpoint, r.Method).Observe(time.Since(start).Seconds())
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", measureDuration("dashboard", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			dashboardHandler(w, r)
			return
		}
		http.NotFound(w, r)
	}))

	mux.HandleFunc("/.well-known/agent-card.json", measureDuration("agent-card", s.handler.WellKnown))
	mux.HandleFunc("/.well-known/agent.json", measureDuration("agent-card", s.handler.WellKnown))

	agentsHandler := measureDuration("agents", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if r.Method == http.MethodPost && (path == "/api/v1/agents" || path == "/api/v1/agents/") {
			s.handler.RegisterAgent(w, r)
			return
		}
		if path == "/api/v1/agents" || path == "/api/v1/agents/" {
			s.handler.ListAgents(w, r)
			return
		}
		if len(path) > len("/api/v1/agents/") {
			subPath := path[len("/api/v1/agents/"):]

			if r.Method == http.MethodDelete && !strings.Contains(subPath, "/") {
				s.handler.DeregisterAgent(w, r)
				return
			}
			if r.Method == http.MethodPost && strings.HasSuffix(subPath, "/approve") {
				s.handler.ApproveAgent(w, r)
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
	})
	mux.HandleFunc("/api/v1/agents", agentsHandler)
	mux.HandleFunc("/api/v1/agents/", agentsHandler)

	mux.HandleFunc("/api/v1/config", measureDuration("config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			s.handler.UpdateConfig(w, r)
		default:
			s.handler.Config(w, r)
		}
	}))
	mux.HandleFunc("/api/v1/search", measureDuration("search", s.handler.Search))

	s.httpServer = &http.Server{Addr: s.addr, Handler: mux}
	ctrl.Log.WithName("registry-api").Info("Starting A2A Registry API server", "addr", s.addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		ctrl.Log.WithName("registry-api").Info("Shutting down registry API server")
		return s.httpServer.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func (s *Server) NeedLeaderElection() bool { return false }
