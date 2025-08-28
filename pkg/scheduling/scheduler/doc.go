// Package scheduler provides task scheduling with support for delayed execution,
// repeating tasks, and cron expressions.
//
// Basic usage:
//
//	s := scheduler.New()
//	s.Start()
//	defer func() { <-s.Stop() }()
//
//	task := workerpool.TaskFunc(func(ctx context.Context) error {
//		fmt.Println("Hello, world!")
//		return nil
//	})
//
//	// Run once after 5 seconds
//	s.ScheduleAfter("hello", task, 5*time.Second)
//
//	// Run every minute
//	s.ScheduleRepeating("heartbeat", task, time.Minute)
//
//	// Run using cron expression
//	s.ScheduleCron("daily", "0 0 * * *", task)
package scheduler
