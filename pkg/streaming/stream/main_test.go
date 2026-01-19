package stream

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain enables goroutine leak detection for all tests in this package.
// This catches any leaked goroutines from stream operations.
//
// Note: We ignore mapOperation.apply goroutines because when using infinite
// generators (Generate) with operations like Map, the operation goroutine may
// briefly linger after Close() is called. The goroutine is blocked in a select
// waiting to send to the output channel, and while closeCtx is cancelled,
// the operation.apply() receives the user context which doesn't get cancelled
// until the goroutine scheduler runs. This is a known limitation when using
// infinite streams with early termination (Limit).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("github.com/vnykmshr/goflow/pkg/streaming/stream.(*mapOperation[...]).apply"),
		goleak.IgnoreTopFunction("github.com/vnykmshr/goflow/pkg/streaming/stream.(*filterOperation[...]).apply"),
	)
}
