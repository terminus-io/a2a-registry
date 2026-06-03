package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	a2aiov1 "github.com/terminus-io/a2a-registry/api/v1"
	"github.com/terminus-io/a2a-registry/internal/registry"
)

// A2ARegistryReconciler reconciles A2ARegistry objects.
type A2ARegistryReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	APIServer *registry.Server
}

// +kubebuilder:rbac:groups=a2a.io,resources=a2aregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=a2a.io,resources=a2aregistries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=a2a.io,resources=a2aregistries/finalizers,verbs=update

// Reconcile handles the reconciliation loop for A2ARegistry.
func (r *A2ARegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the A2ARegistry resource
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

	// Calculate agent counts
	agents := &a2aiov1.A2AAgentList{}
	if err := r.List(ctx, agents); err != nil {
		logger.Error(err, "Failed to list A2AAgents.")
		return ctrl.Result{}, err
	}

	totalAgents := int32(len(agents.Items))
	healthyAgents := int32(0)
	for _, agent := range agents.Items {
		if agent.Spec.Enabled && agent.Status.Health == a2aiov1.A2AAgentHealthHealthy {
			healthyAgents++
		}
	}

	// Update status
	reg.Status.AgentCount = totalAgents
	reg.Status.HealthyAgents = healthyAgents
	reg.Status.Phase = "Active"

	// Update conditions
	now := metav1.Now()
	reg.Status.Conditions = r.mergeCondition(reg.Status.Conditions, metav1.Condition{
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

	// Push updated discovery config to the API server
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

	// Requeue periodically for stats refresh
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// mergeCondition updates an existing condition or appends a new one.
func (r *A2ARegistryReconciler) mergeCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for i, c := range conditions {
		if c.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}

// mapAgentToRegistry enqueues the registry for reconciliation when any agent changes.
func (r *A2ARegistryReconciler) mapAgentToRegistry(ctx context.Context, obj client.Object) []reconcile.Request {
	registries := &a2aiov1.A2ARegistryList{}
	if err := r.List(ctx, registries); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(registries.Items))
	for _, reg := range registries.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: reg.Name},
		})
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *A2ARegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&a2aiov1.A2ARegistry{}).
		Watches(&a2aiov1.A2AAgent{}, handler.EnqueueRequestsFromMapFunc(r.mapAgentToRegistry)).
		Complete(r)
}
