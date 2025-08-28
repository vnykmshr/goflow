/*
Package writer provides asynchronous, buffered writing capabilities for Go applications.

The AsyncWriter interface enables non-blocking write operations by buffering data in memory
and writing to the underlying writer in background goroutines. This provides significant
performance improvements for I/O-bound applications and enables better resource utilization.

Basic Usage:

	// Create async writer
	writer := writer.New(underlyingWriter)
	defer writer.Close()

	// Write data asynchronously (returns immediately)
	writer.WriteString("Hello, async world!")

	// Flush to ensure data is written
	writer.Flush(context.Background())

Key Features:

The async writer provides:
  - Non-blocking write operations with configurable buffer sizes
  - Automatic background flushing at configurable intervals
  - Error handling with configurable retry logic
  - Context-aware operations for cancellation and timeouts
  - Comprehensive statistics and performance monitoring
  - Thread-safe concurrent access from multiple goroutines
  - Configurable backpressure handling (blocking vs non-blocking)
  - Rich callback system for monitoring and debugging

Core Concepts:

Buffer Management:
  - Data is first written to an internal buffer
  - Background goroutines flush buffers to the underlying writer
  - Buffer size and behavior are fully configurable
  - Automatic buffer overflow handling

Background Processing:
  - Dedicated goroutines handle actual I/O operations
  - Write operations return immediately without blocking
  - Automatic flushing based on time intervals
  - Graceful shutdown with data preservation

Error Handling:
  - Configurable retry logic for failed write operations
  - Error callbacks for monitoring and logging
  - Graceful degradation under error conditions
  - Context-aware error propagation

Configuration Options:

Basic Configuration:

	config := writer.Config{
		BufferSize:    64 * 1024,           // 64KB buffer
		FlushInterval: time.Second,         // Auto-flush every second
		BlockOnFull:   true,               // Block when buffer is full
		MaxRetries:    3,                  // Retry failed writes 3 times
		RetryDelay:    100 * time.Millisecond, // Wait between retries
	}

	asyncWriter := writer.NewWithConfig(underlyingWriter, config)

Advanced Configuration with Callbacks:

	config := writer.Config{
		BufferSize:    32 * 1024,
		FlushInterval: 500 * time.Millisecond,
		BlockOnFull:   false,

		// Monitoring callbacks
		OnFlush: func(bytes int, duration time.Duration) {
			log.Printf("Flushed %d bytes in %v", bytes, duration)
		},
		OnError: func(err error) {
			log.Printf("Write error: %v", err)
		},
		OnBufferFull: func() {
			log.Println("Buffer full - consider increasing buffer size")
		},
	}

Write Operations:

Multiple ways to write data:

	// Write byte slice
	err := writer.Write([]byte("data"))

	// Write string
	err := writer.WriteString("text data")

	// Write with context (for cancellation)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := writer.WriteContext(ctx, []byte("context-aware write"))

Buffer Management:

Control buffer behavior:

	// Check buffer status
	size := writer.BufferSize()        // Current buffered bytes
	capacity := writer.BufferCapacity() // Maximum buffer capacity

	// Manual flush
	err := writer.Flush(context.Background())

	// Graceful shutdown (flushes remaining data)
	err := writer.Close()

Backpressure Handling:

Two modes for handling full buffers:

	// Blocking mode (default) - waits for space
	config := writer.Config{
		BlockOnFull: true,
	}

	// Non-blocking mode - returns ErrBufferFull
	config := writer.Config{
		BlockOnFull: false,
		OnBufferFull: func() {
			// Handle buffer full condition
		},
	}

Performance Monitoring:

Comprehensive statistics:

	stats := writer.Stats()

	fmt.Printf("Total writes: %d\n", stats.WriteCount)
	fmt.Printf("Bytes written: %d\n", stats.BytesWritten)
	fmt.Printf("Flush operations: %d\n", stats.FlushCount)
	fmt.Printf("Error count: %d\n", stats.ErrorCount)
	fmt.Printf("Buffer overflows: %d\n", stats.BufferOverflows)
	fmt.Printf("Average write time: %v\n", stats.AverageWriteTime)
	fmt.Printf("Buffer utilization: %.1f%%\n", stats.BufferUtilization*100)

Use Cases:

High-Throughput Logging:

	logFile, _ := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer logFile.Close()

	config := writer.DefaultConfig()
	config.BufferSize = 1024 * 1024    // 1MB buffer for high throughput
	config.FlushInterval = time.Second  // Flush every second

	asyncWriter := writer.NewWithConfig(logFile, config)
	defer asyncWriter.Close()

	// High-frequency logging doesn't block
	for _, logEntry := range logEntries {
		asyncWriter.WriteString(logEntry + "\n")
	}

Network Data Streaming:

	conn, _ := net.Dial("tcp", "server:8080")
	defer conn.Close()

	config := writer.Config{
		BufferSize:  8 * 1024,             // 8KB network buffer
		BlockOnFull: false,                // Don't block on full buffer
		MaxRetries:  5,                    // Retry network failures
		RetryDelay:  100 * time.Millisecond,
		OnError: func(err error) {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Handle network timeouts
			}
		},
	}

	networkWriter := writer.NewWithConfig(conn, config)
	defer networkWriter.Close()

	// Stream data without blocking
	for _, dataChunk := range dataStream {
		err := networkWriter.Write(dataChunk)
		if err == writer.ErrBufferFull {
			// Handle backpressure
			time.Sleep(10 * time.Millisecond)
			continue
		}
	}

Batch Processing:

	file, _ := os.Create("batch_output.txt")
	defer file.Close()

	batchWriter := writer.New(file)
	defer batchWriter.Close()

	// Process items in batches
	for _, batch := range batches {
		for _, item := range batch {
			batchWriter.WriteString(item.String() + "\n")
		}
		// Flush after each batch
		batchWriter.Flush(context.Background())
	}

Concurrent Data Collection:

	var buf bytes.Buffer
	collector := writer.New(&buf)
	defer collector.Close()

	var wg sync.WaitGroup

	// Multiple goroutines writing concurrently
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for _, data := range workerData[workerID] {
				collector.WriteString(fmt.Sprintf("Worker%d: %s\n", workerID, data))
			}
		}(i)
	}

	wg.Wait()
	collector.Flush(context.Background())

Error Handling Strategies:

Retry with Exponential Backoff:

	config := writer.Config{
		MaxRetries: 5,
		RetryDelay: 100 * time.Millisecond,
		OnError: func(err error) {
			log.Printf("Write failed, retrying: %v", err)
		},
	}

Circuit Breaker Pattern:

	var errorCount int64

	config := writer.Config{
		OnError: func(err error) {
			atomic.AddInt64(&errorCount, 1)
			if atomic.LoadInt64(&errorCount) > errorThreshold {
				// Implement circuit breaker logic
				switchToBackupWriter()
			}
		},
	}

Graceful Degradation:

	config := writer.Config{
		BlockOnFull: false,
		OnBufferFull: func() {
			// Drop non-critical writes or switch to synchronous mode
			fallbackToSyncWrite()
		},
	}

Context and Cancellation:

Timeout Handling:

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := writer.WriteContext(ctx, data)
	if err == context.DeadlineExceeded {
		log.Println("Write operation timed out")
	}

	err = writer.Flush(ctx)
	if err == context.DeadlineExceeded {
		log.Println("Flush operation timed out")
	}

Graceful Shutdown:

	// Create cancellable context for shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Shutdown signal handler
	go func() {
		<-shutdownSignal
		cancel() // Cancel all operations
	}()

	// Use context in operations
	err := writer.WriteContext(ctx, data)
	if err == context.Canceled {
		log.Println("Write canceled due to shutdown")
	}

Performance Characteristics:

Throughput:
  - Write operations: O(1) - immediate return to buffer
  - Memory usage: O(buffer_size) - bounded by configuration
  - Background I/O: depends on underlying writer performance
  - Concurrent writes: excellent scalability with internal synchronization

Latency:
  - Write operations: microseconds (buffer copy)
  - Flush operations: depends on underlying writer
  - Auto-flush: configurable interval vs latency trade-off
  - Error recovery: bounded by retry configuration

Memory:
  - Buffer memory: user-configurable
  - Goroutine overhead: 2-3 goroutines per writer instance
  - Internal structures: minimal overhead
  - No memory leaks: proper cleanup in Close()

Best Practices:

 1. Buffer Sizing:
    // Size buffers based on write patterns and available memory
    config.BufferSize = averageWriteSize * expectedBurstCount

 2. Flush Intervals:
    // Balance latency vs performance
    config.FlushInterval = time.Duration(bufferSize / throughputBytes) * time.Second

 3. Error Handling:
    // Always handle errors appropriately
    if err := writer.WriteString(data); err != nil {
    log.Printf("Write failed: %v", err)
    }

 4. Resource Management:
    // Always close writers to prevent resource leaks
    defer writer.Close()

 5. Monitoring:
    // Use statistics for performance tuning
    stats := writer.Stats()
    if stats.BufferUtilization > 0.8 {
    // Consider increasing buffer size
    }

 6. Context Usage:
    // Use contexts for timeout and cancellation
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

 7. Concurrent Access:
    // Writers are thread-safe - no external synchronization needed
    go routineA() { writer.WriteString("A") }
    go routineB() { writer.WriteString("B") }

Comparison with Alternatives:

vs Standard io.Writer:
  - Non-blocking operations
  - Better performance for frequent writes
  - Built-in buffering and retry logic
  - Additional memory usage
  - More complex error handling

vs bufio.Writer:
  - Asynchronous operations
  - Context support
  - Comprehensive monitoring
  - Thread-safe
  - Higher memory overhead
  - More complex lifecycle management

vs Channel-based Patterns:
  - Higher-level abstraction
  - Built-in error handling and retries
  - Performance monitoring
  - Less flexibility for custom flow control
  - Additional goroutine overhead

Thread Safety:

All AsyncWriter operations are safe for concurrent use:

	// Safe: multiple goroutines can write simultaneously
	go writer.WriteString("goroutine 1")
	go writer.WriteString("goroutine 2")
	go writer.WriteString("goroutine 3")

	// Safe: concurrent flush operations
	go writer.Flush(ctx1)
	go writer.Flush(ctx2)

	// Safe: mixed operations
	go writer.WriteString("data")
	go writer.Flush(context.Background())
	go stats := writer.Stats()

Integration Patterns:

With HTTP Servers:

	func handler(w http.ResponseWriter, r *http.Request) {
		asyncWriter := writer.New(w)
		defer asyncWriter.Close()

		// Stream response data
		for _, chunk := range responseData {
			asyncWriter.Write(chunk)
		}

		asyncWriter.Flush(r.Context())
	}

With Logging Frameworks:

	type AsyncLogWriter struct {
		writer writer.AsyncWriter
	}

	func (alw *AsyncLogWriter) Write(p []byte) (n int, err error) {
		return len(p), alw.writer.Write(p)
	}

	// Use with standard log package
	logWriter := &AsyncLogWriter{writer: writer.New(logFile)}
	log.SetOutput(logWriter)

With Data Pipelines:

	// Processing pipeline with async writing
	pipeline := make(chan Data, 100)

	go func() {
		for data := range pipeline {
			processedData := process(data)
			asyncWriter.Write(processedData.Bytes())
		}
	}()

Common Pitfalls and Solutions:

 1. Forgetting to Close:
    // Always ensure proper cleanup
    defer writer.Close()

 2. Not Handling Buffer Full:
    if err == writer.ErrBufferFull {
    // Implement backpressure handling
    }

 3. Ignoring Context Cancellation:
    // Always check context in long-running operations
    select {
    case <-ctx.Done():
    return ctx.Err()
    default:
    // Continue processing
    }

 4. Inadequate Error Handling:
    // Monitor errors and implement appropriate recovery
    config.OnError = func(err error) {
    metrics.ErrorCounter.Inc()
    if shouldReconnect(err) {
    reconnectWriter()
    }
    }

 5. Poor Buffer Sizing:
    // Profile your application to determine optimal buffer sizes
    // Too small: frequent flushes, poor performance
    // Too large: high memory usage, increased latency

Future Enhancements:

Planned features for future versions:
  - Compression integration (gzip, lz4)
  - Automatic buffer size tuning
  - Pluggable flush strategies
  - Metrics integration (Prometheus, etc.)
  - Advanced backpressure algorithms
  - Buffer pooling for reduced GC pressure
*/
package writer
