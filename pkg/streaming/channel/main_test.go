package channel

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain enables goroutine leak detection for all tests in this package.
// This catches any leaked goroutines from backpressure channel operations.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
