package distributed

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedLimiter provides rate limiting across multiple application instances
// using Redis as the coordination backend.
type DistributedLimiter interface {
	// Allow reports whether an event may happen now across all instances.
	Allow(ctx context.Context) bool

	// AllowN reports whether n events may happen now across all instances.
	AllowN(ctx context.Context, n int) bool

	// Wait blocks until an event can happen.
	Wait(ctx context.Context) error

	// WaitN blocks until n events can happen.
	WaitN(ctx context.Context, n int) error

	// Reserve returns information about when to act for n events.
	Reserve(ctx context.Context, n int) (*Reservation, error)

	// SetRate changes the rate limit across all instances.
	SetRate(ctx context.Context, rate float64) error

	// SetBurst changes the burst capacity across all instances.
	SetBurst(ctx context.Context, burst int) error

	// Stats returns current limiter statistics.
	Stats(ctx context.Context) (*Stats, error)

	// Reset clears the limiter state (useful for testing).
	Reset(ctx context.Context) error

	// Close cleanly shuts down the limiter and releases resources.
	Close() error
}

// Reservation holds information about a distributed rate limiting reservation.
type Reservation struct {
	OK         bool
	Delay      time.Duration
	Tokens     int
	AllowedAt  time.Time
	InstanceID string
}

// Stats holds distributed rate limiter statistics.
type Stats struct {
	Rate            float64
	Burst           int
	Tokens          float64
	LastRefill      time.Time
	TotalRequests   int64
	AllowedRequests int64
	DeniedRequests  int64
	ActiveInstances []string
}

// Config holds configuration for distributed rate limiters.
type Config struct {
	// Redis client for coordination
	Redis redis.UniversalClient

	// Key is the Redis key prefix for this limiter
	Key string

	// Rate is the number of tokens added per second
	Rate float64

	// Burst is the maximum number of tokens that can be stored
	Burst int

	// InstanceID uniquely identifies this application instance
	InstanceID string

	// FallbackToLocal enables local rate limiting if Redis is unavailable
	FallbackToLocal bool

	// LocalLimiter is used when Redis is unavailable (if FallbackToLocal is true)
	LocalLimiter interface {
		Allow() bool
		AllowN(n int) bool
		Wait(ctx context.Context) error
		WaitN(ctx context.Context, n int) error
	}

	// RedisTimeout is the timeout for Redis operations
	RedisTimeout time.Duration

	// RefreshInterval controls how often tokens are added (defaults to 100ms)
	RefreshInterval time.Duration

	// KeyTTL is how long Redis keys should live (defaults to 1 hour)
	KeyTTL time.Duration
}

// DefaultConfig returns a default distributed rate limiter configuration.
func DefaultConfig() Config {
	return Config{
		InstanceID:      generateInstanceID(),
		FallbackToLocal: true,
		RedisTimeout:    500 * time.Millisecond,
		RefreshInterval: 100 * time.Millisecond,
		KeyTTL:          time.Hour,
	}
}

// Strategy defines different distributed rate limiting strategies.
type Strategy int

const (
	// TokenBucket uses a distributed token bucket algorithm
	TokenBucket Strategy = iota

	// SlidingWindow uses a distributed sliding window counter
	SlidingWindow

	// FixedWindow uses distributed fixed window counters
	FixedWindow
)

// NewLimiter creates a new distributed rate limiter with the specified strategy.
func NewLimiter(strategy Strategy, config Config) (DistributedLimiter, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	config = applyConfigDefaults(config)

	return createLimiterByStrategy(strategy, config)
}

// validateConfig validates the limiter configuration.
func validateConfig(config Config) error {
	if config.Redis == nil {
		return &ConfigError{"redis client is required"}
	}
	if config.Key == "" {
		return &ConfigError{"key is required"}
	}
	if config.Rate <= 0 {
		return &ConfigError{"rate must be positive"}
	}
	if config.Burst <= 0 {
		return &ConfigError{"burst must be positive"}
	}
	return nil
}

// applyConfigDefaults sets default values for unspecified config fields.
func applyConfigDefaults(config Config) Config {
	if config.InstanceID == "" {
		config.InstanceID = generateInstanceID()
	}
	if config.RedisTimeout == 0 {
		config.RedisTimeout = 500 * time.Millisecond
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 100 * time.Millisecond
	}
	if config.KeyTTL == 0 {
		config.KeyTTL = time.Hour
	}
	return config
}

// createLimiterByStrategy creates the appropriate limiter implementation.
func createLimiterByStrategy(strategy Strategy, config Config) (DistributedLimiter, error) {
	switch strategy {
	case TokenBucket:
		return newRedisTokenBucket(config)
	case SlidingWindow:
		return newRedisSlidingWindow(config)
	case FixedWindow:
		return newRedisFixedWindow(config)
	default:
		return nil, &ConfigError{"unsupported strategy"}
	}
}

// ConfigError represents a configuration error.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return "distributed rate limiter config error: " + e.Message
}

// RedisError represents a Redis operation error.
type RedisError struct {
	Operation string
	Err       error
}

func (e *RedisError) Error() string {
	return "redis error in " + e.Operation + ": " + e.Err.Error()
}

func (e *RedisError) Unwrap() error {
	return e.Err
}
