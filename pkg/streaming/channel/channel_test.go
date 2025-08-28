package channel

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
)

func TestNew(t *testing.T) {
	ch := New[int](10)
	testutil.AssertEqual(t, ch.Cap(), 10)
	testutil.AssertEqual(t, ch.Len(), 0)
	testutil.AssertEqual(t, ch.IsClosed(), false)
}

func TestNewWithConfig(t *testing.T) {
	config := Config{
		BufferSize: 5,
		Strategy:   Drop,
	}

	ch := NewWithConfig[string](config)
	testutil.AssertEqual(t, ch.Cap(), 5)
	testutil.AssertEqual(t, ch.Len(), 0)
}

func TestBasicSendReceive(t *testing.T) {
	ch := New[int](5)
	defer ch.Close()

	ctx := context.Background()

	// Send some values
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertNoError(t, ch.Send(ctx, 2))
	testutil.AssertNoError(t, ch.Send(ctx, 3))

	testutil.AssertEqual(t, ch.Len(), 3)

	// Receive values
	val1, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val1, 1)

	val2, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val2, 2)

	testutil.AssertEqual(t, ch.Len(), 1)
}

func TestTrySendTryReceive(t *testing.T) {
	ch := New[string](2)
	defer ch.Close()

	// Try send when buffer has space
	testutil.AssertNoError(t, ch.TrySend("hello"))
	testutil.AssertNoError(t, ch.TrySend("world"))
	testutil.AssertEqual(t, ch.Len(), 2)

	// Try send when buffer is full (should fail with default strategy)
	config := Config{BufferSize: 2, Strategy: Error}
	ch2 := NewWithConfig[string](config)
	defer ch2.Close()

	testutil.AssertNoError(t, ch2.TrySend("a"))
	testutil.AssertNoError(t, ch2.TrySend("b"))
	testutil.AssertError(t, ch2.TrySend("c"))
	testutil.AssertEqual(t, ch2.TrySend("c"), ErrChannelFull)

	// Try receive
	val, ok, err := ch.TryReceive()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, ok, true)
	testutil.AssertEqual(t, val, "hello")

	// Try receive when empty
	ch3 := New[int](5)
	defer ch3.Close()

	_, ok, err = ch3.TryReceive()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, ok, false)
}

func TestBlockStrategy(t *testing.T) {
	config := Config{
		BufferSize: 2,
		Strategy:   Block,
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertNoError(t, ch.Send(ctx, 2))
	testutil.AssertEqual(t, ch.Len(), 2)

	// Start a goroutine to send (should block)
	var blocked bool
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		blocked = true
		err := ch.Send(ctx, 3)
		testutil.AssertNoError(t, err)
		blocked = false
	}()

	// Give goroutine time to block
	time.Sleep(10 * time.Millisecond)
	testutil.AssertEqual(t, blocked, true)

	// Receive to unblock
	val, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val, 1)

	wg.Wait()
	testutil.AssertEqual(t, blocked, false)
	testutil.AssertEqual(t, ch.Len(), 2)
}

func TestDropStrategy(t *testing.T) {
	var droppedValue interface{}
	var droppedCount int

	config := Config{
		BufferSize: 2,
		Strategy:   Drop,
		OnDrop: func(value interface{}) {
			droppedValue = value
			droppedCount++
		},
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertNoError(t, ch.Send(ctx, 2))

	// Send more (should drop)
	testutil.AssertNoError(t, ch.Send(ctx, 3))
	testutil.AssertEqual(t, droppedCount, 1)
	testutil.AssertEqual(t, droppedValue.(int), 3)
	testutil.AssertEqual(t, ch.Len(), 2)

	// Verify original values are still there
	val1, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val1, 1)

	val2, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val2, 2)
}

func TestDropOldestStrategy(t *testing.T) {
	var droppedValue interface{}
	var droppedCount int

	config := Config{
		BufferSize: 2,
		Strategy:   DropOldest,
		OnDrop: func(value interface{}) {
			droppedValue = value
			droppedCount++
		},
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertNoError(t, ch.Send(ctx, 2))

	// Send more (should drop oldest)
	testutil.AssertNoError(t, ch.Send(ctx, 3))
	testutil.AssertEqual(t, droppedCount, 1)
	testutil.AssertEqual(t, droppedValue.(int), 1) // Oldest value dropped
	testutil.AssertEqual(t, ch.Len(), 2)

	// Verify newest values are there
	val1, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val1, 2)

	val2, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val2, 3)
}

func TestErrorStrategy(t *testing.T) {
	config := Config{
		BufferSize: 2,
		Strategy:   Error,
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertNoError(t, ch.Send(ctx, 2))

	// Send more (should error)
	err := ch.Send(ctx, 3)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrChannelFull)
	testutil.AssertEqual(t, ch.Len(), 2)
}

func TestContextCancellation(t *testing.T) {
	// Test with Error strategy to avoid complex blocking scenarios
	config := Config{
		BufferSize: 1,
		Strategy:   Error,
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))

	// Send should fail with full buffer
	err := ch.Send(ctx, 2)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrChannelFull)

	// Test context cancellation on receive
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ch2 := New[int](1)
	defer ch2.Close()

	_, err = ch2.Receive(ctx)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, context.Canceled)
}

func TestSendReceiveTimeout(t *testing.T) {
	config := Config{
		BufferSize: 1,
		Strategy:   Error,
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))

	// Send should fail with full buffer (Error strategy)
	err := ch.Send(ctx, 2)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrChannelFull)

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	ch2 := New[int](1)
	defer ch2.Close()

	// Sleep to ensure timeout
	time.Sleep(2 * time.Millisecond)

	_, err = ch2.Receive(ctx)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, context.DeadlineExceeded)
}

func TestClose(t *testing.T) {
	ch := New[int](5)

	ctx := context.Background()
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertEqual(t, ch.IsClosed(), false)

	// Close channel
	testutil.AssertNoError(t, ch.Close())
	testutil.AssertEqual(t, ch.IsClosed(), true)

	// Send should fail
	err := ch.Send(ctx, 2)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrChannelClosed)

	// TrySend should fail
	err = ch.TrySend(3)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrChannelClosed)

	// Can still receive existing data
	val, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, val, 1)

	// Receive on empty closed channel should fail
	_, err = ch.Receive(ctx)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrChannelClosed)

	// TryReceive on empty closed channel should fail
	_, _, err = ch.TryReceive()
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrChannelClosed)
}

func TestConcurrentAccess(t *testing.T) {
	ch := New[int](100)
	defer ch.Close()

	ctx := context.Background()
	const numGoroutines = 10
	const messagesPerGoroutine = 100

	var wg sync.WaitGroup
	var sentCount int64
	var receivedCount int64
	var receivedSum int64
	var expectedSum int64

	// Calculate expected sum
	for i := 0; i < numGoroutines*messagesPerGoroutine; i++ {
		expectedSum += int64(i)
	}

	// Start senders
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < messagesPerGoroutine; i++ {
				value := goroutineID*messagesPerGoroutine + i
				err := ch.Send(ctx, value)
				testutil.AssertNoError(t, err)
				atomic.AddInt64(&sentCount, 1)
			}
		}(g)
	}

	// Start receivers
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < messagesPerGoroutine; i++ {
				value, err := ch.Receive(ctx)
				testutil.AssertNoError(t, err)
				atomic.AddInt64(&receivedCount, 1)
				atomic.AddInt64(&receivedSum, int64(value))
			}
		}()
	}

	wg.Wait()

	testutil.AssertEqual(t, sentCount, int64(numGoroutines*messagesPerGoroutine))
	testutil.AssertEqual(t, receivedCount, int64(numGoroutines*messagesPerGoroutine))
	testutil.AssertEqual(t, receivedSum, expectedSum)
}

func TestStats(t *testing.T) {
	ch := New[int](5)
	defer ch.Close()

	ctx := context.Background()

	// Initial stats
	stats := ch.Stats()
	testutil.AssertEqual(t, stats.SendCount, int64(0))
	testutil.AssertEqual(t, stats.ReceiveCount, int64(0))
	testutil.AssertEqual(t, stats.DroppedCount, int64(0))

	// Send some messages
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertNoError(t, ch.Send(ctx, 2))

	stats = ch.Stats()
	testutil.AssertEqual(t, stats.SendCount, int64(2))
	testutil.AssertEqual(t, stats.BufferUtilization, 0.4) // 2/5 = 0.4

	// Receive some messages
	_, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)

	stats = ch.Stats()
	testutil.AssertEqual(t, stats.ReceiveCount, int64(1))
	testutil.AssertEqual(t, stats.BufferUtilization, 0.2) // 1/5 = 0.2
}

func TestDropStrategyStats(t *testing.T) {
	config := Config{
		BufferSize: 2,
		Strategy:   Drop,
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))
	testutil.AssertNoError(t, ch.Send(ctx, 2))

	// Drop messages
	testutil.AssertNoError(t, ch.Send(ctx, 3))
	testutil.AssertNoError(t, ch.Send(ctx, 4))

	stats := ch.Stats()
	testutil.AssertEqual(t, stats.SendCount, int64(2))    // Only successful sends
	testutil.AssertEqual(t, stats.DroppedCount, int64(2)) // Dropped messages
}

func TestBlockingStats(t *testing.T) {
	config := Config{
		BufferSize: 1,
		Strategy:   Block,
		OnBlock: func() {
			// Block callback for testing
		},
	}
	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))

	// Start blocked send
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		testutil.AssertNoError(t, ch.Send(ctx, 2))
	}()

	// Give time to block
	time.Sleep(10 * time.Millisecond)

	// Unblock by receiving
	_, err := ch.Receive(ctx)
	testutil.AssertNoError(t, err)

	wg.Wait()

	stats := ch.Stats()
	testutil.AssertEqual(t, stats.BlockedSends > 0, true)
}

func TestCallbacks(t *testing.T) {
	var dropCalled bool
	var droppedValue interface{}

	config := Config{
		BufferSize: 1,
		Strategy:   Drop,
		OnDrop: func(value interface{}) {
			dropCalled = true
			droppedValue = value
		},
	}

	ch := NewWithConfig[int](config)
	defer ch.Close()

	ctx := context.Background()

	// Fill buffer
	testutil.AssertNoError(t, ch.Send(ctx, 1))

	// Drop message
	testutil.AssertNoError(t, ch.Send(ctx, 2))

	testutil.AssertEqual(t, dropCalled, true)
	testutil.AssertEqual(t, droppedValue.(int), 2)
}

func TestCircularBuffer(t *testing.T) {
	ch := New[int](3)
	defer ch.Close()

	ctx := context.Background()

	// Fill and empty buffer multiple times to test circular nature
	for round := 0; round < 3; round++ {
		// Fill buffer
		for i := 0; i < 3; i++ {
			testutil.AssertNoError(t, ch.Send(ctx, round*3+i))
		}
		testutil.AssertEqual(t, ch.Len(), 3)

		// Empty buffer
		for i := 0; i < 3; i++ {
			val, err := ch.Receive(ctx)
			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, val, round*3+i)
		}
		testutil.AssertEqual(t, ch.Len(), 0)
	}
}

func TestDoubleClose(t *testing.T) {
	ch := New[int](5)

	// First close should succeed
	testutil.AssertNoError(t, ch.Close())
	testutil.AssertEqual(t, ch.IsClosed(), true)

	// Second close should be no-op
	testutil.AssertNoError(t, ch.Close())
	testutil.AssertEqual(t, ch.IsClosed(), true)
}

// Benchmark tests
func BenchmarkSend(b *testing.B) {
	ch := New[int](1000)
	defer ch.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Send(ctx, i)
	}
}

func BenchmarkReceive(b *testing.B) {
	ch := New[int](1000)
	defer ch.Close()

	ctx := context.Background()

	// Pre-fill channel
	for i := 0; i < b.N; i++ {
		ch.Send(ctx, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Receive(ctx)
	}
}

func BenchmarkTrySend(b *testing.B) {
	ch := New[int](1000)
	defer ch.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.TrySend(i)
	}
}

func BenchmarkTryReceive(b *testing.B) {
	ch := New[int](1000)
	defer ch.Close()

	ctx := context.Background()

	// Pre-fill channel
	for i := 0; i < b.N; i++ {
		ch.Send(ctx, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.TryReceive()
	}
}
