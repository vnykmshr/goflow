package validation

import (
	"testing"

	"github.com/vnykmshr/goflow/pkg/common/errors"
)

func TestValidatePositive(t *testing.T) {
	tests := []struct {
		name      string
		module    string
		field     string
		value     int
		wantError bool
	}{
		{"positive value", "test", "count", 10, false},
		{"positive value 1", "test", "count", 1, false},
		{"zero value", "test", "count", 0, true},
		{"negative value", "test", "count", -1, true},
		{"large positive", "test", "count", 1000000, false},
		{"large negative", "test", "count", -1000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositive(tt.module, tt.field, tt.value)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.IsValidationError(err) {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateNonNegative(t *testing.T) {
	tests := []struct {
		name      string
		module    string
		field     string
		value     float64
		wantError bool
	}{
		{"positive value", "test", "rate", 10.5, false},
		{"zero value", "test", "rate", 0.0, false},
		{"negative value", "test", "rate", -1.5, true},
		{"small positive", "test", "rate", 0.001, false},
		{"small negative", "test", "rate", -0.001, true},
		{"large positive", "test", "rate", 99999.99, false},
		{"large negative", "test", "rate", -99999.99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonNegative(tt.module, tt.field, tt.value)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.IsValidationError(err) {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidatePositiveFloat(t *testing.T) {
	tests := []struct {
		name      string
		module    string
		field     string
		value     float64
		wantError bool
	}{
		{"positive value", "test", "rate", 5.5, false},
		{"positive integer", "test", "rate", 10.0, false},
		{"zero value", "test", "rate", 0.0, true},
		{"negative value", "test", "rate", -2.5, true},
		{"small positive", "test", "rate", 0.0001, false},
		{"very small positive", "test", "rate", 1e-10, false},
		{"negative infinity", "test", "rate", -1e308, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveFloat(tt.module, tt.field, tt.value)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.IsValidationError(err) {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateNotNil(t *testing.T) {
	tests := []struct {
		name      string
		module    string
		field     string
		value     interface{}
		wantError bool
	}{
		{"non-nil int", "test", "config", 123, false},
		{"non-nil string", "test", "config", "value", false},
		{"non-nil struct", "test", "config", struct{}{}, false},
		{"non-nil pointer", "test", "config", new(int), false},
		{"non-nil slice", "test", "config", []int{}, false},
		{"non-nil map", "test", "config", map[string]int{}, false},
		{"nil value", "test", "config", nil, true},
		{"nil pointer", "test", "config", (*int)(nil), false}, // typed nil is not nil interface
		{"empty interface", "test", "config", interface{}(nil), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotNil(tt.module, tt.field, tt.value)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.IsValidationError(err) {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateNotEmpty(t *testing.T) {
	tests := []struct {
		name      string
		module    string
		field     string
		value     string
		wantError bool
	}{
		{"non-empty string", "test", "name", "value", false},
		{"single char", "test", "name", "a", false},
		{"whitespace", "test", "name", " ", false}, // Whitespace is not empty
		{"empty string", "test", "name", "", true},
		{"long string", "test", "name", "this is a long value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotEmpty(tt.module, tt.field, tt.value)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.IsValidationError(err) {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidationErrorDetails(t *testing.T) {
	t.Run("ValidatePositive error details", func(t *testing.T) {
		err := ValidatePositive("ratelimit", "burst", -5)
		if err == nil {
			t.Fatal("expected error")
		}

		var valErr *errors.ValidationError
		if !errors.IsValidationError(err) {
			t.Fatal("expected ValidationError")
		}

		// Extract the validation error
		if e, ok := err.(*errors.ValidationError); ok {
			valErr = e
		} else {
			t.Fatal("could not cast to ValidationError")
		}

		if valErr.Module != "ratelimit" {
			t.Errorf("Module = %q, want %q", valErr.Module, "ratelimit")
		}
		if valErr.Field != "burst" {
			t.Errorf("Field = %q, want %q", valErr.Field, "burst")
		}
		if valErr.Value != -5 {
			t.Errorf("Value = %v, want %v", valErr.Value, -5)
		}
		if valErr.Reason != "must be positive" {
			t.Errorf("Reason = %q, want %q", valErr.Reason, "must be positive")
		}
		if valErr.Hint != "value must be greater than 0" {
			t.Errorf("Hint = %q, want %q", valErr.Hint, "value must be greater than 0")
		}
	})

	t.Run("ValidateNonNegative error details", func(t *testing.T) {
		err := ValidateNonNegative("scheduler", "delay", -10.5)
		if err == nil {
			t.Fatal("expected error")
		}

		var valErr *errors.ValidationError
		if e, ok := err.(*errors.ValidationError); ok {
			valErr = e
		} else {
			t.Fatal("could not cast to ValidationError")
		}

		if valErr.Reason != "cannot be negative" {
			t.Errorf("Reason = %q, want %q", valErr.Reason, "cannot be negative")
		}
		if valErr.Hint != "use 0 or a positive value" {
			t.Errorf("Hint = %q, want %q", valErr.Hint, "use 0 or a positive value")
		}
	})

	t.Run("ValidateNotEmpty error details", func(t *testing.T) {
		err := ValidateNotEmpty("config", "key", "")
		if err == nil {
			t.Fatal("expected error")
		}

		var valErr *errors.ValidationError
		if e, ok := err.(*errors.ValidationError); ok {
			valErr = e
		} else {
			t.Fatal("could not cast to ValidationError")
		}

		if valErr.Reason != "cannot be empty" {
			t.Errorf("Reason = %q, want %q", valErr.Reason, "cannot be empty")
		}
		if valErr.Hint != "provide a non-empty key" {
			t.Errorf("Hint = %q, want contains 'key'", valErr.Hint)
		}
	})
}

func TestValidationErrorWrapping(t *testing.T) {
	// All validation errors should wrap ErrInvalidConfiguration
	t.Run("errors wrap ErrInvalidConfiguration", func(t *testing.T) {
		testCases := []struct {
			name string
			err  error
		}{
			{"ValidatePositive", ValidatePositive("test", "field", -1)},
			{"ValidateNonNegative", ValidateNonNegative("test", "field", -1.0)},
			{"ValidatePositiveFloat", ValidatePositiveFloat("test", "field", 0.0)},
			{"ValidateNotNil", ValidateNotNil("test", "field", nil)},
			{"ValidateNotEmpty", ValidateNotEmpty("test", "field", "")},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.err == nil {
					t.Fatal("expected error")
				}
				if !errors.IsValidationError(tc.err) {
					t.Error("error should be a ValidationError")
				}
				// Check if it wraps ErrInvalidConfiguration
				var valErr *errors.ValidationError
				if ok := errors.IsValidationError(tc.err); ok {
					if e, ok := tc.err.(*errors.ValidationError); ok {
						valErr = e
						if wrapped := valErr.Unwrap(); wrapped != errors.ErrInvalidConfiguration {
							t.Errorf("should unwrap to ErrInvalidConfiguration, got %v", wrapped)
						}
					}
				}
			})
		}
	})
}
