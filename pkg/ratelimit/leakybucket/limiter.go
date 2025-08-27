package leakybucket

import (
	"context"
	"sync"
	"time"

	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
)

// Limiter controls the rate at which events are allowed to happen using
// a leaky bucket algorithm. Unlike token bucket, it enforces a constant
// smooth output rate by "leaking" requests at a fixed interval.
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
	// must wait before an event can happen.
	Reserve() *Reservation

	// ReserveN returns a Reservation that indicates how long the caller
	// must wait before n events can happen.
	ReserveN(n int) *Reservation

	// SetLeakRate changes the leak rate. It preserves the current capacity.
	SetLeakRate(rate bucket.Limit)

	// SetCapacity changes the bucket capacity. It preserves the current leak rate.
	SetCapacity(capacity int)

	// LeakRate returns the current leak rate.
	LeakRate() bucket.Limit

	// Capacity returns the current bucket capacity.
	Capacity() int

	// Level returns the current fill level of the bucket.
	Level() float64

	// Available returns the available space in the bucket.
	Available() float64
}

// Reservation holds information about an event that should happen
// in the future. It can be used to cancel the reservation or
// check when the event should occur.
type Reservation struct {
	ok        bool
	timeToAct time.Time
	tokens    int
	limit     bucket.Limit
	lim       *leakyBucket
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

// Cancel cancels the reservation. This removes the reserved level
// from the bucket if the reservation was valid.
func (r *Reservation) Cancel() {
	if !r.ok {
		return
	}
	r.lim.cancelReservation(r)
}

// Config holds configuration options for creating a new Limiter.
type Config struct {
	// LeakRate is the rate at which requests leak from the bucket (requests per second).
	LeakRate bucket.Limit

	// Capacity is the maximum number of requests the bucket can hold.
	Capacity int

	// Clock provides the current time. If nil, SystemClock is used.
	Clock bucket.Clock

	// InitialLevel is the initial fill level of the bucket.
	// If negative, starts empty (0).
	InitialLevel int
}

// leakyBucket implements the Limiter interface using a leaky bucket algorithm.
type leakyBucket struct {
	mu       sync.Mutex
	leakRate bucket.Limit
	capacity int
	level    float64
	lastLeak time.Time
	clock    bucket.Clock
}

// New creates a new leaky bucket rate limiter with the specified leak rate and capacity.
// The limiter starts empty.
func New(leakRate bucket.Limit, capacity int) Limiter {
	return NewWithConfig(Config{
		LeakRate:     leakRate,
		Capacity:     capacity,
		Clock:        bucket.SystemClock{},
		InitialLevel: -1, // Start empty
	})
}

// NewWithConfig creates a new leaky bucket rate limiter with the specified configuration.
func NewWithConfig(config Config) Limiter {
	if config.LeakRate < 0 {
		panic("leak rate must not be negative")
	}
	if config.Capacity <= 0 {
		panic("capacity must be positive")
	}
	if config.Clock == nil {
		config.Clock = bucket.SystemClock{}
	}

	initialLevel := float64(config.InitialLevel)
	if config.InitialLevel < 0 {
		initialLevel = 0 // Start empty
	}
	// Ensure initial level doesn't exceed capacity
	if initialLevel > float64(config.Capacity) {
		initialLevel = float64(config.Capacity)
	}

	return &leakyBucket{
		leakRate: config.LeakRate,
		capacity: config.Capacity,
		level:    initialLevel,
		lastLeak: config.Clock.Now(),
		clock:    config.Clock,
	}
}
