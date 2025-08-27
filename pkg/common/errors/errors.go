package errors

import "errors"

// Common error types used across the goflow library

var (
	// ErrClosed indicates that an operation was attempted on a closed resource
	ErrClosed = errors.New("resource is closed")

	// ErrTimeout indicates that an operation timed out
	ErrTimeout = errors.New("operation timed out")

	// ErrCapacityExceeded indicates that a capacity limit was exceeded
	ErrCapacityExceeded = errors.New("capacity exceeded")

	// ErrInvalidConfiguration indicates invalid configuration parameters
	ErrInvalidConfiguration = errors.New("invalid configuration")

	// ErrRateLimited indicates that a request was rate limited
	ErrRateLimited = errors.New("rate limited")
)

// IsRetryable returns true if the error indicates a condition that might
// be resolved by retrying the operation
func IsRetryable(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrRateLimited)
}

// IsTemporary returns true if the error indicates a temporary condition
func IsTemporary(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrCapacityExceeded)
}