package context

import (
	"context"
	"time"
)

// WithDeadlineOrCancel creates a context that is canceled either when the parent
// is canceled or when the deadline is reached, whichever comes first
func WithDeadlineOrCancel(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, deadline)
}

// WithTimeoutOrCancel creates a context that is canceled either when the parent
// is canceled or when the timeout duration elapses, whichever comes first
func WithTimeoutOrCancel(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// IsCanceled returns true if the context has been canceled
func IsCanceled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// IsTimedOut returns true if the context was canceled due to a timeout
func IsTimedOut(ctx context.Context) bool {
	return ctx.Err() == context.DeadlineExceeded
}
