package benchmark

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/vnykmshr/goflow/pkg/streaming/channel"
)

// BenchmarkChannelSend measures send operation performance.
func BenchmarkChannelSend(b *testing.B) {
	bufferSizes := []int{10, 100, 1000}

	for _, bufSize := range bufferSizes {
		b.Run(sizeLabel(bufSize), func(b *testing.B) {
			ch := channel.NewWithConfig[int](channel.Config{
				BufferSize: bufSize,
				Strategy:   channel.Block,
			})
			defer func() { _ = ch.Close() }()

			// Consumer goroutine
			done := make(chan struct{})
			go func() {
				defer close(done)
				ctx := context.Background()
				for {
					_, err := ch.Receive(ctx)
					if err != nil {
						return
					}
				}
			}()

			b.ReportAllocs()
			b.ResetTimer()
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				_ = ch.Send(ctx, i)
			}
			b.StopTimer()

			_ = ch.Close()
			<-done
		})
	}
}

// BenchmarkChannelReceive measures receive operation performance.
func BenchmarkChannelReceive(b *testing.B) {
	ch := channel.NewWithConfig[int](channel.Config{
		BufferSize: 1000,
		Strategy:   channel.Block,
	})
	defer func() { _ = ch.Close() }()

	// Pre-fill channel
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		_ = ch.Send(ctx, i)
	}

	// Producer goroutine to keep filling
	done := make(chan struct{})
	go func() {
		defer close(done)
		i := 1000
		for {
			if err := ch.Send(ctx, i); err != nil {
				return
			}
			i++
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ch.Receive(ctx)
	}
	b.StopTimer()

	_ = ch.Close()
	<-done
}

// BenchmarkChannelContention measures performance under concurrent access.
func BenchmarkChannelContention(b *testing.B) {
	contentionLevels := []int{2, 4, 8, 16}

	for _, producers := range contentionLevels {
		b.Run(contentionLabel(producers), func(b *testing.B) {
			ch := channel.NewWithConfig[int](channel.Config{
				BufferSize: 100,
				Strategy:   channel.Block,
			})
			defer func() { _ = ch.Close() }()

			// Consumer goroutines (half the producers)
			consumers := producers / 2
			if consumers < 1 {
				consumers = 1
			}

			var consumerWg sync.WaitGroup
			consumerWg.Add(consumers)
			for i := 0; i < consumers; i++ {
				go func() {
					defer consumerWg.Done()
					ctx := context.Background()
					for {
						_, err := ch.Receive(ctx)
						if err != nil {
							return
						}
					}
				}()
			}

			b.ReportAllocs()
			b.ResetTimer()

			var producerWg sync.WaitGroup
			perProducer := b.N / producers
			producerWg.Add(producers)

			for p := 0; p < producers; p++ {
				go func() {
					defer producerWg.Done()
					ctx := context.Background()
					for i := 0; i < perProducer; i++ {
						_ = ch.Send(ctx, i)
					}
				}()
			}

			producerWg.Wait()
			b.StopTimer()

			_ = ch.Close()
			consumerWg.Wait()
		})
	}
}

// BenchmarkDropStrategy measures Drop backpressure strategy.
func BenchmarkDropStrategy(b *testing.B) {
	ch := channel.NewWithConfig[int](channel.Config{
		BufferSize: 10,
		Strategy:   channel.Drop,
	})
	defer func() { _ = ch.Close() }()

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ch.Send(ctx, i)
	}
}

// BenchmarkDropOldestStrategy measures DropOldest backpressure strategy.
func BenchmarkDropOldestStrategy(b *testing.B) {
	ch := channel.NewWithConfig[int](channel.Config{
		BufferSize: 10,
		Strategy:   channel.DropOldest,
	})
	defer func() { _ = ch.Close() }()

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ch.Send(ctx, i)
	}
}

// BenchmarkChannelTryOperations measures non-blocking operations.
func BenchmarkChannelTryOperations(b *testing.B) {
	b.Run("TrySend_Empty", func(b *testing.B) {
		ch := channel.NewWithConfig[int](channel.Config{
			BufferSize: 100,
			Strategy:   channel.Block,
		})
		defer func() { _ = ch.Close() }()

		// Consumer to prevent blocking
		done := make(chan struct{})
		go func() {
			defer close(done)
			ctx := context.Background()
			for {
				_, err := ch.Receive(ctx)
				if err != nil {
					return
				}
			}
		}()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ch.TrySend(i)
		}
		b.StopTimer()

		_ = ch.Close()
		<-done
	})

	b.Run("TryReceive_HasData", func(b *testing.B) {
		ch := channel.NewWithConfig[int](channel.Config{
			BufferSize: 1000,
			Strategy:   channel.Block,
		})
		defer func() { _ = ch.Close() }()

		// Pre-fill
		ctx := context.Background()
		for i := 0; i < 1000; i++ {
			_ = ch.Send(ctx, i)
		}

		// Producer to keep filling
		done := make(chan struct{})
		go func() {
			defer close(done)
			i := 1000
			for {
				if err := ch.Send(ctx, i); err != nil {
					return
				}
				i++
			}
		}()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = ch.TryReceive()
		}
		b.StopTimer()

		_ = ch.Close()
		<-done
	})
}

// contentionLabel returns a readable label for contention levels.
func contentionLabel(level int) string {
	return strconv.Itoa(level) + "producers"
}
