package v1

import (
	"context"
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var a2aregistrylog = ctrl.Log.WithName("a2aregistry-webhook")

// SetupA2ARegistryWebhook registers the webhook with the manager.
func SetupA2ARegistryWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &A2ARegistry{}).
		WithValidator(&A2ARegistryValidator{}).
		WithDefaulter(&A2ARegistryDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-a2a-io-v1-a2aregistry,mutating=false,failurePolicy=fail,groups=a2a.io,resources=a2aregistries,verbs=create;update,versions=v1,name=va2aregistry.kb.io,sideEffects=None,admissionReviewVersions=v1

// A2ARegistryValidator validates A2ARegistry resources.
type A2ARegistryValidator struct{}

var _ admission.Validator[*A2ARegistry] = &A2ARegistryValidator{}

// ValidateCreate validates the A2ARegistry on creation.
func (v *A2ARegistryValidator) ValidateCreate(ctx context.Context, obj *A2ARegistry) (admission.Warnings, error) {
	a2aregistrylog.Info("validate create", "name", obj.Name)
	return nil, validateA2ARegistry(obj)
}

// ValidateUpdate validates the A2ARegistry on update.
func (v *A2ARegistryValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *A2ARegistry) (admission.Warnings, error) {
	a2aregistrylog.Info("validate update", "name", newObj.Name)
	return nil, validateA2ARegistry(newObj)
}

// ValidateDelete validates the A2ARegistry on deletion.
func (v *A2ARegistryValidator) ValidateDelete(ctx context.Context, obj *A2ARegistry) (admission.Warnings, error) {
	a2aregistrylog.Info("validate delete", "name", obj.Name)
	return nil, nil
}

func validateA2ARegistry(r *A2ARegistry) error {
	var errs []string

	// Validate discovery scope
	if r.Spec.Discovery.Scope != "" {
		scope := strings.ToLower(r.Spec.Discovery.Scope)
		if scope != "cluster" && scope != "namespace" {
			errs = append(errs, fmt.Sprintf("spec.discovery.scope must be 'Cluster' or 'Namespace', got '%s'", r.Spec.Discovery.Scope))
		}
		if scope == "namespace" && len(r.Spec.Discovery.Namespaces) == 0 {
			errs = append(errs, "spec.discovery.namespaces must not be empty when scope is 'Namespace'")
		}
	}

	// Validate API server port
	if r.Spec.APIServer.Port != 0 {
		if r.Spec.APIServer.Port < 1 || r.Spec.APIServer.Port > 65535 {
			errs = append(errs, fmt.Sprintf("spec.apiServer.port must be between 1 and 65535, got %d", r.Spec.APIServer.Port))
		}
	}

	// Validate health check defaults
	if r.Spec.HealthCheck.IntervalSeconds > 0 && r.Spec.HealthCheck.IntervalSeconds < 10 {
		errs = append(errs, "spec.healthCheck.intervalSeconds must be at least 10")
	}
	if r.Spec.HealthCheck.TimeoutSeconds > 0 &&
		r.Spec.HealthCheck.IntervalSeconds > 0 &&
		r.Spec.HealthCheck.TimeoutSeconds > r.Spec.HealthCheck.IntervalSeconds {
		errs = append(errs, "spec.healthCheck.timeoutSeconds must not exceed intervalSeconds")
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// +kubebuilder:webhook:path=/mutate-a2a-io-v1-a2aregistry,mutating=true,failurePolicy=fail,groups=a2a.io,resources=a2aregistries,verbs=create;update,versions=v1,name=ma2aregistry.kb.io,sideEffects=None,admissionReviewVersions=v1

// A2ARegistryDefaulter sets default values on A2ARegistry resources.
type A2ARegistryDefaulter struct{}

var _ admission.Defaulter[*A2ARegistry] = &A2ARegistryDefaulter{}

// Default sets default values on the A2ARegistry.
func (d *A2ARegistryDefaulter) Default(ctx context.Context, obj *A2ARegistry) error {
	a2aregistrylog.Info("default", "name", obj.Name)

	if obj.Spec.Discovery.Scope == "" {
		obj.Spec.Discovery.Scope = "Cluster"
	}
	if obj.Spec.APIServer.Port == 0 {
		obj.Spec.APIServer.Port = 8082
	}
	if obj.Spec.APIServer.BindAddress == "" {
		obj.Spec.APIServer.BindAddress = "0.0.0.0"
	}
	if obj.Spec.HealthCheck.IntervalSeconds == 0 {
		obj.Spec.HealthCheck.IntervalSeconds = 60
	}
	if obj.Spec.HealthCheck.TimeoutSeconds == 0 {
		obj.Spec.HealthCheck.TimeoutSeconds = 10
	}

	return nil
}
