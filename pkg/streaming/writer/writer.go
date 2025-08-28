package writer

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ErrWriterClosed is returned when attempting to write to a closed writer.
var ErrWriterClosed = errors.New("writer is closed")

// ErrBufferFull is returned when the internal buffer is full and cannot accept more data.
var ErrBufferFull = errors.New("buffer is full")

// AsyncWriter provides asynchronous, buffered writing capabilities.
// It buffers write operations in memory and flushes them to the underlying
// writer in a background goroutine, providing non-blocking write operations.
type AsyncWriter interface {
	// Write writes data asynchronously. Returns immediately without blocking.
	// If the buffer is full and blocking is disabled, returns ErrBufferFull.
	Write(data []byte) error

	// WriteString writes a string asynchronously.
	WriteString(s string) error

	// WriteContext writes data with context support for cancellation.
	WriteContext(ctx context.Context, data []byte) error

	// Flush forces all buffered data to be written to the underlying writer.
	// This operation blocks until all data is flushed or context is canceled.
	Flush(ctx context.Context) error

	// Close gracefully shuts down the writer, flushing any remaining data.
	// After Close returns, no more writes are accepted.
	Close() error

	// Stats returns statistics about the writer's performance.
	Stats() Stats

	// IsClosed returns true if the writer is closed.
	IsClosed() bool

	// BufferSize returns the current number of buffered bytes.
	BufferSize() int

	// BufferCapacity returns the maximum buffer capacity.
	BufferCapacity() int
}

// Stats holds statistics about async writer performance.
type Stats struct {
	// BytesWritten is the total number of bytes written.
	BytesWritten int64

	// WriteCount is the total number of write operations.
	WriteCount int64

	// FlushCount is the total number of flush operations.
	FlushCount int64

	// ErrorCount is the total number of errors encountered.
	ErrorCount int64

	// BufferOverflows is the number of times the buffer was full.
	BufferOverflows int64

	// AverageWriteTime is the average time per write operation.
	AverageWriteTime time.Duration

	// TotalWriteTime is the total time spent writing.
	TotalWriteTime time.Duration

	// LastWriteTime is the timestamp of the last write operation.
	LastWriteTime time.Time

	// BufferUtilization is the current buffer utilization (0.0 to 1.0).
	BufferUtilization float64
}

// Config holds configuration options for AsyncWriter.
type Config struct {
	// BufferSize is the size of the internal buffer in bytes.
	// Default: 64KB
	BufferSize int

	// FlushInterval is how often to flush the buffer automatically.
	// Set to 0 to disable automatic flushing.
	// Default: 1 second
	FlushInterval time.Duration

	// BlockOnFull determines behavior when buffer is full.
	// If true, Write operations will block until space is available.
	// If false, Write operations will return ErrBufferFull immediately.
	// Default: true
	BlockOnFull bool

	// MaxRetries is the number of times to retry failed write operations.
	// Default: 3
	MaxRetries int

	// RetryDelay is the delay between retries.
	// Default: 100ms
	RetryDelay time.Duration

	// OnError is called when write errors occur.
	OnError func(error)

	// OnFlush is called after each flush operation.
	OnFlush func(bytesWritten int, duration time.Duration)

	// OnBufferFull is called when the buffer becomes full.
	OnBufferFull func()
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		BufferSize:    64 * 1024, // 64KB
		FlushInterval: time.Second,
		BlockOnFull:   true,
		MaxRetries:    3,
		RetryDelay:    100 * time.Millisecond,
	}
}

// writeRequest represents a write operation request.
type writeRequest struct {
	data []byte
	ctx  context.Context
	done chan error // Channel to signal completion
}

// asyncWriter implements AsyncWriter.
type asyncWriter struct {
	underlying io.Writer
	config     Config

	// Buffer and synchronization
	buffer   []byte
	bufferMu sync.RWMutex

	// Communication channels
	writeCh chan writeRequest
	flushCh chan chan error
	closeCh chan chan error

	// Background goroutine management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// State
	closed int32 // atomic

	// Statistics
	stats     Stats
	statsMu   sync.RWMutex
	startTime time.Time
}

// New creates a new AsyncWriter with default configuration.
func New(w io.Writer) AsyncWriter {
	return NewWithConfig(w, DefaultConfig())
}

// NewWithConfig creates a new AsyncWriter with the specified configuration.
func NewWithConfig(w io.Writer, config Config) AsyncWriter {
	if config.BufferSize <= 0 {
		config.BufferSize = DefaultConfig().BufferSize
	}
	if config.MaxRetries < 0 {
		config.MaxRetries = DefaultConfig().MaxRetries
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = DefaultConfig().RetryDelay
	}

	ctx, cancel := context.WithCancel(context.Background())

	aw := &asyncWriter{
		underlying: w,
		config:     config,
		buffer:     make([]byte, 0, config.BufferSize),
		writeCh:    make(chan writeRequest, 100), // Buffered channel for requests
		flushCh:    make(chan chan error, 10),
		closeCh:    make(chan chan error, 1),
		ctx:        ctx,
		cancel:     cancel,
		startTime:  time.Now(),
	}

	// Start background writer goroutine
	aw.wg.Add(1)
	go aw.writerLoop()

	// Start automatic flush goroutine if enabled
	if config.FlushInterval > 0 {
		aw.wg.Add(1)
		go aw.flushLoop()
	}

	return aw
}

// Write implements AsyncWriter.Write.
func (aw *asyncWriter) Write(data []byte) error {
	return aw.WriteContext(context.Background(), data)
}

// WriteString implements AsyncWriter.WriteString.
func (aw *asyncWriter) WriteString(s string) error {
	return aw.WriteContext(context.Background(), []byte(s))
}

// WriteContext implements AsyncWriter.WriteContext.
//
//nolint:gocyclo
func (aw *asyncWriter) WriteContext(ctx context.Context, data []byte) error {
	if aw.IsClosed() {
		return ErrWriterClosed
	}

	if len(data) == 0 {
		return nil
	}

	// Check if buffer would overflow
	aw.bufferMu.RLock()
	bufferLen := len(aw.buffer)
	wouldOverflow := bufferLen+len(data) > cap(aw.buffer)
	aw.bufferMu.RUnlock()

	if wouldOverflow && !aw.config.BlockOnFull {
		aw.updateStats(func(s *Stats) {
			s.BufferOverflows++
		})
		if aw.config.OnBufferFull != nil {
			aw.config.OnBufferFull()
		}
		return ErrBufferFull
	}

	// Create write request
	req := writeRequest{
		data: make([]byte, len(data)),
		ctx:  ctx,
		done: make(chan error, 1),
	}
	copy(req.data, data)

	// Send request to background goroutine
	select {
	case aw.writeCh <- req:
		// Request sent successfully
	case <-ctx.Done():
		return ctx.Err()
	case <-aw.ctx.Done():
		return ErrWriterClosed
	}

	// Wait for completion if blocking
	if aw.config.BlockOnFull {
		select {
		case err := <-req.done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-aw.ctx.Done():
			return ErrWriterClosed
		}
	}

	return nil
}

// Flush implements AsyncWriter.Flush.
func (aw *asyncWriter) Flush(ctx context.Context) error {
	if aw.IsClosed() {
		return ErrWriterClosed
	}

	done := make(chan error, 1)

	select {
	case aw.flushCh <- done:
		// Request sent
	case <-ctx.Done():
		return ctx.Err()
	case <-aw.ctx.Done():
		return ErrWriterClosed
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-aw.ctx.Done():
		return ErrWriterClosed
	}
}

// Close implements AsyncWriter.Close.
func (aw *asyncWriter) Close() error {
	if !atomic.CompareAndSwapInt32(&aw.closed, 0, 1) {
		return nil // Already closed
	}

	done := make(chan error, 1)

	// Send close request
	select {
	case aw.closeCh <- done:
		// Request sent
	default:
		// Channel might be closed already
		aw.cancel()
		aw.wg.Wait()
		return nil
	}

	// Wait for completion
	err := <-done

	// Wait for background goroutines to finish
	aw.wg.Wait()

	return err
}

// Stats implements AsyncWriter.Stats.
func (aw *asyncWriter) Stats() Stats {
	aw.statsMu.RLock()
	defer aw.statsMu.RUnlock()

	stats := aw.stats

	// Calculate buffer utilization
	aw.bufferMu.RLock()
	if cap(aw.buffer) > 0 {
		stats.BufferUtilization = float64(len(aw.buffer)) / float64(cap(aw.buffer))
	}
	aw.bufferMu.RUnlock()

	// Calculate average write time
	if stats.WriteCount > 0 {
		stats.AverageWriteTime = time.Duration(int64(stats.TotalWriteTime) / stats.WriteCount)
	}

	return stats
}

// IsClosed implements AsyncWriter.IsClosed.
func (aw *asyncWriter) IsClosed() bool {
	return atomic.LoadInt32(&aw.closed) != 0
}

// BufferSize implements AsyncWriter.BufferSize.
func (aw *asyncWriter) BufferSize() int {
	aw.bufferMu.RLock()
	defer aw.bufferMu.RUnlock()
	return len(aw.buffer)
}

// BufferCapacity implements AsyncWriter.BufferCapacity.
func (aw *asyncWriter) BufferCapacity() int {
	aw.bufferMu.RLock()
	defer aw.bufferMu.RUnlock()
	return cap(aw.buffer)
}

// writerLoop is the main background goroutine that handles write operations.
//
//nolint:gocyclo
func (aw *asyncWriter) writerLoop() {
	defer aw.wg.Done()

	for {
		select {
		case req := <-aw.writeCh:
			err := aw.handleWriteRequest(req)
			if req.done != nil {
				select {
				case req.done <- err:
				case <-req.ctx.Done():
				case <-aw.ctx.Done():
				}
			}

		case done := <-aw.flushCh:
			err := aw.flushBuffer()
			select {
			case done <- err:
			default:
			}

		case done := <-aw.closeCh:
			// Flush remaining data and close
			err := aw.flushBuffer()
			aw.cancel()
			select {
			case done <- err:
			default:
			}
			return

		case <-aw.ctx.Done():
			// Context canceled, exit
			return
		}
	}
}

// flushLoop automatically flushes the buffer at regular intervals.
func (aw *asyncWriter) flushLoop() {
	defer aw.wg.Done()

	ticker := time.NewTicker(aw.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = aw.flushBuffer() // Ignore error in automatic flush
		case <-aw.ctx.Done():
			return
		}
	}
}

// handleWriteRequest processes a write request.
func (aw *asyncWriter) handleWriteRequest(req writeRequest) error {
	startTime := time.Now()

	// Add data to buffer
	aw.bufferMu.Lock()
	// Check if buffer has space
	if len(aw.buffer)+len(req.data) > cap(aw.buffer) {
		aw.bufferMu.Unlock()
		// Flush buffer first
		if err := aw.flushBuffer(); err != nil {
			aw.updateStats(func(s *Stats) {
				s.ErrorCount++
			})
			if aw.config.OnError != nil {
				aw.config.OnError(err)
			}
			return err
		}
		aw.bufferMu.Lock()
	}

	aw.buffer = append(aw.buffer, req.data...)
	aw.bufferMu.Unlock()

	// Update statistics
	duration := time.Since(startTime)
	aw.updateStats(func(s *Stats) {
		s.WriteCount++
		s.BytesWritten += int64(len(req.data))
		s.TotalWriteTime += duration
		s.LastWriteTime = time.Now()
	})

	return nil
}

// flushBuffer writes all buffered data to the underlying writer.
func (aw *asyncWriter) flushBuffer() error {
	aw.bufferMu.Lock()
	if len(aw.buffer) == 0 {
		aw.bufferMu.Unlock()
		return nil
	}

	// Copy buffer to avoid holding lock during write
	data := make([]byte, len(aw.buffer))
	copy(data, aw.buffer)
	aw.buffer = aw.buffer[:0] // Reset buffer
	aw.bufferMu.Unlock()

	// Write data with retries
	startTime := time.Now()
	bytesWritten, err := aw.writeWithRetries(data)
	duration := time.Since(startTime)

	// Update statistics
	aw.updateStats(func(s *Stats) {
		s.FlushCount++
		if err != nil {
			s.ErrorCount++
		}
	})

	// Call flush callback
	if aw.config.OnFlush != nil {
		aw.config.OnFlush(bytesWritten, duration)
	}

	if err != nil && aw.config.OnError != nil {
		aw.config.OnError(err)
	}

	return err
}

// writeWithRetries writes data with retry logic.
func (aw *asyncWriter) writeWithRetries(data []byte) (int, error) {
	var totalWritten int
	var lastErr error

	for attempt := 0; attempt <= aw.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(aw.config.RetryDelay):
			case <-aw.ctx.Done():
				return totalWritten, aw.ctx.Err()
			}
		}

		written, err := aw.underlying.Write(data[totalWritten:])
		totalWritten += written

		if err != nil {
			lastErr = err
			continue
		}

		if totalWritten >= len(data) {
			return totalWritten, nil
		}
	}

	return totalWritten, lastErr
}

// updateStats safely updates statistics.
func (aw *asyncWriter) updateStats(updater func(*Stats)) {
	aw.statsMu.Lock()
	defer aw.statsMu.Unlock()
	updater(&aw.stats)
}
