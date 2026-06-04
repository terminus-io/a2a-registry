package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/metrics"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

// A2ARegistryReconciler reconciles A2ARegistry objects.
type A2ARegistryReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	APIServer *registry.Server
	Recorder  record.EventRecorder
}

// +kubebuilder:rbac:groups=a2a.io,resources=a2aregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=a2a.io,resources=a2aregistries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=a2a.io,resources=a2aregistries/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for A2ARegistry.
func (r *A2ARegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	reg := &a2aiov1.A2ARegistry{}
	err := r.Get(ctx, req.NamespacedName, reg)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("A2ARegistry resource not found, ignoring since it must have been deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get A2ARegistry resource.")
		return ctrl.Result{}, err
	}

	if !reg.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(reg, A2AFinalizer) {
			controllerutil.RemoveFinalizer(reg, A2AFinalizer)
			if err := r.Update(ctx, reg); err != nil {
				logger.Error(err, "Failed to remove finalizer.")
				return ctrl.Result{}, err
			}
			logger.Info("Finalizer removed, registry deletion allowed.")
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(reg, A2AFinalizer) {
		controllerutil.AddFinalizer(reg, A2AFinalizer)
		if err := r.Update(ctx, reg); err != nil {
			logger.Error(err, "Failed to add finalizer.")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	agents := &a2aiov1.A2AAgentList{}
	if err := r.List(ctx, agents); err != nil {
		logger.Error(err, "Failed to list A2AAgents.")
		return ctrl.Result{}, err
	}

	totalAgents := int32(len(agents.Items))
	healthyAgents := int32(0)

	metrics.AgentCount.Reset()
	for _, agent := range agents.Items {
		if agent.Spec.Enabled && agent.Status.Health == a2aiov1.A2AAgentHealthHealthy {
			healthyAgents++
		}
		phase := string(agent.Status.Phase)
		if phase == "" {
			phase = "Pending"
		}
		health := string(agent.Status.Health)
		if health == "" {
			health = "Unknown"
		}
		metrics.AgentCount.WithLabelValues(phase, health).Inc()
	}

	// Prune agents unreachable for more than 7 days
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for i := range agents.Items {
		agent := &agents.Items[i]
		if agent.Status.Phase == a2aiov1.A2AAgentPhaseUnreachable &&
			agent.Status.LastHeartbeat != nil &&
			agent.Status.LastHeartbeat.Time.Before(cutoff) {
			logger.Info("Cleaning up stale unreachable agent.",
				"name", agent.Name, "namespace", agent.Namespace,
				"lastHeartbeat", agent.Status.LastHeartbeat.Time)
			r.Recorder.Eventf(agent, corev1.EventTypeNormal, "AgentPruned",
				"Agent has been unreachable since %s and was automatically removed.",
				agent.Status.LastHeartbeat.Time.Format(time.RFC3339))
			if err := r.Delete(ctx, agent); err != nil {
				logger.Error(err, "Failed to delete stale agent.",
					"name", agent.Name, "namespace", agent.Namespace)
			}
		}
	}

	reg.Status.AgentCount = totalAgents
	reg.Status.HealthyAgents = healthyAgents
	reg.Status.Phase = "Active"

	now := metav1.Now()
	reg.Status.Conditions = MergeCondition(reg.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: reg.Generation,
		LastTransitionTime: now,
		Reason:             "RegistryActive",
		Message:            "Registry is active and tracking agents.",
	})

	if err := r.Status().Update(ctx, reg); err != nil {
		logger.Error(err, "Failed to update A2ARegistry status.")
		return ctrl.Result{}, err
	}

	if reg.Status.AgentCount != totalAgents || reg.Status.HealthyAgents != healthyAgents {
		r.Recorder.Eventf(reg, corev1.EventTypeNormal, "RegistryActive",
		"Registry tracking %d agents (%d healthy).", totalAgents, healthyAgents)
		}

	if r.APIServer != nil {
		r.APIServer.UpdateConfig(registry.DiscoveryConfig{
			Scope:         reg.Spec.Discovery.Scope,
			LabelSelector: reg.Spec.Discovery.LabelSelector,
			Namespaces:    reg.Spec.Discovery.Namespaces,
		})
	}

	logger.Info("A2ARegistry reconciled successfully.",
		"totalAgents", totalAgents,
		"healthyAgents", healthyAgents,
	)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}


func (r *A2ARegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&a2aiov1.A2ARegistry{}).
		Complete(r)
}
