package registry

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
)

//go:embed dashboard.html
var dashboardHTML []byte

// dashboardHandler serves the embedded dashboard page.
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(dashboardHTML)
}

// ApproveAgent handles agent approval via HTTP POST.
// The handler patches the agent's status conditions to add an Approved condition.
func (h *Handler) ApproveAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := extractNameFromPath(r.URL.Path, "/api/v1/agents/")
	name = strings.TrimSuffix(name, "/approve")
	if name == "" {
		http.Error(w, "Agent name is required.", http.StatusBadRequest)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	agent := &a2aiov1.A2AAgent{}
	if err := h.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, agent); err != nil {
		http.Error(w, fmt.Sprintf("Agent not found: %v", err), http.StatusNotFound)
		return
	}

	now := metav1.Now()
	approved := metav1.Condition{
		Type:               "Approved",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: agent.Generation,
		LastTransitionTime: now,
		Reason:             "Approved",
		Message:            "Agent has been approved via dashboard.",
	}

	agent.Status.Conditions = mergeRegistryConditions(agent.Status.Conditions, approved)

	if err := h.client.Status().Update(ctx, agent); err != nil {
		http.Error(w, fmt.Sprintf("Failed to approve agent: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"approved"}`))
}

// mergeRegistryConditions updates or appends a condition (same logic as controllers but local copy).
func mergeRegistryConditions(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for i, c := range conditions {
		if c.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}
