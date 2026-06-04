package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// AgentCount tracks the number of registered A2A agents by phase and health.
	AgentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "a2a_registry_agent_count",
			Help: "Number of registered A2A agents.",
		},
		[]string{"phase", "health"},
	)

	// HealthCheckDuration tracks the duration of agent health checks.
	HealthCheckDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "a2a_registry_health_check_duration_seconds",
			Help:    "Duration of agent health checks in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	// HealthCheckFailuresTotal counts the total number of failed agent health checks.
	HealthCheckFailuresTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "a2a_registry_health_check_failures_total",
			Help: "Total number of failed agent health checks.",
		},
	)

	// AgentCardFetchErrorsTotal counts errors when fetching agent cards.
	AgentCardFetchErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "a2a_registry_agent_card_fetch_errors_total",
			Help: "Total number of errors fetching agent cards from agent endpoints.",
		},
	)

	// RegistrationsTotal counts the total number of agent registrations.
	RegistrationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "a2a_registry_registrations_total",
			Help: "Total number of agent registrations.",
		},
	)

	// DeregistrationsTotal counts the total number of agent deregistrations.
	DeregistrationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "a2a_registry_deregistrations_total",
			Help: "Total number of agent deregistrations.",
		},
	)

	// APIRequestDuration tracks the duration of registry API requests.
	APIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "a2a_registry_api_request_duration_seconds",
			Help:    "Duration of registry API requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "method"},
	)
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		AgentCount,
		HealthCheckDuration,
		HealthCheckFailuresTotal,
		AgentCardFetchErrorsTotal,
		RegistrationsTotal,
		DeregistrationsTotal,
		APIRequestDuration,
	)
}
