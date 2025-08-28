package bucket

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vnykmshr/goflow/pkg/metrics"
)

// MetricsLimiter wraps a Limiter with Prometheus metrics collection.
type MetricsLimiter struct {
	limiter  Limiter
	name     string
	registry *metrics.Registry
	enabled  bool
}

// NewWithMetrics creates a new token bucket limiter with metrics enabled.
func NewWithMetrics(rate Limit, burst int, name string) Limiter {
	// Use a separate registry for each metrics-enabled component to avoid conflicts
	registry := prometheus.NewRegistry()
	config := metrics.Config{
		Enabled:  true,
		Registry: registry,
	}

	return NewWithConfigAndMetrics(Config{
		Rate:          rate,
		Burst:         burst,
		Clock:         SystemClock{},
		InitialTokens: -1,
	}, name, config)
}

// NewWithConfigAndMetrics creates a new token bucket limiter with custom config and metrics.
func NewWithConfigAndMetrics(config Config, name string, metricsConfig metrics.Config) Limiter {
	baseLimiter := NewWithConfig(config)

	if !metricsConfig.Enabled {
		return baseLimiter
	}

	registry := metrics.DefaultRegistry
	if metricsConfig.Registry != nil {
		// Create custom registry if provided
		registry = metrics.NewRegistry(metricsConfig.Registry)
	}

	return &MetricsLimiter{
		limiter:  baseLimiter,
		name:     name,
		registry: registry,
		enabled:  true,
	}
}

// Allow reports whether an event may happen now.
func (ml *MetricsLimiter) Allow() bool {
	return ml.AllowN(1)
}

// AllowN reports whether n events may happen now.
func (ml *MetricsLimiter) AllowN(n int) bool {
	if ml.enabled {
		ml.registry.RateLimitRequests.WithLabelValues("token_bucket", ml.name).Add(float64(n))
	}

	allowed := ml.limiter.AllowN(n)

	if ml.enabled {
		if allowed {
			ml.registry.RateLimitAllowed.WithLabelValues("token_bucket", ml.name).Add(float64(n))
		} else {
			ml.registry.RateLimitDenied.WithLabelValues("token_bucket", ml.name).Add(float64(n))
		}

		// Update current token count
		ml.registry.RateLimitTokens.WithLabelValues("token_bucket", ml.name).Set(ml.limiter.Tokens())
	}

	return allowed
}

// Wait blocks until an event can happen.
func (ml *MetricsLimiter) Wait(ctx context.Context) error {
	return ml.WaitN(ctx, 1)
}

// WaitN blocks until n events can happen.
func (ml *MetricsLimiter) WaitN(ctx context.Context, n int) error {
	start := time.Now()

	if ml.enabled {
		ml.registry.RateLimitRequests.WithLabelValues("token_bucket", ml.name).Add(float64(n))
	}

	err := ml.limiter.WaitN(ctx, n)

	if ml.enabled {
		duration := time.Since(start)
		ml.registry.RateLimitWaitTime.WithLabelValues("token_bucket", ml.name).Observe(duration.Seconds())

		if err == nil {
			ml.registry.RateLimitAllowed.WithLabelValues("token_bucket", ml.name).Add(float64(n))
		} else {
			ml.registry.RateLimitDenied.WithLabelValues("token_bucket", ml.name).Add(float64(n))
		}

		// Update current token count
		ml.registry.RateLimitTokens.WithLabelValues("token_bucket", ml.name).Set(ml.limiter.Tokens())
	}

	return err
}

// Reserve returns a Reservation that indicates how long the caller must wait.
func (ml *MetricsLimiter) Reserve() *Reservation {
	return ml.ReserveN(1)
}

// ReserveN returns a Reservation that indicates how long the caller must wait.
func (ml *MetricsLimiter) ReserveN(n int) *Reservation {
	if ml.enabled {
		ml.registry.RateLimitRequests.WithLabelValues("token_bucket", ml.name).Add(float64(n))
	}

	reservation := ml.limiter.ReserveN(n)

	if ml.enabled {
		if reservation.OK() {
			ml.registry.RateLimitAllowed.WithLabelValues("token_bucket", ml.name).Add(float64(n))
		} else {
			ml.registry.RateLimitDenied.WithLabelValues("token_bucket", ml.name).Add(float64(n))
		}

		// Update current token count
		ml.registry.RateLimitTokens.WithLabelValues("token_bucket", ml.name).Set(ml.limiter.Tokens())
	}

	return reservation
}

// SetLimit changes the rate limit.
func (ml *MetricsLimiter) SetLimit(limit Limit) {
	ml.limiter.SetLimit(limit)
}

// SetBurst changes the burst size.
func (ml *MetricsLimiter) SetBurst(burst int) {
	ml.limiter.SetBurst(burst)
}

// Limit returns the current rate limit.
func (ml *MetricsLimiter) Limit() Limit {
	return ml.limiter.Limit()
}

// Burst returns the current burst size.
func (ml *MetricsLimiter) Burst() int {
	return ml.limiter.Burst()
}

// Tokens returns the number of tokens currently available.
func (ml *MetricsLimiter) Tokens() float64 {
	tokens := ml.limiter.Tokens()

	if ml.enabled {
		ml.registry.RateLimitTokens.WithLabelValues("token_bucket", ml.name).Set(tokens)
	}

	return tokens
}

// EnableMetrics enables metrics collection.
func (ml *MetricsLimiter) EnableMetrics(config metrics.Config) error {
	ml.enabled = config.Enabled

	if config.Registry != nil {
		ml.registry = metrics.NewRegistry(config.Registry)
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
