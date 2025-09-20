package stream

import (
	"context"
	"sync/atomic"
)

// sliceSource implements Source for slices.
type sliceSource[T any] struct {
	slice []T
	index int64
}

func (s *sliceSource[T]) Next(ctx context.Context) (T, bool, error) {
	var zero T

	currentIndex := atomic.AddInt64(&s.index, 1) - 1
	if currentIndex >= int64(len(s.slice)) {
		return zero, false, nil
	}

	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()
	default:
		return s.slice[currentIndex], true, nil
	}
}

func (s *sliceSource[T]) Close() error {
	return nil
}

// channelSource implements Source for channels.
type channelSource[T any] struct {
	ch <-chan T
}

func (s *channelSource[T]) Next(ctx context.Context) (T, bool, error) {
	var zero T

	select {
	case value, ok := <-s.ch:
		if !ok {
			return zero, false, nil
		}
		return value, true, nil
	case <-ctx.Done():
		return zero, false, ctx.Err()
	}
}

func (s *channelSource[T]) Close() error {
	return nil
}

// generatorSource implements Source for generator functions.
type generatorSource[T any] struct {
	generator func() T
}

func (s *generatorSource[T]) Next(ctx context.Context) (T, bool, error) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, false, ctx.Err()
	default:
		return s.generator(), true, nil
	}
}

func (s *generatorSource[T]) Close() error {
	return nil
}

// emptySource implements Source for empty streams.
type emptySource[T any] struct{}

func (s *emptySource[T]) Next(_ context.Context) (T, bool, error) {
	var zero T
	return zero, false, nil
}

func (s *emptySource[T]) Close() error {
	return nil
}

// mappingSource implements Source that transforms elements from one type to another.
type mappingSource[From, To any] struct {
	originalSource Source[From]
	mapper         func(From) To
}

func (s *mappingSource[From, To]) Next(ctx context.Context) (To, bool, error) {
	var zero To

	value, hasMore, err := s.originalSource.Next(ctx)
	if err != nil {
		return zero, false, err
	}

	if !hasMore {
		return zero, false, nil
	}

	return s.mapper(value), true, nil
}

func (s *mappingSource[From, To]) Close() error {
	return s.originalSource.Close()
}
