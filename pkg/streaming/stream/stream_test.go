package stream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
)

func TestFromSlice(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	stream := FromSlice(slice)
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 5)
	testutil.AssertEqual(t, result[0], 1)
	testutil.AssertEqual(t, result[4], 5)
}

func TestEmpty(t *testing.T) {
	stream := Empty[int]()
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 0)

	count, err := Empty[string]().Count(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, count, int64(0))
}

func TestFromChannel(t *testing.T) {
	ch := make(chan string, 3)
	ch <- "hello"
	ch <- "world"
	ch <- "test"
	close(ch)

	stream := FromChannel(ch)
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 3)
	testutil.AssertEqual(t, result[0], "hello")
	testutil.AssertEqual(t, result[1], "world")
	testutil.AssertEqual(t, result[2], "test")
}

func TestFilter(t *testing.T) {
	stream := FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}).
		Filter(func(x int) bool { return x%2 == 0 })
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 5)
	testutil.AssertEqual(t, result[0], 2)
	testutil.AssertEqual(t, result[4], 10)
}

func TestMap(t *testing.T) {
	stream := FromSlice([]int{1, 2, 3, 4, 5}).
		Map(func(x int) int { return x * 2 })
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 5)
	testutil.AssertEqual(t, result[0], 2)
	testutil.AssertEqual(t, result[4], 10)
}

func TestMapTo(t *testing.T) {
	stream := FromSlice([]int{1, 2, 3}).
		MapTo(func(x int) interface{} { return fmt.Sprintf("number-%d", x) })
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 3)
	testutil.AssertEqual(t, result[0].(string), "number-1")
	testutil.AssertEqual(t, result[2].(string), "number-3")
}

func TestChainedOperations(t *testing.T) {
	stream := FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}).
		Filter(func(x int) bool { return x%2 == 0 }). // 2, 4, 6, 8, 10
		Map(func(x int) int { return x * 3 }).        // 6, 12, 18, 24, 30
		Skip(1).                                      // 12, 18, 24, 30
		Limit(2)                                      // 12, 18
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 2)
	testutil.AssertEqual(t, result[0], 12)
	testutil.AssertEqual(t, result[1], 18)
}

func TestDistinct(t *testing.T) {
	stream := FromSlice([]int{1, 2, 2, 3, 3, 3, 4, 4, 5}).
		Distinct()
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 5)

	// Should contain each number exactly once
	expected := []int{1, 2, 3, 4, 5}
	for i, v := range expected {
		testutil.AssertEqual(t, result[i], v)
	}
}

func TestSorted(t *testing.T) {
	stream := FromSlice([]int{5, 2, 8, 1, 9, 3}).
		Sorted(func(a, b int) int {
			if a < b {
				return -1
			} else if a > b {
				return 1
			}
			return 0
		})
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 6)

	expected := []int{1, 2, 3, 5, 8, 9}
	for i, v := range expected {
		testutil.AssertEqual(t, result[i], v)
	}
}

func TestSkipAndLimit(t *testing.T) {
	// Test Skip
	stream := FromSlice([]int{1, 2, 3, 4, 5}).Skip(2)
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 3)
	testutil.AssertEqual(t, result[0], 3)

	// Test Limit
	stream = FromSlice([]int{1, 2, 3, 4, 5}).Limit(3)
	defer stream.Close()

	result, err = stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 3)
	testutil.AssertEqual(t, result[2], 3)
}

func TestPeek(t *testing.T) {
	var peeked []int

	stream := FromSlice([]int{1, 2, 3}).
		Peek(func(x int) {
			peeked = append(peeked, x)
		})
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 3)
	testutil.AssertEqual(t, len(peeked), 3)
	testutil.AssertEqual(t, peeked[0], 1)
	testutil.AssertEqual(t, result[0], 1) // Original unchanged
}

func TestForEach(t *testing.T) {
	var collected []int
	stream := FromSlice([]int{1, 2, 3, 4, 5})

	err := stream.ForEach(context.Background(), func(x int) {
		collected = append(collected, x*2)
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(collected), 5)
	testutil.AssertEqual(t, collected[0], 2)
	testutil.AssertEqual(t, collected[4], 10)
}

func TestReduce(t *testing.T) {
	stream := FromSlice([]int{1, 2, 3, 4, 5})
	defer stream.Close()

	sum, err := stream.Reduce(context.Background(), 0, func(acc, x int) int {
		return acc + x
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, sum, 15) // 1+2+3+4+5 = 15
}

func TestCollect(t *testing.T) {
	stream := FromSlice([]string{"hello", "world", "test"})
	defer stream.Close()

	result, err := stream.Collect(
		context.Background(),
		func() interface{} { return &[]string{} },
		func(acc interface{}, value string) {
			slice := acc.(*[]string)
			*slice = append(*slice, value)
		},
		func(acc1, acc2 interface{}) interface{} {
			s1 := acc1.(*[]string)
			s2 := acc2.(*[]string)
			*s1 = append(*s1, *s2...)
			return s1
		},
	)
	testutil.AssertNoError(t, err)

	collected := result.(*[]string)
	testutil.AssertEqual(t, len(*collected), 3)
}

func TestCount(t *testing.T) {
	stream := FromSlice([]string{"a", "b", "c", "d"})
	defer stream.Close()

	count, err := stream.Count(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, count, int64(4))
}

func TestFindFirst(t *testing.T) {
	// Test with non-empty stream
	stream := FromSlice([]int{10, 20, 30})
	defer stream.Close()

	value, found, err := stream.FindFirst(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, found, true)
	testutil.AssertEqual(t, value, 10)

	// Test with empty stream
	stream = Empty[int]()
	defer stream.Close()

	value, found, err = stream.FindFirst(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, found, false)
	testutil.AssertEqual(t, value, 0)
}

func TestAnyMatch(t *testing.T) {
	stream := FromSlice([]int{1, 2, 3, 4, 5})
	defer stream.Close()

	// Should find even number
	hasEven, err := stream.AnyMatch(context.Background(), func(x int) bool {
		return x%2 == 0
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, hasEven, true)
}

func TestAllMatch(t *testing.T) {
	stream := FromSlice([]int{2, 4, 6, 8})
	defer stream.Close()

	// All are even
	allEven, err := stream.AllMatch(context.Background(), func(x int) bool {
		return x%2 == 0
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, allEven, true)

	stream = FromSlice([]int{1, 2, 3, 4})
	defer stream.Close()

	// Not all are even
	allEven, err = stream.AllMatch(context.Background(), func(x int) bool {
		return x%2 == 0
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, allEven, false)
}

func TestNoneMatch(t *testing.T) {
	stream := FromSlice([]int{1, 3, 5, 7})
	defer stream.Close()

	// None are even
	noneEven, err := stream.NoneMatch(context.Background(), func(x int) bool {
		return x%2 == 0
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, noneEven, true)
}

func TestMinMax(t *testing.T) {
	stream := FromSlice([]int{5, 2, 8, 1, 9, 3})

	// Test Min
	minVal, found, err := stream.Min(context.Background(), func(a, b int) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, found, true)
	testutil.AssertEqual(t, minVal, 1)

	// Test Max (need new stream since first one is consumed)
	stream = FromSlice([]int{5, 2, 8, 1, 9, 3})
	defer stream.Close()

	maxVal, found, err := stream.Max(context.Background(), func(a, b int) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	})
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, found, true)
	testutil.AssertEqual(t, maxVal, 9)
}

func TestContextCancellation(t *testing.T) {
	// Create a context that will be canceled
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Create a stream with a slow generator
	stream := Generate(func() int {
		time.Sleep(50 * time.Millisecond) // Slow operation
		return 1
	}).Limit(100)
	defer stream.Close()

	// This should be canceled due to context timeout
	_, err := stream.Count(ctx)
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, context.DeadlineExceeded)
}

func TestStreamClosing(t *testing.T) {
	stream := FromSlice([]int{1, 2, 3})

	// Stream should not be closed initially
	testutil.AssertEqual(t, stream.IsClosed(), false)

	// Close the stream
	err := stream.Close()
	testutil.AssertNoError(t, err)

	// Stream should be closed now
	testutil.AssertEqual(t, stream.IsClosed(), true)

	// Operations on closed stream should return error
	_, err = stream.Count(context.Background())
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, ErrStreamClosed)
}

func TestConcurrentAccess(t *testing.T) {
	// Test that multiple operations can be chained without issues
	results := make(chan []int, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			stream := FromSlice([]int{1, 2, 3, 4, 5}).
				Map(func(x int) int { return x * id }).
				Filter(func(x int) bool { return x > 0 })
			defer stream.Close()

			result, err := stream.ToSlice(context.Background())
			if err != nil {
				t.Errorf("Goroutine %d failed: %v", id, err)
				return
			}

			results <- result
		}(i + 1)
	}

	// Collect all results
	for i := 0; i < 10; i++ {
		result := <-results
		testutil.AssertEqual(t, len(result), 5)
	}
}

func TestFlatMap(t *testing.T) {
	// Create a stream that maps each number to a stream of its duplicates
	stream := FromSlice([]int{1, 2, 3}).
		FlatMap(func(x int) Stream[int] {
			return FromSlice([]int{x, x}) // Each number appears twice
		})
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 6) // 1,1,2,2,3,3
	testutil.AssertEqual(t, result[0], 1)
	testutil.AssertEqual(t, result[1], 1)
	testutil.AssertEqual(t, result[2], 2)
	testutil.AssertEqual(t, result[5], 3)
}

func TestGenerateInfinite(t *testing.T) {
	counter := 0
	stream := Generate(func() int {
		counter++
		return counter
	}).Limit(5) // Limit to avoid infinite execution
	defer stream.Close()

	result, err := stream.ToSlice(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(result), 5)
	testutil.AssertEqual(t, result[0], 1)
	testutil.AssertEqual(t, result[4], 5)
}

// Benchmark tests
func BenchmarkStreamOperations(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := FromSlice(slice).
			Filter(func(x int) bool { return x%2 == 0 }).
			Map(func(x int) int { return x * 2 }).
			Limit(100)

		_, err := stream.Count(context.Background())
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkToSlice(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := FromSlice(slice)
		_, err := stream.ToSlice(context.Background())
		if err != nil {
			b.Fatal(err)
		}
	}
}
