package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/healthcheck"
	"github.com/terminus-io/a2a-registry/internal/metrics"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

// A2AAgentReconciler reconciles A2AAgent objects.
type A2AAgentReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	AgentCardResolver *registry.AgentCardResolver
	HealthChecker     *healthcheck.Checker
	Recorder          record.EventRecorder
}

// +kubebuilder:rbac:groups=a2a.io,resources=a2aagents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=a2a.io,resources=a2aagents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=a2a.io,resources=a2aagents/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=a2a.io,resources=a2aregistries,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for A2AAgent.
func (r *A2AAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

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

	// Handle finalizer logic
	if !agent.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(agent, A2AFinalizer) {
			controllerutil.RemoveFinalizer(agent, A2AFinalizer)
			if err := r.Update(ctx, agent); err != nil {
				logger.Error(err, "Failed to remove finalizer.")
				return ctrl.Result{}, err
			}
			logger.Info("Finalizer removed, agent deletion allowed.")
			metrics.DeregistrationsTotal.Inc()
			r.Recorder.Event(agent, corev1.EventTypeNormal, "FinalizerRemoved", "Agent deletion completed.")
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(agent, A2AFinalizer) {
		controllerutil.AddFinalizer(agent, A2AFinalizer)
		if err := r.Update(ctx, agent); err != nil {
			logger.Error(err, "Failed to add finalizer.")
			return ctrl.Result{}, err
		}
		// Continue reconciliation instead of returning early, so that
		// approval checks and health checks can proceed immediately.
		// The GenerationChangedPredicate would otherwise filter out the
		// update event (finalizer changes don't increment generation).
	}

	// Record registration time on first reconciliation
	if agent.Status.RegisteredAt == nil {
		now := metav1.Now()
		agent.Status.RegisteredAt = &now
		metrics.RegistrationsTotal.Inc()
	}

	// Check for URL conflict with other agents
	if conflict := r.checkURLConflict(ctx, agent); conflict != "" {
		agent.Status.Phase = a2aiov1.A2AAgentPhaseError
		agent.Status.Health = a2aiov1.A2AAgentHealthUnhealthy
		agent.Status.Message = conflict
		if err := r.Status().Update(ctx, agent); err != nil {
			logger.Error(err, "Failed to update URL conflict status.")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
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

	// Read registry config for registration policies
	registryConfig := r.getRegistryConfig(ctx)
	previousPhase := agent.Status.Phase

	// Check if approval is required and agent is not yet approved
	if registryConfig != nil && registryConfig.Registration.RequireApproval {
		if !isApproved(agent) {
			logger.Info("Agent requires approval, skipping health check.")
			agent.Status.Phase = a2aiov1.A2AAgentPhasePending
			agent.Status.Health = a2aiov1.A2AAgentHealthUnknown
			agent.Status.Message = "Waiting for approval."
			agent.Status.Conditions = MergeCondition(agent.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeApproved,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: agent.Generation,
				LastTransitionTime: metav1.Now(),
				Reason:             "PendingApproval",
				Message:            "Agent requires manual approval.",
			})
			if err := r.Status().Update(ctx, agent); err != nil {
				logger.Error(err, "Failed to update approval status.")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		agent.Status.Conditions = MergeCondition(agent.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeApproved,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: agent.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "Approved",
			Message:            "Agent has been approved.",
		})
	}

	// Priority: per-agent > global registry default > hardcoded 60s
	intervalSeconds := int32(60)
	if registryConfig != nil && registryConfig.HealthCheck.IntervalSeconds > 0 {
		intervalSeconds = registryConfig.HealthCheck.IntervalSeconds
	}
	if agent.Spec.HealthCheck != nil {
		if agent.Spec.HealthCheck.IntervalSeconds > 0 {
			intervalSeconds = agent.Spec.HealthCheck.IntervalSeconds
		}
	}

	auth := r.buildAuthConfig(ctx, agent)

	var healthResult *healthcheck.Result
	now := metav1.Now()

	if registryConfig != nil && !registryConfig.Registration.RequireHealthCheck {
		logger.Info("Health check disabled by registry config.")
		agent.Status.Phase = a2aiov1.A2AAgentPhaseReady
		agent.Status.Health = a2aiov1.A2AAgentHealthHealthy
		agent.Status.LastHeartbeat = &now
		agent.Status.Message = "Health check disabled by registry configuration."
		agent.Status.ConsecutiveFailures = 0
	} else {
		logger.Info("Performing health check for agent.", "url", agent.Spec.URL)
		healthResult = r.HealthChecker.CheckWithAuth(ctx, agent, auth)

		metrics.HealthCheckDuration.Observe(healthResult.Latency.Seconds())
		if !healthResult.Healthy {
			metrics.HealthCheckFailuresTotal.Inc()
		}

		if healthResult.Healthy {
			agent.Status.Phase = a2aiov1.A2AAgentPhaseReady
			agent.Status.Health = a2aiov1.A2AAgentHealthHealthy
			agent.Status.LastHeartbeat = &now
			agent.Status.Message = fmt.Sprintf("Health check passed in %s.", healthResult.Latency)

			if healthResult.CardHash != "" {
				agent.Status.AgentCardHash = healthResult.CardHash
			}

			agent.Status.ConsecutiveFailures = 0

			if previousPhase == a2aiov1.A2AAgentPhaseError ||
				previousPhase == a2aiov1.A2AAgentPhaseUnreachable {
				r.Recorder.Eventf(agent, corev1.EventTypeNormal, "HealthCheckRecovered",
					"Agent recovered from %s to Ready.", previousPhase)
			}

			if registryConfig != nil && registryConfig.Registration.RequireCardMatch && healthResult.Card != nil {
				if mismatch := checkCardMatch(agent, healthResult.Card); mismatch != "" {
					agent.Status.Health = a2aiov1.A2AAgentHealthUnhealthy
					agent.Status.Phase = a2aiov1.A2AAgentPhaseError
					agent.Status.Message = fmt.Sprintf("Agent card mismatch: %s", mismatch)
					r.Recorder.Eventf(agent, corev1.EventTypeWarning, "CardMismatch",
						"Agent card does not match spec: %s", mismatch)
				}
			}
		} else {
			agent.Status.Health = a2aiov1.A2AAgentHealthUnhealthy
			agent.Status.Message = fmt.Sprintf("Health check failed: %s", healthResult.Error)

			failureThreshold := int32(3)
			if agent.Spec.HealthCheck != nil && agent.Spec.HealthCheck.FailureThreshold > 0 {
				failureThreshold = agent.Spec.HealthCheck.FailureThreshold
			}

			agent.Status.ConsecutiveFailures++
			if agent.Status.ConsecutiveFailures >= failureThreshold {
				agent.Status.Phase = a2aiov1.A2AAgentPhaseUnreachable
				if previousPhase != a2aiov1.A2AAgentPhaseUnreachable {
					r.Recorder.Eventf(agent, corev1.EventTypeWarning, "AgentUnreachable",
						"Agent is unreachable after %d consecutive failures: %s",
						agent.Status.ConsecutiveFailures, healthResult.Error)
				}
			} else {
				agent.Status.Phase = a2aiov1.A2AAgentPhaseError
				if previousPhase != a2aiov1.A2AAgentPhaseError {
					r.Recorder.Eventf(agent, corev1.EventTypeWarning, "HealthCheckFailed",
						"Health check failed: %s", healthResult.Error)
				}
			}
		}
	}

	agent.Status.ObservedGeneration = agent.Generation
	isHealthy := agent.Status.Health == a2aiov1.A2AAgentHealthHealthy
	r.updateConditions(agent, isHealthy)

	if err := r.Status().Update(ctx, agent); err != nil {
		logger.Error(err, "Failed to update A2AAgent status.")
		return ctrl.Result{}, err
	}

	logger.Info("A2AAgent reconciled successfully.",
		"phase", agent.Status.Phase,
		"health", agent.Status.Health,
	)

	return ctrl.Result{RequeueAfter: time.Duration(intervalSeconds) * time.Second}, nil
}

// checkURLConflict checks if any other agent uses the same URL.
func (r *A2AAgentReconciler) checkURLConflict(ctx context.Context, agent *a2aiov1.A2AAgent) string {
	all := &a2aiov1.A2AAgentList{}
	if err := r.List(ctx, all); err != nil {
		return ""
	}
	for _, a := range all.Items {
		if a.Name == agent.Name && a.Namespace == agent.Namespace {
			continue // skip self
		}
		if a.Spec.URL == agent.Spec.URL {
			return fmt.Sprintf("URL conflict: %q is already registered by agent %q in namespace %q",
				agent.Spec.URL, a.Name, a.Namespace)
		}
	}
	return ""
}

func (r *A2AAgentReconciler) getRegistryConfig(ctx context.Context) *a2aiov1.A2ARegistrySpec {
	registries := &a2aiov1.A2ARegistryList{}
	if err := r.List(ctx, registries); err != nil || len(registries.Items) == 0 {
		return nil
	}
	return &registries.Items[0].Spec
}

func checkCardMatch(agent *a2aiov1.A2AAgent, card *a2a.AgentCard) string {
	if agent.Spec.Name != "" && agent.Spec.Name != card.Name {
		return fmt.Sprintf("name mismatch: spec=%q card=%q", agent.Spec.Name, card.Name)
	}
	if agent.Spec.Description != "" && agent.Spec.Description != card.Description {
		return "description mismatch"
	}
	if agent.Spec.Version != "" && agent.Spec.Version != card.Version {
		return fmt.Sprintf("version mismatch: spec=%q card=%q", agent.Spec.Version, card.Version)
	}
	for _, specSkill := range agent.Spec.Skills {
		found := false
		for _, cardSkill := range card.Skills {
			if specSkill.ID == cardSkill.ID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Sprintf("skill %q in spec not found in agent card", specSkill.ID)
		}
	}
	return ""
}

func (r *A2AAgentReconciler) buildAuthConfig(ctx context.Context, agent *a2aiov1.A2AAgent) *registry.AuthConfig {
	if agent.Spec.Authentication == nil {
		return nil
	}
	auth := &registry.AuthConfig{
		Schemes: agent.Spec.Authentication.Schemes,
	}
	if agent.Spec.Authentication.SecretRef != nil {
		secret, err := r.readSecret(ctx, agent.Namespace, agent.Spec.Authentication.SecretRef.Name)
		if err != nil {
			ctrl.Log.WithName("a2aagent-controller").Error(err, "Failed to read auth secret",
				"secret", agent.Spec.Authentication.SecretRef.Name)
			return auth
		}
		auth.SecretData = secret
	}
	return auth
}

func (r *A2AAgentReconciler) readSecret(ctx context.Context, namespace, name string) (map[string][]byte, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret); err != nil {
		return nil, err
	}
	return secret.Data, nil
}

func (r *A2AAgentReconciler) updateConditions(agent *a2aiov1.A2AAgent, healthy bool) {
	now := metav1.Now()

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

	agent.Status.Conditions = MergeCondition(agent.Status.Conditions, healthCondition)
	agent.Status.Conditions = MergeCondition(agent.Status.Conditions, readyCondition)
}

func isApproved(agent *a2aiov1.A2AAgent) bool {
	for _, c := range agent.Status.Conditions {
		if c.Type == ConditionTypeApproved && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}


func (r *A2AAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&a2aiov1.A2AAgent{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
