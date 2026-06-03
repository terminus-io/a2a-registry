package v1

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// a2aagentlog is for logging in this package.
var a2aagentlog = ctrl.Log.WithName("a2aagent-webhook")

// SetupA2AAgentWebhook registers the webhook with the manager.
func SetupA2AAgentWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &A2AAgent{}).
		WithValidator(&A2AAgentValidator{}).
		WithDefaulter(&A2AAgentDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-a2a-io-v1-a2aagent,mutating=false,failurePolicy=fail,groups=a2a.io,resources=a2aagents,verbs=create;update,versions=v1,name=va2aagent.kb.io,sideEffects=None,admissionReviewVersions=v1

// A2AAgentValidator validates A2AAgent resources.
type A2AAgentValidator struct{}

var _ admission.Validator[*A2AAgent] = &A2AAgentValidator{}

// ValidateCreate validates the A2AAgent on creation.
func (v *A2AAgentValidator) ValidateCreate(ctx context.Context, obj *A2AAgent) (admission.Warnings, error) {
	a2aagentlog.Info("validate create", "name", obj.Name)
	return nil, validateA2AAgent(obj)
}

// ValidateUpdate validates the A2AAgent on update.
func (v *A2AAgentValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *A2AAgent) (admission.Warnings, error) {
	a2aagentlog.Info("validate update", "name", newObj.Name)
	return nil, validateA2AAgent(newObj)
}

// ValidateDelete validates the A2AAgent on deletion.
func (v *A2AAgentValidator) ValidateDelete(ctx context.Context, obj *A2AAgent) (admission.Warnings, error) {
	a2aagentlog.Info("validate delete", "name", obj.Name)
	return nil, nil
}

// validateA2AAgent performs the actual validation logic.
func validateA2AAgent(a *A2AAgent) error {
	var errs []string

	// Name is required
	if a.Spec.Name == "" {
		errs = append(errs, "spec.name must not be empty")
	}

	// URL is required
	if a.Spec.URL == "" {
		errs = append(errs, "spec.url must not be empty")
	} else {
		// Validate URL format
		parsedURL, err := url.Parse(a.Spec.URL)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			errs = append(errs, "spec.url must be a valid HTTP or HTTPS URL")
		}
	}

	// Skill ID uniqueness
	skillIDs := make(map[string]bool)
	for _, skill := range a.Spec.Skills {
		if skill.ID == "" {
			errs = append(errs, "spec.skills[].id must not be empty")
			continue
		}
		if skillIDs[skill.ID] {
			errs = append(errs, fmt.Sprintf("duplicate skill ID: %s", skill.ID))
		}
		skillIDs[skill.ID] = true

		if skill.Name == "" {
			errs = append(errs, fmt.Sprintf("spec.skills[%s].name must not be empty", skill.ID))
		}
	}

	// Health check sanity
	if a.Spec.HealthCheck != nil {
		if a.Spec.HealthCheck.IntervalSeconds > 0 && a.Spec.HealthCheck.IntervalSeconds < 10 {
			errs = append(errs, "spec.healthCheck.intervalSeconds must be at least 10")
		}
		if a.Spec.HealthCheck.TimeoutSeconds > 0 &&
			a.Spec.HealthCheck.IntervalSeconds > 0 &&
			a.Spec.HealthCheck.TimeoutSeconds > a.Spec.HealthCheck.IntervalSeconds {
			errs = append(errs, "spec.healthCheck.timeoutSeconds must not exceed intervalSeconds")
		}
	}

	// Protocol version format (basic semver check)
	if a.Spec.ProtocolVersion != "" {
		parts := strings.Split(a.Spec.ProtocolVersion, ".")
		if len(parts) < 2 {
			errs = append(errs, "spec.protocolVersion must be a valid version (e.g. \"1.0\")")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// +kubebuilder:webhook:path=/mutate-a2a-io-v1-a2aagent,mutating=true,failurePolicy=fail,groups=a2a.io,resources=a2aagents,verbs=create;update,versions=v1,name=ma2aagent.kb.io,sideEffects=None,admissionReviewVersions=v1

// A2AAgentDefaulter sets default values on A2AAgent resources.
type A2AAgentDefaulter struct{}

var _ admission.Defaulter[*A2AAgent] = &A2AAgentDefaulter{}

// Default sets default values on the A2AAgent.
func (d *A2AAgentDefaulter) Default(ctx context.Context, obj *A2AAgent) error {
	a2aagentlog.Info("default", "name", obj.Name)

	// Default protocol version
	if obj.Spec.ProtocolVersion == "" {
		obj.Spec.ProtocolVersion = "1.0"
	}

	// Default health check config
	if obj.Spec.HealthCheck == nil {
		obj.Spec.HealthCheck = &HealthCheckConfig{
			IntervalSeconds:  60,
			TimeoutSeconds:   10,
			FailureThreshold: 3,
		}
	} else {
		if obj.Spec.HealthCheck.IntervalSeconds == 0 {
			obj.Spec.HealthCheck.IntervalSeconds = 60
		}
		if obj.Spec.HealthCheck.TimeoutSeconds == 0 {
			obj.Spec.HealthCheck.TimeoutSeconds = 10
		}
		if obj.Spec.HealthCheck.FailureThreshold == 0 {
			obj.Spec.HealthCheck.FailureThreshold = 3
		}
	}

	return nil
}
