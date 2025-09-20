package writer

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Example demonstrates basic async writer usage.
func Example() {
	// Create an underlying writer (could be file, network, etc.)
	var buf bytes.Buffer

	// Create async writer with default configuration
	writer := New(&buf)
	defer func() { _ = writer.Close() }()

	// Write data asynchronously - returns immediately
	_ = writer.WriteString("Hello, ")
	_ = writer.WriteString("async ")
	_ = writer.WriteString("world!")

	// Flush to ensure all data is written
	_ = writer.Flush(context.Background())

	fmt.Println(buf.String())
	// Output: Hello, async world!
}

// Example_configuration demonstrates various configuration options.
func Example_configuration() {
	var buf bytes.Buffer

	// Custom configuration
	config := Config{
		BufferSize:    1024,                   // 1KB buffer
		FlushInterval: 100 * time.Millisecond, // Auto-flush every 100ms
		BlockOnFull:   true,                   // Block when buffer is full
		MaxRetries:    5,                      // Retry failed writes
		RetryDelay:    50 * time.Millisecond,  // Delay between retries
	}

	writer := NewWithConfig(&buf, config)

	// Write data
	_ = writer.WriteString("Configured writer example")

	// Let auto-flush handle the writing
	time.Sleep(150 * time.Millisecond)

	// Ensure all background processing is complete
	_ = writer.Flush(context.Background())
	_ = writer.Close()

	fmt.Println(buf.String())
	// Output: Configured writer example
}

// Example_nonBlocking demonstrates non-blocking behavior.
func Example_nonBlocking() {
	var buf bytes.Buffer

	config := DefaultConfig()
	config.BlockOnFull = false // Don't block on full buffer
	config.BufferSize = 20     // Small buffer for demo

	writer := NewWithConfig(&buf, config)

	// Write data that fits in buffer
	err := writer.WriteString("fits")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Small write succeeded")
	}

	// Try to write large data that exceeds buffer
	largeData := strings.Repeat("x", 30)
	err = writer.WriteString(largeData)
	if err == ErrBufferFull {
		fmt.Println("Large write failed: buffer full")
	}

	// Flush the successful write
	_ = writer.Flush(context.Background())
	_ = writer.Close()
	fmt.Printf("Buffer contains: %s\n", buf.String())

	// Output:
	// Small write succeeded
	// Large write failed: buffer full
	// Buffer contains: fits
}

// Example_callbacks demonstrates using callbacks for monitoring.
func Example_callbacks() {
	var buf bytes.Buffer

	config := DefaultConfig()
	config.OnFlush = func(bytes int, _ time.Duration) {
		fmt.Printf("Flushed %d bytes\n", bytes)
	}
	config.OnBufferFull = func() {
		fmt.Println("Buffer is full!")
	}
	config.OnError = func(err error) {
		fmt.Printf("Write error: %v\n", err)
	}

	writer := NewWithConfig(&buf, config)
	defer func() { _ = writer.Close() }()

	// Write some data
	_ = writer.WriteString("Hello with callbacks")

	// Flush to trigger callback
	_ = writer.Flush(context.Background())

	// Output:
	// Flushed 20 bytes
}

// Example_statistics demonstrates monitoring writer performance.
func Example_statistics() {
	var buf bytes.Buffer
	writer := New(&buf)
	defer func() { _ = writer.Close() }()

	// Perform some writes
	for i := 0; i < 5; i++ {
		_ = writer.WriteString(fmt.Sprintf("Write %d\n", i))
	}

	// Flush and get stats
	_ = writer.Flush(context.Background())
	stats := writer.Stats()

	fmt.Printf("Writes: %d\n", stats.WriteCount)
	fmt.Printf("Bytes: %d\n", stats.BytesWritten)
	fmt.Printf("Flushes: %d\n", stats.FlushCount)
	fmt.Printf("Errors: %d\n", stats.ErrorCount)
	fmt.Printf("Buffer utilization: %.1f%%\n", stats.BufferUtilization*100)

	// Output:
	// Writes: 5
	// Bytes: 40
	// Flushes: 1
	// Errors: 0
	// Buffer utilization: 0.0%
}

// Example_fileWriting demonstrates writing to a file asynchronously.
func Example_fileWriting() {
	// Create a temporary file
	file, err := os.CreateTemp("", "async_writer_example_*.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.Remove(file.Name()) }() // Clean up

	// Create async writer for the file
	writer := New(file)
	defer func() { _ = writer.Close() }()

	// Write data asynchronously
	for i := 0; i < 3; i++ {
		_ = writer.WriteString(fmt.Sprintf("Line %d\n", i+1))
	}

	// Flush to ensure data is written
	_ = writer.Flush(context.Background())

	// Close the file
	_ = file.Close()

	// Read back and display
	content, err := os.ReadFile(file.Name())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(content))
	// Output:
	// Line 1
	// Line 2
	// Line 3
}

// Example_concurrent demonstrates concurrent writes from multiple goroutines.
func Example_concurrent() {
	var buf bytes.Buffer
	writer := New(&buf)
	defer func() { _ = writer.Close() }()

	var wg sync.WaitGroup
	numGoroutines := 3
	writesPerGoroutine := 3

	// Start multiple goroutines writing concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				_ = writer.WriteString(fmt.Sprintf("G%d-W%d ", id, j))
			}
		}(i)
	}

	wg.Wait()

	// Flush all writes
	_ = writer.Flush(context.Background())

	result := buf.String()

	// Count the writes (should be 9 total)
	writeCount := len(strings.Fields(result))
	fmt.Printf("Total writes: %d\n", writeCount)

	// Check stats
	stats := writer.Stats()
	fmt.Printf("Recorded writes: %d\n", stats.WriteCount)

	// Output:
	// Total writes: 9
	// Recorded writes: 9
}

// Example_errorHandling demonstrates error handling and retries.
func Example_errorHandling() {
	// Create a writer that will fail initially
	failingWriter := &failingWriter{failCount: 2}

	config := DefaultConfig()
	config.MaxRetries = 3
	config.RetryDelay = 1 * time.Millisecond

	writer := NewWithConfig(failingWriter, config)
	defer func() { _ = writer.Close() }()

	// Write data (will succeed after retries)
	_ = writer.WriteString("persistent data")

	err := writer.Flush(context.Background())
	if err != nil {
		fmt.Printf("Final error: %v\n", err)
	} else {
		fmt.Println("Write succeeded after retries")
		fmt.Printf("Final data: %s\n", failingWriter.buf.String())
	}

	// Output:
	// Write succeeded after retries
	// Final data: persistent data
}

// Example_contextCancellation demonstrates context-aware operations.
func Example_contextCancellation() {
	var buf bytes.Buffer
	writer := New(&buf)
	defer func() { _ = writer.Close() }()

	// Write some data
	_ = writer.WriteString("context test")

	// Create a context that will timeout quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// This flush will likely be canceled
	err := writer.Flush(ctx)
	if err == context.DeadlineExceeded {
		fmt.Println("Flush was canceled due to timeout")
	} else {
		fmt.Println("Flush completed")
	}

	// Try again with proper context
	err = writer.Flush(context.Background())
	if err == nil {
		fmt.Println("Second flush succeeded")
		fmt.Printf("Data: %s\n", buf.String())
	}

	// Output:
	// Flush was canceled due to timeout
	// Second flush succeeded
	// Data: context test
}

// Example_autoFlush demonstrates automatic flushing.
func Example_autoFlush() {
	var buf bytes.Buffer

	config := DefaultConfig()
	config.FlushInterval = 50 * time.Millisecond // Auto-flush every 50ms

	writer := NewWithConfig(&buf, config)

	// Write data
	_ = writer.WriteString("auto-flush example")

	// Wait for auto-flush
	time.Sleep(100 * time.Millisecond)

	// Ensure all background processing is complete
	_ = writer.Flush(context.Background())
	_ = writer.Close()

	// Data should be automatically written
	fmt.Printf("Auto-flushed data: %s\n", buf.String())

	// Output:
	// Auto-flushed data: auto-flush example
}

// failingWriter is a test writer that fails for the first few writes
type failingWriter struct {
	buf       bytes.Buffer
	failCount int
	attempts  int
}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	fw.attempts++
	if fw.attempts <= fw.failCount {
		return 0, fmt.Errorf("simulated failure")
	}
	return fw.buf.Write(p)
}
