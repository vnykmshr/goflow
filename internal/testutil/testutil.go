package testutil

import (
  "context"
  "testing"
  "time"
)

// TestTimeout is the default timeout for tests
const TestTimeout = 5 * time.Second

// WithTimeout creates a context with the default test timeout
func WithTimeout(t *testing.T) (context.Context, context.CancelFunc) {
  t.Helper()
  return context.WithTimeout(context.Background(), TestTimeout)
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
  t.Helper()
  if err != nil {
    t.Fatalf("unexpected error: %v", err)
  }
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
  t.Helper()
  if err == nil {
    t.Fatal("expected error, got nil")
  }
}

// AssertEqual fails the test if got != want
func AssertEqual[T comparable](t *testing.T, got, want T) {
  t.Helper()
  if got != want {
    t.Fatalf("got %v, want %v", got, want)
  }
}