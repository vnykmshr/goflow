// Package errors provides common error types and utilities for the goflow library.
package errors

import (
	"errors"
	"fmt"
)

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

// ValidationError represents a configuration validation error with actionable details
type ValidationError struct {
	Module string      // The module where the error occurred
	Field  string      // The configuration field that's invalid
	Value  interface{} // The invalid value provided
	Reason string      // Why the value is invalid
	Hint   string      // Suggestion for fixing the issue
}

func (e *ValidationError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: invalid %s=%v (%s) - %s", e.Module, e.Field, e.Value, e.Reason, e.Hint)
	}
	return fmt.Sprintf("%s: invalid %s=%v (%s)", e.Module, e.Field, e.Value, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidConfiguration
}

// NewValidationError creates a new validation error with helpful details
func NewValidationError(module, field string, value interface{}, reason string) *ValidationError {
	return &ValidationError{
		Module: module,
		Field:  field,
		Value:  value,
		Reason: reason,
	}
}

// WithHint adds a helpful suggestion to a validation error
func (e *ValidationError) WithHint(hint string) *ValidationError {
	e.Hint = hint
	return e
}

// OperationError represents an error during library operation with context
type OperationError struct {
	Module    string // The module where the error occurred
	Operation string // The operation that failed
	Cause     error  // The underlying error
	Context   string // Additional context about the failure
}

func (e *OperationError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%s.%s failed: %v (%s)", e.Module, e.Operation, e.Cause, e.Context)
	}
	return fmt.Sprintf("%s.%s failed: %v", e.Module, e.Operation, e.Cause)
}

func (e *OperationError) Unwrap() error {
	return e.Cause
}

// NewOperationError creates a new operation error with context
func NewOperationError(module, operation string, cause error) *OperationError {
	return &OperationError{
		Module:    module,
		Operation: operation,
		Cause:     cause,
	}
}

// WithContext adds additional context to an operation error
func (e *OperationError) WithContext(context string) *OperationError {
	e.Context = context
	return e
}

// IsRetryable returns true if the error indicates a condition that might
// be resolved by retrying the operation
func IsRetryable(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrRateLimited)
}

// IsTemporary returns true if the error indicates a temporary condition
func IsTemporary(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrCapacityExceeded)
}

// IsValidationError returns true if the error is a validation error
func IsValidationError(err error) bool {
	var valErr *ValidationError
	return errors.As(err, &valErr)
}
