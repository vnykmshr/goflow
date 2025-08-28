package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Config holds configuration for metrics collection.
type Config struct {
	// Enabled controls whether metrics collection is active.
	Enabled bool

	// Registry is the Prometheus registry to use. If nil, uses prometheus.DefaultRegisterer.
	Registry prometheus.Registerer

	// Namespace overrides the default "goflow" namespace for metrics.
	Namespace string

	// Labels are additional labels to add to all metrics.
	Labels prometheus.Labels
}

// DefaultConfig returns a default metrics configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:   true,
		Registry:  prometheus.DefaultRegisterer,
		Namespace: "goflow",
		Labels:    nil,
	}
}

// Instrumentable is an interface for components that can be instrumented with metrics.
type Instrumentable interface {
	// EnableMetrics enables metrics collection for this component.
	EnableMetrics(config Config) error
	
	// DisableMetrics disables metrics collection for this component.
	DisableMetrics()
	
	// MetricsEnabled returns true if metrics are currently enabled.
	MetricsEnabled() bool
}