// Package validation provides common validation utilities for the goflow library.
package validation

import (
	gferrors "github.com/vnykmshr/goflow/pkg/common/errors"
)

// ValidatePositive validates that an integer value is positive (> 0).
// Returns a ValidationError if the value is not positive.
func ValidatePositive(module, field string, value int) error {
	if value <= 0 {
		return gferrors.NewValidationError(module, field, value, "must be positive").
			WithHint("value must be greater than 0")
	}
	return nil
}

// ValidateNonNegative validates that a numeric value is non-negative (>= 0).
// Returns a ValidationError if the value is negative.
func ValidateNonNegative(module, field string, value float64) error {
	if value < 0 {
		return gferrors.NewValidationError(module, field, value, "cannot be negative").
			WithHint("use 0 or a positive value")
	}
	return nil
}

// ValidatePositiveFloat validates that a float64 value is positive (> 0).
// Returns a ValidationError if the value is not positive.
func ValidatePositiveFloat(module, field string, value float64) error {
	if value <= 0 {
		return gferrors.NewValidationError(module, field, value, "must be positive").
			WithHint("value must be greater than 0")
	}
	return nil
}

// ValidateNotNil validates that an interface value is not nil.
// Returns a ValidationError if the value is nil.
func ValidateNotNil(module, field string, value interface{}) error {
	if value == nil {
		return gferrors.NewValidationError(module, field, nil, "cannot be nil").
			WithHint("provide a valid " + field)
	}
	return nil
}

// ValidateNotEmpty validates that a string value is not empty.
// Returns a ValidationError if the string is empty.
func ValidateNotEmpty(module, field string, value string) error {
	if value == "" {
		return gferrors.NewValidationError(module, field, value, "cannot be empty").
			WithHint("provide a non-empty " + field)
	}
	return nil
}
