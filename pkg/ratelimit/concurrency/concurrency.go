package concurrency

import (
	"context"
)

// Acquire attempts to acquire one permit without blocking.
func (cl *concurrencyLimiter) Acquire() bool {
	return cl.AcquireN(1)
}

// AcquireN attempts to acquire n permits without blocking.
func (cl *concurrencyLimiter) AcquireN(n int) bool {
	if n <= 0 {
		return true
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.available >= n {
		cl.available -= n
		cl.inUse += n
		return true
	}
	return false
}

// Wait blocks until one permit is available.
func (cl *concurrencyLimiter) Wait(ctx context.Context) error {
	return cl.WaitN(ctx, 1)
}

// WaitN blocks until n permits are available.
func (cl *concurrencyLimiter) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	// Check if context is already canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	cl.mu.Lock()

	// Fast path: permits available immediately
	if cl.available >= n {
		cl.available -= n
		cl.inUse += n
		cl.mu.Unlock()
		return nil
	}

	// Slow path: need to wait
	ready := make(chan struct{})
	w := waiter{
		n:      n,
		ready:  ready,
		cancel: ctx.Done(),
	}
	cl.waiters = append(cl.waiters, w)
	cl.mu.Unlock()

	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		// Remove ourselves from waiters list and return context error
		cl.removeWaiter(ready)
		return ctx.Err()
	}
}

// Release releases one permit back to the limiter.
func (cl *concurrencyLimiter) Release() {
	cl.ReleaseN(1)
}

// ReleaseN releases n permits back to the limiter.
func (cl *concurrencyLimiter) ReleaseN(n int) {
	if n <= 0 {
		return
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.inUse < n {
		panic("concurrency: released more permits than acquired")
	}

	cl.available += n
	cl.inUse -= n

	// Wake up waiters if possible
	cl.notifyWaiters()
}

// SetCapacity changes the maximum number of concurrent operations allowed.
func (cl *concurrencyLimiter) SetCapacity(newCapacity int) {
	if newCapacity <= 0 {
		panic("capacity must be positive")
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()

	oldCapacity := cl.capacity
	cl.capacity = newCapacity

	if newCapacity > oldCapacity {
		// Capacity increased, add new permits
		increase := newCapacity - oldCapacity
		cl.available += increase
		cl.notifyWaiters()
	} else if newCapacity < oldCapacity {
		// Capacity decreased, reduce available permits if possible
		reduction := oldCapacity - newCapacity
		if cl.available >= reduction {
			cl.available -= reduction
		} else {
			// Can't reduce available below 0, will naturally adjust as operations complete
			cl.available = 0
		}
	}
}

// Capacity returns the maximum number of concurrent operations allowed.
func (cl *concurrencyLimiter) Capacity() int {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.capacity
}

// Available returns the number of permits currently available.
func (cl *concurrencyLimiter) Available() int {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.available
}

// InUse returns the number of permits currently in use.
func (cl *concurrencyLimiter) InUse() int {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.inUse
}

// notifyWaiters wakes up waiting goroutines if permits are available.
// Must be called with cl.mu held.
func (cl *concurrencyLimiter) notifyWaiters() {
	var remainingWaiters []waiter

	for _, w := range cl.waiters {
		// Check if waiter was canceled
		select {
		case <-w.cancel:
			// Skip canceled waiters
			continue
		default:
		}

		// Check if we can satisfy this waiter
		if cl.available >= w.n {
			cl.available -= w.n
			cl.inUse += w.n
			close(w.ready) // Signal waiter
		} else {
			// Keep waiter for next time
			remainingWaiters = append(remainingWaiters, w)
		}
	}

	cl.waiters = remainingWaiters
}

// removeWaiter removes a waiter from the waiters list.
func (cl *concurrencyLimiter) removeWaiter(ready chan struct{}) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	var remainingWaiters []waiter
	for _, w := range cl.waiters {
		if w.ready != ready {
			remainingWaiters = append(remainingWaiters, w)
		}
	}
	cl.waiters = remainingWaiters
}
