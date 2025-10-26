package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestCommonErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrClosed", ErrClosed, "resource is closed"},
		{"ErrTimeout", ErrTimeout, "operation timed out"},
		{"ErrCapacityExceeded", ErrCapacityExceeded, "capacity exceeded"},
		{"ErrInvalidConfiguration", ErrInvalidConfiguration, "invalid configuration"},
		{"ErrRateLimited", ErrRateLimited, "rate limited"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error should not be nil")
			}
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ValidationError
		want string
	}{
		{
			name: "without hint",
			err: &ValidationError{
				Module: "ratelimit",
				Field:  "rate",
				Value:  -1,
				Reason: "must be positive",
			},
			want: "ratelimit: invalid rate=-1 (must be positive)",
		},
		{
			name: "with hint",
			err: &ValidationError{
				Module: "ratelimit",
				Field:  "burst",
				Value:  0,
				Reason: "must be positive",
				Hint:   "use a value greater than 0",
			},
			want: "ratelimit: invalid burst=0 (must be positive) - use a value greater than 0",
		},
		{
			name: "string value",
			err: &ValidationError{
				Module: "scheduler",
				Field:  "cron",
				Value:  "",
				Reason: "cannot be empty",
			},
			want: "scheduler: invalid cron= (cannot be empty)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationError_Unwrap(t *testing.T) {
	verr := &ValidationError{
		Module: "test",
		Field:  "field",
		Value:  0,
		Reason: "test",
	}

	unwrapped := verr.Unwrap()
	if unwrapped != ErrInvalidConfiguration {
		t.Errorf("Unwrap() = %v, want ErrInvalidConfiguration", unwrapped)
	}

	if !errors.Is(verr, ErrInvalidConfiguration) {
		t.Error("ValidationError should wrap ErrInvalidConfiguration")
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("module", "field", 123, "test reason")

	if err.Module != "module" {
		t.Errorf("Module = %q, want %q", err.Module, "module")
	}
	if err.Field != "field" {
		t.Errorf("Field = %q, want %q", err.Field, "field")
	}
	if err.Value != 123 {
		t.Errorf("Value = %v, want %v", err.Value, 123)
	}
	if err.Reason != "test reason" {
		t.Errorf("Reason = %q, want %q", err.Reason, "test reason")
	}
	if err.Hint != "" {
		t.Errorf("Hint = %q, want empty string", err.Hint)
	}
}

func TestValidationError_WithHint(t *testing.T) {
	err := NewValidationError("test", "field", 0, "invalid").
		WithHint("try using a positive value")

	if err.Hint != "try using a positive value" {
		t.Errorf("Hint = %q, want %q", err.Hint, "try using a positive value")
	}

	// Should return same instance for chaining
	result := err.WithHint("new hint")
	if result != err {
		t.Error("WithHint should return the same instance")
	}
}

func TestOperationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *OperationError
		want string
	}{
		{
			name: "without context",
			err: &OperationError{
				Module:    "stream",
				Operation: "Write",
				Cause:     errors.New("write failed"),
			},
			want: "stream.Write failed: write failed",
		},
		{
			name: "with context",
			err: &OperationError{
				Module:    "channel",
				Operation: "Send",
				Cause:     errors.New("buffer full"),
				Context:   "exceeded capacity of 100",
			},
			want: "channel.Send failed: buffer full (exceeded capacity of 100)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOperationError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	opErr := &OperationError{
		Module:    "test",
		Operation: "test",
		Cause:     cause,
	}

	unwrapped := opErr.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	if !errors.Is(opErr, cause) {
		t.Error("OperationError should wrap the cause error")
	}
}

func TestNewOperationError(t *testing.T) {
	cause := errors.New("test cause")
	err := NewOperationError("module", "operation", cause)

	if err.Module != "module" {
		t.Errorf("Module = %q, want %q", err.Module, "module")
	}
	if err.Operation != "operation" {
		t.Errorf("Operation = %q, want %q", err.Operation, "operation")
	}
	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
	if err.Context != "" {
		t.Errorf("Context = %q, want empty string", err.Context)
	}
}

func TestOperationError_WithContext(t *testing.T) {
	err := NewOperationError("test", "op", errors.New("err")).
		WithContext("additional context")

	if err.Context != "additional context" {
		t.Errorf("Context = %q, want %q", err.Context, "additional context")
	}

	// Should return same instance for chaining
	result := err.WithContext("new context")
	if result != err {
		t.Error("WithContext should return the same instance")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout error", ErrTimeout, true},
		{"rate limited error", ErrRateLimited, true},
		{"closed error", ErrClosed, false},
		{"capacity exceeded", ErrCapacityExceeded, false},
		{"random error", errors.New("random"), false},
		{"wrapped timeout", &OperationError{Cause: ErrTimeout}, true},
		{"wrapped rate limited", &OperationError{Cause: ErrRateLimited}, true},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTemporary(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout error", ErrTimeout, true},
		{"capacity exceeded", ErrCapacityExceeded, true},
		{"rate limited error", ErrRateLimited, false},
		{"closed error", ErrClosed, false},
		{"random error", errors.New("random"), false},
		{"wrapped timeout", &OperationError{Cause: ErrTimeout}, true},
		{"wrapped capacity", &OperationError{Cause: ErrCapacityExceeded}, true},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTemporary(tt.err); got != tt.want {
				t.Errorf("IsTemporary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"validation error",
			&ValidationError{Module: "test", Field: "field", Value: 0, Reason: "test"},
			true,
		},
		{
			"wrapped validation error",
			&OperationError{Cause: &ValidationError{Module: "test", Field: "field", Value: 0, Reason: "test"}},
			true,
		},
		{"operation error", &OperationError{Cause: errors.New("test")}, false},
		{"standard error", errors.New("test"), false},
		{"timeout error", ErrTimeout, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidationError(tt.err); got != tt.want {
				t.Errorf("IsValidationError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	// Test that all error messages are properly formatted and contain expected parts
	t.Run("ValidationError message components", func(t *testing.T) {
		err := NewValidationError("mymodule", "myfield", 42, "must be less than 10").
			WithHint("use a value between 0 and 10")

		msg := err.Error()

		expectedParts := []string{"mymodule", "myfield", "42", "must be less than 10", "use a value between 0 and 10"}
		for _, part := range expectedParts {
			if !strings.Contains(msg, part) {
				t.Errorf("error message should contain %q, got %q", part, msg)
			}
		}
	})

	t.Run("OperationError message components", func(t *testing.T) {
		err := NewOperationError("mymodule", "MyOp", errors.New("connection refused")).
			WithContext("server unreachable")

		msg := err.Error()

		expectedParts := []string{"mymodule", "MyOp", "connection refused", "server unreachable"}
		for _, part := range expectedParts {
			if !strings.Contains(msg, part) {
				t.Errorf("error message should contain %q, got %q", part, msg)
			}
		}
	})
}
