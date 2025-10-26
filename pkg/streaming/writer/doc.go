/*
Package writer provides asynchronous buffered writing for Go applications.

AsyncWriter buffers data in memory and writes to the underlying writer in the background,
improving performance for I/O-bound workloads.

# Quick Start

	file, _ := os.Create("output.txt")
	w := writer.New(file)
	defer w.Close()

	w.WriteString("Hello, async world!")
	w.Flush(context.Background())

# Configuration

Configure buffer size, flush intervals, and error handling:

	config := writer.Config{
		BufferSize:    64 * 1024,           // 64KB buffer
		FlushInterval: time.Second,         // Auto-flush every second
		BlockOnFull:   true,                // Block when buffer full
		MaxRetries:    3,                   // Retry failed writes
	}

	w := writer.NewWithConfig(underlyingWriter, config)

# Writing Data

	// Write bytes
	w.Write([]byte("data"))

	// Write string
	w.WriteString("text")

	// Write with context (for cancellation/timeout)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	w.WriteContext(ctx, []byte("data"))

# Backpressure Handling

When the buffer fills up, you can choose to block or drop writes:

	config := writer.Config{
		BlockOnFull: true,  // Block until space available
	}

	// Or

	config := writer.Config{
		BlockOnFull: false, // Drop writes when full
		OnBufferFull: func() {
			log.Println("Buffer full, dropping write")
		},
	}

# Monitoring

Track performance with callbacks:

	config := writer.Config{
		OnFlush: func(bytes int, duration time.Duration) {
			log.Printf("Flushed %d bytes in %v", bytes, duration)
		},
		OnError: func(err error) {
			log.Printf("Write error: %v", err)
		},
	}

# Statistics

Get current buffer stats:

	stats := w.Stats()
	fmt.Printf("Written: %d bytes, Flushed: %d bytes, Pending: %d bytes\n",
		stats.BytesWritten, stats.BytesFlushed, stats.BytesPending)

# Graceful Shutdown

Always close the writer to flush remaining data:

	defer w.Close()  // Flushes and waits for completion

	// Or with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w.FlushAndClose(ctx)

# Use Cases

High-Volume Logging:

	logWriter := writer.New(logFile)
	defer logWriter.Close()

	for _, entry := range logEntries {
		logWriter.WriteString(entry + "\n")
	}

Network Streaming:

	conn, _ := net.Dial("tcp", "server:8080")
	w := writer.NewWithConfig(conn, writer.Config{
		BufferSize:    8 * 1024,
		FlushInterval: 100 * time.Millisecond,
	})

	for data := range dataChannel {
		w.Write(data)
	}

File Writing with Error Handling:

	file, _ := os.Create("output.dat")
	w := writer.NewWithConfig(file, writer.Config{
		MaxRetries: 5,
		OnError: func(err error) {
			log.Printf("Write failed: %v", err)
		},
	})

	w.Write(largeData)
	w.Flush(context.Background())

# Thread Safety

AsyncWriter is safe for concurrent use from multiple goroutines.

# Performance Notes

- Larger buffers reduce syscalls but increase memory usage
- Shorter flush intervals reduce data loss risk but increase overhead
- Non-blocking mode improves write latency but may drop data
- Retries help with transient errors but can hide persistent issues

See example tests for more usage patterns.
*/
package writer
