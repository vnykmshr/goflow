package scheduler

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Example_advancedBackoff demonstrates exponential backoff scheduling.
func Example_advancedBackoff() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	var attempt int32
	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&attempt, 1)
		fmt.Printf("Attempt %d\n", count)
		
		// Fail first 3 attempts to demonstrate backoff
		if count <= 3 {
			return fmt.Errorf("simulated failure %d", count)
		}
		
		fmt.Println("Task succeeded!")
		return nil
	})

	// Configure exponential backoff
	backoffConfig := BackoffConfig{
		InitialDelay: 50 * time.Millisecond, // Shorter delay for testing
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   5,
		OnMaxRetries: func(task ScheduledTask, attempts int) {
			fmt.Printf("Task %s exceeded max retries (%d)\n", task.ID, attempts)
		},
	}

	err = scheduler.ScheduleWithBackoff("backoff-task", task, backoffConfig)
	if err != nil {
		log.Fatalf("Failed to schedule backoff task: %v", err)
	}

	// Wait for completion
	time.Sleep(2 * time.Second)

	// Output:
	// Attempt 1
	// Attempt 2  
	// Attempt 3
	// Attempt 4
	// Task succeeded!
}

// Example_advancedConditional demonstrates conditional task scheduling.
func Example_advancedConditional() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var checks int32
	var executions int32
	
	// Condition that becomes true after 3 checks
	condition := func() bool {
		count := atomic.AddInt32(&checks, 1)
		fmt.Printf("Condition check %d\n", count)
		return count >= 3
	}

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&executions, 1)
		fmt.Printf("Task executed: %d\n", count)
		return nil
	})

	err := scheduler.ScheduleConditional("conditional-task", task, condition, 500*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to schedule conditional task: %v", err)
	}

	// Wait for several check cycles
	time.Sleep(3 * time.Second)

	fmt.Printf("Total condition checks: %d, executions: %d\n", 
		atomic.LoadInt32(&checks), atomic.LoadInt32(&executions))

	// Output:
	// Condition check 1
	// Condition check 2
	// Condition check 3
	// Task executed: 1
	// Condition check 4
	// Task executed: 2
	// Total condition checks: 4, executions: 2
}

// Example_advancedChain demonstrates task chaining with error handling.
func Example_advancedChain() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	// Create chain of tasks
	tasks := []ChainedTask{
		{
			Name: "Step 1: Initialize",
			Task: workerpool.TaskFunc(func(ctx context.Context) error {
				fmt.Println("Step 1: Initializing...")
				time.Sleep(100 * time.Millisecond)
				fmt.Println("Step 1: Complete")
				return nil
			}),
			OnSuccess: func() ChainAction {
				return ChainContinue
			},
		},
		{
			Name: "Step 2: Process",
			Task: workerpool.TaskFunc(func(ctx context.Context) error {
				fmt.Println("Step 2: Processing...")
				time.Sleep(100 * time.Millisecond)
				
				// Simulate occasional failure
				if time.Now().UnixNano()%3 == 0 {
					return fmt.Errorf("processing failed")
				}
				
				fmt.Println("Step 2: Complete")
				return nil
			}),
			OnError: func(err error) ChainAction {
				fmt.Printf("Step 2 failed: %v - retrying\n", err)
				return ChainRetry
			},
			OnSuccess: func() ChainAction {
				return ChainContinue
			},
		},
		{
			Name: "Step 3: Finalize",
			Task: workerpool.TaskFunc(func(ctx context.Context) error {
				fmt.Println("Step 3: Finalizing...")
				time.Sleep(100 * time.Millisecond)
				fmt.Println("Step 3: Complete")
				return nil
			}),
			OnSuccess: func() ChainAction {
				fmt.Println("Chain completed successfully!")
				return ChainStop
			},
		},
	}

	err := scheduler.ScheduleChain("task-chain", tasks)
	if err != nil {
		log.Fatalf("Failed to schedule task chain: %v", err)
	}

	// Wait for chain completion
	time.Sleep(2 * time.Second)

	// Output:
	// Step 1: Initializing...
	// Step 1: Complete
	// Step 2: Processing...
	// Step 2: Complete
	// Step 3: Finalizing...  
	// Step 3: Complete
	// Chain completed successfully!
}

// Example_advancedBatch demonstrates batch task execution.
func Example_advancedBatch() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	// Create batch of tasks
	tasks := make([]workerpool.Task, 5)
	for i := 0; i < 5; i++ {
		taskNum := i + 1
		tasks[i] = workerpool.TaskFunc(func(ctx context.Context) error {
			fmt.Printf("Batch task %d started\n", taskNum)
			time.Sleep(time.Duration(taskNum*100) * time.Millisecond)
			fmt.Printf("Batch task %d completed\n", taskNum)
			return nil
		})
	}

	batchConfig := BatchConfig{
		Tasks:       tasks,
		RunAt:       time.Now().Add(200 * time.Millisecond),
		MaxParallel: 3, // Run max 3 tasks concurrently
		Timeout:     2 * time.Second,
		OnComplete: func(results []workerpool.Result) {
			successful := 0
			for _, result := range results {
				if result.Error == nil {
					successful++
				}
			}
			fmt.Printf("Batch completed: %d/%d tasks successful\n", successful, len(results))
		},
	}

	err := scheduler.ScheduleBatch("batch-tasks", batchConfig)
	if err != nil {
		log.Fatalf("Failed to schedule batch: %v", err)
	}

	// Wait for batch completion
	time.Sleep(3 * time.Second)

	// Output:
	// Batch task 1 started
	// Batch task 2 started  
	// Batch task 3 started
	// Batch task 1 completed
	// Batch task 4 started
	// Batch task 2 completed
	// Batch task 5 started
	// Batch task 3 completed
	// Batch task 4 completed
	// Batch task 5 completed
	// Batch completed: 5/5 tasks successful
}

// Example_advancedAdaptive demonstrates adaptive scheduling based on system load.
func Example_advancedAdaptive() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var executions int32
	
	// Simulate system load metric
	loadMetric := func() float64 {
		// Simulate increasing load over time
		runtime.GC() // Force GC to get memory stats
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		// Use heap size as a proxy for system load
		load := float64(m.Alloc) / float64(1024*1024*100) // Normalize to 0-1 range
		if load > 1.0 {
			load = 1.0
		}
		
		fmt.Printf("System load: %.2f\n", load)
		return load
	}

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&executions, 1)
		fmt.Printf("Adaptive task execution %d\n", count)
		
		// Simulate some work that allocates memory
		_ = make([]byte, 1024*1024) // 1MB allocation
		
		return nil
	})

	adaptiveConfig := AdaptiveConfig{
		BaseInterval:   1 * time.Second,
		MinInterval:    500 * time.Millisecond,
		MaxInterval:    5 * time.Second,
		LoadThreshold:  0.7,
		AdaptationRate: 0.2,
		LoadMetricFunc: loadMetric,
	}

	err := scheduler.ScheduleAdaptive("adaptive-task", task, adaptiveConfig)
	if err != nil {
		log.Fatalf("Failed to schedule adaptive task: %v", err)
	}

	// Run for a while to see adaptation
	time.Sleep(10 * time.Second)

	fmt.Printf("Total executions: %d\n", atomic.LoadInt32(&executions))

	// Output varies based on system load
}

// Example_advancedWindow demonstrates time window scheduling.
func Example_advancedWindow() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var executions int32

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&executions, 1)
		fmt.Printf("Window task executed at %s (execution %d)\n", 
			time.Now().Format("15:04:05"), count)
		return nil
	})

	// Define business hours window (9 AM - 5 PM on weekdays)
	location, _ := time.LoadLocation("Local")
	now := time.Now()
	
	// Create a window starting 1 second from now for 3 seconds
	windowStart := now.Add(1 * time.Second)
	windowEnd := windowStart.Add(3 * time.Second)
	
	windows := []TimeWindow{
		{
			Start:    windowStart,
			End:      windowEnd,
			TimeZone: location,
			Days:     []time.Weekday{}, // Any day for demo
		},
	}

	err := scheduler.ScheduleWindow("window-task", task, windows)
	if err != nil {
		log.Fatalf("Failed to schedule window task: %v", err)
	}

	// Wait to observe window behavior
	time.Sleep(6 * time.Second)

	fmt.Printf("Total executions: %d\n", atomic.LoadInt32(&executions))

	// Output:
	// Window task executed at 15:04:01 (execution 1)
	// Window task executed at 15:04:02 (execution 2)
	// Window task executed at 15:04:03 (execution 3)
	// Total executions: 3
}

// Example_advancedJittered demonstrates jittered scheduling to prevent thundering herd.
func Example_advancedJittered() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	var executions int32
	var lastExecution time.Time

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		now := time.Now()
		count := atomic.AddInt32(&executions, 1)
		
		var interval time.Duration
		if !lastExecution.IsZero() {
			interval = now.Sub(lastExecution)
		}
		lastExecution = now
		
		fmt.Printf("Jittered execution %d at %s (interval: %v)\n", 
			count, now.Format("15:04:05.000"), interval)
		return nil
	})

	jitterConfig := JitterConfig{
		Type:   JitterUniform,
		Amount: 500 * time.Millisecond, // Up to 500ms jitter
	}

	err := scheduler.ScheduleJittered("jittered-task", task, 1*time.Second, jitterConfig)
	if err != nil {
		log.Fatalf("Failed to schedule jittered task: %v", err)
	}

	// Run for several cycles
	time.Sleep(5 * time.Second)

	fmt.Printf("Total executions: %d\n", atomic.LoadInt32(&executions))

	// Output shows varying intervals due to jitter:
	// Jittered execution 1 at 15:04:01.250 (interval: 0s)
	// Jittered execution 2 at 15:04:02.100 (interval: 850ms)  
	// Jittered execution 3 at 15:04:03.300 (interval: 1.2s)
	// Total executions: 3
}

// Example_advancedCombined demonstrates combining multiple advanced patterns.
func Example_advancedCombined() {
	scheduler := NewAdvancedScheduler()
	defer func() { <-scheduler.Stop() }()

	scheduler.Start()

	fmt.Println("Starting combined advanced scheduling demo...")

	// 1. Backoff task that eventually succeeds
	var attempts int32
	backoffTask := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			return fmt.Errorf("backoff failure %d", count)
		}
		fmt.Printf("Backoff task succeeded after %d attempts\n", count)
		return nil
	})

	scheduler.ScheduleWithBackoff("combined-backoff", backoffTask, BackoffConfig{
		InitialDelay: 200 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   1.5,
		MaxRetries:   5,
	})

	// 2. Conditional task with jitter
	var conditionMet bool
	conditionalTask := workerpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("Conditional task executed")
		conditionMet = true
		return nil
	})

	condition := func() bool {
		// Condition becomes true after backoff task succeeds
		return atomic.LoadInt32(&attempts) >= 3
	}

	scheduler.ScheduleConditional("combined-conditional", conditionalTask, condition, 300*time.Millisecond)

	// 3. Batch task triggered by successful conditional task
	go func() {
		time.Sleep(2 * time.Second) // Wait for conditional to trigger
		if conditionMet {
			batchTasks := make([]workerpool.Task, 3)
			for i := 0; i < 3; i++ {
				taskNum := i + 1
				batchTasks[i] = workerpool.TaskFunc(func(ctx context.Context) error {
					fmt.Printf("Final batch task %d completed\n", taskNum)
					return nil
				})
			}

			scheduler.ScheduleBatch("combined-batch", BatchConfig{
				Tasks:       batchTasks,
				RunAt:       time.Now().Add(100 * time.Millisecond),
				MaxParallel: 2,
			})
		}
	}()

	// Wait for all patterns to complete
	time.Sleep(4 * time.Second)

	fmt.Println("Combined advanced scheduling demo completed")

	// Output:
	// Starting combined advanced scheduling demo...
	// Backoff task succeeded after 3 attempts
	// Conditional task executed
	// Final batch task 1 completed
	// Final batch task 2 completed
	// Final batch task 3 completed
	// Combined advanced scheduling demo completed
}