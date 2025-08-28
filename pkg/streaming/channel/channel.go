package channel

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// BackpressureStrategy defines how the channel handles backpressure when full.
type BackpressureStrategy int

const (
	// Block strategy blocks the producer until space is available.
	Block BackpressureStrategy = iota

	// Drop strategy drops the newest message when buffer is full.
	Drop

	// DropOldest strategy drops the oldest message when buffer is full.
	DropOldest

	// Error strategy returns an error when buffer is full.
	Error
)

// ErrChannelFull is returned when the channel buffer is full and strategy is Error.
var ErrChannelFull = errors.New("channel buffer is full")

// ErrChannelClosed is returned when attempting to operate on a closed channel.
var ErrChannelClosed = errors.New("channel is closed")

// BackpressureChannel provides a channel with configurable backpressure handling.
type BackpressureChannel[T any] interface {
	// Send sends a value to the channel.
	Send(ctx context.Context, value T) error

	// TrySend attempts to send a value without blocking.
	TrySend(value T) error

	// Receive receives a value from the channel.
	Receive(ctx context.Context) (T, error)

	// TryReceive attempts to receive a value without blocking.
	TryReceive() (T, bool, error)

	// Close closes the channel for sending.
	Close() error

	// IsClosed returns true if the channel is closed.
	IsClosed() bool

	// Len returns the current number of buffered elements.
	Len() int

	// Cap returns the buffer capacity.
	Cap() int

	// Stats returns channel statistics.
	Stats() Stats
}

// Stats holds statistics about channel performance.
type Stats struct {
	// SendCount is the total number of send operations.
	SendCount int64

	// ReceiveCount is the total number of receive operations.
	ReceiveCount int64

	// DroppedCount is the total number of dropped messages.
	DroppedCount int64

	// BlockedSends is the number of sends that had to block.
	BlockedSends int64

	// AverageSendTime is the average time per send operation.
	AverageSendTime time.Duration

	// AverageReceiveTime is the average time per receive operation.
	AverageReceiveTime time.Duration

	// BufferUtilization is the current buffer utilization (0.0 to 1.0).
	BufferUtilization float64

	// LastSendTime is the timestamp of the last send operation.
	LastSendTime time.Time

	// LastReceiveTime is the timestamp of the last receive operation.
	LastReceiveTime time.Time
}

// Config holds configuration for BackpressureChannel.
type Config struct {
	// BufferSize is the size of the channel buffer.
	BufferSize int

	// Strategy defines how backpressure is handled.
	Strategy BackpressureStrategy

	// OnDrop is called when a message is dropped (for Drop/DropOldest strategies).
	OnDrop func(value interface{})

	// OnBlock is called when a send operation blocks (for Block strategy).
	OnBlock func()

	// OnError is called when an error occurs.
	OnError func(error)

	// SendTimeout is the maximum time to wait for send operations (0 = no timeout).
	SendTimeout time.Duration

	// ReceiveTimeout is the maximum time to wait for receive operations (0 = no timeout).
	ReceiveTimeout time.Duration
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		BufferSize:     100,
		Strategy:       Block,
		SendTimeout:    0,
		ReceiveTimeout: 0,
	}
}

// backpressureChannel implements BackpressureChannel.
type backpressureChannel[T any] struct {
	config Config
	buffer []T
	mu     sync.RWMutex

	// Channel state
	head   int
	tail   int
	count  int
	closed int32

	// Synchronization
	sendCond *sync.Cond
	recvCond *sync.Cond

	// Statistics
	stats     Stats
	statsMu   sync.RWMutex
	startTime time.Time
}

// New creates a new BackpressureChannel with default configuration.
func New[T any](bufferSize int) BackpressureChannel[T] {
	config := DefaultConfig()
	config.BufferSize = bufferSize
	return NewWithConfig[T](config)
}

// NewWithConfig creates a new BackpressureChannel with the specified configuration.
func NewWithConfig[T any](config Config) BackpressureChannel[T] {
	if config.BufferSize <= 0 {
		config.BufferSize = DefaultConfig().BufferSize
	}

	ch := &backpressureChannel[T]{
		config:    config,
		buffer:    make([]T, config.BufferSize),
		startTime: time.Now(),
	}

	ch.sendCond = sync.NewCond(&ch.mu)
	ch.recvCond = sync.NewCond(&ch.mu)

	return ch
}

// Send implements BackpressureChannel.Send.
func (ch *backpressureChannel[T]) Send(ctx context.Context, value T) error {
	startTime := time.Now()
	defer func() {
		ch.updateSendStats(time.Since(startTime))
	}()

	if ch.IsClosed() {
		return ErrChannelClosed
	}

	// Handle timeout
	if ch.config.SendTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ch.config.SendTimeout)
		defer cancel()
	}

	switch ch.config.Strategy {
	case Block:
		return ch.blockingSend(ctx, value)
	case Drop:
		return ch.dropSend(value)
	case DropOldest:
		return ch.dropOldestSend(value)
	case Error:
		return ch.errorSend(value)
	default:
		return ch.blockingSend(ctx, value)
	}
}

// TrySend implements BackpressureChannel.TrySend.
func (ch *backpressureChannel[T]) TrySend(value T) error {
	if ch.IsClosed() {
		return ErrChannelClosed
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.count >= len(ch.buffer) {
		switch ch.config.Strategy {
		case Drop:
			ch.updateStats(func(s *Stats) { s.DroppedCount++ })
			if ch.config.OnDrop != nil {
				ch.config.OnDrop(value)
			}
			return nil
		case DropOldest:
			// Drop oldest and add new
			ch.dropOldestLocked()
			ch.addToBufferLocked(value)
			ch.updateStats(func(s *Stats) {
				s.SendCount++
				s.DroppedCount++
			})
			return nil
		default:
			return ErrChannelFull
		}
	}

	ch.addToBufferLocked(value)
	ch.updateStats(func(s *Stats) { s.SendCount++ })
	ch.recvCond.Signal()

	return nil
}

// Receive implements BackpressureChannel.Receive.
func (ch *backpressureChannel[T]) Receive(ctx context.Context) (T, error) {
	startTime := time.Now()
	defer func() {
		ch.updateReceiveStats(time.Since(startTime))
	}()

	// Handle timeout
	if ch.config.ReceiveTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ch.config.ReceiveTimeout)
		defer cancel()
	}

	return ch.blockingReceive(ctx)
}

// TryReceive implements BackpressureChannel.TryReceive.
func (ch *backpressureChannel[T]) TryReceive() (T, bool, error) {
	var zero T

	if ch.IsClosed() {
		return zero, false, ErrChannelClosed
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.count == 0 {
		if atomic.LoadInt32(&ch.closed) != 0 {
			return zero, false, ErrChannelClosed
		}
		return zero, false, nil
	}

	value := ch.removeFromBufferLocked()
	ch.updateStats(func(s *Stats) { s.ReceiveCount++ })
	ch.sendCond.Signal()

	return value, true, nil
}

// Close implements BackpressureChannel.Close.
func (ch *backpressureChannel[T]) Close() error {
	if !atomic.CompareAndSwapInt32(&ch.closed, 0, 1) {
		return nil // Already closed
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.sendCond.Broadcast()
	ch.recvCond.Broadcast()

	return nil
}

// IsClosed implements BackpressureChannel.IsClosed.
func (ch *backpressureChannel[T]) IsClosed() bool {
	return atomic.LoadInt32(&ch.closed) != 0
}

// Len implements BackpressureChannel.Len.
func (ch *backpressureChannel[T]) Len() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.count
}

// Cap implements BackpressureChannel.Cap.
func (ch *backpressureChannel[T]) Cap() int {
	return len(ch.buffer)
}

// Stats implements BackpressureChannel.Stats.
func (ch *backpressureChannel[T]) Stats() Stats {
	ch.statsMu.RLock()
	defer ch.statsMu.RUnlock()

	stats := ch.stats

	// Calculate buffer utilization
	ch.mu.RLock()
	if len(ch.buffer) > 0 {
		stats.BufferUtilization = float64(ch.count) / float64(len(ch.buffer))
	}
	ch.mu.RUnlock()

	// Calculate average times
	if stats.SendCount > 0 {
		stats.AverageSendTime = time.Duration(int64(stats.AverageSendTime) / stats.SendCount)
	}
	if stats.ReceiveCount > 0 {
		stats.AverageReceiveTime = time.Duration(int64(stats.AverageReceiveTime) / stats.ReceiveCount)
	}

	return stats
}

// blockingSend sends with blocking strategy.
func (ch *backpressureChannel[T]) blockingSend(ctx context.Context, value T) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for ch.count >= len(ch.buffer) && !ch.IsClosed() {
		if ch.config.OnBlock != nil {
			ch.config.OnBlock()
		}
		ch.updateStats(func(s *Stats) { s.BlockedSends++ })

		// Check for context cancellation before waiting
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Wait for space with timeout checking
		ch.sendCond.Wait()
	}

	if ch.IsClosed() {
		return ErrChannelClosed
	}

	ch.addToBufferLocked(value)
	ch.updateStats(func(s *Stats) { s.SendCount++ })
	ch.recvCond.Signal()

	return nil
}

// dropSend sends with drop strategy.
func (ch *backpressureChannel[T]) dropSend(value T) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.count >= len(ch.buffer) {
		ch.updateStats(func(s *Stats) { s.DroppedCount++ })
		if ch.config.OnDrop != nil {
			ch.config.OnDrop(value)
		}
		return nil
	}

	ch.addToBufferLocked(value)
	ch.updateStats(func(s *Stats) { s.SendCount++ })
	ch.recvCond.Signal()

	return nil
}

// dropOldestSend sends with drop oldest strategy.
func (ch *backpressureChannel[T]) dropOldestSend(value T) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.count >= len(ch.buffer) {
		oldValue := ch.removeFromBufferLocked()
		ch.updateStats(func(s *Stats) { s.DroppedCount++ })
		if ch.config.OnDrop != nil {
			ch.config.OnDrop(oldValue)
		}
	}

	ch.addToBufferLocked(value)
	ch.updateStats(func(s *Stats) { s.SendCount++ })
	ch.recvCond.Signal()

	return nil
}

// errorSend sends with error strategy.
func (ch *backpressureChannel[T]) errorSend(value T) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.count >= len(ch.buffer) {
		return ErrChannelFull
	}

	ch.addToBufferLocked(value)
	ch.updateStats(func(s *Stats) { s.SendCount++ })
	ch.recvCond.Signal()

	return nil
}

// blockingReceive receives with blocking.
func (ch *backpressureChannel[T]) blockingReceive(ctx context.Context) (T, error) {
	var zero T

	ch.mu.Lock()
	defer ch.mu.Unlock()

	for ch.count == 0 && !ch.IsClosed() {
		// Check for context cancellation before waiting
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Wait for data
		ch.recvCond.Wait()
	}

	if ch.count == 0 && ch.IsClosed() {
		return zero, ErrChannelClosed
	}

	value := ch.removeFromBufferLocked()
	ch.updateStats(func(s *Stats) { s.ReceiveCount++ })
	ch.sendCond.Signal()

	return value, nil
}

// addToBufferLocked adds a value to the buffer (must hold lock).
func (ch *backpressureChannel[T]) addToBufferLocked(value T) {
	ch.buffer[ch.tail] = value
	ch.tail = (ch.tail + 1) % len(ch.buffer)
	ch.count++
}

// removeFromBufferLocked removes a value from the buffer (must hold lock).
func (ch *backpressureChannel[T]) removeFromBufferLocked() T {
	value := ch.buffer[ch.head]
	var zero T
	ch.buffer[ch.head] = zero // Clear reference
	ch.head = (ch.head + 1) % len(ch.buffer)
	ch.count--
	return value
}

// dropOldestLocked drops the oldest value from the buffer (must hold lock).
func (ch *backpressureChannel[T]) dropOldestLocked() T {
	if ch.count == 0 {
		var zero T
		return zero
	}
	return ch.removeFromBufferLocked()
}

// updateStats safely updates statistics.
func (ch *backpressureChannel[T]) updateStats(updater func(*Stats)) {
	ch.statsMu.Lock()
	defer ch.statsMu.Unlock()
	updater(&ch.stats)
}

// updateSendStats updates send-related statistics.
func (ch *backpressureChannel[T]) updateSendStats(duration time.Duration) {
	ch.updateStats(func(s *Stats) {
		s.AverageSendTime += duration
		s.LastSendTime = time.Now()
	})
}

// updateReceiveStats updates receive-related statistics.
func (ch *backpressureChannel[T]) updateReceiveStats(duration time.Duration) {
	ch.updateStats(func(s *Stats) {
		s.AverageReceiveTime += duration
		s.LastReceiveTime = time.Now()
	})
}
