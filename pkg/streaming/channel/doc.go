/*
Package channel provides backpressure-aware channels for flow control in concurrent applications.

The channel package implements a sophisticated channel abstraction that goes beyond Go's built-in
channels by providing configurable backpressure strategies, comprehensive statistics, and
context-aware operations.

Core Features:

Backpressure channels address the common problem in producer-consumer scenarios where producers
generate data faster than consumers can process it. This package provides multiple strategies
to handle such scenarios gracefully.

Key Components:
  - BackpressureChannel: Generic channel with configurable backpressure handling
  - Multiple backpressure strategies (Block, Drop, DropOldest, Error)
  - Context-aware operations with timeout support
  - Comprehensive performance monitoring and statistics
  - Thread-safe concurrent access

Backpressure Strategies:

Block Strategy:
The default strategy that blocks producers when the buffer is full, providing natural
flow control by slowing down fast producers.

	config := Config{
		BufferSize: 10,
		Strategy:   Block,
	}
	ch := NewWithConfig[int](config)

	// This will block if buffer is full
	err := ch.Send(ctx, value)

Drop Strategy:
Drops new messages when the buffer is full, preserving the oldest data. Useful when
newer data is less important than preserving existing data.

	config := Config{
		BufferSize: 10,
		Strategy:   Drop,
		OnDrop: func(value interface{}) {
			log.Printf("Dropped message: %v", value)
		},
	}
	ch := NewWithConfig[int](config)

DropOldest Strategy:
Drops the oldest messages when the buffer is full, preserving the newest data.
Useful when recent data is more important than historical data.

	config := Config{
		BufferSize: 10,
		Strategy:   DropOldest,
		OnDrop: func(value interface{}) {
			log.Printf("Dropped oldest: %v", value)
		},
	}
	ch := NewWithConfig[int](config)

Error Strategy:
Returns an error when trying to send to a full buffer, allowing the application
to decide how to handle the situation.

	config := Config{
		BufferSize: 10,
		Strategy:   Error,
	}
	ch := NewWithConfig[int](config)

	err := ch.Send(ctx, value)
	if err == ErrChannelFull {
		// Handle full buffer
	}

Basic Usage:

Creating Channels:

	// Simple channel with default configuration
	ch := New[string](100)
	defer ch.Close()

	// Channel with custom configuration
	config := Config{
		BufferSize: 50,
		Strategy:   DropOldest,
		SendTimeout: 5 * time.Second,
	}
	ch := NewWithConfig[Message](config)
	defer ch.Close()

Sending and Receiving:

	ctx := context.Background()

	// Blocking send/receive
	err := ch.Send(ctx, "hello")
	value, err := ch.Receive(ctx)

	// Non-blocking send/receive
	err := ch.TrySend("world")
	value, ok, err := ch.TryReceive()
	if !ok {
		// No data available
	}

Context and Timeout Support:

All operations support context cancellation and timeouts for robust error handling:

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ch.Send(ctx, data)
	if err == context.DeadlineExceeded {
		// Operation timed out
	}

	// Configuration-based timeouts
	config := Config{
		SendTimeout:    100 * time.Millisecond,
		ReceiveTimeout: 200 * time.Millisecond,
	}

Performance Monitoring:

The channel provides comprehensive statistics for monitoring performance and
debugging bottlenecks:

	stats := ch.Stats()
	fmt.Printf("Send count: %d\n", stats.SendCount)
	fmt.Printf("Receive count: %d\n", stats.ReceiveCount)
	fmt.Printf("Dropped count: %d\n", stats.DroppedCount)
	fmt.Printf("Blocked sends: %d\n", stats.BlockedSends)
	fmt.Printf("Buffer utilization: %.1f%%\n", stats.BufferUtilization*100)
	fmt.Printf("Average send time: %v\n", stats.AverageSendTime)

Common Patterns:

Producer-Consumer with Backpressure:

	jobs := New[Job](50) // Buffer limits memory usage
	results := New[Result](100)

	// Producer (may be slowed by backpressure)
	go func() {
		for job := range jobSource {
			jobs.Send(ctx, job) // Blocks when buffer is full
		}
		jobs.Close()
	}()

	// Consumer
	go func() {
		for {
			job, err := jobs.Receive(ctx)
			if err != nil {
				break // Channel closed
			}
			result := processJob(job)
			results.Send(ctx, result)
		}
		results.Close()
	}()

Rate Limiting:

	// Limit concurrent operations
	limiter := New[struct{}](maxConcurrent)

	for _, task := range tasks {
		go func(t Task) {
			// Acquire permit
			limiter.Send(ctx, struct{}{})
			defer limiter.Receive(ctx) // Release permit

			processTask(t)
		}(task)
	}

Event Streaming with Data Loss Tolerance:

	config := Config{
		BufferSize: 1000,
		Strategy:   DropOldest, // Keep latest events
		OnDrop: func(value interface{}) {
			metrics.DroppedEvents.Inc()
		},
	}
	events := NewWithConfig[Event](config)

	// Fast producer
	go func() {
		for event := range eventStream {
			events.TrySend(event) // Never blocks
		}
	}()

Request Batching:

	requests := New[Request](100)

	// Batch processor
	go func() {
		batch := make([]Request, 0, 10)
		ticker := time.NewTicker(100 * time.Millisecond)

		for {
			select {
			case req, err := <-requests.Receive(ctx):
				if err != nil {
					return
				}
				batch = append(batch, req)

				if len(batch) >= 10 {
					processBatch(batch)
					batch = batch[:0]
				}

			case <-ticker.C:
				if len(batch) > 0 {
					processBatch(batch)
					batch = batch[:0]
				}
			}
		}
	}()

Circuit Breaker Pattern:

	config := Config{
		BufferSize: 10,
		Strategy:   Error,
	}
	ch := NewWithConfig[Request](config)

	func handleRequest(req Request) error {
		err := ch.TrySend(req)
		if err == ErrChannelFull {
			// Circuit breaker logic
			return errors.New("service overloaded")
		}
		return nil
	}

Error Handling:

The channel provides specific error types for different failure modes:

	err := ch.Send(ctx, data)
	switch {
	case err == nil:
		// Success
	case err == ErrChannelFull:
		// Buffer is full (Error strategy)
	case err == ErrChannelClosed:
		// Channel was closed
	case err == context.DeadlineExceeded:
		// Operation timed out
	case err == context.Canceled:
		// Context was canceled
	default:
		// Other error
	}

Channel Lifecycle:

Proper resource management is important for preventing goroutine leaks:

	ch := New[Data](100)

	// Always close channels when done
	defer ch.Close()

	// Or close explicitly after use
	func processData(data []Data) error {
		ch := New[Data](10)
		defer ch.Close()

		// Use channel...
		return nil
	}

Integration Patterns:

Worker Pool Integration:

	type WorkerPool struct {
		jobs    BackpressureChannel[Job]
		results BackpressureChannel[Result]
		workers int
	}

	func NewWorkerPool(workers, bufferSize int) *WorkerPool {
		return &WorkerPool{
			jobs:    New[Job](bufferSize),
			results: New[Result](bufferSize),
			workers: workers,
		}
	}

	func (p *WorkerPool) Start(ctx context.Context) {
		for i := 0; i < p.workers; i++ {
			go p.worker(ctx)
		}
	}

	func (p *WorkerPool) Submit(job Job) error {
		return p.jobs.TrySend(job)
	}

Pipeline Processing:

	stage1 := New[RawData](100)
	stage2 := New[ProcessedData](100)
	stage3 := New[EnrichedData](100)

	// Pipeline stages
	go processStage1(stage1, stage2)
	go processStage2(stage2, stage3)
	go processStage3(stage3, output)

Load Balancing:

	workers := make([]BackpressureChannel[Task], numWorkers)
	for i := range workers {
		workers[i] = New[Task](workerBufferSize)
	}

	// Round-robin distribution
	func distributeTask(task Task) error {
		worker := workers[taskID % len(workers)]
		return worker.TrySend(task)
	}

Performance Characteristics:

Memory Usage:
- Fixed memory allocation based on buffer size
- No dynamic memory allocation during normal operation
- Memory is released when channel is closed

Throughput:
- High throughput for buffered operations
- Minimal locking overhead with optimized synchronization
- Performance scales well with buffer size

Latency:
- Low latency for non-blocking operations (TrySend/TryReceive)
- Blocking operations depend on consumer processing speed
- Context cancellation provides prompt operation termination

Scalability:
- Designed for high-concurrency scenarios
- Lock-free statistics updates where possible
- Efficient circular buffer implementation

Best Practices:

1. Buffer Sizing:
  - Size buffers based on expected burst capacity
  - Consider memory constraints vs. throughput requirements
  - Monitor buffer utilization to optimize sizing

2. Strategy Selection:
  - Use Block for flow control and natural backpressure
  - Use Drop/DropOldest when data loss is acceptable
  - Use Error for explicit error handling

3. Context Usage:
  - Always use context for cancellation and timeouts
  - Set appropriate timeouts based on SLA requirements
  - Handle context errors gracefully

4. Monitoring:
  - Use statistics to identify bottlenecks
  - Monitor dropped message rates
  - Track buffer utilization trends

5. Resource Management:
  - Always close channels when done
  - Use defer for automatic cleanup
  - Avoid goroutine leaks by proper error handling

6. Error Handling:
  - Handle all error cases explicitly
  - Use appropriate strategies for different error types
  - Implement circuit breaker patterns for overload protection

Comparison with Standard Channels:

Standard Go channels provide basic communication but lack advanced flow control:

	// Standard channel - limited options
	ch := make(chan int, 100)

	// Backpressure channel - rich configuration
	ch := NewWithConfig[int](Config{
		BufferSize: 100,
		Strategy:   DropOldest,
		OnDrop:     logDroppedMessage,
		SendTimeout: 1 * time.Second,
	})

Advantages of backpressure channels:
- Configurable backpressure handling
- Built-in statistics and monitoring
- Context-aware operations with timeouts
- Callback support for events
- Multiple access patterns (blocking/non-blocking)

Use backpressure channels when you need:
- Advanced flow control beyond basic buffering
- Monitoring and observability of channel operations
- Flexible error handling strategies
- Integration with context-based cancellation

Thread Safety:

All operations on BackpressureChannel are thread-safe and can be called concurrently
from multiple goroutines. The implementation uses efficient synchronization primitives
to minimize contention while ensuring correctness.

	// Safe to use from multiple goroutines
	go func() {
		for data := range producer {
			ch.Send(ctx, data)
		}
	}()

	go func() {
		for {
			data, err := ch.Receive(ctx)
			if err != nil {
				break
			}
			process(data)
		}
	}()

The channel implementation is designed for high-performance concurrent access while
maintaining strong consistency guarantees.
*/
package channel
