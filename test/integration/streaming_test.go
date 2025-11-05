package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
	"github.com/vnykmshr/goflow/pkg/streaming/channel"
	"github.com/vnykmshr/goflow/pkg/streaming/stream"
	"github.com/vnykmshr/goflow/pkg/streaming/writer"
)

// TestStreamWithChannelAndWriter tests the complete streaming pipeline:
// Stream -> Channel -> Writer, verifying data flows correctly through all components.
func TestStreamWithChannelAndWriter(t *testing.T) {
	// Create a buffered channel
	ch := channel.New[string](10)
	defer func() { _ = ch.Close() }()

	// Create a writer
	underlying := testutil.NewMockWriter()
	w := writer.New(underlying)
	defer func() { _ = w.Close() }()

	ctx := context.Background()

	// Send data through the channel in a goroutine
	go func() {
		for i := 0; i < 5; i++ {
			data := string(rune('A' + i))
			if err := ch.Send(ctx, data); err != nil {
				t.Errorf("failed to send %s: %v", data, err)
			}
		}
	}()

	// Receive and write data
	var received int
	for i := 0; i < 5; i++ {
		data, err := ch.Receive(ctx)
		if err != nil {
			t.Fatalf("failed to receive: %v", err)
		}

		if err := w.WriteString(data); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
		received++
	}

	// Flush to ensure all data is written
	if err := w.Flush(ctx); err != nil {
		t.Fatalf("failed to flush: %v", err)
	}

	if received != 5 {
		t.Errorf("received %d items, want 5", received)
	}

	// Verify all data was written
	written := underlying.String()
	expected := "ABCDE"
	if written != expected {
		t.Errorf("written = %q, want %q", written, expected)
	}

	t.Logf("Successfully streamed %d items through channel to writer", received)
}

// TestStreamProcessingPipeline tests a complex stream processing workflow
// with multiple transformations and concurrent operations.
func TestStreamProcessingPipeline(t *testing.T) {
	var processed int32

	// Create a stream processing pipeline
	result, err := stream.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}).
		Filter(func(x int) bool { return x%2 == 0 }). // Even numbers: 2,4,6,8,10
		Map(func(x int) int { return x * 2 }).        // Double: 4,8,12,16,20
		Limit(3).                                     // Take first 3: 4,8,12
		Peek(func(_ int) {
			atomic.AddInt32(&processed, 1)
		}).
		ToSlice(context.Background())

	if err != nil {
		t.Fatalf("stream processing failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("result length = %d, want 3", len(result))
	}

	expected := []int{4, 8, 12}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %d, want %d", i, result[i], v)
		}
	}

	// Peek should be called after Limit, so should see exactly 3 items
	if atomic.LoadInt32(&processed) != 3 {
		t.Errorf("processed = %d, want 3", processed)
	}

	t.Logf("Stream pipeline processed %d items correctly", len(result))
}

// TestChannelBackpressure tests that channel strategies (Block, Drop, DropOldest)
// work correctly under load.
func TestChannelBackpressure(t *testing.T) {
	t.Run("Block strategy", func(t *testing.T) {
		config := channel.Config{
			BufferSize: 2,
			Strategy:   channel.Block,
		}
		ch := channel.NewWithConfig[int](config)
		defer func() { _ = ch.Close() }()

		ctx := context.Background()

		// Fill buffer
		testutil.AssertNoError(t, ch.Send(ctx, 1))
		testutil.AssertNoError(t, ch.Send(ctx, 2))

		// This would block, so test with timeout
		ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := ch.Send(ctx2, 3)
		// Should timeout because buffer is full and we're not receiving
		if err != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", err)
		}
	})

	t.Run("Drop strategy", func(t *testing.T) {
		var dropped int32
		config := channel.Config{
			BufferSize: 2,
			Strategy:   channel.Drop,
			OnDrop: func(_ interface{}) {
				atomic.AddInt32(&dropped, 1)
			},
		}
		ch := channel.NewWithConfig[int](config)
		defer func() { _ = ch.Close() }()

		ctx := context.Background()

		// Fill buffer
		testutil.AssertNoError(t, ch.Send(ctx, 1))
		testutil.AssertNoError(t, ch.Send(ctx, 2))

		// This should drop
		testutil.AssertNoError(t, ch.Send(ctx, 3))

		if atomic.LoadInt32(&dropped) != 1 {
			t.Errorf("dropped = %d, want 1", dropped)
		}

		stats := ch.Stats()
		if stats.DroppedCount != 1 {
			t.Errorf("stats.DroppedCount = %d, want 1", stats.DroppedCount)
		}
	})

	t.Run("DropOldest strategy", func(t *testing.T) {
		var dropped int32
		config := channel.Config{
			BufferSize: 2,
			Strategy:   channel.DropOldest,
			OnDrop: func(_ interface{}) {
				atomic.AddInt32(&dropped, 1)
			},
		}
		ch := channel.NewWithConfig[int](config)
		defer func() { _ = ch.Close() }()

		ctx := context.Background()

		// Fill buffer
		testutil.AssertNoError(t, ch.Send(ctx, 1))
		testutil.AssertNoError(t, ch.Send(ctx, 2))

		// This should drop oldest (1) and add 3
		testutil.AssertNoError(t, ch.Send(ctx, 3))

		if atomic.LoadInt32(&dropped) != 1 {
			t.Errorf("dropped = %d, want 1", dropped)
		}

		// Receive should get 2 (oldest was dropped)
		val, err := ch.Receive(ctx)
		testutil.AssertNoError(t, err)
		if val != 2 {
			t.Errorf("first value = %d, want 2 (1 was dropped)", val)
		}
	})
}

// TestWriterConcurrency tests that multiple goroutines can safely write
// to the same writer concurrently.
func TestWriterConcurrency(t *testing.T) {
	underlying := testutil.NewMockWriter()
	w := writer.New(underlying)
	defer func() { _ = w.Close() }()

	const goroutines = 10
	const writesPerGoroutine = 100

	var written int32

	done := make(chan bool, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(_ int) {
			defer func() { done <- true }()

			for j := 0; j < writesPerGoroutine; j++ {
				if err := w.WriteString("X"); err != nil {
					t.Errorf("write failed: %v", err)
					return
				}
				atomic.AddInt32(&written, 1)
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Flush to ensure all writes complete
	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	expected := goroutines * writesPerGoroutine
	if atomic.LoadInt32(&written) != int32(expected) {
		t.Errorf("written = %d, want %d", written, expected)
	}

	// Check writer stats
	stats := w.Stats()
	if stats.WriteCount != int64(expected) {
		t.Errorf("stats.WriteCount = %d, want %d", stats.WriteCount, expected)
	}

	// Verify total bytes written (each write is 1 byte 'X')
	if stats.BytesWritten != int64(expected) {
		t.Errorf("stats.BytesWritten = %d, want %d", stats.BytesWritten, expected)
	}

	t.Logf("Concurrent writes: %d goroutines × %d writes = %d total writes",
		goroutines, writesPerGoroutine, expected)
}

// TestStreamContextCancellation verifies that stream operations respect
// context cancellation and clean up properly.
func TestStreamContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Create a slow stream that will be canceled
	stream := stream.Generate(func() int {
		time.Sleep(100 * time.Millisecond) // Slower than timeout
		return 1
	}).Limit(100)
	defer func() { _ = stream.Close() }()

	_, err := stream.Count(ctx)

	// Should be canceled
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}

	t.Log("Stream correctly handled context cancellation")
}
