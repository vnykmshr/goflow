package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Stage represents a single processing stage in a pipeline.
type Stage interface {
	// Execute processes the input data and returns the result.
	Execute(ctx context.Context, input interface{}) (interface{}, error)

	// Name returns a unique identifier for this stage.
	Name() string
}

// StageFunc is a function type that implements the Stage interface.
type StageFunc struct {
	name string
	fn   func(ctx context.Context, input interface{}) (interface{}, error)
}

// Execute implements the Stage interface for StageFunc.
func (sf *StageFunc) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	return sf.fn(ctx, input)
}

// Name returns the stage name.
func (sf *StageFunc) Name() string {
	return sf.name
}

// NewStageFunc creates a new stage from a function.
func NewStageFunc(name string, fn func(ctx context.Context, input interface{}) (interface{}, error)) Stage {
	return &StageFunc{name: name, fn: fn}
}

// Pipeline represents a series of stages that process data sequentially or in parallel.
type Pipeline interface {
	// Execute runs the pipeline with the given input data.
	Execute(ctx context.Context, input interface{}) (*Result, error)

	// ExecuteAsync runs the pipeline asynchronously and returns a channel for the result.
	ExecuteAsync(ctx context.Context, input interface{}) <-chan *Result

	// AddStage adds a stage to the pipeline.
	AddStage(stage Stage) Pipeline

	// AddStageFunc adds a stage function to the pipeline.
	AddStageFunc(name string, fn func(ctx context.Context, input interface{}) (interface{}, error)) Pipeline

	// SetWorkerPool sets the worker pool for parallel execution.
	SetWorkerPool(pool workerpool.Pool) Pipeline

	// SetTimeout sets the default timeout for pipeline execution.
	SetTimeout(timeout time.Duration) Pipeline

	// GetStages returns all stages in the pipeline.
	GetStages() []Stage

	// Stats returns pipeline execution statistics.
	Stats() Stats
}

// Result represents the outcome of a pipeline execution.
type Result struct {
	// Input is the original input data
	Input interface{}

	// Output is the final output data
	Output interface{}

	// Error is any error that occurred during execution
	Error error

	// Duration is the total execution time
	Duration time.Duration

	// StageResults contains results from each stage
	StageResults []StageResult

	// StartTime is when the pipeline execution started
	StartTime time.Time

	// EndTime is when the pipeline execution finished
	EndTime time.Time
}

// StageResult represents the result of a single stage execution.
type StageResult struct {
	// StageName is the name of the stage
	StageName string

	// Input is the input to this stage
	Input interface{}

	// Output is the output from this stage
	Output interface{}

	// Error is any error from this stage
	Error error

	// Duration is how long this stage took
	Duration time.Duration

	// StartTime is when the stage started
	StartTime time.Time

	// EndTime is when the stage finished
	EndTime time.Time
}

// Stats holds pipeline execution statistics.
type Stats struct {
	TotalExecutions int64
	SuccessfulRuns  int64
	FailedRuns      int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	StageStats      map[string]StageStats
	LastExecutionAt time.Time
}

// StageStats holds statistics for individual stages.
type StageStats struct {
	Name            string
	ExecutionCount  int64
	SuccessCount    int64
	ErrorCount      int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
}

// Config holds pipeline configuration options.
type Config struct {
	// WorkerPool is the worker pool for parallel execution.
	// If nil, stages run sequentially.
	WorkerPool workerpool.Pool

	// Timeout is the default timeout for pipeline execution.
	Timeout time.Duration

	// OnStageStart is called when a stage starts execution.
	OnStageStart func(stageName string, input interface{})

	// OnStageComplete is called when a stage completes.
	OnStageComplete func(result StageResult)

	// OnPipelineStart is called when pipeline execution starts.
	OnPipelineStart func(input interface{})

	// OnPipelineComplete is called when pipeline execution completes.
	OnPipelineComplete func(result Result)

	// OnError is called when an error occurs.
	OnError func(stageName string, err error)

	// StopOnError determines if pipeline should stop on first error.
	// If false, errors are collected and pipeline continues.
	StopOnError bool

	// MaxConcurrency limits concurrent stage execution when using worker pool.
	// 0 means no limit.
	MaxConcurrency int
}

// pipeline implements the Pipeline interface.
type pipeline struct {
	stages []Stage
	config Config
	stats  Stats
	mu     sync.RWMutex
}

// New creates a new pipeline with default configuration.
func New() Pipeline {
	return NewWithConfig(Config{
		StopOnError: true,
	})
}

// NewWithConfig creates a new pipeline with the specified configuration.
func NewWithConfig(config Config) Pipeline {
	return &pipeline{
		stages: make([]Stage, 0),
		config: config,
		stats: Stats{
			StageStats: make(map[string]StageStats),
		},
	}
}

// Execute runs the pipeline with the given input data.
func (p *pipeline) Execute(ctx context.Context, input interface{}) (*Result, error) {
	resultCh := p.ExecuteAsync(ctx, input)
	result := <-resultCh
	return result, result.Error
}

// ExecuteAsync runs the pipeline asynchronously and returns a channel for the result.
func (p *pipeline) ExecuteAsync(ctx context.Context, input interface{}) <-chan *Result {
	resultCh := make(chan *Result, 1)

	go func() {
		defer close(resultCh)

		startTime := time.Now()
		result := &Result{
			Input:        input,
			StartTime:    startTime,
			StageResults: make([]StageResult, 0, len(p.stages)),
		}

		// Call pipeline start callback
		if p.config.OnPipelineStart != nil {
			p.config.OnPipelineStart(input)
		}

		// Apply timeout if configured
		if p.config.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, p.config.Timeout)
			defer cancel()
		}

		// Execute stages
		output, err := p.executeStages(ctx, input, result)

		// Set final results
		result.Output = output
		result.Error = err
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		// Update statistics
		p.updateStats(result)

		// Call pipeline complete callback
		if p.config.OnPipelineComplete != nil {
			p.config.OnPipelineComplete(*result)
		}

		resultCh <- result
	}()

	return resultCh
}

// executeStages runs all stages in the pipeline.
func (p *pipeline) executeStages(ctx context.Context, input interface{}, result *Result) (interface{}, error) {
	currentOutput := input

	for _, stage := range p.stages {
		select {
		case <-ctx.Done():
			return currentOutput, ctx.Err()
		default:
		}

		stageResult := p.executeStage(ctx, stage, currentOutput)
		result.StageResults = append(result.StageResults, stageResult)

		if stageResult.Error != nil {
			if p.config.OnError != nil {
				p.config.OnError(stage.Name(), stageResult.Error)
			}

			if p.config.StopOnError {
				return currentOutput, stageResult.Error
			}
		} else {
			currentOutput = stageResult.Output
		}
	}

	return currentOutput, nil
}

// executeStage executes a single stage.
func (p *pipeline) executeStage(ctx context.Context, stage Stage, input interface{}) StageResult {
	startTime := time.Now()

	// Call stage start callback
	if p.config.OnStageStart != nil {
		p.config.OnStageStart(stage.Name(), input)
	}

	var output interface{}
	var err error

	// Execute stage (potentially using worker pool)
	if p.config.WorkerPool != nil {
		output, err = p.executeStageInPool(ctx, stage, input)
	} else {
		output, err = stage.Execute(ctx, input)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	stageResult := StageResult{
		StageName: stage.Name(),
		Input:     input,
		Output:    output,
		Error:     err,
		Duration:  duration,
		StartTime: startTime,
		EndTime:   endTime,
	}

	// Update stage statistics
	p.updateStageStats(stage.Name(), stageResult)

	// Call stage complete callback
	if p.config.OnStageComplete != nil {
		p.config.OnStageComplete(stageResult)
	}

	return stageResult
}

// executeStageInPool executes a stage using the worker pool.
func (p *pipeline) executeStageInPool(ctx context.Context, stage Stage, input interface{}) (interface{}, error) {
	resultCh := make(chan interface{}, 1)
	errorCh := make(chan error, 1)

	task := workerpool.TaskFunc(func(taskCtx context.Context) error {
		output, err := stage.Execute(taskCtx, input)
		if err != nil {
			errorCh <- err
			return err
		}
		resultCh <- output
		return nil
	})

	// Submit task to worker pool with context propagation
	if err := p.config.WorkerPool.SubmitWithContext(ctx, task); err != nil {
		return input, fmt.Errorf("failed to submit stage %s to worker pool: %w", stage.Name(), err)
	}

	// Wait for result
	select {
	case output := <-resultCh:
		return output, nil
	case err := <-errorCh:
		return input, err
	case <-ctx.Done():
		return input, ctx.Err()
	}
}

// AddStage adds a stage to the pipeline.
func (p *pipeline) AddStage(stage Stage) Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stages = append(p.stages, stage)

	// Initialize stage stats if not exists
	if _, exists := p.stats.StageStats[stage.Name()]; !exists {
		p.stats.StageStats[stage.Name()] = StageStats{
			Name: stage.Name(),
		}
	}

	return p
}

// AddStageFunc adds a stage function to the pipeline.
func (p *pipeline) AddStageFunc(name string, fn func(ctx context.Context, input interface{}) (interface{}, error)) Pipeline {
	return p.AddStage(NewStageFunc(name, fn))
}

// SetWorkerPool sets the worker pool for parallel execution.
func (p *pipeline) SetWorkerPool(pool workerpool.Pool) Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.WorkerPool = pool
	return p
}

// SetTimeout sets the default timeout for pipeline execution.
func (p *pipeline) SetTimeout(timeout time.Duration) Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Timeout = timeout
	return p
}

// GetStages returns all stages in the pipeline.
func (p *pipeline) GetStages() []Stage {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stages := make([]Stage, len(p.stages))
	copy(stages, p.stages)
	return stages
}

// Stats returns pipeline execution statistics.
func (p *pipeline) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Create a copy to avoid race conditions
	statsCopy := p.stats
	statsCopy.StageStats = make(map[string]StageStats)
	for k, v := range p.stats.StageStats {
		statsCopy.StageStats[k] = v
	}

	// Calculate average duration
	if statsCopy.TotalExecutions > 0 {
		statsCopy.AverageDuration = time.Duration(int64(statsCopy.TotalDuration) / statsCopy.TotalExecutions)
	}

	return statsCopy
}

// updateStats updates pipeline statistics.
func (p *pipeline) updateStats(result *Result) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalExecutions++
	p.stats.TotalDuration += result.Duration
	p.stats.LastExecutionAt = result.EndTime

	if result.Error == nil {
		p.stats.SuccessfulRuns++
	} else {
		p.stats.FailedRuns++
	}
}

// updateStageStats updates statistics for a specific stage.
func (p *pipeline) updateStageStats(stageName string, result StageResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats, exists := p.stats.StageStats[stageName]
	if !exists {
		stats = StageStats{Name: stageName}
	}

	stats.ExecutionCount++
	stats.TotalDuration += result.Duration

	if result.Error == nil {
		stats.SuccessCount++
	} else {
		stats.ErrorCount++
	}

	// Calculate average duration
	if stats.ExecutionCount > 0 {
		stats.AverageDuration = time.Duration(int64(stats.TotalDuration) / stats.ExecutionCount)
	}

	p.stats.StageStats[stageName] = stats
}
