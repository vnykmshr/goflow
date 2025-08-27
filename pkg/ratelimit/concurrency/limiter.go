package concurrency

import (
	"context"
	"sync"
)

// Limiter controls the number of concurrent operations that can happen
// at any given time. It acts as a semaphore with additional features
// like context support and state inspection.
type Limiter interface {
	// Acquire attempts to acquire a permit for one operation.
	// It returns true if a permit was available, false otherwise.
	// This method does not block.
	Acquire() bool

	// AcquireN attempts to acquire n permits for operations.
	// It returns true if all permits were available, false otherwise.
	// This method does not block.
	AcquireN(n int) bool

	// Wait blocks until a permit is available for one operation.
	// It returns an error if the context is canceled or deadline exceeded.
	Wait(ctx context.Context) error

	// WaitN blocks until n permits are available for operations.
	// It returns an error if the context is canceled or deadline exceeded.
	WaitN(ctx context.Context, n int) error

	// Release releases one permit back to the limiter.
	// It panics if more permits are released than were acquired.
	Release()

	// ReleaseN releases n permits back to the limiter.
	// It panics if more permits are released than were acquired.
	ReleaseN(n int)

	// SetCapacity changes the maximum number of concurrent operations allowed.
	// If the new capacity is less than current usage, it will take effect
	// as operations complete and permits are released.
	SetCapacity(capacity int)

	// Capacity returns the maximum number of concurrent operations allowed.
	Capacity() int

	// Available returns the number of permits currently available.
	Available() int

	// InUse returns the number of permits currently in use.
	InUse() int
}

// Config holds configuration options for creating a new concurrency Limiter.
type Config struct {
	// Capacity is the maximum number of concurrent operations allowed.
	Capacity int

	// InitialAvailable is the initial number of available permits.
	// If negative or greater than Capacity, defaults to Capacity.
	InitialAvailable int
}

// concurrencyLimiter implements the Limiter interface using a semaphore approach.
type concurrencyLimiter struct {
	mu        sync.Mutex
	capacity  int
	available int
	inUse     int // Track actual permits in use
	waiters   []waiter
}

// waiter represents a goroutine waiting for permits
type waiter struct {
	n      int             // number of permits needed
	ready  chan struct{}   // signaled when permits are available
	cancel <-chan struct{} // context cancellation channel
}

// New creates a new concurrency limiter with the specified capacity.
// All permits start available.
func New(capacity int) Limiter {
	return NewWithConfig(Config{
		Capacity:         capacity,
		InitialAvailable: -1, // Use capacity as default
	})
}

// NewWithConfig creates a new concurrency limiter with the specified configuration.
func NewWithConfig(config Config) Limiter {
	if config.Capacity <= 0 {
		panic("capacity must be positive")
	}

	initialAvailable := config.InitialAvailable
	if config.InitialAvailable < 0 || config.InitialAvailable > config.Capacity {
		initialAvailable = config.Capacity
	}

	return &concurrencyLimiter{
		capacity:  config.Capacity,
		available: initialAvailable,
		inUse:     config.Capacity - initialAvailable,
		waiters:   make([]waiter, 0),
	}
}
