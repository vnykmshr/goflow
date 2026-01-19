package workerpool

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain enables goroutine leak detection for all tests in this package.
// This catches any leaked goroutines from worker pool operations.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
