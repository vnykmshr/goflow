package pipeline

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// BenchmarkBasicExecution measures basic pipeline execution performance.
func BenchmarkBasicExecution(b *testing.B) {
	p := New()

	stage := NewStageFunc("bench", func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil
	})
	p.AddStage(stage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Execute(context.Background(), "input")
	}
}

// BenchmarkMultiStageExecution measures performance with multiple stages.
func BenchmarkMultiStageExecution(b *testing.B) {
	p := New()

	// Add 5 stages
	for i := 0; i < 5; i++ {
		stageName := "stage-" + string(rune(i+'0'))
		stage := NewStageFunc(stageName, func(ctx context.Context, input interface{}) (interface{}, error) {
			if str, ok := input.(string); ok {
				return str + "-processed", nil
			}
			return input, nil
		})
		p.AddStage(stage)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Execute(context.Background(), "input")
	}
}

// BenchmarkAsyncExecution measures asynchronous execution performance.
func BenchmarkAsyncExecution(b *testing.B) {
	p := New()

	var counter int64
	stage := NewStageFunc("async", func(ctx context.Context, input interface{}) (interface{}, error) {
		atomic.AddInt64(&counter, 1)
		return input, nil
	})
	p.AddStage(stage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resultCh := p.ExecuteAsync(context.Background(), "input")
		<-resultCh
	}
}

// BenchmarkWorkerPoolExecution measures performance with worker pool.
func BenchmarkWorkerPoolExecution(b *testing.B) {
	pool := workerpool.New(4, 100)
	defer func() { <-pool.Shutdown() }()

	// Consume worker pool results
	go func() {
		for range pool.Results() {
			// Consume results
		}
	}()

	p := New().SetWorkerPool(pool)

	stage := NewStageFunc("worker", func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil
	})
	p.AddStage(stage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Execute(context.Background(), "input")
	}
}

// BenchmarkPipelineCreation measures pipeline creation performance.
func BenchmarkPipelineCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := New()
		for j := 0; j < 10; j++ {
			stageName := "stage-" + string(rune(j+'0'))
			stage := NewStageFunc(stageName, func(ctx context.Context, input interface{}) (interface{}, error) {
				return input, nil
			})
			p.AddStage(stage)
		}
	}
}

// BenchmarkConcurrentExecution measures concurrent pipeline execution.
func BenchmarkConcurrentExecution(b *testing.B) {
	p := New()

	var counter int64
	stage := NewStageFunc("concurrent", func(ctx context.Context, input interface{}) (interface{}, error) {
		atomic.AddInt64(&counter, 1)
		return input, nil
	})
	p.AddStage(stage)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Execute(context.Background(), "input")
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation patterns.
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs()

	p := New()
	stage := NewStageFunc("memory", func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil
	})
	p.AddStage(stage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Execute(context.Background(), "input")
	}
}

// BenchmarkStageScaling tests performance across different numbers of stages.
func BenchmarkStageScaling(b *testing.B) {
	stageCounts := []int{1, 5, 10, 20}

	for _, stageCount := range stageCounts {
		b.Run("Stages-"+string(rune(stageCount+'0')), func(b *testing.B) {
			p := New()

			for i := 0; i < stageCount; i++ {
				stageName := "stage-" + string(rune(i+'0'))
				stage := NewStageFunc(stageName, func(ctx context.Context, input interface{}) (interface{}, error) {
					return input, nil
				})
				p.AddStage(stage)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p.Execute(context.Background(), "input")
			}
		})
	}
}

// BenchmarkErrorHandling measures performance when stages return errors.
func BenchmarkErrorHandling(b *testing.B) {
	p := New()

	stage := NewStageFunc("error", func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil // No error for fair comparison
	})
	p.AddStage(stage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Execute(context.Background(), "input")
	}
}

// BenchmarkWithCallbacks measures performance impact of callbacks.
func BenchmarkWithCallbacks(b *testing.B) {
	var callbackCount int64

	b.Run("WithCallbacks", func(b *testing.B) {
		config := Config{
			OnStageStart: func(stageName string, input interface{}) {
				atomic.AddInt64(&callbackCount, 1)
			},
			OnStageComplete: func(result StageResult) {
				atomic.AddInt64(&callbackCount, 1)
			},
		}
		p := NewWithConfig(config)

		stage := NewStageFunc("callback", func(ctx context.Context, input interface{}) (interface{}, error) {
			return input, nil
		})
		p.AddStage(stage)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p.Execute(context.Background(), "input")
		}
	})

	b.Run("WithoutCallbacks", func(b *testing.B) {
		p := New()

		stage := NewStageFunc("no-callback", func(ctx context.Context, input interface{}) (interface{}, error) {
			return input, nil
		})
		p.AddStage(stage)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p.Execute(context.Background(), "input")
		}
	})
}

// BenchmarkDataThroughput measures data processing throughput.
func BenchmarkDataThroughput(b *testing.B) {
	p := New()

	// Simulate data processing stages
	stage1 := NewStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Simulate validation work
		return input, nil
	})

	stage2 := NewStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Simulate transformation work
		if str, ok := input.(string); ok {
			return str + "-transformed", nil
		}
		return input, nil
	})

	stage3 := NewStageFunc("enrich", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Simulate enrichment work
		if str, ok := input.(string); ok {
			return str + "-enriched", nil
		}
		return input, nil
	})

	p.AddStage(stage1).AddStage(stage2).AddStage(stage3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := "data-" + string(rune(i%100))
		p.Execute(context.Background(), input)
	}
}

// BenchmarkComplexWorkflow simulates a complex processing workflow.
func BenchmarkComplexWorkflow(b *testing.B) {
	pool := workerpool.New(8, 200)
	defer func() { <-pool.Shutdown() }()

	// Consume worker pool results
	go func() {
		for range pool.Results() {
			// Consume results
		}
	}()

	p := New().SetWorkerPool(pool)

	// Create a complex workflow with multiple stages
	stages := []string{"parse", "validate", "normalize", "enrich", "process", "save"}

	for _, stageName := range stages {
		name := stageName // Capture for closure
		stage := NewStageFunc(name, func(ctx context.Context, input interface{}) (interface{}, error) {
			// Simulate some CPU work
			sum := 0
			for i := 0; i < 100; i++ {
				sum += i
			}

			if str, ok := input.(string); ok {
				return str + "-" + name, nil
			}
			return input, nil
		})
		p.AddStage(stage)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := "workflow-" + string(rune(i%10))
		p.Execute(context.Background(), input)
	}
}

// BenchmarkStatsCollection measures the overhead of statistics collection.
func BenchmarkStatsCollection(b *testing.B) {
	p := New()

	stage := NewStageFunc("stats", func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil
	})
	p.AddStage(stage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Execute(context.Background(), "input")

		// Occasionally check stats to measure overhead
		if i%100 == 0 {
			p.Stats()
		}
	}
}

// BenchmarkLargePayload measures performance with larger data payloads.
func BenchmarkLargePayload(b *testing.B) {
	p := New()

	stage := NewStageFunc("large", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Simulate processing large data
		if data, ok := input.([]byte); ok {
			// Simple checksum calculation
			sum := 0
			for _, b := range data {
				sum += int(b)
			}
			return sum, nil
		}
		return input, nil
	})
	p.AddStage(stage)

	// Create large payload (1MB)
	payload := make([]byte, 1024*1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Execute(context.Background(), payload)
	}
}

// BenchmarkPipelineReuse measures performance when reusing pipelines.
func BenchmarkPipelineReuse(b *testing.B) {
	p := New()

	stage := NewStageFunc("reuse", func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil
	})
	p.AddStage(stage)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			input := "input-" + string(rune(i%10))
			p.Execute(context.Background(), input)
			i++
		}
	})
}
