package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// Example_contextTimeout demonstrates task scheduling with timeouts.
func Example_contextTimeout() {
	scheduler := NewContextAwareScheduler()
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	var completed int32
	var timedOut int32

	// Task that takes a long time to complete
	longTask := workerpool.TaskFunc(func(ctx context.Context) error {
		select {
		case <-time.After(2 * time.Second):
			atomic.AddInt32(&completed, 1)
			fmt.Println("Long task completed")
			return nil
		case <-ctx.Done():
			atomic.AddInt32(&timedOut, 1)
			fmt.Println("Long task timed out")
			return ctx.Err()
		}
	})

	// Schedule task with 500ms timeout (will timeout)
	err = scheduler.ScheduleWithTimeout("timeout-task", longTask, time.Now().Add(100*time.Millisecond), 500*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to schedule timeout task: %v", err)
	}

	// Wait for execution
	time.Sleep(1 * time.Second)

	fmt.Printf("Completed: %d, Timed out: %d\n", atomic.LoadInt32(&completed), atomic.LoadInt32(&timedOut))

	// Output:
	// Long task timed out
	// Completed: 0, Timed out: 1
}

// Example_contextCancellation demonstrates task cancellation via context.
func Example_contextCancellation() {
	scheduler := NewContextAwareScheduler()
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	var completed int32
	var cancelled int32

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		select {
		case <-time.After(1 * time.Second):
			atomic.AddInt32(&completed, 1)
			fmt.Println("Task completed normally")
			return nil
		case <-ctx.Done():
			atomic.AddInt32(&cancelled, 1)
			fmt.Println("Task was cancelled")
			return ctx.Err()
		}
	})

	// Schedule cancellable task
	cancel, err := scheduler.ScheduleWithCancellation("cancellable-task", task, time.Now().Add(100*time.Millisecond))
	if err != nil {
		log.Fatalf("Failed to schedule cancellable task: %v", err)
	}

	// Cancel after 300ms
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
		fmt.Println("Task cancelled via context")
	}()

	// Wait for execution
	time.Sleep(1500 * time.Millisecond)

	fmt.Printf("Completed: %d, Cancelled: %d\n", atomic.LoadInt32(&completed), atomic.LoadInt32(&cancelled))

	// Output:
	// Task cancelled via context
	// Task was cancelled
	// Completed: 0, Cancelled: 1
}

// Example_contextPropagation demonstrates context value propagation.
func Example_contextPropagation() {
	// Create scheduler with context propagation for specific keys
	scheduler := WithContextPropagation(Config{}, "user_id", "request_id")
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		userID := ctx.Value("user_id")
		requestID := ctx.Value("request_id")
		
		fmt.Printf("Task executing with user_id: %v, request_id: %v\n", userID, requestID)
		return nil
	})

	// Create context with values
	ctx := context.Background()
	ctx = context.WithValue(ctx, "user_id", "user123")
	ctx = context.WithValue(ctx, "request_id", "req456")
	ctx = context.WithValue(ctx, "ignored_key", "ignored_value")

	err = scheduler.ScheduleWithContext(ctx, "propagation-task", task, time.Now().Add(100*time.Millisecond))
	if err != nil {
		log.Fatalf("Failed to schedule task: %v", err)
	}

	// Wait for execution
	time.Sleep(500 * time.Millisecond)

	// Check propagation stats
	stats := scheduler.GetContextStats()
	fmt.Printf("Context propagations: %d\n", stats.ContextPropagations)

	// Output:
	// Task executing with user_id: user123, request_id: req456
	// Context propagations: 2
}

// Example_contextDeadline demonstrates deadline-based scheduling.
func Example_contextDeadline() {
	scheduler := NewContextAwareScheduler()
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	var executed int32

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		
		if deadline, ok := ctx.Deadline(); ok {
			fmt.Printf("Task running with deadline: %v (time remaining: %v)\n", 
				deadline.Format("15:04:05.000"), time.Until(deadline))
		}
		
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Task completed within deadline")
		return nil
	})

	// Schedule task with deadline
	deadline := time.Now().Add(500 * time.Millisecond)
	err = scheduler.ScheduleWithDeadline("deadline-task", task, time.Now().Add(50*time.Millisecond), deadline)
	if err != nil {
		log.Fatalf("Failed to schedule deadline task: %v", err)
	}

	// Wait for execution
	time.Sleep(800 * time.Millisecond)

	fmt.Printf("Tasks executed: %d\n", atomic.LoadInt32(&executed))

	// Output:
	// Task running with deadline: [time] (time remaining: [duration])
	// Task completed within deadline
	// Tasks executed: 1
}

// Example_contextGlobalTimeout demonstrates global timeout settings.
func Example_contextGlobalTimeout() {
	scheduler := WithGlobalTimeout(Config{}, 200*time.Millisecond)
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	var completed int32
	var timedOut int32

	fastTask := workerpool.TaskFunc(func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&completed, 1)
		fmt.Println("Fast task completed")
		return nil
	})

	slowTask := workerpool.TaskFunc(func(ctx context.Context) error {
		select {
		case <-time.After(500 * time.Millisecond):
			atomic.AddInt32(&completed, 1)
			fmt.Println("Slow task completed")
			return nil
		case <-ctx.Done():
			atomic.AddInt32(&timedOut, 1)
			fmt.Println("Slow task timed out due to global timeout")
			return ctx.Err()
		}
	})

	// Both tasks will use global timeout
	scheduler.Schedule("fast-task", fastTask, time.Now().Add(50*time.Millisecond))
	scheduler.Schedule("slow-task", slowTask, time.Now().Add(100*time.Millisecond))

	// Wait for execution
	time.Sleep(1 * time.Second)

	fmt.Printf("Completed: %d, Timed out: %d\n", atomic.LoadInt32(&completed), atomic.LoadInt32(&timedOut))

	// Output:
	// Fast task completed
	// Slow task timed out due to global timeout
	// Completed: 1, Timed out: 1
}

// Example_contextRepeatingTasks demonstrates context with repeating tasks.
func Example_contextRepeatingTasks() {
	scheduler := NewContextAwareScheduler()
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	var executions int32

	task := workerpool.TaskFunc(func(ctx context.Context) error {
		count := atomic.AddInt32(&executions, 1)
		
		select {
		case <-ctx.Done():
			fmt.Printf("Execution %d cancelled\n", count)
			return ctx.Err()
		default:
			fmt.Printf("Execution %d completed\n", count)
			return nil
		}
	})

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Schedule repeating task with context
	err = scheduler.ScheduleRepeatingWithContext(ctx, "repeating-task", task, 200*time.Millisecond, 0)
	if err != nil {
		log.Fatalf("Failed to schedule repeating task: %v", err)
	}

	// Let it run a few times, then cancel
	time.Sleep(600 * time.Millisecond)
	cancel()
	fmt.Println("Context cancelled")
	
	// Wait a bit more to see cancellation effect
	time.Sleep(400 * time.Millisecond)

	fmt.Printf("Total executions: %d\n", atomic.LoadInt32(&executions))

	// Output:
	// Execution 1 completed
	// Execution 2 completed
	// Execution 3 completed
	// Context cancelled
	// Execution 4 cancelled
	// Total executions: 4
}

// Example_contextStatistics demonstrates context usage statistics.
func Example_contextStatistics() {
	scheduler := NewContextAwareScheduler()
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Create several tasks with different behaviors
	fastTask := workerpool.TaskFunc(func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	slowTask := workerpool.TaskFunc(func(ctx context.Context) error {
		time.Sleep(300 * time.Millisecond)
		return nil
	})

	timeoutTask := workerpool.TaskFunc(func(ctx context.Context) error {
		select {
		case <-time.After(500 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Schedule different types of tasks
	scheduler.Schedule("fast-1", fastTask, time.Now().Add(50*time.Millisecond))
	scheduler.Schedule("fast-2", fastTask, time.Now().Add(100*time.Millisecond))
	scheduler.ScheduleWithTimeout("slow-timeout", slowTask, time.Now().Add(150*time.Millisecond), 100*time.Millisecond)
	
	cancel, _ := scheduler.ScheduleWithCancellation("cancellable", timeoutTask, time.Now().Add(200*time.Millisecond))
	
	// Cancel one task
	time.Sleep(250 * time.Millisecond)
	cancel()

	// Wait for all tasks to complete or timeout
	time.Sleep(800 * time.Millisecond)

	// Get statistics
	stats := scheduler.GetContextStats()
	fmt.Printf("Context Statistics:\n")
	fmt.Printf("  Active contexts: %d\n", stats.ActiveContexts)
	fmt.Printf("  Cancelled tasks: %d\n", stats.CancelledTasks)
	fmt.Printf("  Timeout tasks: %d\n", stats.TimeoutTasks)
	fmt.Printf("  Average task duration: %v\n", stats.AverageTaskDuration.Round(time.Millisecond))

	// Output:
	// Context Statistics:
	//   Active contexts: 0
	//   Cancelled tasks: 1
	//   Timeout tasks: 1
	//   Average task duration: [duration]
}

// Example_contextComplexScenario demonstrates a complex real-world scenario.
func Example_contextComplexScenario() {
	// Setup scheduler with propagation and global timeout
	scheduler := WithContextPropagation(Config{}, "session_id", "trace_id")
	scheduler.SetGlobalTimeout(2 * time.Second)
	
	defer func() { <-scheduler.Stop() }()

	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Simulate user request processing
	processRequest := func(sessionID, traceID string) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "session_id", sessionID)
		ctx = context.WithValue(ctx, "trace_id", traceID)

		// Data processing task
		dataTask := workerpool.TaskFunc(func(ctx context.Context) error {
			fmt.Printf("Processing data for session %v, trace %v\n", 
				ctx.Value("session_id"), ctx.Value("trace_id"))
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		// Notification task with specific timeout
		notifyTask := workerpool.TaskFunc(func(ctx context.Context) error {
			fmt.Printf("Sending notification for session %v\n", ctx.Value("session_id"))
			time.Sleep(50 * time.Millisecond)
			return nil
		})

		// Background cleanup task
		cleanupTask := workerpool.TaskFunc(func(ctx context.Context) error {
			fmt.Printf("Cleanup for session %v\n", ctx.Value("session_id"))
			time.Sleep(200 * time.Millisecond)
			return nil
		})

		// Schedule tasks with different context configurations
		scheduler.ScheduleWithContext(ctx, fmt.Sprintf("data-%s", sessionID), dataTask, time.Now().Add(50*time.Millisecond))
		
		scheduler.ScheduleWithTimeout(fmt.Sprintf("notify-%s", sessionID), notifyTask, time.Now().Add(200*time.Millisecond), 100*time.Millisecond)
		
		cancel, _ := scheduler.ScheduleWithCancellation(fmt.Sprintf("cleanup-%s", sessionID), cleanupTask, time.Now().Add(400*time.Millisecond))
		
		// Cancel cleanup for session1 after 300ms
		if sessionID == "session1" {
			go func() {
				time.Sleep(300 * time.Millisecond)
				cancel()
				fmt.Printf("Cancelled cleanup for %s\n", sessionID)
			}()
		}
	}

	// Process multiple requests
	processRequest("session1", "trace123")
	processRequest("session2", "trace456")

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Show final statistics
	stats := scheduler.GetContextStats()
	fmt.Printf("Final stats - Cancelled: %d, Propagations: %d\n", 
		stats.CancelledTasks, stats.ContextPropagations)

	// Output:
	// Processing data for session session1, trace trace123
	// Processing data for session session2, trace trace456
	// Sending notification for session session1
	// Sending notification for session session2
	// Cancelled cleanup for session1
	// Cleanup for session session2
	// Final stats - Cancelled: 1, Propagations: 6
}