package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/healthcheck"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

// A2AAgentReconciler reconciles A2AAgent objects.
type A2AAgentReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	AgentCardResolver *registry.AgentCardResolver
	HealthChecker     *healthcheck.Checker
}

// +kubebuilder:rbac:groups=a2a.io,resources=a2aagents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=a2a.io,resources=a2aagents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=a2a.io,resources=a2aagents/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles the reconciliation loop for A2AAgent.
func (r *A2AAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the A2AAgent resource
	agent := &a2aiov1.A2AAgent{}
	err := r.Get(ctx, req.NamespacedName, agent)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("A2AAgent resource not found, ignoring since it must have been deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get A2AAgent resource.")
		return ctrl.Result{}, err
	}

	// If agent is disabled, set phase to Pending and skip health checks
	if !agent.Spec.Enabled {
		logger.Info("Agent is disabled, skipping reconciliation.")
		if agent.Status.Phase != a2aiov1.A2AAgentPhasePending {
			agent.Status.Phase = a2aiov1.A2AAgentPhasePending
			agent.Status.Health = a2aiov1.A2AAgentHealthUnknown
			agent.Status.Message = "Agent is disabled."
			if err := r.Status().Update(ctx, agent); err != nil {
				logger.Error(err, "Failed to update status for disabled agent.")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Determine health check interval from agent spec, fallback to defaults
	intervalSeconds := int32(60)
	if agent.Spec.HealthCheck != nil {
		if agent.Spec.HealthCheck.IntervalSeconds > 0 {
			intervalSeconds = agent.Spec.HealthCheck.IntervalSeconds
		}
	}

	// Perform health check
	logger.Info("Performing health check for agent.", "url", agent.Spec.URL)
	healthResult := r.HealthChecker.Check(ctx, agent)

	// Update status based on health check result
	now := metav1.Now()

	if healthResult.Healthy {
		agent.Status.Phase = a2aiov1.A2AAgentPhaseReady
		agent.Status.Health = a2aiov1.A2AAgentHealthHealthy
		agent.Status.LastHeartbeat = &now
		agent.Status.Message = fmt.Sprintf("Health check passed in %s.", healthResult.Latency)

		// Update agent card hash if it changed
		if healthResult.CardHash != "" {
			agent.Status.AgentCardHash = healthResult.CardHash
		}
	} else {
		agent.Status.Health = a2aiov1.A2AAgentHealthUnhealthy
		agent.Status.Message = fmt.Sprintf("Health check failed: %s", healthResult.Error)

		// Only mark as Unreachable after consecutive failures
		if agent.Status.Phase == a2aiov1.A2AAgentPhaseReady ||
			agent.Status.Phase == a2aiov1.A2AAgentPhasePending ||
			agent.Status.Phase == "" {
			// Check failure threshold
			failureThreshold := int32(3)
			if agent.Spec.HealthCheck != nil && agent.Spec.HealthCheck.FailureThreshold > 0 {
				failureThreshold = agent.Spec.HealthCheck.FailureThreshold
			}

			// Simple failure tracking: if we just failed, set to Error first,
			// only set to Unreachable after threshold
			if agent.Status.Phase == a2aiov1.A2AAgentPhaseError {
				agent.Status.Phase = a2aiov1.A2AAgentPhaseUnreachable
			} else {
				agent.Status.Phase = a2aiov1.A2AAgentPhaseError
			}
			_ = failureThreshold // For future use with condition-based failure counting
		}
	}

	// Update observed generation
	agent.Status.ObservedGeneration = agent.Generation

	// Update conditions
	r.updateConditions(agent, healthResult.Healthy)

	// Update status
	if err := r.Status().Update(ctx, agent); err != nil {
		logger.Error(err, "Failed to update A2AAgent status.")
		return ctrl.Result{}, err
	}

	logger.Info("A2AAgent reconciled successfully.",
		"phase", agent.Status.Phase,
		"health", agent.Status.Health,
	)

	// Requeue after the health check interval
	return ctrl.Result{RequeueAfter: time.Duration(intervalSeconds) * time.Second}, nil
}

// updateConditions updates the standard K8s conditions on the agent status.
func (r *A2AAgentReconciler) updateConditions(agent *a2aiov1.A2AAgent, healthy bool) {
	now := metav1.Now()

	// HealthChecked condition
	healthCondition := metav1.Condition{
		Type:               "HealthChecked",
		ObservedGeneration: agent.Generation,
		LastTransitionTime: now,
	}
	if healthy {
		healthCondition.Status = metav1.ConditionTrue
		healthCondition.Reason = "HealthCheckPassed"
		healthCondition.Message = "Agent is reachable and healthy."
	} else {
		healthCondition.Status = metav1.ConditionFalse
		healthCondition.Reason = "HealthCheckFailed"
		healthCondition.Message = agent.Status.Message
	}

	// Ready condition
	readyCondition := metav1.Condition{
		Type:               "Ready",
		ObservedGeneration: agent.Generation,
		LastTransitionTime: now,
	}
	if agent.Status.Phase == a2aiov1.A2AAgentPhaseReady {
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.Reason = "AgentReady"
		readyCondition.Message = "Agent is registered and healthy."
	} else {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "AgentNotReady"
		readyCondition.Message = agent.Status.Message
	}

	// Merge or append conditions
	agent.Status.Conditions = r.mergeCondition(agent.Status.Conditions, healthCondition)
	agent.Status.Conditions = r.mergeCondition(agent.Status.Conditions, readyCondition)
}

// mergeCondition updates an existing condition or appends a new one.
func (r *A2AAgentReconciler) mergeCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for i, c := range conditions {
		if c.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *A2AAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&a2aiov1.A2AAgent{}).
		Complete(r)
}
