package benchmark

import (
	"context"
	"testing"

	"github.com/vnykmshr/goflow/pkg/streaming/stream"
)

// BenchmarkFromSlice measures stream creation from slice.
func BenchmarkFromSlice(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		data := make([]int, size)
		for i := range data {
			data[i] = i
		}

		b.Run(sizeLabel(size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				s := stream.FromSlice(data)
				_ = s.Close()
			}
		})
	}
}

// BenchmarkFilter measures filter operation performance.
func BenchmarkFilter(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		data := make([]int, size)
		for i := range data {
			data[i] = i
		}

		b.Run(sizeLabel(size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				s := stream.FromSlice(data).
					Filter(func(n int) bool { return n%2 == 0 })
				_, _ = s.ToSlice(context.Background())
			}
		})
	}
}

// BenchmarkMap measures map operation performance.
func BenchmarkMap(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		data := make([]int, size)
		for i := range data {
			data[i] = i
		}

		b.Run(sizeLabel(size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				s := stream.FromSlice(data).
					Map(func(n int) int { return n * 2 })
				_, _ = s.ToSlice(context.Background())
			}
		})
	}
}

// BenchmarkChainedOperations measures chained filter+map performance.
func BenchmarkChainedOperations(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		data := make([]int, size)
		for i := range data {
			data[i] = i
		}

		b.Run(sizeLabel(size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				s := stream.FromSlice(data).
					Filter(func(n int) bool { return n%2 == 0 }).
					Map(func(n int) int { return n * 2 }).
					Filter(func(n int) bool { return n > 100 })
				_, _ = s.ToSlice(context.Background())
			}
		})
	}
}

// BenchmarkToSlice measures terminal operation performance.
func BenchmarkToSlice(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		data := make([]int, size)
		for i := range data {
			data[i] = i
		}

		b.Run(sizeLabel(size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				s := stream.FromSlice(data)
				_, _ = s.ToSlice(context.Background())
			}
		})
	}
}

// BenchmarkForEach measures forEach terminal operation.
func BenchmarkForEach(b *testing.B) {
	data := make([]int, 1000)
	for i := range data {
		data[i] = i
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := stream.FromSlice(data)
		_ = s.ForEach(context.Background(), func(_ int) {})
	}
}

// BenchmarkReduce measures reduce terminal operation.
func BenchmarkReduce(b *testing.B) {
	data := make([]int, 1000)
	for i := range data {
		data[i] = i
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := stream.FromSlice(data)
		_, _ = s.Reduce(context.Background(), 0, func(acc, n int) int { return acc + n })
	}
}

// BenchmarkDistinct measures distinct operation with varying uniqueness.
func BenchmarkDistinct(b *testing.B) {
	// 50% duplicates
	data := make([]int, 1000)
	for i := range data {
		data[i] = i % 500
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := stream.FromSlice(data).Distinct()
		_, _ = s.ToSlice(context.Background())
	}
}

// BenchmarkSkipLimit measures skip and limit operations.
func BenchmarkSkipLimit(b *testing.B) {
	data := make([]int, 10000)
	for i := range data {
		data[i] = i
	}

	b.Run("Skip1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			s := stream.FromSlice(data).Skip(1000)
			_, _ = s.ToSlice(context.Background())
		}
	})

	b.Run("Limit100", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			s := stream.FromSlice(data).Limit(100)
			_, _ = s.ToSlice(context.Background())
		}
	})

	b.Run("Skip1000_Limit100", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			s := stream.FromSlice(data).Skip(1000).Limit(100)
			_, _ = s.ToSlice(context.Background())
		}
	})
}

// sizeLabel returns a readable label for benchmark sizes.
func sizeLabel(size int) string {
	switch {
	case size >= 10000:
		return "10k"
	case size >= 1000:
		return "1k"
	case size >= 100:
		return "100"
	default:
		return "10"
	}
}
