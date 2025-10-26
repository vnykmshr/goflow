/*
Package pipeline provides multi-stage data processing with error handling and monitoring.

Process data through sequential stages with support for timeouts, worker pools, and callbacks.

# Quick Start

	p := pipeline.New()

	p.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
		return input, nil  // Validation logic
	})

	p.AddStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
		return processedInput, nil  // Transform logic
	})

	result, err := p.Execute(context.Background(), inputData)
	fmt.Printf("Output: %v\n", result.Output)

# Configuration

	pool, _ := workerpool.NewSafe(8, 100)

	config := pipeline.Config{
		WorkerPool:     pool,
		Timeout:        30 * time.Second,
		StopOnError:    false,              // Continue on errors
		MaxConcurrency: 4,                  // Parallel stages
	}

	p := pipeline.NewWithConfig(config)

# Adding Stages

Function-based stages:

	p.AddStageFunc("process", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Processing logic
		return output, nil
	})

Custom stage types:

	type MyStage struct{}

	func (s *MyStage) Name() string { return "my-stage" }

	func (s *MyStage) Execute(ctx context.Context, input interface{}) (interface{}, error) {
		// Custom logic
		return output, nil
	}

	p.AddStage(&MyStage{})

# Execution

Synchronous:

	result, err := p.Execute(ctx, data)

Asynchronous:

	resultCh := p.ExecuteAsync(ctx, data)
	result := <-resultCh

With timeout:

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := p.Execute(ctx, data)

# Results

	result, _ := p.Execute(ctx, input)

	fmt.Printf("Output: %v\n", result.Output)
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Stages completed: %d\n", len(result.StageResults))

	// Check stage-specific results
	for _, stageResult := range result.StageResults {
		fmt.Printf("Stage %s: %v (took %v)\n",
			stageResult.StageName, stageResult.Output, stageResult.Duration)
	}

# Error Handling

Stop on first error (default):

	config := pipeline.Config{
		StopOnError: true,
	}

Continue on errors:

	config := pipeline.Config{
		StopOnError: false,
		OnError: func(stageName string, err error) {
			log.Printf("Stage %s failed: %v", stageName, err)
		},
	}

# Monitoring

Track pipeline execution with callbacks:

	config := pipeline.Config{
		OnStageStart: func(stageName string, input interface{}) {
			log.Printf("Starting: %s", stageName)
		},
		OnStageComplete: func(result pipeline.StageResult) {
			log.Printf("%s completed in %v", result.StageName, result.Duration)
		},
		OnPipelineComplete: func(result pipeline.Result) {
			log.Printf("Pipeline completed: %d stages in %v",
				len(result.StageResults), result.Duration)
		},
	}

# Statistics

	stats := p.Stats()
	fmt.Printf("Total executions: %d\n", stats.TotalExecutions)
	fmt.Printf("Successful: %d, Failed: %d\n", stats.SuccessfulRuns, stats.FailedRuns)
	fmt.Printf("Average duration: %v\n", stats.AverageDuration)

# Worker Pool Integration

Process stages in a worker pool for better resource management:

	pool := workerpool.NewWithConfig(workerpool.Config{
		WorkerCount: 10,
		QueueSize:   100,
	})

	config := pipeline.Config{
		WorkerPool: pool,
	}

	p := pipeline.NewWithConfig(config)

# Use Cases

Data Validation and Transformation:

	p := pipeline.New()
	p.AddStageFunc("validate", validateData)
	p.AddStageFunc("sanitize", sanitizeData)
	p.AddStageFunc("transform", transformData)
	p.AddStageFunc("persist", persistData)

	result, _ := p.Execute(ctx, userInput)

ETL Pipeline:

	p := pipeline.New()
	p.AddStageFunc("extract", extractFromSource)
	p.AddStageFunc("transform", applyBusinessLogic)
	p.AddStageFunc("load", loadToDestination)

	p.Execute(ctx, dataSource)

Request Processing:

	p := pipeline.New()
	p.AddStageFunc("auth", authenticateRequest)
	p.AddStageFunc("validate", validateRequest)
	p.AddStageFunc("process", processRequest)
	p.AddStageFunc("respond", formatResponse)

	result, _ := p.Execute(ctx, httpRequest)

# Thread Safety

Pipelines can be safely executed concurrently from multiple goroutines.

# Performance Notes

- StopOnError=true fails faster but may waste work
- StopOnError=false processes all stages but may do unnecessary work
- MaxConcurrency limits resource usage but may slow execution
- Worker pools reuse goroutines, reducing overhead for many executions

See example tests for more patterns.
*/
package pipeline
