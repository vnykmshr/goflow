package stream

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain enables goroutine leak detection for all tests in this package.
// This catches any leaked goroutines from stream operations.
//
// Stream goroutines now properly terminate when Close() is called, thanks to
// internal context cancellation added in v1.3.0.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
