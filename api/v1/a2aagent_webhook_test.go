package v1

import (
	"context"
	"testing"
)

func TestValidateA2AAgent_EmptyName(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "",
			URL:  "http://example.com",
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestValidateA2AAgent_EmptyURL(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "",
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestValidateA2AAgent_InvalidURLScheme(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "ftp://example.com",
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for invalid URL scheme")
	}
}

func TestValidateA2AAgent_DuplicateSkillID(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
			Skills: []A2AAgentSkillSpec{
				{ID: "greeting", Name: "Greeting"},
				{ID: "greeting", Name: "Another Greeting"},
			},
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for duplicate skill ID")
	}
}

func TestValidateA2AAgent_EmptySkillID(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
			Skills: []A2AAgentSkillSpec{
				{ID: "", Name: "Nameless"},
			},
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for empty skill ID")
	}
}

func TestValidateA2AAgent_EmptySkillName(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
			Skills: []A2AAgentSkillSpec{
				{ID: "skill1", Name: ""},
			},
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for empty skill name")
	}
}

func TestValidateA2AAgent_HealthCheckIntervalTooSmall(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
			HealthCheck: &HealthCheckConfig{
				IntervalSeconds: 5,
			},
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for interval < 10")
	}
}

func TestValidateA2AAgent_TimeoutExceedsInterval(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
			HealthCheck: &HealthCheckConfig{
				IntervalSeconds: 10,
				TimeoutSeconds:  15,
			},
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for timeout > interval")
	}
}

func TestValidateA2AAgent_InvalidProtocolVersion(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name:            "test-agent",
			URL:             "http://example.com",
			ProtocolVersion: "1",
		},
	}
	err := validateA2AAgent(agent)
	if err == nil {
		t.Fatal("expected error for invalid protocol version")
	}
}

func TestValidateA2AAgent_ValidAgent(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
			Skills: []A2AAgentSkillSpec{
				{ID: "greeting", Name: "Greeting"},
			},
			ProtocolVersion: "1.0",
		},
	}
	err := validateA2AAgent(agent)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateA2AAgent_ValidHTTPS(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "https://example.com:8443/agent",
		},
	}
	err := validateA2AAgent(agent)
	if err != nil {
		t.Errorf("expected no error for HTTPS URL, got: %v", err)
	}
}

func TestA2AAgentDefaulter_SetsProtocolVersion(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
		},
	}
	d := &A2AAgentDefaulter{}
	if err := d.Default(context.Background(), agent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Spec.ProtocolVersion != "1.0" {
		t.Errorf("expected ProtocolVersion '1.0', got '%s'", agent.Spec.ProtocolVersion)
	}
}

func TestA2AAgentDefaulter_PreservesExplicitProtocolVersion(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name:            "test-agent",
			URL:             "http://example.com",
			ProtocolVersion: "2.0",
		},
	}
	d := &A2AAgentDefaulter{}
	if err := d.Default(context.Background(), agent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Spec.ProtocolVersion != "2.0" {
		t.Errorf("expected ProtocolVersion '2.0', got '%s'", agent.Spec.ProtocolVersion)
	}
}

func TestA2AAgentDefaulter_SetsDefaultHealthCheck(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
		},
	}
	d := &A2AAgentDefaulter{}
	if err := d.Default(context.Background(), agent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Spec.HealthCheck == nil {
		t.Fatal("expected HealthCheck to be set")
	}
	if agent.Spec.HealthCheck.IntervalSeconds != 60 {
		t.Errorf("expected IntervalSeconds 60, got %d", agent.Spec.HealthCheck.IntervalSeconds)
	}
	if agent.Spec.HealthCheck.TimeoutSeconds != 10 {
		t.Errorf("expected TimeoutSeconds 10, got %d", agent.Spec.HealthCheck.TimeoutSeconds)
	}
	if agent.Spec.HealthCheck.FailureThreshold != 3 {
		t.Errorf("expected FailureThreshold 3, got %d", agent.Spec.HealthCheck.FailureThreshold)
	}
}

func TestA2AAgentDefaulter_FillsPartialHealthCheck(t *testing.T) {
	agent := &A2AAgent{
		Spec: A2AAgentSpec{
			Name: "test-agent",
			URL:  "http://example.com",
			HealthCheck: &HealthCheckConfig{
				IntervalSeconds: 120,
				// TimeoutSeconds and FailureThreshold left at zero
			},
		},
	}
	d := &A2AAgentDefaulter{}
	if err := d.Default(context.Background(), agent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Spec.HealthCheck.IntervalSeconds != 120 {
		t.Errorf("expected IntervalSeconds 120, got %d", agent.Spec.HealthCheck.IntervalSeconds)
	}
	if agent.Spec.HealthCheck.TimeoutSeconds != 10 {
		t.Errorf("expected default TimeoutSeconds 10, got %d", agent.Spec.HealthCheck.TimeoutSeconds)
	}
	if agent.Spec.HealthCheck.FailureThreshold != 3 {
		t.Errorf("expected default FailureThreshold 3, got %d", agent.Spec.HealthCheck.FailureThreshold)
	}
}
