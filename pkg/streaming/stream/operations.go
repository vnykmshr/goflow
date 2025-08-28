package stream

import (
	"context"
	"sort"
	"sync/atomic"
)

// filterOperation filters elements based on a predicate.
type filterOperation[T any] struct {
	predicate func(T) bool
}

// apply implements operation[T] interface.
//
//nolint:unused
func (f *filterOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	for element := range input {
		if element.err != nil || element.end {
			output <- element
			if element.end {
				break
			}
			continue
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		if f.predicate(element.value) {
			select {
			case output <- element:
			case <-ctx.Done():
				output <- streamElement[T]{err: ctx.Err()}
				return ctx.Err()
			}
		}
	}
	return nil
}

// mapOperation transforms elements using a mapper function.
type mapOperation[T any] struct {
	mapper func(T) T
}

// apply implements operation[T] interface.
//
//nolint:unused
func (m *mapOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	for element := range input {
		if element.err != nil || element.end {
			output <- element
			if element.end {
				break
			}
			continue
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		newElement := streamElement[T]{
			value: m.mapper(element.value),
			err:   element.err,
			end:   element.end,
		}

		select {
		case output <- newElement:
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		}
	}
	return nil
}

// flatMapOperation flattens nested streams.
type flatMapOperation[T any] struct {
	mapper func(T) Stream[T]
}

//nolint:gocyclo,unused
func (f *flatMapOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	for element := range input {
		if element.err != nil || element.end {
			output <- element
			if element.end {
				break
			}
			continue
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		// Get the mapped stream and consume it
		mappedStream := f.mapper(element.value)
		defer mappedStream.Close()

		// Execute the mapped stream and forward its elements
		mappedCh, err := mappedStream.(*stream[T]).execute(ctx)
		if err != nil {
			output <- streamElement[T]{err: err}
			continue
		}

		for mappedElement := range mappedCh {
			if mappedElement.end {
				break
			}

			select {
			case output <- mappedElement:
			case <-ctx.Done():
				output <- streamElement[T]{err: ctx.Err()}
				return ctx.Err()
			}
		}
	}
	return nil
}

// distinctOperation removes duplicate elements.
type distinctOperation[T any] struct {
	seen map[interface{}]bool //nolint:unused
}

// apply implements operation[T] interface.
//
//nolint:unused
func (d *distinctOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	if d.seen == nil {
		d.seen = make(map[interface{}]bool)
	}

	for element := range input {
		if element.err != nil || element.end {
			output <- element
			if element.end {
				break
			}
			continue
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		// Use the value as a key (this works for comparable types)
		key := interface{}(element.value)
		if !d.seen[key] {
			d.seen[key] = true

			select {
			case output <- element:
			case <-ctx.Done():
				output <- streamElement[T]{err: ctx.Err()}
				return ctx.Err()
			}
		}
	}
	return nil
}

// sortOperation sorts all elements (requires collecting all elements first).
type sortOperation[T any] struct {
	compare func(a, b T) int
}

// apply implements operation[T] interface.
//
//nolint:unused
func (s *sortOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	elements := make([]T, 0, 100) // Pre-allocate with reasonable initial capacity

	// Collect all elements first
	for element := range input {
		if element.err != nil {
			output <- element
			return nil
		}
		if element.end {
			break
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		elements = append(elements, element.value)
	}

	// Sort elements
	sort.Slice(elements, func(i, j int) bool {
		return s.compare(elements[i], elements[j]) < 0
	})

	// Send sorted elements
	for _, value := range elements {
		select {
		case output <- streamElement[T]{value: value}:
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		}
	}

	// Send end marker
	output <- streamElement[T]{end: true}
	return nil
}

// skipOperation skips the first n elements.
type skipOperation[T any] struct {
	count   int64
	skipped int64 //nolint:unused
}

// apply implements operation[T] interface.
//
//nolint:unused
func (s *skipOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	for element := range input {
		if element.err != nil || element.end {
			output <- element
			if element.end {
				break
			}
			continue
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		currentSkipped := atomic.AddInt64(&s.skipped, 1)
		if currentSkipped > s.count {
			select {
			case output <- element:
			case <-ctx.Done():
				output <- streamElement[T]{err: ctx.Err()}
				return ctx.Err()
			}
		}
	}
	return nil
}

// limitOperation limits the number of elements.
type limitOperation[T any] struct {
	maxSize int64
	count   int64 //nolint:unused
}

// apply implements operation[T] interface.
//
//nolint:unused
func (l *limitOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	for element := range input {
		if element.err != nil {
			output <- element
			return nil
		}
		if element.end {
			output <- element
			break
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		currentCount := atomic.AddInt64(&l.count, 1)
		if currentCount <= l.maxSize {
			select {
			case output <- element:
			case <-ctx.Done():
				output <- streamElement[T]{err: ctx.Err()}
				return ctx.Err()
			}

			if currentCount == l.maxSize {
				// Send end marker after reaching limit
				output <- streamElement[T]{end: true}
				break
			}
		} else {
			// Exceeded limit, send end marker
			output <- streamElement[T]{end: true}
			break
		}
	}
	return nil
}

// peekOperation performs an action on each element without modifying the stream.
type peekOperation[T any] struct {
	action func(T)
}

// apply implements operation[T] interface.
//
//nolint:unused
func (p *peekOperation[T]) apply(ctx context.Context, input <-chan streamElement[T], output chan<- streamElement[T]) error {
	for element := range input {
		if element.err != nil || element.end {
			output <- element
			if element.end {
				break
			}
			continue
		}

		select {
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		default:
		}

		// Perform the peek action
		p.action(element.value)

		// Forward the element unchanged
		select {
		case output <- element:
		case <-ctx.Done():
			output <- streamElement[T]{err: ctx.Err()}
			return ctx.Err()
		}
	}
	return nil
}
