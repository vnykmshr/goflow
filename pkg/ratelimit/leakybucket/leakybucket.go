package leakybucket

import (
	"context"
	"math"
	"time"

	"github.com/vnykmshr/goflow/pkg/ratelimit/bucket"
)

// Allow reports whether an event may happen now.
func (lb *leakyBucket) Allow() bool {
	return lb.AllowN(1)
}

// AllowN reports whether n events may happen now.
func (lb *leakyBucket) AllowN(n int) bool {
	return lb.reserveN(lb.clock.Now(), n, 0).ok
}

// Wait blocks until an event can happen.
func (lb *leakyBucket) Wait(ctx context.Context) error {
	return lb.WaitN(ctx, 1)
}

// WaitN blocks until n events can happen.
func (lb *leakyBucket) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	// Check if context is already canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	now := lb.clock.Now()
	r := lb.reserveN(now, n, math.MaxInt64)
	if !r.OK() {
		return context.DeadlineExceeded // Rate limited
	}

	delay := r.DelayFrom(now)
	if delay <= 0 {
		// Check context one more time before returning success
		select {
		case <-ctx.Done():
			r.Cancel()
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Successfully waited for the required time
		return nil
	case <-ctx.Done():
		r.Cancel() // Cancel the reservation since we're not using it
		return ctx.Err()
	}
}

// Reserve returns a reservation for one event.
func (lb *leakyBucket) Reserve() *Reservation {
	return lb.ReserveN(1)
}

// ReserveN returns a reservation for n events.
func (lb *leakyBucket) ReserveN(n int) *Reservation {
	return lb.reserveN(lb.clock.Now(), n, math.MaxInt64)
}

// SetLeakRate changes the leak rate.
func (lb *leakyBucket) SetLeakRate(newRate bucket.Limit) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := lb.clock.Now()
	lb.leak(now)
	lb.leakRate = newRate
}

// SetCapacity changes the bucket capacity.
func (lb *leakyBucket) SetCapacity(newCapacity int) {
	if newCapacity <= 0 {
		panic("capacity must be positive")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := lb.clock.Now()
	lb.leak(now)
	lb.capacity = newCapacity

	// Reduce level to new capacity if necessary
	if lb.level > float64(newCapacity) {
		lb.level = float64(newCapacity)
	}
}

// LeakRate returns the current leak rate.
func (lb *leakyBucket) LeakRate() bucket.Limit {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.leakRate
}

// Capacity returns the current bucket capacity.
func (lb *leakyBucket) Capacity() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.capacity
}

// Level returns the current fill level of the bucket.
func (lb *leakyBucket) Level() float64 {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := lb.clock.Now()
	lb.leak(now)
	return lb.level
}

// Available returns the available space in the bucket.
func (lb *leakyBucket) Available() float64 {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := lb.clock.Now()
	lb.leak(now)
	return float64(lb.capacity) - lb.level
}

// reserveN reserves n events at the given time with a maximum wait duration.
func (lb *leakyBucket) reserveN(now time.Time, n int, maxWait time.Duration) *Reservation {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if n <= 0 {
		return &Reservation{
			ok:        true,
			timeToAct: now,
			tokens:    0,
			limit:     lb.leakRate,
			lim:       lb,
		}
	}

	if lb.leakRate == bucket.Inf {
		// Infinite rate means immediate processing
		return &Reservation{
			ok:        true,
			timeToAct: now,
			tokens:    n,
			limit:     lb.leakRate,
			lim:       lb,
		}
	}

	// Process any leaks first
	lb.leak(now)

	// Check if we can accommodate the requests immediately
	available := float64(lb.capacity) - lb.level
	if float64(n) <= available {
		lb.level += float64(n)
		return &Reservation{
			ok:        true,
			timeToAct: now,
			tokens:    n,
			limit:     lb.leakRate,
			lim:       lb,
		}
	}

	// Calculate wait time for requests to leak out
	overflow := float64(n) - available
	waitTime := time.Duration(0)

	if lb.leakRate > 0 {
		waitTime = time.Duration(float64(time.Second) * overflow / float64(lb.leakRate))
	} else {
		// Zero leak rate means we can never accommodate overflow
		return &Reservation{
			ok:     false,
			tokens: n,
			limit:  lb.leakRate,
			lim:    lb,
		}
	}

	if waitTime > maxWait {
		return &Reservation{
			ok:     false,
			tokens: n,
			limit:  lb.leakRate,
			lim:    lb,
		}
	}

	// Reserve the space (level can exceed capacity temporarily)
	lb.level += float64(n)
	timeToAct := now.Add(waitTime)

	return &Reservation{
		ok:        true,
		timeToAct: timeToAct,
		tokens:    n,
		limit:     lb.leakRate,
		lim:       lb,
	}
}

// leak processes requests that have leaked out of the bucket since the last leak.
func (lb *leakyBucket) leak(now time.Time) {
	if lb.leakRate == bucket.Inf {
		// Infinite rate means bucket is always empty
		lb.level = 0
		lb.lastLeak = now
		return
	}

	if lb.leakRate == 0 {
		// Zero rate means no leaking
		lb.lastLeak = now
		return
	}

	elapsed := now.Sub(lb.lastLeak)
	if elapsed <= 0 {
		return
	}

	// Calculate how much should leak based on elapsed time
	leakAmount := elapsed.Seconds() * float64(lb.leakRate)
	lb.level = math.Max(0, lb.level-leakAmount)
	lb.lastLeak = now
}

// cancelReservation removes reserved level from a canceled reservation.
func (lb *leakyBucket) cancelReservation(r *Reservation) {
	if !r.ok {
		return
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := lb.clock.Now()
	lb.leak(now)

	// Remove the reserved level
	lb.level = math.Max(0, lb.level-float64(r.tokens))
}
