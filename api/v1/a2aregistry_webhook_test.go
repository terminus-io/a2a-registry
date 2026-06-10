package v1

import (
	"context"
	"testing"
)

func TestValidateA2ARegistry_InvalidScope(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			Discovery: DiscoveryConfig{Scope: "InvalidValue"},
		},
	}
	err := validateA2ARegistry(reg)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}

func TestValidateA2ARegistry_NamespaceScopeWithoutNamespaces(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			Discovery: DiscoveryConfig{Scope: "Namespace"},
		},
	}
	err := validateA2ARegistry(reg)
	if err == nil {
		t.Fatal("expected error for namespace scope without namespaces")
	}
}

func TestValidateA2ARegistry_ValidClusterScope(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			Discovery: DiscoveryConfig{Scope: "Cluster"},
		},
	}
	err := validateA2ARegistry(reg)
	if err != nil {
		t.Errorf("expected no error for Cluster scope, got: %v", err)
	}
}

func TestValidateA2ARegistry_ValidNamespaceScope(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			Discovery: DiscoveryConfig{
				Scope:      "Namespace",
				Namespaces: []string{"ns1", "ns2"},
			},
		},
	}
	err := validateA2ARegistry(reg)
	if err != nil {
		t.Errorf("expected no error for valid namespace scope, got: %v", err)
	}
}

func TestValidateA2ARegistry_InvalidPort(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			APIServer: APIServerConfig{Port: 99999},
		},
	}
	err := validateA2ARegistry(reg)
	if err == nil {
		t.Fatal("expected error for port > 65535")
	}
}

func TestValidateA2ARegistry_ZeroPortPasses(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			APIServer: APIServerConfig{Port: 0},
		},
	}
	err := validateA2ARegistry(reg)
	if err != nil {
		t.Errorf("expected no error for zero port (default), got: %v", err)
	}
}

func TestValidateA2ARegistry_ValidPort(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			APIServer: APIServerConfig{Port: 8443},
		},
	}
	err := validateA2ARegistry(reg)
	if err != nil {
		t.Errorf("expected no error for valid port, got: %v", err)
	}
}

func TestValidateA2ARegistry_HealthCheckIntervalMin(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			HealthCheck: HealthCheckDefaults{IntervalSeconds: 1},
		},
	}
	err := validateA2ARegistry(reg)
	if err != nil {
		t.Errorf("expected no error for interval = 1, got: %v", err)
	}
}

func TestValidateA2ARegistry_TimeoutExceedsInterval(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			HealthCheck: HealthCheckDefaults{
				IntervalSeconds: 10,
				TimeoutSeconds:  15,
			},
		},
	}
	err := validateA2ARegistry(reg)
	if err == nil {
		t.Fatal("expected error for timeout > interval")
	}
}

func TestA2ARegistryDefaulter_SetsDefaults(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{},
	}
	d := &A2ARegistryDefaulter{}
	if err := d.Default(context.Background(), reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Spec.Discovery.Scope != "Cluster" {
		t.Errorf("expected default scope 'Cluster', got '%s'", reg.Spec.Discovery.Scope)
	}
	if reg.Spec.APIServer.Port != 8082 {
		t.Errorf("expected default port 8082, got %d", reg.Spec.APIServer.Port)
	}
	if reg.Spec.APIServer.BindAddress != "0.0.0.0" {
		t.Errorf("expected default bindAddress '0.0.0.0', got '%s'", reg.Spec.APIServer.BindAddress)
	}
	if reg.Spec.HealthCheck.IntervalSeconds != 60 {
		t.Errorf("expected default interval 60, got %d", reg.Spec.HealthCheck.IntervalSeconds)
	}
	if reg.Spec.HealthCheck.TimeoutSeconds != 10 {
		t.Errorf("expected default timeout 10, got %d", reg.Spec.HealthCheck.TimeoutSeconds)
	}
}

func TestA2ARegistryDefaulter_PreservesExplicitValues(t *testing.T) {
	reg := &A2ARegistry{
		Spec: A2ARegistrySpec{
			Discovery: DiscoveryConfig{Scope: "Namespace"},
			APIServer: APIServerConfig{Port: 9090, BindAddress: "127.0.0.1"},
			HealthCheck: HealthCheckDefaults{
				IntervalSeconds: 120,
				TimeoutSeconds:  15,
			},
		},
	}
	d := &A2ARegistryDefaulter{}
	if err := d.Default(context.Background(), reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Spec.Discovery.Scope != "Namespace" {
		t.Errorf("expected scope 'Namespace', got '%s'", reg.Spec.Discovery.Scope)
	}
	if reg.Spec.APIServer.Port != 9090 {
		t.Errorf("expected port 9090, got %d", reg.Spec.APIServer.Port)
	}
	if reg.Spec.APIServer.BindAddress != "127.0.0.1" {
		t.Errorf("expected bindAddress '127.0.0.1', got '%s'", reg.Spec.APIServer.BindAddress)
	}
}
