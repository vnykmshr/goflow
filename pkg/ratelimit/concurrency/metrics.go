package concurrency

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/vnykmshr/goflow/pkg/metrics"
)

// MetricsLimiter wraps a concurrency Limiter with Prometheus metrics collection.
type MetricsLimiter struct {
	limiter  Limiter
	name     string
	registry *metrics.Registry
	enabled  bool
}

// NewWithMetrics creates a new concurrency limiter with metrics enabled.
func NewWithMetrics(capacity int, name string) Limiter {
	// Use a separate registry for each metrics-enabled component to avoid conflicts
	registry := prometheus.NewRegistry()
	config := metrics.Config{
		Enabled:  true,
		Registry: registry,
	}

	return NewWithConfigAndMetrics(Config{
		Capacity:         capacity,
		InitialAvailable: -1,
	}, name, config)
}

// NewWithConfigAndMetrics creates a new concurrency limiter with custom config and metrics.
func NewWithConfigAndMetrics(config Config, name string, metricsConfig metrics.Config) Limiter {
	baseLimiter, err := NewWithConfigSafe(config)
	if err != nil {
		panic("invalid concurrency limiter configuration: " + err.Error())
	}

	if !metricsConfig.Enabled {
		return baseLimiter
	}

	registry := metrics.DefaultRegistry
	if metricsConfig.Registry != nil {
		// Create custom registry if provided
		registry = metrics.NewRegistry(metricsConfig.Registry)
	}

	ml := &MetricsLimiter{
		limiter:  baseLimiter,
		name:     name,
		registry: registry,
		enabled:  true,
	}

	// Initialize metrics
	ml.updateMetrics()

	return ml
}

// updateMetrics updates the current state metrics.
func (ml *MetricsLimiter) updateMetrics() {
	if !ml.enabled {
		return
	}

	ml.registry.ConcurrencyActive.WithLabelValues(ml.name).Set(float64(ml.limiter.InUse()))
	ml.registry.ConcurrencyWaiting.WithLabelValues(ml.name).Set(0) // Updated when waiting
}

// Acquire attempts to acquire one permit without blocking.
func (ml *MetricsLimiter) Acquire() bool {
	return ml.AcquireN(1)
}

// AcquireN attempts to acquire n permits without blocking.
func (ml *MetricsLimiter) AcquireN(n int) bool {
	acquired := ml.limiter.AcquireN(n)

	if ml.enabled {
		ml.updateMetrics()
	}

	return acquired
}

// Wait blocks until one permit is available.
func (ml *MetricsLimiter) Wait(ctx context.Context) error {
	return ml.WaitN(ctx, 1)
}

// WaitN blocks until n permits are available.
func (ml *MetricsLimiter) WaitN(ctx context.Context, n int) error {
	start := time.Now()

	if ml.enabled {
		// Increment waiting count
		ml.registry.ConcurrencyWaiting.WithLabelValues(ml.name).Inc()
	}

	err := ml.limiter.WaitN(ctx, n)

	if ml.enabled {
		// Decrement waiting count
		ml.registry.ConcurrencyWaiting.WithLabelValues(ml.name).Dec()

		// Record wait duration
		duration := time.Since(start)
		ml.registry.RateLimitWaitTime.WithLabelValues("concurrency", ml.name).Observe(duration.Seconds())

		ml.updateMetrics()
	}

	return err
}

// Release releases one permit back to the limiter.
func (ml *MetricsLimiter) Release() {
	ml.ReleaseN(1)
}

// ReleaseN releases n permits back to the limiter.
func (ml *MetricsLimiter) ReleaseN(n int) {
	ml.limiter.ReleaseN(n)

	if ml.enabled {
		ml.updateMetrics()
	}
}

// SetCapacity changes the maximum number of concurrent operations allowed.
func (ml *MetricsLimiter) SetCapacity(capacity int) {
	ml.limiter.SetCapacity(capacity)

	if ml.enabled {
		ml.updateMetrics()
	}
}

// Capacity returns the maximum number of concurrent operations allowed.
func (ml *MetricsLimiter) Capacity() int {
	return ml.limiter.Capacity()
}

// Available returns the number of permits currently available.
func (ml *MetricsLimiter) Available() int {
	return ml.limiter.Available()
}

// InUse returns the number of permits currently in use.
func (ml *MetricsLimiter) InUse() int {
	inUse := ml.limiter.InUse()

	if ml.enabled {
		ml.registry.ConcurrencyActive.WithLabelValues(ml.name).Set(float64(inUse))
	}

	return inUse
}

// EnableMetrics enables metrics collection.
func (ml *MetricsLimiter) EnableMetrics(config metrics.Config) error {
	ml.enabled = config.Enabled

	if config.Registry != nil {
		ml.registry = metrics.NewRegistry(config.Registry)
	}

	if ml.enabled {
		ml.updateMetrics()
	}

	return nil
}

// DisableMetrics disables metrics collection.
func (ml *MetricsLimiter) DisableMetrics() {
	ml.enabled = false
}

// MetricsEnabled returns true if metrics are currently enabled.
func (ml *MetricsLimiter) MetricsEnabled() bool {
	return ml.enabled
}
