package bucket

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/vnykmshr/goflow/pkg/common/errors"
)

// Limit represents the maximum frequency of events per unit time.
// A zero Limit allows no events. Use Inf for unlimited rates.
type Limit float64

// Inf is the infinite rate limit; it allows all events.
var Inf = Limit(math.Inf(1))

// Every converts a minimum time interval between events to a Limit.
func Every(interval time.Duration) Limit {
	if interval <= 0 {
		return Inf
	}
	return Limit(time.Second) / Limit(interval)
}

// Limiter controls how frequently events are allowed to happen using
// a token bucket algorithm. It supports burst traffic by maintaining
// a reservoir of tokens that can be consumed quickly.
type Limiter interface {
	// Allow reports whether an event may happen now. It does not block.
	Allow() bool

	// AllowN reports whether n events may happen now. It does not block.
	AllowN(n int) bool

	// Wait blocks until an event can happen. It returns an error
	// if the context is canceled or the deadline is exceeded.
	Wait(ctx context.Context) error

	// WaitN blocks until n events can happen. It returns an error
	// if the context is canceled or the deadline is exceeded.
	WaitN(ctx context.Context, n int) error

	// Reserve returns a Reservation that indicates how long the caller
	// must wait before n events can happen.
	Reserve() *Reservation

	// ReserveN returns a Reservation that indicates how long the caller
	// must wait before n events can happen.
	ReserveN(n int) *Reservation

	// SetLimit changes the rate limit. It preserves the current burst size.
	SetLimit(limit Limit)

	// SetBurst changes the burst size. It preserves the current rate limit.
	SetBurst(burst int)

	// Limit returns the current rate limit.
	Limit() Limit

	// Burst returns the current burst size.
	Burst() int

	// Tokens returns the number of tokens currently available.
	Tokens() float64
}

// Reservation holds information about an event that should happen
// in the future. It can be used to cancel the reservation or
// check when the event should occur.
type Reservation struct {
	ok        bool
	timeToAct time.Time
	tokens    int
	limit     Limit
	lim       *tokenBucket
}

// OK returns whether the reservation is valid.
func (r *Reservation) OK() bool {
	return r.ok
}

// Delay returns the time until the reservation should act.
// If the reservation is not OK, Delay returns zero.
func (r *Reservation) Delay() time.Duration {
	if !r.ok {
		return 0
	}
	delay := time.Until(r.timeToAct)
	if delay < 0 {
		return 0
	}
	return delay
}

// DelayFrom returns the time until the reservation should act,
// measured from the given time.
func (r *Reservation) DelayFrom(now time.Time) time.Duration {
	if !r.ok {
		return 0
	}
	delay := r.timeToAct.Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}

// Cancel cancels the reservation. This restores the tokens
// to the limiter if the reservation was valid.
func (r *Reservation) Cancel() {
	if !r.ok {
		return
	}
	r.lim.cancelReservation(r)
}

// Clock provides the current time. It can be mocked for testing.
type Clock interface {
	Now() time.Time
}

// SystemClock implements Clock using the system time.
type SystemClock struct{}

// Now returns the current system time.
func (SystemClock) Now() time.Time {
	return time.Now()
}

// Config holds configuration options for creating a new Limiter.
type Config struct {
	// Rate is the number of tokens added per second.
	Rate Limit

	// Burst is the maximum number of tokens that can be stored.
	Burst int

	// Clock provides the current time. If nil, SystemClock is used.
	Clock Clock

	// InitialTokens is the number of tokens to start with.
	// If negative, starts with full capacity.
	InitialTokens int
}

// tokenBucket implements the Limiter interface using a token bucket algorithm.
type tokenBucket struct {
	mu         sync.Mutex
	limit      Limit
	burst      int
	tokens     float64
	lastUpdate time.Time
	clock      Clock
}

// NewSafe creates a new rate limiter with validation that returns an error instead of panicking.
// This is the recommended way to create rate limiters for production use.
func NewSafe(rate Limit, burst int) (Limiter, error) {
	return NewWithConfigSafe(Config{
		Rate:          rate,
		Burst:         burst,
		Clock:         SystemClock{},
		InitialTokens: -1, // Start with full capacity
	})
}

// NewWithConfigSafe creates a new rate limiter with validation that returns an error instead of panicking.
// This is the recommended way to create rate limiters for production use.
func NewWithConfigSafe(config Config) (Limiter, error) {
	if config.Rate < 0 {
		return nil, errors.NewValidationError("bucket", "rate", config.Rate, "rate cannot be negative").
			WithHint("use 0 for no rate limit or a positive value")
	}
	if config.Burst <= 0 {
		return nil, errors.NewValidationError("bucket", "burst", config.Burst, "burst must be positive").
			WithHint("burst determines how many tokens can be consumed instantly")
	}
	if config.Clock == nil {
		config.Clock = SystemClock{}
	}

	initialTokens := float64(config.InitialTokens)
	if config.InitialTokens < 0 {
		initialTokens = float64(config.Burst)
	}

	return &tokenBucket{
		limit:      config.Rate,
		burst:      config.Burst,
		tokens:     initialTokens,
		lastUpdate: config.Clock.Now(),
		clock:      config.Clock,
	}, nil
}

