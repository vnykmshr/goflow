package pipeline

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vnykmshr/goflow/internal/testutil"
	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// TestStage is a helper for testing pipeline stages.
type TestStage struct {
	name     string
	executed *int32
	duration time.Duration
	err      error
	output   interface{}
}

func (ts *TestStage) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	atomic.AddInt32(ts.executed, 1)

	if ts.duration > 0 {
		select {
		case <-time.After(ts.duration):
		case <-ctx.Done():
			return input, ctx.Err()
		}
	}

	if ts.err != nil {
		return input, ts.err
	}

	if ts.output != nil {
		return ts.output, nil
	}

	// Default behavior: append stage name to input
	if str, ok := input.(string); ok {
		return str + "->" + ts.name, nil
	}

	return input, nil
}

func (ts *TestStage) Name() string {
	return ts.name
}

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("pipeline should not be nil")
	}

	stages := p.GetStages()
	testutil.AssertEqual(t, len(stages), 0)
}

func TestNewWithConfig(t *testing.T) {
	config := Config{
		Timeout:     time.Second,
		StopOnError: false,
	}

	p := NewWithConfig(config)
	if p == nil {
		t.Fatal("pipeline should not be nil")
	}

	stages := p.GetStages()
	testutil.AssertEqual(t, len(stages), 0)
}

func TestAddStage(t *testing.T) {
	p := New()
	var executed int32
	stage := &TestStage{name: "test", executed: &executed}

	result := p.AddStage(stage)
	testutil.AssertEqual(t, result, p) // Should return self for chaining

	stages := p.GetStages()
	testutil.AssertEqual(t, len(stages), 1)
	testutil.AssertEqual(t, stages[0].Name(), "test")
}

func TestAddStageFunc(t *testing.T) {
	p := New()
	var executed int32

	p.AddStageFunc("func-stage", func(_ context.Context, input interface{}) (interface{}, error) {
		atomic.AddInt32(&executed, 1)
		return input, nil
	})

	stages := p.GetStages()
	testutil.AssertEqual(t, len(stages), 1)
	testutil.AssertEqual(t, stages[0].Name(), "func-stage")
}

func TestBasicExecution(t *testing.T) {
	p := New()
	var executed1, executed2 int32

	stage1 := &TestStage{name: "stage1", executed: &executed1}
	stage2 := &TestStage{name: "stage2", executed: &executed2}

	p.AddStage(stage1).AddStage(stage2)

	result, err := p.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err)

	if result == nil {
		t.Fatal("result should not be nil")
		return // Prevent nil pointer dereference in linter analysis
	}

	testutil.AssertEqual(t, result.Input, "input")
	testutil.AssertEqual(t, result.Output, "input->stage1->stage2")
	testutil.AssertEqual(t, len(result.StageResults), 2)
	testutil.AssertEqual(t, atomic.LoadInt32(&executed1), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&executed2), int32(1))
}

func TestExecuteAsync(t *testing.T) {
	p := New()
	var executed int32
	stage := &TestStage{name: "async", executed: &executed, duration: 50 * time.Millisecond}

	p.AddStage(stage)

	resultCh := p.ExecuteAsync(context.Background(), "test")
	result := <-resultCh

	testutil.AssertNoError(t, result.Error)
	testutil.AssertEqual(t, result.Input, "test")
	testutil.AssertEqual(t, result.Output, "test->async")
	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))
}

func TestStageError(t *testing.T) {
	p := New()
	var executed1, executed2 int32

	stage1 := &TestStage{name: "stage1", executed: &executed1}
	stage2 := &TestStage{name: "stage2", executed: &executed2, err: fmt.Errorf("stage2 failed")}
	stage3 := &TestStage{name: "stage3", executed: new(int32)}

	p.AddStage(stage1).AddStage(stage2).AddStage(stage3)

	result, err := p.Execute(context.Background(), "input")
	testutil.AssertError(t, err)

	if result == nil {
		t.Fatal("result should not be nil")
		return // Prevent nil pointer dereference in linter analysis
	}

	testutil.AssertEqual(t, result.Input, "input")
	testutil.AssertEqual(t, len(result.StageResults), 2) // Should stop on error
	testutil.AssertEqual(t, atomic.LoadInt32(&executed1), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&executed2), int32(1))
}

func TestStageErrorContinue(t *testing.T) {
	config := Config{
		StopOnError: false,
	}
	p := NewWithConfig(config)

	var executed1, executed2, executed3 int32

	stage1 := &TestStage{name: "stage1", executed: &executed1}
	stage2 := &TestStage{name: "stage2", executed: &executed2, err: fmt.Errorf("stage2 failed")}
	stage3 := &TestStage{name: "stage3", executed: &executed3}

	p.AddStage(stage1).AddStage(stage2).AddStage(stage3)

	result, err := p.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err) // No error because we continue on errors

	testutil.AssertEqual(t, len(result.StageResults), 3) // All stages executed
	testutil.AssertEqual(t, atomic.LoadInt32(&executed1), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&executed2), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&executed3), int32(1))

	// Check that stage2 has an error but stage3 processed stage1's output
	testutil.AssertError(t, result.StageResults[1].Error)
	testutil.AssertEqual(t, result.StageResults[2].Input, "input->stage1")
}

func TestContextCancellation(t *testing.T) {
	p := New()
	var executed int32
	stage := &TestStage{name: "slow", executed: &executed, duration: time.Second}

	p.AddStage(stage)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := p.Execute(ctx, "input")
	testutil.AssertError(t, err)
	testutil.AssertEqual(t, err, context.DeadlineExceeded)

	if result != nil {
		testutil.AssertEqual(t, result.Input, "input")
	}
}

func TestPipelineTimeout(t *testing.T) {
	config := Config{
		Timeout:     5 * time.Millisecond,
		StopOnError: true,
	}
	p := NewWithConfig(config)

	var executed int32
	stage := &TestStage{name: "slow", executed: &executed, duration: 500 * time.Millisecond}

	p.AddStage(stage)

	result, err := p.Execute(context.Background(), "input")
	if err == nil {
		t.Logf("Expected timeout error but got nil, duration was %v", result.Duration)
		t.Logf("Stage executed: %d", atomic.LoadInt32(&executed))
	}
	testutil.AssertError(t, err)
	if err != context.DeadlineExceeded {
		t.Logf("Expected DeadlineExceeded but got: %v", err)
	}
}

func TestWorkerPoolExecution(t *testing.T) {
	pool := workerpool.New(2, 10) //nolint:staticcheck // OK in tests
	defer func() { <-pool.Shutdown() }()

	p := New().SetWorkerPool(pool)

	var executed1, executed2 int32
	stage1 := &TestStage{name: "stage1", executed: &executed1, duration: 10 * time.Millisecond}
	stage2 := &TestStage{name: "stage2", executed: &executed2, duration: 10 * time.Millisecond}

	p.AddStage(stage1).AddStage(stage2)

	// Consume worker pool results
	go func() {
		for range pool.Results() {
			_ = 1 // Consume results
		}
	}()

	result, err := p.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, result.Output, "input->stage1->stage2")
	testutil.AssertEqual(t, atomic.LoadInt32(&executed1), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&executed2), int32(1))
}

func TestSetTimeout(t *testing.T) {
	p := New()
	result := p.SetTimeout(time.Second)
	testutil.AssertEqual(t, result, p) // Should return self for chaining
}

func TestStats(t *testing.T) {
	p := New()

	// Initially no stats
	stats := p.Stats()
	testutil.AssertEqual(t, stats.TotalExecutions, int64(0))
	testutil.AssertEqual(t, stats.SuccessfulRuns, int64(0))
	testutil.AssertEqual(t, stats.FailedRuns, int64(0))

	// Add stages and execute
	var executed int32
	stage := &TestStage{name: "test", executed: &executed}
	p.AddStage(stage)

	// Execute successfully
	_, err := p.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err)

	stats = p.Stats()
	testutil.AssertEqual(t, stats.TotalExecutions, int64(1))
	testutil.AssertEqual(t, stats.SuccessfulRuns, int64(1))
	testutil.AssertEqual(t, stats.FailedRuns, int64(0))

	// Check stage stats
	stageStats, exists := stats.StageStats["test"]
	testutil.AssertEqual(t, exists, true)
	testutil.AssertEqual(t, stageStats.ExecutionCount, int64(1))
	testutil.AssertEqual(t, stageStats.SuccessCount, int64(1))
	testutil.AssertEqual(t, stageStats.ErrorCount, int64(0))

	// Execute with error
	stage.err = fmt.Errorf("test error")
	_, err = p.Execute(context.Background(), "input")
	testutil.AssertError(t, err)

	stats = p.Stats()
	testutil.AssertEqual(t, stats.TotalExecutions, int64(2))
	testutil.AssertEqual(t, stats.SuccessfulRuns, int64(1))
	testutil.AssertEqual(t, stats.FailedRuns, int64(1))

	// Check updated stage stats
	stageStats, exists = stats.StageStats["test"]
	testutil.AssertEqual(t, exists, true)
	testutil.AssertEqual(t, stageStats.ExecutionCount, int64(2))
	testutil.AssertEqual(t, stageStats.SuccessCount, int64(1))
	testutil.AssertEqual(t, stageStats.ErrorCount, int64(1))
}

func TestCallbacks(t *testing.T) {
	var pipelineStarted, pipelineCompleted int32
	var stageStarted, stageCompleted int32
	var errorCallback int32

	config := Config{
		StopOnError: true, // Ensure we stop on first error
		OnPipelineStart: func(_ interface{}) {
			atomic.AddInt32(&pipelineStarted, 1)
		},
		OnPipelineComplete: func(_ Result) {
			atomic.AddInt32(&pipelineCompleted, 1)
		},
		OnStageStart: func(_ string, _ interface{}) {
			atomic.AddInt32(&stageStarted, 1)
		},
		OnStageComplete: func(_ StageResult) {
			atomic.AddInt32(&stageCompleted, 1)
		},
		OnError: func(_ string, _ error) {
			atomic.AddInt32(&errorCallback, 1)
		},
	}

	p := NewWithConfig(config)

	var executed int32
	stage := &TestStage{name: "test", executed: &executed, err: fmt.Errorf("test error")}
	p.AddStage(stage)

	_, err := p.Execute(context.Background(), "input")
	testutil.AssertError(t, err)

	testutil.AssertEqual(t, atomic.LoadInt32(&pipelineStarted), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&pipelineCompleted), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&stageStarted), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&stageCompleted), int32(1))
	testutil.AssertEqual(t, atomic.LoadInt32(&errorCallback), int32(1))
}

func TestStageFunc(t *testing.T) {
	var executed int32
	stage := NewStageFunc("func-test", func(_ context.Context, _ interface{}) (interface{}, error) {
		atomic.AddInt32(&executed, 1)
		return "processed", nil
	})

	testutil.AssertEqual(t, stage.Name(), "func-test")

	output, err := stage.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, output, "processed")
	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(1))
}

func TestResultStructure(t *testing.T) {
	p := New()

	var executed int32
	stage := &TestStage{name: "test", executed: &executed, duration: 10 * time.Millisecond}
	p.AddStage(stage)

	result, err := p.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err)

	// Check result structure
	testutil.AssertEqual(t, result.Input, "input")
	testutil.AssertEqual(t, result.Output, "input->test")
	testutil.AssertEqual(t, len(result.StageResults), 1)

	// Check timing
	if result.Duration <= 0 {
		t.Error("result duration should be positive")
	}
	if result.StartTime.IsZero() || result.EndTime.IsZero() {
		t.Error("result should have start and end times")
	}

	// Check stage result
	stageResult := result.StageResults[0]
	testutil.AssertEqual(t, stageResult.StageName, "test")
	testutil.AssertEqual(t, stageResult.Input, "input")
	testutil.AssertEqual(t, stageResult.Output, "input->test")
	testutil.AssertNoError(t, stageResult.Error)

	if stageResult.Duration <= 0 {
		t.Error("stage result duration should be positive")
	}
}

func TestConcurrentExecution(t *testing.T) {
	p := New()

	var executed int32
	stage := &TestStage{name: "concurrent", executed: &executed}
	p.AddStage(stage)

	// Run multiple concurrent executions
	done := make(chan struct{})
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			input := fmt.Sprintf("input-%d", id)
			result, err := p.Execute(context.Background(), input)
			testutil.AssertNoError(t, err)

			expected := fmt.Sprintf("input-%d->concurrent", id)
			testutil.AssertEqual(t, result.Output.(string), expected)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all executions completed
	testutil.AssertEqual(t, atomic.LoadInt32(&executed), int32(numGoroutines))

	// Check stats
	stats := p.Stats()
	testutil.AssertEqual(t, stats.TotalExecutions, int64(numGoroutines))
	testutil.AssertEqual(t, stats.SuccessfulRuns, int64(numGoroutines))
}

func TestEmptyPipeline(t *testing.T) {
	p := New()

	result, err := p.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, result.Input, "input")
	testutil.AssertEqual(t, result.Output, "input") // No stages, so output == input
	testutil.AssertEqual(t, len(result.StageResults), 0)
}

func TestStageResultTiming(t *testing.T) {
	p := New()

	duration := 50 * time.Millisecond
	var executed int32
	stage := &TestStage{name: "timed", executed: &executed, duration: duration}
	p.AddStage(stage)

	result, err := p.Execute(context.Background(), "input")
	testutil.AssertNoError(t, err)

	stageResult := result.StageResults[0]

	// Duration should be approximately the sleep time
	if stageResult.Duration < duration || stageResult.Duration > duration*2 {
		t.Errorf("expected duration around %v, got %v", duration, stageResult.Duration)
	}
}
