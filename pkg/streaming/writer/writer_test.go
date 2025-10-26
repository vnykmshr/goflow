package writer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
)

func TestNew(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	testutil.AssertEqual(t, writer.IsClosed(), false)
	testutil.AssertEqual(t, writer.BufferSize(), 0)
	testutil.AssertEqual(t, writer.BufferCapacity() > 0, true)
}

func TestNewWithConfig(t *testing.T) {
	underlying := testutil.NewMockWriter()
	config := Config{
		BufferSize:    1024,
		FlushInterval: 100 * time.Millisecond,
		BlockOnFull:   false,
		MaxRetries:    5,
		RetryDelay:    50 * time.Millisecond,
	}

	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	testutil.AssertEqual(t, writer.IsClosed(), false)
	testutil.AssertEqual(t, writer.BufferCapacity(), 1024)
}

func TestBasicWrite(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	// Write some data
	data := []byte("Hello, World!")
	err := writer.Write(data)
	testutil.AssertNoError(t, err)

	// Flush to ensure data is written
	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.String(), "Hello, World!")
}

func TestWriteString(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	err := writer.WriteString("Hello, World!")
	testutil.AssertNoError(t, err)

	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.String(), "Hello, World!")
}

func TestMultipleWrites(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	// Write multiple pieces of data
	writes := []string{"Hello", ", ", "World", "!"}
	for _, data := range writes {
		err := writer.WriteString(data)
		testutil.AssertNoError(t, err)
	}

	// Flush all data
	err := writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.String(), "Hello, World!")
}

func TestAsyncWrite(t *testing.T) {
	underlying := testutil.NewMockWriter()
	// Add some delay to test asynchronous behavior
	underlying.SetWriteDelay(10 * time.Millisecond)

	config := DefaultConfig()
	config.BlockOnFull = false
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	// Write should return immediately even with slow underlying writer
	start := time.Now()
	err := writer.WriteString("Hello, World!")
	elapsed := time.Since(start)

	testutil.AssertNoError(t, err)
	// Should complete much faster than the 10ms write delay
	testutil.AssertEqual(t, elapsed < 5*time.Millisecond, true)

	// Wait for background write to complete
	time.Sleep(50 * time.Millisecond)
	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.String(), "Hello, World!")
}

func TestBuffering(t *testing.T) {
	underlying := testutil.NewMockWriter()
	config := DefaultConfig()
	config.FlushInterval = 0 // Disable automatic flushing
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	// Write data but don't flush
	err := writer.WriteString("buffered data")
	testutil.AssertNoError(t, err)

	// Data should be buffered, not written to underlying writer yet
	testutil.AssertEqual(t, underlying.Len(), 0)
	testutil.AssertEqual(t, writer.BufferSize() > 0, true)

	// Now flush
	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	// Data should now be written
	testutil.AssertEqual(t, underlying.String(), "buffered data")
	testutil.AssertEqual(t, writer.BufferSize(), 0)
}

func TestAutoFlush(t *testing.T) {
	underlying := testutil.NewMockWriter()
	config := DefaultConfig()
	config.FlushInterval = 50 * time.Millisecond
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	// Write data
	err := writer.WriteString("auto flush test")
	testutil.AssertNoError(t, err)

	// Wait for auto flush
	time.Sleep(100 * time.Millisecond)

	// Data should be automatically flushed
	testutil.AssertEqual(t, underlying.String(), "auto flush test")
}

func TestBufferFull(t *testing.T) {
	underlying := testutil.NewMockWriter()
	config := DefaultConfig()
	config.BufferSize = 10 // Small buffer
	config.BlockOnFull = false
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	// Fill buffer beyond capacity
	largeData := strings.Repeat("x", 20)
	err := writer.WriteString(largeData)

	// Should get buffer full error since blocking is disabled
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrBufferFull)

	stats := writer.Stats()
	testutil.AssertEqual(t, stats.BufferOverflows > 0, true)
}

func TestBlockOnFull(t *testing.T) {
	underlying := testutil.NewMockWriter()
	config := DefaultConfig()
	config.BufferSize = 10 // Small buffer
	config.BlockOnFull = true
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	// This should eventually succeed by triggering flushes
	largeData := strings.Repeat("x", 20)
	err := writer.WriteString(largeData)
	testutil.AssertNoError(t, err)

	// Flush to ensure data is written
	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.String(), largeData)
}

func TestWriteErrors(t *testing.T) {
	underlying := testutil.NewMockWriter()
	underlying.SetAlwaysError(errors.New("write failed"))

	config := DefaultConfig()
	config.MaxRetries = 0 // No retries for faster test
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	err := writer.WriteString("test")
	testutil.AssertNoError(t, err) // Write itself succeeds (async)

	// Error should occur during flush
	err = writer.Flush(context.Background())
	testutil.AssertError(t, err)

	stats := writer.Stats()
	testutil.AssertEqual(t, stats.ErrorCount > 0, true)
}

func TestRetries(t *testing.T) {
	underlying := testutil.NewMockWriter()
	underlying.SetErrorOnNth(1) // Error on first write, succeed on retry

	config := DefaultConfig()
	config.MaxRetries = 2
	config.RetryDelay = 10 * time.Millisecond
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	err := writer.WriteString("retry test")
	testutil.AssertNoError(t, err)

	// Should succeed after retry
	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.String(), "retry test")
	testutil.AssertEqual(t, underlying.WriteCount() > 1, true) // Should have retried
}

func TestContextCancellation(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	// Create context that will be canceled
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := writer.WriteString("test data")
	testutil.AssertNoError(t, err)

	// This should be canceled due to timeout
	err = writer.Flush(ctx)
	if err != nil {
		testutil.AssertEqual(t, err, context.DeadlineExceeded)
	}
}

func TestStats(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	// Initial stats
	stats := writer.Stats()
	testutil.AssertEqual(t, stats.WriteCount, int64(0))
	testutil.AssertEqual(t, stats.BytesWritten, int64(0))

	// Write some data
	testData := "Hello, World!"
	err := writer.WriteString(testData)
	testutil.AssertNoError(t, err)

	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	// Check updated stats
	stats = writer.Stats()
	testutil.AssertEqual(t, stats.WriteCount, int64(1))
	testutil.AssertEqual(t, stats.BytesWritten, int64(len(testData)))
	testutil.AssertEqual(t, stats.FlushCount, int64(1))
	testutil.AssertEqual(t, stats.ErrorCount, int64(0))
	testutil.AssertEqual(t, stats.BufferUtilization >= 0.0 && stats.BufferUtilization <= 1.0, true)
	testutil.AssertEqual(t, stats.AverageWriteTime >= 0, true)
}

func TestClose(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)

	// Write some data
	err := writer.WriteString("test data")
	testutil.AssertNoError(t, err)

	// Close should flush remaining data
	err = writer.Close()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, writer.IsClosed(), true)
	testutil.AssertEqual(t, underlying.String(), "test data")

	// Writing to closed writer should fail
	err = writer.WriteString("more data")
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrWriterClosed)
}

func TestConcurrentWrites(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	const numGoroutines = 10
	const writesPerGoroutine = 100

	var wg sync.WaitGroup

	// Start multiple goroutines writing concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < writesPerGoroutine; j++ {
				data := fmt.Sprintf("goroutine-%d-write-%d\n", id, j)
				err := writer.WriteString(data)
				if err != nil {
					t.Errorf("Write failed: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Flush all data
	err := writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	// Check that all data was written
	result := underlying.String()
	expectedWrites := numGoroutines * writesPerGoroutine
	actualWrites := strings.Count(result, "goroutine-")

	testutil.AssertEqual(t, actualWrites, expectedWrites)

	// Check stats
	stats := writer.Stats()
	testutil.AssertEqual(t, stats.WriteCount, int64(expectedWrites))
}

func TestCallbacks(t *testing.T) {
	t.Run("BufferFull", func(t *testing.T) {
		underlying := testutil.NewMockWriter()

		var bufferFullCalled bool

		config := DefaultConfig()
		config.BufferSize = 5 // Small buffer to trigger buffer full
		config.BlockOnFull = false
		config.OnBufferFull = func() {
			bufferFullCalled = true
		}

		writer := NewWithConfig(underlying, config)
		defer func() { _ = writer.Close() }()

		// Test buffer full callback
		largeData := strings.Repeat("x", 10)
		_ = writer.WriteString(largeData) // Should trigger buffer full

		testutil.AssertEqual(t, bufferFullCalled, true)
	})

	t.Run("Flush", func(t *testing.T) {
		underlying := testutil.NewMockWriter()

		var flushCalled bool
		var flushBytes int

		config := DefaultConfig()
		config.OnFlush = func(bytes int, _ time.Duration) {
			flushCalled = true
			flushBytes = bytes
		}

		writer := NewWithConfig(underlying, config)
		defer func() { _ = writer.Close() }()

		// Write data and flush
		err := writer.WriteString("test")
		testutil.AssertNoError(t, err)

		err = writer.Flush(context.Background())
		testutil.AssertNoError(t, err)

		testutil.AssertEqual(t, flushCalled, true)
		testutil.AssertEqual(t, flushBytes > 0, true)
	})

	t.Run("Error", func(t *testing.T) {
		underlying := testutil.NewMockWriter()
		underlying.SetAlwaysError(errors.New("test error"))

		var errorCalled bool

		config := DefaultConfig()
		config.OnError = func(_ error) {
			errorCalled = true
		}

		writer := NewWithConfig(underlying, config)
		defer func() { _ = writer.Close() }()

		// Write and flush should trigger error callback
		err := writer.WriteString("error test")
		testutil.AssertNoError(t, err) // Write to buffer succeeds

		flushErr := writer.Flush(context.Background())
		testutil.AssertError(t, flushErr) // Flush should fail

		// Allow time for error callback to be processed
		time.Sleep(50 * time.Millisecond)

		testutil.AssertEqual(t, errorCalled, true)
	})
}

func TestEmptyWrites(t *testing.T) {
	underlying := testutil.NewMockWriter()
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	// Empty writes should be no-ops
	err := writer.Write(nil)
	testutil.AssertNoError(t, err)

	err = writer.Write([]byte{})
	testutil.AssertNoError(t, err)

	err = writer.WriteString("")
	testutil.AssertNoError(t, err)

	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.Len(), 0)

	stats := writer.Stats()
	testutil.AssertEqual(t, stats.WriteCount, int64(0))
	testutil.AssertEqual(t, stats.BytesWritten, int64(0))
}

func TestLargeWrites(t *testing.T) {
	underlying := testutil.NewMockWriter()
	config := DefaultConfig()
	config.BufferSize = 1024 // 1KB buffer
	writer := NewWithConfig(underlying, config)
	defer func() { _ = writer.Close() }()

	// Write data larger than buffer
	largeData := strings.Repeat("x", 2048) // 2KB
	err := writer.WriteString(largeData)
	testutil.AssertNoError(t, err)

	err = writer.Flush(context.Background())
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, underlying.String(), largeData)
}

// Benchmark tests
func BenchmarkWrite(b *testing.B) {
	underlying := &bytes.Buffer{}
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = writer.Write(data)
	}

	_ = writer.Flush(context.Background())
}

func BenchmarkWriteString(b *testing.B) {
	underlying := &bytes.Buffer{}
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	data := "benchmark data string"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = writer.WriteString(data)
	}

	_ = writer.Flush(context.Background())
}

func BenchmarkConcurrentWrites(b *testing.B) {
	underlying := &bytes.Buffer{}
	writer := New(underlying)
	defer func() { _ = writer.Close() }()

	data := []byte("concurrent benchmark data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = writer.Write(data)
		}
	})

	_ = writer.Flush(context.Background())
}
