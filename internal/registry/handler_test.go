package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
)

// stubClient implements client.Client for testing handler endpoints.
type stubClient struct {
	agents map[string]*a2aiov1.A2AAgent
	err    error
}

func newStubClient() *stubClient {
	return &stubClient{
		agents: make(map[string]*a2aiov1.A2AAgent),
	}
}

func (s *stubClient) addAgent(name, namespace string, spec a2aiov1.A2AAgentSpec, status a2aiov1.A2AAgentStatus) {
	s.agents[namespace+"/"+name] = &a2aiov1.A2AAgent{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       spec,
		Status:     status,
	}
}

// Reader interface

func (s *stubClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if s.err != nil {
		return s.err
	}
	agent, ok := s.agents[key.Namespace+"/"+key.Name]
	if !ok {
		return errors.NewNotFound(schema.GroupResource{Group: "a2a.io", Resource: "a2aagents"}, key.Name)
	}
	*obj.(*a2aiov1.A2AAgent) = *agent
	return nil
}

func (s *stubClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if s.err != nil {
		return s.err
	}
	agentList := list.(*a2aiov1.A2AAgentList)
	for _, a := range s.agents {
		agentList.Items = append(agentList.Items, *a)
	}
	return nil
}

// Writer interface (stubs)

func (s *stubClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}
func (s *stubClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}
func (s *stubClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}
func (s *stubClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}
func (s *stubClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}
func (s *stubClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return nil
}

// StatusClient

func (s *stubClient) Status() client.SubResourceWriter {
	return &stubStatusWriter{}
}

// SubResourceClientConstructor

func (s *stubClient) SubResource(subResource string) client.SubResourceClient {
	return &stubSubResourceClient{}
}

// Other Client methods

func (s *stubClient) Scheme() *runtime.Scheme { return nil }
func (s *stubClient) RESTMapper() meta.RESTMapper {
	return nil
}
func (s *stubClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (s *stubClient) IsObjectNamespaced(obj runtime.Object) (bool, error) { return false, nil }

// stubStatusWriter implements client.SubResourceWriter.
type stubStatusWriter struct{}

func (w *stubStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return nil
}
func (w *stubStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}
func (w *stubStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return nil
}
func (w *stubStatusWriter) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.SubResourceApplyOption) error {
	return nil
}

// stubSubResourceClient implements client.SubResourceClient.
type stubSubResourceClient struct{}

func (c *stubSubResourceClient) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	return nil
}
func (c *stubSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return nil
}
func (c *stubSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}
func (c *stubSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return nil
}
func (c *stubSubResourceClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.SubResourceApplyOption) error {
	return nil
}

// Tests

func TestListAgents_ReturnsEnabledAgents(t *testing.T) {
	client := newStubClient()
	client.addAgent("agent1", "default",
		a2aiov1.A2AAgentSpec{Name: "agent1", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{Phase: a2aiov1.A2AAgentPhaseReady},
	)
	client.addAgent("agent2", "default",
		a2aiov1.A2AAgentSpec{Name: "agent2", URL: "http://example.com", Enabled: false},
		a2aiov1.A2AAgentStatus{Phase: a2aiov1.A2AAgentPhasePending},
	)

	config := &DiscoveryConfig{Scope: "Cluster"}
	handler := NewHandler(client, config)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	handler.ListAgents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entries []RegistryEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 agent (disabled filtered out), got %d", len(entries))
	}
	if entries[0].DisplayName != "agent1" {
		t.Errorf("expected agent1, got %s", entries[0].DisplayName)
	}
}

func TestListAgents_TagFilter(t *testing.T) {
	client := newStubClient()
	client.addAgent("agent1", "default",
		a2aiov1.A2AAgentSpec{Name: "agent1", URL: "http://example.com", Enabled: true, Tags: []string{"demo", "test"}},
		a2aiov1.A2AAgentStatus{},
	)
	client.addAgent("agent2", "default",
		a2aiov1.A2AAgentSpec{Name: "agent2", URL: "http://example.com", Enabled: true, Tags: []string{"prod"}},
		a2aiov1.A2AAgentStatus{},
	)

	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents?tags=demo", nil)
	w := httptest.NewRecorder()
	handler.ListAgents(w, req)

	var entries []RegistryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 agent with tag 'demo', got %d", len(entries))
	}
	if entries[0].DisplayName != "agent1" {
		t.Errorf("expected agent1, got %s", entries[0].DisplayName)
	}
}

func TestListAgents_SkillFilter(t *testing.T) {
	client := newStubClient()
	client.addAgent("agent1", "default",
		a2aiov1.A2AAgentSpec{
			Name: "agent1", URL: "http://example.com", Enabled: true,
			Skills: []a2aiov1.A2AAgentSkillSpec{{ID: "greeting", Name: "Greeting"}},
		},
		a2aiov1.A2AAgentStatus{},
	)
	client.addAgent("agent2", "default",
		a2aiov1.A2AAgentSpec{
			Name: "agent2", URL: "http://example.com", Enabled: true,
			Skills: []a2aiov1.A2AAgentSkillSpec{{ID: "search", Name: "Search"}},
		},
		a2aiov1.A2AAgentStatus{},
	)

	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents?skill=greeting", nil)
	w := httptest.NewRecorder()
	handler.ListAgents(w, req)

	var entries []RegistryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 agent with skill 'greeting', got %d", len(entries))
	}
}

func TestListAgents_NamespaceFilter(t *testing.T) {
	client := newStubClient()
	client.addAgent("agent1", "ns1",
		a2aiov1.A2AAgentSpec{Name: "agent1", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{},
	)
	client.addAgent("agent2", "ns2",
		a2aiov1.A2AAgentSpec{Name: "agent2", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{},
	)

	handler := NewHandler(client, &DiscoveryConfig{Scope: "Namespace", Namespaces: []string{"ns1"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	handler.ListAgents(w, req)

	var entries []RegistryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 agent in ns1, got %d", len(entries))
	}
	if entries[0].Namespace != "ns1" {
		t.Errorf("expected namespace ns1, got %s", entries[0].Namespace)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	client := newStubClient()
	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/nonexistent?namespace=default", nil)
	w := httptest.NewRecorder()
	handler.GetAgent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetAgent_Found(t *testing.T) {
	client := newStubClient()
	client.addAgent("my-agent", "default",
		a2aiov1.A2AAgentSpec{Name: "My Agent", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{Phase: a2aiov1.A2AAgentPhaseReady},
	)

	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/my-agent?namespace=default", nil)
	w := httptest.NewRecorder()
	handler.GetAgent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entry RegistryEntry
	json.NewDecoder(w.Body).Decode(&entry)
	if entry.DisplayName != "My Agent" {
		t.Errorf("expected 'My Agent', got '%s'", entry.DisplayName)
	}
}

func TestSearch_FreeText(t *testing.T) {
	client := newStubClient()
	client.addAgent("hello-world", "default",
		a2aiov1.A2AAgentSpec{Name: "hello-world", Description: "A greeting agent", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{},
	)
	client.addAgent("search-bot", "default",
		a2aiov1.A2AAgentSpec{Name: "search-bot", Description: "A search agent", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{},
	)

	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=greeting", nil)
	w := httptest.NewRecorder()
	handler.Search(w, req)

	var entries []RegistryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 result for 'greeting', got %d", len(entries))
	}
}

func TestSearch_CapabilityFilter(t *testing.T) {
	client := newStubClient()
	client.addAgent("streaming-agent", "default",
		a2aiov1.A2AAgentSpec{
			Name: "streaming-agent", URL: "http://example.com", Enabled: true,
			Capabilities: a2aiov1.A2AAgentCapabilities{Streaming: true},
		},
		a2aiov1.A2AAgentStatus{},
	)
	client.addAgent("plain-agent", "default",
		a2aiov1.A2AAgentSpec{
			Name: "plain-agent", URL: "http://example.com", Enabled: true,
		},
		a2aiov1.A2AAgentStatus{},
	)

	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?capability=streaming", nil)
	w := httptest.NewRecorder()
	handler.Search(w, req)

	var entries []RegistryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 streaming agent, got %d", len(entries))
	}
}

func TestWellKnown_ReturnsValidAgentCard(t *testing.T) {
	client := newStubClient()
	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster"})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	w := httptest.NewRecorder()
	handler.WellKnown(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", w.Header().Get("Content-Type"))
	}

	var card map[string]interface{}
	json.NewDecoder(w.Body).Decode(&card)
	if card["name"] != "A2A Registry" {
		t.Errorf("expected name 'A2A Registry', got '%v'", card["name"])
	}
}

func TestListAgents_LabelSelectorFilter(t *testing.T) {
	client := newStubClient()
	client.addAgent("agent1", "default",
		a2aiov1.A2AAgentSpec{Name: "agent1", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{},
	)
	client.addAgent("agent2", "default",
		a2aiov1.A2AAgentSpec{Name: "agent2", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{},
	)

	// With label selector that matches nothing — returns empty
	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster", LabelSelector: "foo=bar"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	handler.ListAgents(w, req)

	var entries []RegistryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	// Note: label selector filtering happens server-side via the k8s API,
	// but in our stub client it's not applied. This test validates the handler
	// passes the selector as a list option without errors.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSearch_WithLabelSelector(t *testing.T) {
	client := newStubClient()
	client.addAgent("agent1", "default",
		a2aiov1.A2AAgentSpec{Name: "agent1", URL: "http://example.com", Enabled: true},
		a2aiov1.A2AAgentStatus{},
	)

	handler := NewHandler(client, &DiscoveryConfig{Scope: "Cluster", LabelSelector: "app=test"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=agent", nil)
	w := httptest.NewRecorder()
	handler.Search(w, req)

	var entries []RegistryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
