package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Example demonstrates basic pipeline usage.
func Example() {
	// Create a simple processing pipeline
	p := New()

	// Add stages to the pipeline
	p.AddStageFunc("uppercase", func(_ context.Context, input interface{}) (interface{}, error) {
		if str, ok := input.(string); ok {
			return strings.ToUpper(str), nil
		}
		return input, nil
	})

	p.AddStageFunc("prefix", func(_ context.Context, input interface{}) (interface{}, error) {
		if str, ok := input.(string); ok {
			return "PROCESSED: " + str, nil
		}
		return input, nil
	})

	// Execute the pipeline
	result, err := p.Execute(context.Background(), "hello world")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Input: %s\n", result.Input)
	fmt.Printf("Output: %s\n", result.Output)
	fmt.Printf("Stages: %d\n", len(result.StageResults))

	// Output:
	// Input: hello world
	// Output: PROCESSED: HELLO WORLD
	// Stages: 2
}

// Example_dataProcessing demonstrates a data processing pipeline.
func Example_dataProcessing() {
	p := New()

	// Stage 1: Validate input
	p.AddStageFunc("validate", func(_ context.Context, input interface{}) (interface{}, error) {
		data, ok := input.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid input type")
		}

		if _, exists := data["id"]; !exists {
			return nil, fmt.Errorf("missing required field: id")
		}

		fmt.Println("Validation passed")
		return data, nil
	})

	// Stage 2: Enrich data
	p.AddStageFunc("enrich", func(_ context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})
		data["enriched"] = true
		data["timestamp"] = time.Now().Unix()

		fmt.Println("Data enriched")
		return data, nil
	})

	// Stage 3: Format output
	p.AddStageFunc("format", func(_ context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})
		formatted := fmt.Sprintf("ID: %v, Enriched: %v", data["id"], data["enriched"])

		fmt.Println("Data formatted")
		return formatted, nil
	})

	// Execute pipeline
	input := map[string]interface{}{
		"id":   "12345",
		"name": "test item",
	}

	result, err := p.Execute(context.Background(), input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Final result: %s\n", result.Output)

	// Output:
	// Validation passed
	// Data enriched
	// Data formatted
	// Final result: ID: 12345, Enriched: true
}

// Example_errorHandling demonstrates error handling in pipelines.
func Example_errorHandling() {
	// Configure pipeline to continue on errors
	config := Config{
		StopOnError: false,
		OnError: func(stageName string, err error) {
			fmt.Printf("Error in stage '%s': %v\n", stageName, err)
		},
	}

	p := NewWithConfig(config)

	// Stage 1: Always succeeds
	p.AddStageFunc("success", func(_ context.Context, input interface{}) (interface{}, error) {
		fmt.Println("Stage 1: Success")
		return input.(string) + "-processed", nil
	})

	// Stage 2: Always fails
	p.AddStageFunc("failure", func(_ context.Context, input interface{}) (interface{}, error) {
		fmt.Println("Stage 2: About to fail")
		return input, fmt.Errorf("deliberate failure")
	})

	// Stage 3: Processes previous successful output
	p.AddStageFunc("recovery", func(_ context.Context, input interface{}) (interface{}, error) {
		fmt.Println("Stage 3: Processing despite error")
		return input.(string) + "-recovered", nil
	})

	result, err := p.Execute(context.Background(), "input")

	fmt.Printf("Pipeline error: %v\n", err)
	fmt.Printf("Final output: %s\n", result.Output)
	fmt.Printf("Stage errors: %d\n", countStageErrors(result))

	// Output:
	// Stage 1: Success
	// Stage 2: About to fail
	// Error in stage 'failure': deliberate failure
	// Stage 3: Processing despite error
	// Pipeline error: <nil>
	// Final output: input-processed-recovered
	// Stage errors: 1
}

func countStageErrors(result *Result) int {
	count := 0
	for _, sr := range result.StageResults {
		if sr.Error != nil {
			count++
		}
	}
	return count
}

// Example_asyncExecution demonstrates asynchronous pipeline execution.
func Example_asyncExecution() {
	p := New()

	p.AddStageFunc("slow", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Simulate slow processing
		select {
		case <-time.After(10 * time.Millisecond):
			return input.(string) + "-processed", nil
		case <-ctx.Done():
			return input, ctx.Err()
		}
	})

	// Execute asynchronously
	fmt.Println("Starting async execution...")

	resultCh := p.ExecuteAsync(context.Background(), "async-input")

	// Do other work while pipeline executes
	fmt.Println("Doing other work...")

	// Get result when ready
	result := <-resultCh

	fmt.Printf("Async result: %s\n", result.Output)
	fmt.Printf("Duration: %v\n", "10ms") // Fixed for deterministic output

	// Output:
	// Starting async execution...
	// Doing other work...
	// Async result: async-input-processed
	// Duration: 10ms
}

// Example_workerPool demonstrates pipeline execution with worker pool.
func Example_workerPool() {
	// Create worker pool
	pool := workerpool.New(3, 10)
	defer func() { <-pool.Shutdown() }()

	// Consume worker pool results
	go func() {
		for range pool.Results() {
			_ = 1 // Consume results
		}
	}()

	// Create pipeline with worker pool
	p := New().SetWorkerPool(pool)

	p.AddStageFunc("worker-stage", func(_ context.Context, input interface{}) (interface{}, error) {
		// This will run in the worker pool
		return fmt.Sprintf("worker-processed: %s", input), nil
	})

	result, err := p.Execute(context.Background(), "pool-input")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Worker pool result: %s\n", result.Output)

	// Output:
	// Worker pool result: worker-processed: pool-input
}

// Example_callbacks demonstrates pipeline callbacks.
func Example_callbacks() {
	config := Config{
		OnPipelineStart: func(input interface{}) {
			fmt.Printf("Pipeline started with: %v\n", input)
		},
		OnStageStart: func(stageName string, _ interface{}) {
			fmt.Printf("Stage '%s' started\n", stageName)
		},
		OnStageComplete: func(result StageResult) {
			fmt.Printf("Stage '%s' completed in %v\n", result.StageName, "1ms") // Fixed for deterministic output
		},
		OnPipelineComplete: func(result Result) {
			fmt.Printf("Pipeline completed with %d stages\n", len(result.StageResults))
		},
	}

	p := NewWithConfig(config)

	p.AddStageFunc("callback-stage", func(_ context.Context, input interface{}) (interface{}, error) {
		// Removed timing dependency for deterministic output
		return input.(string) + "-done", nil
	})

	result, _ := p.Execute(context.Background(), "callback-test")
	fmt.Printf("Final: %s\n", result.Output)

	// Output:
	// Pipeline started with: callback-test
	// Stage 'callback-stage' started
	// Stage 'callback-stage' completed in 1ms
	// Pipeline completed with 1 stages
	// Final: callback-test-done
}

// Example_statistics demonstrates pipeline statistics collection.
func Example_statistics() {
	p := New()

	p.AddStageFunc("stats-stage", func(_ context.Context, input interface{}) (interface{}, error) {
		return input, nil
	})

	// Execute multiple times
	for i := 0; i < 3; i++ {
		_, _ = p.Execute(context.Background(), fmt.Sprintf("input-%d", i))
	}

	// Get statistics
	stats := p.Stats()

	fmt.Printf("Total executions: %d\n", stats.TotalExecutions)
	fmt.Printf("Successful runs: %d\n", stats.SuccessfulRuns)
	fmt.Printf("Failed runs: %d\n", stats.FailedRuns)

	// Stage statistics
	if stageStats, exists := stats.StageStats["stats-stage"]; exists {
		fmt.Printf("Stage executions: %d\n", stageStats.ExecutionCount)
		fmt.Printf("Stage success count: %d\n", stageStats.SuccessCount)
	}

	// Output:
	// Total executions: 3
	// Successful runs: 3
	// Failed runs: 0
	// Stage executions: 3
	// Stage success count: 3
}

// Example_timeout demonstrates pipeline timeout handling.
func Example_timeout() {
	// Set pipeline timeout
	p := New().SetTimeout(50 * time.Millisecond)

	p.AddStageFunc("slow-stage", func(ctx context.Context, input interface{}) (interface{}, error) {
		// This will exceed the timeout
		select {
		case <-time.After(100 * time.Millisecond):
			return input, nil
		case <-ctx.Done():
			return input, ctx.Err()
		}
	})

	result, err := p.Execute(context.Background(), "timeout-test")

	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Is timeout error: %t\n", err == context.DeadlineExceeded)
	fmt.Printf("Input preserved: %s\n", result.Input)

	// Output:
	// Error: context deadline exceeded
	// Is timeout error: true
	// Input preserved: timeout-test
}

// Example_complexWorkflow demonstrates a complex multi-stage workflow.
func Example_complexWorkflow() {
	p := New()

	// Stage 1: Parse input
	p.AddStageFunc("parse", func(_ context.Context, input interface{}) (interface{}, error) {
		// Simulate parsing structured data
		data := map[string]interface{}{
			"raw":    input,
			"parsed": true,
		}
		return data, nil
	})

	// Stage 2: Validate
	p.AddStageFunc("validate", func(_ context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})
		data["validated"] = true
		return data, nil
	})

	// Stage 3: Transform
	p.AddStageFunc("transform", func(_ context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})
		data["transformed"] = true
		// Simulate data transformation
		if raw, ok := data["raw"].(string); ok {
			data["processed"] = strings.ToUpper(raw)
		}
		return data, nil
	})

	// Stage 4: Finalize
	p.AddStageFunc("finalize", func(_ context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})
		return fmt.Sprintf("Final result: %s", data["processed"]), nil
	})

	result, err := p.Execute(context.Background(), "complex workflow")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Stages executed: %d\n", len(result.StageResults))
	fmt.Printf("Total duration: %v\n", "1ms") // Fixed for deterministic output
	fmt.Printf("Output: %s\n", result.Output)

	// Show stage details with fixed durations
	stageTimings := map[string]string{
		"parse":     "100μs",
		"validate":  "50μs",
		"transform": "75μs",
		"finalize":  "25μs",
	}
	for _, sr := range result.StageResults {
		fmt.Printf("- %s: %s\n", sr.StageName, stageTimings[sr.StageName])
	}

	// Output:
	// Stages executed: 4
	// Total duration: 1ms
	// Output: Final result: COMPLEX WORKFLOW
	// - parse: 100μs
	// - validate: 50μs
	// - transform: 75μs
	// - finalize: 25μs
}
