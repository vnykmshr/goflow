package bucket

import (
	"context"
	"math"
	"time"
)

// Allow reports whether an event may happen now.
func (tb *tokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN reports whether n events may happen now.
func (tb *tokenBucket) AllowN(n int) bool {
	return tb.reserveN(tb.clock.Now(), n, 0).ok
}

// Wait blocks until an event can happen.
func (tb *tokenBucket) Wait(ctx context.Context) error {
	return tb.WaitN(ctx, 1)
}

// WaitN blocks until n events can happen.
func (tb *tokenBucket) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	// Check if context is already canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	now := tb.clock.Now()
	r := tb.reserveN(now, n, math.MaxInt64)
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
func (tb *tokenBucket) Reserve() *Reservation {
	return tb.ReserveN(1)
}

// ReserveN returns a reservation for n events.
func (tb *tokenBucket) ReserveN(n int) *Reservation {
	return tb.reserveN(tb.clock.Now(), n, math.MaxInt64)
}

// SetLimit changes the rate limit.
func (tb *tokenBucket) SetLimit(newLimit Limit) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.clock.Now()
	tb.updateTokens(now)
	tb.limit = newLimit
}

// SetBurst changes the burst size.
func (tb *tokenBucket) SetBurst(newBurst int) {
	if newBurst <= 0 {
		panic("burst must be positive")
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.clock.Now()
	tb.updateTokens(now)
	tb.burst = newBurst

	// Limit tokens to new burst capacity
	if tb.tokens > float64(newBurst) {
		tb.tokens = float64(newBurst)
	}
}

// Limit returns the current rate limit.
func (tb *tokenBucket) Limit() Limit {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.limit
}

// Burst returns the current burst size.
func (tb *tokenBucket) Burst() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.burst
}

// Tokens returns the number of tokens currently available.
func (tb *tokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.clock.Now()
	tb.updateTokens(now)
	return tb.tokens
}

// reserveN reserves n tokens at the given time with a maximum wait duration.
func (tb *tokenBucket) reserveN(now time.Time, n int, maxWait time.Duration) *Reservation {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if n <= 0 {
		return &Reservation{
			ok:        true,
			timeToAct: now,
			tokens:    0,
			limit:     tb.limit,
			lim:       tb,
		}
	}

	if tb.limit == Inf {
		return &Reservation{
			ok:        true,
			timeToAct: now,
			tokens:    n,
			limit:     tb.limit,
			lim:       tb,
		}
	}

	if tb.limit == 0 {
		// Zero rate: can only use initial tokens
		tb.updateTokens(now)
		if tb.tokens >= float64(n) {
			tb.tokens -= float64(n)
			return &Reservation{
				ok:        true,
				timeToAct: now,
				tokens:    n,
				limit:     tb.limit,
				lim:       tb,
			}
		}
		// Can't fulfill request with zero rate
		return &Reservation{
			ok:     false,
			tokens: n,
			limit:  tb.limit,
			lim:    tb,
		}
	}

	tb.updateTokens(now)

	// Check if we have enough tokens available immediately
	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return &Reservation{
			ok:        true,
			timeToAct: now,
			tokens:    n,
			limit:     tb.limit,
			lim:       tb,
		}
	}

	// Calculate how long we need to wait
	tokensNeeded := float64(n) - tb.tokens
	waitTime := time.Duration(float64(time.Second) * tokensNeeded / float64(tb.limit))

	if waitTime > maxWait {
		return &Reservation{
			ok:     false,
			tokens: n,
			limit:  tb.limit,
			lim:    tb,
		}
	}

	// Reserve the tokens and update the state
	tb.tokens -= float64(n) // Can go negative
	timeToAct := now.Add(waitTime)

	return &Reservation{
		ok:        true,
		timeToAct: timeToAct,
		tokens:    n,
		limit:     tb.limit,
		lim:       tb,
	}
}

// updateTokens adds tokens based on the time elapsed since the last update.
func (tb *tokenBucket) updateTokens(now time.Time) {
	if tb.limit == Inf {
		tb.tokens = float64(tb.burst)
		tb.lastUpdate = now
		return
	}

	if tb.limit == 0 {
		// Zero rate means no refill
		tb.lastUpdate = now
		return
	}

	elapsed := now.Sub(tb.lastUpdate)
	if elapsed <= 0 {
		return
	}

	// Add tokens based on elapsed time
	tokensToAdd := elapsed.Seconds() * float64(tb.limit)
	tb.tokens = math.Min(tb.tokens+tokensToAdd, float64(tb.burst))
	tb.lastUpdate = now
}

// cancelReservation restores tokens from a canceled reservation.
func (tb *tokenBucket) cancelReservation(r *Reservation) {
	if !r.ok {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.clock.Now()
	tb.updateTokens(now)

	// Restore the tokens
	tb.tokens = math.Min(tb.tokens+float64(r.tokens), float64(tb.burst))
}
