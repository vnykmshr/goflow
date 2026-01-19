package stream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// ErrStreamClosed is returned when attempting to operate on a closed stream.
var ErrStreamClosed = errors.New("stream is closed")

// Stream represents a sequence of elements supporting sequential and parallel operations.
// Streams are lazy; computation on the source data is only performed when a terminal
// operation is initiated, and source elements are consumed only as needed.
type Stream[T any] interface {
	// Intermediate operations (lazy, return new Stream)

	// Filter returns a stream consisting of elements that match the given predicate.
	Filter(predicate func(T) bool) Stream[T]

	// Map returns a stream consisting of the results of applying the given function to elements.
	Map(mapper func(T) T) Stream[T]

	// MapTo transforms elements to a different type and returns a new typed stream.
	MapTo(mapper func(T) interface{}) Stream[interface{}]

	// FlatMap returns a stream consisting of results of replacing each element with
	// the contents of a mapped stream produced by applying the provided mapping function.
	FlatMap(mapper func(T) Stream[T]) Stream[T]

	// Distinct returns a stream consisting of distinct elements (according to equality).
	Distinct() Stream[T]

	// Sorted returns a stream consisting of elements sorted according to natural order.
	// The compare function should return negative if a < b, 0 if a == b, positive if a > b.
	Sorted(compare func(a, b T) int) Stream[T]

	// Skip returns a stream consisting of remaining elements after skipping n elements.
	Skip(n int64) Stream[T]

	// Limit returns a stream consisting of elements truncated to be no longer than maxSize.
	Limit(maxSize int64) Stream[T]

	// Peek returns a stream consisting of elements, additionally performing the provided
	// action on each element as elements are consumed.
	Peek(action func(T)) Stream[T]

	// Terminal operations (eager, consume the stream)

	// ForEach performs an action for each element of the stream.
	ForEach(ctx context.Context, action func(T)) error

	// Reduce performs a reduction on elements using the provided identity and combining function.
	Reduce(ctx context.Context, identity T, accumulator func(T, T) T) (T, error)

	// Collect performs a mutable reduction operation on elements.
	Collect(ctx context.Context, supplier func() interface{}, accumulator func(interface{}, T), combiner func(interface{}, interface{}) interface{}) (interface{}, error)

	// ToSlice returns a slice containing all elements.
	ToSlice(ctx context.Context) ([]T, error)

	// Count returns the count of elements.
	Count(ctx context.Context) (int64, error)

	// AnyMatch returns whether any elements match the given predicate.
	AnyMatch(ctx context.Context, predicate func(T) bool) (bool, error)

	// AllMatch returns whether all elements match the given predicate.
	AllMatch(ctx context.Context, predicate func(T) bool) (bool, error)

	// NoneMatch returns whether no elements match the given predicate.
	NoneMatch(ctx context.Context, predicate func(T) bool) (bool, error)

	// FindFirst returns the first element, if present.
	FindFirst(ctx context.Context) (T, bool, error)

	// FindAny returns any element, if present.
	FindAny(ctx context.Context) (T, bool, error)

	// Min returns the minimum element according to the provided comparator.
	Min(ctx context.Context, compare func(a, b T) int) (T, bool, error)

	// Max returns the maximum element according to the provided comparator.
	Max(ctx context.Context, compare func(a, b T) int) (T, bool, error)

	// Stream control

	// Close closes the stream and releases resources.
	Close() error

	// IsClosed returns true if the stream is closed.
	IsClosed() bool
}

// Source represents a data source for streams.
type Source[T any] interface {
	// Next returns the next element and true, or zero value and false if no more elements.
	Next(ctx context.Context) (T, bool, error)
	// Close closes the source and releases resources.
	Close() error
}

// stream is the default implementation of Stream.
type stream[T any] struct {
	source     Source[T]
	closed     int32 // atomic
	pipeline   []operation[T]
	mu         sync.RWMutex
	cancelExec context.CancelFunc // cancels execute goroutines when stream is closed
}

// operation represents a stream operation that can be applied to elements.
type operation[T any] interface {
	apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error
}

// streamElement wraps an element with error information.
type streamElement[T any] struct {
	value T
	err   error
	end   bool // indicates end of stream
}

// New creates a new Stream from a Source.
func New[T any](source Source[T]) Stream[T] {
	return &stream[T]{
		source:   source,
		pipeline: make([]operation[T], 0),
	}
}

// FromSlice creates a Stream from a slice.
func FromSlice[T any](slice []T) Stream[T] {
	return New(&sliceSource[T]{
		slice: slice,
		index: 0,
	})
}

// FromChannel creates a Stream from a channel.
func FromChannel[T any](ch <-chan T) Stream[T] {
	return New(&channelSource[T]{
		ch: ch,
	})
}

// Generate creates an infinite Stream from a generator function.
func Generate[T any](generator func() T) Stream[T] {
	return New(&generatorSource[T]{
		generator: generator,
	})
}

// Empty creates an empty Stream.
func Empty[T any]() Stream[T] {
	return New(&emptySource[T]{})
}

// Filter implementation
func (s *stream[T]) Filter(predicate func(T) bool) Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &filterOperation[T]{predicate: predicate}

	return newStream
}

// Map implementation
func (s *stream[T]) Map(mapper func(T) T) Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &mapOperation[T]{mapper: mapper}

	return newStream
}

// MapTo implementation
func (s *stream[T]) MapTo(mapper func(T) interface{}) Stream[interface{}] {
	// Create a new stream of interface{} type
	newStream := &stream[interface{}]{
		source: &mappingSource[T, interface{}]{
			originalSource: s.source,
			mapper:         mapper,
		},
		pipeline: make([]operation[interface{}], 0),
	}

	// Convert pipeline operations to interface{} operations
	for _, op := range s.pipeline {
		// Note: This is a simplification. In a real implementation,
		// we'd need to properly convert each operation type.
		if filterOp, ok := op.(*filterOperation[T]); ok {
			newStream.pipeline = append(newStream.pipeline, &filterOperation[interface{}]{
				predicate: func(v interface{}) bool {
					if tv, ok := v.(T); ok {
						return filterOp.predicate(tv)
					}
					return false
				},
			})
		}
	}

	return newStream
}

// FlatMap implementation
func (s *stream[T]) FlatMap(mapper func(T) Stream[T]) Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &flatMapOperation[T]{mapper: mapper}

	return newStream
}

// Distinct implementation
func (s *stream[T]) Distinct() Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &distinctOperation[T]{}

	return newStream
}

// Sorted implementation
func (s *stream[T]) Sorted(compare func(a, b T) int) Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &sortOperation[T]{compare: compare}

	return newStream
}

// Skip implementation
func (s *stream[T]) Skip(n int64) Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &skipOperation[T]{count: n}

	return newStream
}

// Limit implementation
func (s *stream[T]) Limit(maxSize int64) Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &limitOperation[T]{maxSize: maxSize}

	return newStream
}

// Peek implementation
func (s *stream[T]) Peek(action func(T)) Stream[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStream := &stream[T]{
		source:   s.source,
		pipeline: make([]operation[T], len(s.pipeline)+1),
	}
	copy(newStream.pipeline, s.pipeline)
	newStream.pipeline[len(s.pipeline)] = &peekOperation[T]{action: action}

	return newStream
}

// ForEach implementation
func (s *stream[T]) ForEach(ctx context.Context, action func(T)) error {
	if s.IsClosed() {
		return ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return err
	}

	for element := range ch {
		if element.err != nil {
			return element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			action(element.value)
		}
	}

	return nil
}

// ToSlice implementation
func (s *stream[T]) ToSlice(ctx context.Context) ([]T, error) {
	if s.IsClosed() {
		return nil, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return nil, err
	}

	var result []T
	for element := range ch {
		if element.err != nil {
			return nil, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			result = append(result, element.value)
		}
	}

	return result, nil
}

// Count implementation
func (s *stream[T]) Count(ctx context.Context) (int64, error) {
	if s.IsClosed() {
		return 0, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	for element := range ch {
		if element.err != nil {
			return 0, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			count++
		}
	}

	return count, nil
}

// Reduce implementation
func (s *stream[T]) Reduce(ctx context.Context, identity T, accumulator func(T, T) T) (T, error) {
	if s.IsClosed() {
		return identity, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return identity, err
	}

	result := identity
	for element := range ch {
		if element.err != nil {
			return identity, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return identity, ctx.Err()
		default:
			result = accumulator(result, element.value)
		}
	}

	return result, nil
}

// Collect implementation
func (s *stream[T]) Collect(ctx context.Context, supplier func() interface{}, accumulator func(interface{}, T), _ func(interface{}, interface{}) interface{}) (interface{}, error) {
	if s.IsClosed() {
		return nil, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return nil, err
	}

	result := supplier()
	for element := range ch {
		if element.err != nil {
			return nil, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			accumulator(result, element.value)
		}
	}

	return result, nil
}

// FindFirst implementation
func (s *stream[T]) FindFirst(ctx context.Context) (T, bool, error) {
	var zero T
	if s.IsClosed() {
		return zero, false, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return zero, false, err
	}

	for element := range ch {
		if element.err != nil {
			return zero, false, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return zero, false, ctx.Err()
		default:
			return element.value, true, nil
		}
	}

	return zero, false, nil
}

// FindAny implementation (same as FindFirst for sequential streams)
func (s *stream[T]) FindAny(ctx context.Context) (T, bool, error) {
	return s.FindFirst(ctx)
}

// AnyMatch implementation
func (s *stream[T]) AnyMatch(ctx context.Context, predicate func(T) bool) (bool, error) {
	if s.IsClosed() {
		return false, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return false, err
	}

	for element := range ch {
		if element.err != nil {
			return false, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			if predicate(element.value) {
				return true, nil
			}
		}
	}

	return false, nil
}

// AllMatch implementation
func (s *stream[T]) AllMatch(ctx context.Context, predicate func(T) bool) (bool, error) {
	if s.IsClosed() {
		return true, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return true, err
	}

	for element := range ch {
		if element.err != nil {
			return false, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			if !predicate(element.value) {
				return false, nil
			}
		}
	}

	return true, nil
}

// NoneMatch implementation
func (s *stream[T]) NoneMatch(ctx context.Context, predicate func(T) bool) (bool, error) {
	result, err := s.AnyMatch(ctx, predicate)
	return !result, err
}

// Min implementation
func (s *stream[T]) Min(ctx context.Context, compare func(a, b T) int) (T, bool, error) {
	var zero T
	if s.IsClosed() {
		return zero, false, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return zero, false, err
	}

	var minValue T
	found := false

	for element := range ch {
		if element.err != nil {
			return zero, false, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return zero, false, ctx.Err()
		default:
			if !found || compare(element.value, minValue) < 0 {
				minValue = element.value
				found = true
			}
		}
	}

	return minValue, found, nil
}

// Max implementation
func (s *stream[T]) Max(ctx context.Context, compare func(a, b T) int) (T, bool, error) {
	var zero T
	if s.IsClosed() {
		return zero, false, ErrStreamClosed
	}

	defer func() { _ = s.Close() }()

	ch, err := s.execute(ctx)
	if err != nil {
		return zero, false, err
	}

	var maxValue T
	found := false

	for element := range ch {
		if element.err != nil {
			return zero, false, element.err
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			return zero, false, ctx.Err()
		default:
			if !found || compare(element.value, maxValue) > 0 {
				maxValue = element.value
				found = true
			}
		}
	}

	return maxValue, found, nil
}

// Close implementation
func (s *stream[T]) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil // Already closed
	}

	// Cancel any running execute goroutines
	s.mu.Lock()
	if s.cancelExec != nil {
		s.cancelExec()
		s.cancelExec = nil
	}
	s.mu.Unlock()

	if s.source != nil {
		return s.source.Close()
	}

	return nil
}

// IsClosed implementation
func (s *stream[T]) IsClosed() bool {
	return atomic.LoadInt32(&s.closed) != 0
}

// execute runs the stream pipeline and returns a channel of results.
func (s *stream[T]) execute(ctx context.Context) (<-chan streamElement[T], error) {
	if s.IsClosed() {
		return nil, ErrStreamClosed
	}

	// Create internal cancellable context for Close() to terminate goroutines.
	// This is independent of the user's context - user context cancellation
	// should propagate errors, but Close() should exit cleanly.
	closeCtx, cancel := context.WithCancel(context.Background())

	// Store cancel function so Close() can terminate goroutines
	s.mu.Lock()
	// Cancel any previous execution context (shouldn't happen in normal use)
	if s.cancelExec != nil {
		s.cancelExec()
	}
	s.cancelExec = cancel
	s.mu.Unlock()

	// Create the initial channel from source
	sourceCh := make(chan streamElement[T], 100)

	// Start source goroutine
	go func() {
		defer close(sourceCh)

		for {
			// Check both user context and close context
			select {
			case <-ctx.Done():
				sourceCh <- streamElement[T]{err: ctx.Err()}
				return
			case <-closeCtx.Done():
				// Close() was called - exit cleanly without error
				return
			default:
			}

			value, hasMore, err := s.source.Next(ctx)
			if err != nil {
				// Check if Close() was called
				select {
				case <-closeCtx.Done():
					return
				default:
				}
				sourceCh <- streamElement[T]{err: err}
				return
			}

			if !hasMore {
				sourceCh <- streamElement[T]{end: true}
				return
			}

			select {
			case sourceCh <- streamElement[T]{value: value}:
			case <-ctx.Done():
				sourceCh <- streamElement[T]{err: ctx.Err()}
				return
			case <-closeCtx.Done():
				// Close() was called - exit cleanly
				return
			}
		}
	}()

	// Apply pipeline operations
	currentCh := (<-chan streamElement[T])(sourceCh)

	for _, op := range s.pipeline {
		nextCh := make(chan streamElement[T], 100)

		go func(operation operation[T], input <-chan streamElement[T], output chan<- streamElement[T], closeCh <-chan struct{}) {
			defer close(output)
			if err := operation.apply(ctx, input, output); err != nil {
				// Don't propagate error if Close() was called
				select {
				case <-closeCh:
					return
				default:
				}
				// Propagate operation error to downstream consumers (fail-fast)
				output <- streamElement[T]{err: err}
			}
		}(op, currentCh, nextCh, closeCtx.Done())

		currentCh = nextCh
	}

	return currentCh, nil
}
