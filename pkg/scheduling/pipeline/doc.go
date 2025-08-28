/*
Package pipeline provides a flexible and powerful pipeline processing system for Go applications.

A pipeline enables sequential processing of data through multiple stages, with support for
parallel execution, error handling, monitoring, and comprehensive lifecycle management.

Basic Usage:

	pipeline := pipeline.New()

	// Add processing stages
	pipeline.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Validation logic
		return input, nil
	})

	pipeline.AddStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Transformation logic
		return processedInput, nil
	})

	// Execute pipeline
	result, err := pipeline.Execute(context.Background(), inputData)
	if err != nil {
		log.Fatal("Pipeline failed:", err)
	}

	fmt.Printf("Output: %v\n", result.Output)

Key Features:

The pipeline system provides:
  - Sequential stage execution with data flow between stages
  - Asynchronous execution with result channels
  - Integration with worker pools for parallel stage execution
  - Comprehensive error handling with continue-on-error option
  - Context-aware processing with timeout and cancellation support
  - Real-time statistics and performance monitoring
  - Flexible callback system for lifecycle events
  - Thread-safe concurrent pipeline execution

Stage Interface:

Stages implement a simple interface for maximum flexibility:

	type Stage interface {
		Execute(ctx context.Context, input interface{}) (interface{}, error)
		Name() string
	}

The StageFunc type provides convenient function-based stages:

	stage := pipeline.NewStageFunc("process", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Processing logic here
		return processedData, nil
	})

Configuration Options:

Advanced configuration through Config struct:

	config := pipeline.Config{
		WorkerPool:      workerpool.New(8, 100),
		Timeout:         30 * time.Second,
		StopOnError:     false,  // Continue processing on stage errors
		MaxConcurrency:  4,      // Limit concurrent stage execution
		OnStageStart: func(stageName string, input interface{}) {
			log.Printf("Starting stage: %s", stageName)
		},
		OnStageComplete: func(result pipeline.StageResult) {
			log.Printf("Stage %s completed in %v", result.StageName, result.Duration)
		},
		OnError: func(stageName string, err error) {
			log.Printf("Error in stage %s: %v", stageName, err)
		},
	}

	p := pipeline.NewWithConfig(config)

Execution Methods:

Multiple ways to execute pipelines:

	// Synchronous execution
	result, err := pipeline.Execute(ctx, data)

	// Asynchronous execution
	resultCh := pipeline.ExecuteAsync(ctx, data)
	result := <-resultCh

	// With timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := pipeline.Execute(ctx, data)

Results and Monitoring:

Comprehensive result information:

	result, err := pipeline.Execute(ctx, input)

	fmt.Printf("Input: %v\n", result.Input)
	fmt.Printf("Output: %v\n", result.Output)
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Stages: %d\n", len(result.StageResults))

	// Inspect individual stage results
	for _, stageResult := range result.StageResults {
		fmt.Printf("Stage %s: %v (took %v)\n",
			stageResult.StageName,
			stageResult.Error == nil,
			stageResult.Duration)
	}

Statistics and Performance:

Real-time pipeline statistics:

	stats := pipeline.Stats()
	fmt.Printf("Total executions: %d\n", stats.TotalExecutions)
	fmt.Printf("Success rate: %.2f%%\n",
		float64(stats.SuccessfulRuns)/float64(stats.TotalExecutions)*100)
	fmt.Printf("Average duration: %v\n", stats.AverageDuration)

	// Per-stage statistics
	for stageName, stageStats := range stats.StageStats {
		fmt.Printf("Stage %s: %d executions, %v avg duration\n",
			stageName, stageStats.ExecutionCount, stageStats.AverageDuration)
	}

Use Cases:

Data Processing Pipeline:

	pipeline := pipeline.New()

	// Data validation
	pipeline.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})
		if _, exists := data["id"]; !exists {
			return nil, errors.New("missing required field: id")
		}
		return data, nil
	})

	// Data enrichment
	pipeline.AddStageFunc("enrich", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(map[string]interface{})
		data["enriched_at"] = time.Now()
		data["enriched"] = true
		return data, nil
	})

	// Data transformation
	pipeline.AddStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
		// Transform data format
		return transformToOutputFormat(input), nil
	})

Image Processing Pipeline:

	pool := workerpool.New(4, 50) // CPU-intensive work
	pipeline := pipeline.New().SetWorkerPool(pool)

	pipeline.AddStageFunc("load", func(ctx context.Context, input interface{}) (interface{}, error) {
		filename := input.(string)
		return loadImage(filename)
	})

	pipeline.AddStageFunc("resize", func(ctx context.Context, input interface{}) (interface{}, error) {
		image := input.(Image)
		return resizeImage(image, 800, 600), nil
	})

	pipeline.AddStageFunc("filter", func(ctx context.Context, input interface{}) (interface{}, error) {
		image := input.(Image)
		return applyFilters(image), nil
	})

	pipeline.AddStageFunc("save", func(ctx context.Context, input interface{}) (interface{}, error) {
		image := input.(Image)
		return saveImage(image, "output.jpg")
	})

API Request Processing:

	config := pipeline.Config{
		Timeout:     30 * time.Second,
		StopOnError: false,
		OnError: func(stageName string, err error) {
			metrics.ErrorCounter.WithLabelValues(stageName).Inc()
		},
	}

	pipeline := pipeline.NewWithConfig(config)

	pipeline.AddStageFunc("authenticate", func(ctx context.Context, input interface{}) (interface{}, error) {
		request := input.(*http.Request)
		return authenticateUser(request)
	})

	pipeline.AddStageFunc("authorize", func(ctx context.Context, input interface{}) (interface{}, error) {
		user := input.(User)
		return checkPermissions(user)
	})

	pipeline.AddStageFunc("process", func(ctx context.Context, input interface{}) (interface{}, error) {
		user := input.(User)
		return processBusinessLogic(user)
	})

ETL (Extract, Transform, Load):

	pipeline := pipeline.New()

	// Extract
	pipeline.AddStageFunc("extract", func(ctx context.Context, input interface{}) (interface{}, error) {
		source := input.(DataSource)
		return extractData(source)
	})

	// Transform
	pipeline.AddStageFunc("clean", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(RawData)
		return cleanData(data), nil
	})

	pipeline.AddStageFunc("normalize", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(CleanData)
		return normalizeData(data), nil
	})

	pipeline.AddStageFunc("enrich", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(NormalizedData)
		return enrichData(data), nil
	})

	// Load
	pipeline.AddStageFunc("load", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(ProcessedData)
		return loadToDatabase(data)
	})

Machine Learning Pipeline:

	pipeline := pipeline.New()

	pipeline.AddStageFunc("preprocess", func(ctx context.Context, input interface{}) (interface{}, error) {
		rawData := input.(Dataset)
		return preprocessData(rawData), nil
	})

	pipeline.AddStageFunc("feature_extract", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(PreprocessedData)
		return extractFeatures(data), nil
	})

	pipeline.AddStageFunc("predict", func(ctx context.Context, input interface{}) (interface{}, error) {
		features := input.(FeatureVector)
		return model.Predict(features), nil
	})

	pipeline.AddStageFunc("postprocess", func(ctx context.Context, input interface{}) (interface{}, error) {
		prediction := input.(Prediction)
		return formatResult(prediction), nil
	})

Error Handling Strategies:

Stop on First Error (Default):

	pipeline := pipeline.New() // StopOnError: true by default

	result, err := pipeline.Execute(ctx, data)
	if err != nil {
		// Handle pipeline failure
		log.Printf("Pipeline failed: %v", err)
	}

Continue on Errors:

	config := pipeline.Config{
		StopOnError: false,
		OnError: func(stageName string, err error) {
			log.Printf("Stage %s failed: %v", stageName, err)
			// Log error but continue processing
		},
	}

	pipeline := pipeline.NewWithConfig(config)

	result, err := pipeline.Execute(ctx, data)
	// err will be nil even if some stages failed

	// Check individual stage results for errors
	for _, stageResult := range result.StageResults {
		if stageResult.Error != nil {
			log.Printf("Stage %s failed: %v", stageResult.StageName, stageResult.Error)
		}
	}

Context and Cancellation:

Pipelines fully support context cancellation and timeouts:

	// Pipeline timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := pipeline.Execute(ctx, data)
	if err == context.DeadlineExceeded {
		log.Println("Pipeline timed out")
	}

	// Manual cancellation
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Cancel after some condition
		time.Sleep(10 * time.Second)
		cancel()
	}()

	result, err := pipeline.Execute(ctx, data)

Worker Pool Integration:

Leverage worker pools for CPU-intensive stages:

	// Create worker pool sized for CPU-bound work
	pool := workerpool.New(runtime.NumCPU(), 100)
	defer pool.Shutdown()

	// Configure pipeline to use worker pool
	pipeline := pipeline.New().SetWorkerPool(pool)

	// CPU-intensive stages will now run in worker pool
	pipeline.AddStageFunc("compute", func(ctx context.Context, input interface{}) (interface{}, error) {
		return performHeavyComputation(input), nil
	})

Performance Characteristics:

  - Stage execution: Sequential by default, parallel with worker pool
  - Memory usage: Bounded by input/output data and stage count
  - Execution latency: Sum of individual stage latencies
  - Throughput: Limited by slowest stage (bottleneck analysis)
  - Error overhead: Minimal when no errors occur

Thread Safety:

All pipeline operations are safe for concurrent use from multiple goroutines.
The same pipeline instance can process multiple inputs simultaneously.

	// Safe to call concurrently
	go pipeline.Execute(ctx, input1)
	go pipeline.Execute(ctx, input2)
	go pipeline.Execute(ctx, input3)

Best Practices:

1. Design stages to be stateless and pure functions when possible
2. Use meaningful stage names for debugging and monitoring
3. Implement proper error handling in each stage
4. Set appropriate timeouts for long-running pipelines
5. Use worker pools for CPU-intensive stages
6. Monitor pipeline performance and bottlenecks
7. Keep stage logic focused and single-purpose
8. Use callbacks for logging, metrics, and observability
9. Test stages independently before integration
10. Consider data size and memory usage in stage design

Comparison with Alternatives:

vs Manual Sequential Processing:
  - Built-in error handling and recovery
  - Comprehensive monitoring and statistics
  - Flexible execution models (sync/async)
  - Better code organization and reusability

vs Channel-based Pipelines:
  - Higher-level abstraction with less boilerplate
  - Built-in result collection and error aggregation
  - Integrated worker pool support
  - Better observability and debugging

vs Workflow Engines:
  - Lightweight and embeddable in applications
  - Type-safe interface with compile-time checking
  - Better performance for simple sequential workflows
  - Less overhead for in-process data processing

Common Patterns:

Fan-out/Fan-in with Worker Pools:

	pool := workerpool.New(10, 100)
	pipeline := pipeline.New().SetWorkerPool(pool)

	pipeline.AddStageFunc("parallel_process", func(ctx context.Context, input interface{}) (interface{}, error) {
		// This stage will run in parallel across multiple workers
		return processItemInParallel(input), nil
	})

Conditional Processing:

	pipeline.AddStageFunc("conditional", func(ctx context.Context, input interface{}) (interface{}, error) {
		data := input.(ProcessingData)

		if data.RequiresSpecialProcessing {
			return performSpecialProcessing(data), nil
		}

		return performStandardProcessing(data), nil
	})

Data Validation and Sanitization:

	pipeline.AddStageFunc("validate", func(ctx context.Context, input interface{}) (interface{}, error) {
		if err := validateInput(input); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		return sanitizeInput(input), nil
	})

Result Aggregation:

	pipeline.AddStageFunc("aggregate", func(ctx context.Context, input interface{}) (interface{}, error) {
		items := input.([]ProcessedItem)
		return aggregateResults(items), nil
	})

Performance Monitoring:

	config := pipeline.Config{
		OnStageComplete: func(result pipeline.StageResult) {
			metrics.StageHistogram.WithLabelValues(result.StageName).
				Observe(result.Duration.Seconds())

			if result.Error != nil {
				metrics.StageErrors.WithLabelValues(result.StageName).Inc()
			}
		},
	}
*/
package pipeline
