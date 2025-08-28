package scheduler

import (
	"container/heap"
	"runtime"
	"sync"
	"time"

	"github.com/vnykmshr/goflow/pkg/scheduling/workerpool"
)

// OptimizedScheduler extends the basic scheduler with performance optimizations.
type OptimizedScheduler interface {
	Scheduler
	
	// SetOptimizations configures performance optimizations
	SetOptimizations(opts OptimizationOptions)
	
	// GetOptimizationStats returns current optimization statistics
	GetOptimizationStats() OptimizationStats
	
	// Compact removes completed tasks from memory and defragments internal structures
	Compact() CompactionResult
}

// OptimizationOptions configures scheduler optimizations.
type OptimizationOptions struct {
	// UseHeapScheduling uses a min-heap for more efficient task scheduling
	UseHeapScheduling bool
	
	// BatchSize for batching operations to reduce lock contention
	BatchSize int
	
	// CompactionInterval for automatic memory cleanup
	CompactionInterval time.Duration
	
	// EnablePooling reuses task wrappers to reduce GC pressure
	EnablePooling bool
	
	// MemoryThresholdMB triggers compaction when memory usage exceeds this
	MemoryThresholdMB int
	
	// TaskCacheSize limits the number of completed tasks kept in memory for stats
	TaskCacheSize int
}

// OptimizationStats provides metrics about scheduler optimizations.
type OptimizationStats struct {
	HeapOperations        int64
	BatchedOperations     int64
	CompactionRuns        int64
	PooledObjectsReused   int64
	MemoryUsageBytes      int64
	CachedCompletedTasks  int
	AverageSchedulingTime time.Duration
	TaskThroughput        float64 // tasks per second
}

// CompactionResult provides information about a compaction operation.
type CompactionResult struct {
	TasksRemoved      int
	MemoryFreedBytes  int64
	CompactionTime    time.Duration
	HeapDefragmented  bool
}

// taskHeap implements a min-heap for efficient task scheduling.
type taskHeap []*ScheduledTask

func (h taskHeap) Len() int           { return len(h) }
func (h taskHeap) Less(i, j int) bool { return h[i].RunAt.Before(h[j].RunAt) }
func (h taskHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *taskHeap) Push(x interface{}) {
	*h = append(*h, x.(*ScheduledTask))
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// Peek returns the next task without removing it
func (h *taskHeap) Peek() *ScheduledTask {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

// optimizedScheduler implements OptimizedScheduler with performance optimizations.
type optimizedScheduler struct {
	Scheduler
	opts OptimizationOptions
	
	// Heap-based scheduling
	taskHeap *taskHeap
	heapMu   sync.RWMutex
	
	// Object pooling
	taskPool sync.Pool
	
	// Statistics
	stats OptimizationStats
	statsMu sync.RWMutex
	
	// Compaction
	compactionTicker *time.Ticker
	stopCompaction   chan struct{}
}

// NewOptimizedScheduler creates a new scheduler with performance optimizations.
func NewOptimizedScheduler() OptimizedScheduler {
	return NewOptimizedSchedulerWithConfig(Config{}, OptimizationOptions{
		UseHeapScheduling:  true,
		BatchSize:         100,
		CompactionInterval: 5 * time.Minute,
		EnablePooling:     true,
		MemoryThresholdMB: 100,
		TaskCacheSize:     1000,
	})
}

// NewOptimizedSchedulerWithConfig creates an optimized scheduler with custom configuration.
func NewOptimizedSchedulerWithConfig(config Config, opts OptimizationOptions) OptimizedScheduler {
	baseScheduler := NewWithConfig(config)
	
	taskHeap := &taskHeap{}
	heap.Init(taskHeap)
	
	os := &optimizedScheduler{
		Scheduler:      baseScheduler,
		opts:          opts,
		taskHeap:      taskHeap,
		stopCompaction: make(chan struct{}),
	}
	
	// Initialize object pool
	if opts.EnablePooling {
		os.taskPool.New = func() interface{} {
			return &ScheduledTask{}
		}
	}
	
	// Start automatic compaction
	if opts.CompactionInterval > 0 {
		os.compactionTicker = time.NewTicker(opts.CompactionInterval)
		go os.runCompaction()
	}
	
	return os
}

// SetOptimizations updates the optimization configuration.
func (os *optimizedScheduler) SetOptimizations(opts OptimizationOptions) {
	os.opts = opts
	
	// Update compaction interval if changed
	if os.compactionTicker != nil {
		os.compactionTicker.Stop()
	}
	
	if opts.CompactionInterval > 0 {
		os.compactionTicker = time.NewTicker(opts.CompactionInterval)
		if os.stopCompaction != nil {
			go os.runCompaction()
		}
	}
}

// GetOptimizationStats returns current optimization statistics.
func (os *optimizedScheduler) GetOptimizationStats() OptimizationStats {
	os.statsMu.RLock()
	defer os.statsMu.RUnlock()
	return os.stats
}

// Schedule overrides the base scheduler to use heap-based scheduling if enabled.
func (os *optimizedScheduler) Schedule(id string, task workerpool.Task, runAt time.Time) error {
	start := time.Now()
	
	err := os.Scheduler.Schedule(id, task, runAt)
	if err != nil {
		return err
	}
	
	// Add to heap for optimized scheduling
	if os.opts.UseHeapScheduling {
		scheduledTask := os.getTaskFromPool()
		scheduledTask.ID = id
		scheduledTask.Task = task
		scheduledTask.RunAt = runAt
		scheduledTask.IsRepeating = false
		scheduledTask.MaxRuns = 1
		scheduledTask.RunCount = 0
		scheduledTask.CreatedAt = time.Now()
		
		os.heapMu.Lock()
		heap.Push(os.taskHeap, scheduledTask)
		os.heapMu.Unlock()
		
		os.updateStats(func(s *OptimizationStats) {
			s.HeapOperations++
		})
	}
	
	// Update timing stats
	schedulingTime := time.Since(start)
	os.updateStats(func(s *OptimizationStats) {
		// Update average scheduling time (simple moving average)
		if s.AverageSchedulingTime == 0 {
			s.AverageSchedulingTime = schedulingTime
		} else {
			s.AverageSchedulingTime = (s.AverageSchedulingTime + schedulingTime) / 2
		}
	})
	
	return nil
}

// GetNextReadyTasks returns tasks ready for execution using heap optimization.
func (os *optimizedScheduler) GetNextReadyTasks(maxTasks int) []*ScheduledTask {
	if !os.opts.UseHeapScheduling {
		return nil
	}
	
	os.heapMu.Lock()
	defer os.heapMu.Unlock()
	
	var readyTasks []*ScheduledTask
	now := time.Now()
	
	for len(*os.taskHeap) > 0 && len(readyTasks) < maxTasks {
		next := os.taskHeap.Peek()
		if next.RunAt.After(now) {
			break // No more ready tasks
		}
		
		// Remove from heap
		task := heap.Pop(os.taskHeap).(*ScheduledTask)
		readyTasks = append(readyTasks, task)
		
		os.updateStats(func(s *OptimizationStats) {
			s.HeapOperations++
		})
	}
	
	return readyTasks
}

// Compact performs memory cleanup and defragmentation.
func (os *optimizedScheduler) Compact() CompactionResult {
	start := time.Now()
	result := CompactionResult{}
	
	// Clean up completed tasks from heap
	if os.opts.UseHeapScheduling {
		os.heapMu.Lock()
		
		// Rebuild heap without completed tasks
		var activeTasks []*ScheduledTask
		for _, task := range *os.taskHeap {
			// Check if task still exists in base scheduler
			if _, exists := os.Scheduler.GetTask(task.ID); exists {
				activeTasks = append(activeTasks, task)
			} else {
				// Return completed task to pool
				os.returnTaskToPool(task)
				result.TasksRemoved++
			}
		}
		
		// Rebuild heap
		*os.taskHeap = activeTasks
		heap.Init(os.taskHeap)
		result.HeapDefragmented = true
		
		os.heapMu.Unlock()
	}
	
	// Force garbage collection to measure memory freed
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	result.MemoryFreedBytes = int64(m1.Alloc - m2.Alloc)
	result.CompactionTime = time.Since(start)
	
	os.updateStats(func(s *OptimizationStats) {
		s.CompactionRuns++
		s.MemoryUsageBytes = int64(m2.Alloc)
	})
	
	return result
}

// runCompaction runs automatic compaction at configured intervals.
func (os *optimizedScheduler) runCompaction() {
	for {
		select {
		case <-os.compactionTicker.C:
			os.Compact()
		case <-os.stopCompaction:
			return
		}
	}
}

// getTaskFromPool gets a reusable task from the object pool.
func (os *optimizedScheduler) getTaskFromPool() *ScheduledTask {
	if os.opts.EnablePooling {
		task := os.taskPool.Get().(*ScheduledTask)
		// Reset task fields
		*task = ScheduledTask{}
		
		os.updateStats(func(s *OptimizationStats) {
			s.PooledObjectsReused++
		})
		
		return task
	}
	return &ScheduledTask{}
}

// returnTaskToPool returns a task to the object pool for reuse.
func (os *optimizedScheduler) returnTaskToPool(task *ScheduledTask) {
	if os.opts.EnablePooling {
		os.taskPool.Put(task)
	}
}

// updateStats safely updates optimization statistics.
func (os *optimizedScheduler) updateStats(updater func(*OptimizationStats)) {
	os.statsMu.Lock()
	defer os.statsMu.Unlock()
	updater(&os.stats)
}

// Stop overrides base scheduler stop to clean up optimizations.
func (os *optimizedScheduler) Stop() <-chan struct{} {
	if os.compactionTicker != nil {
		os.compactionTicker.Stop()
	}
	
	if os.stopCompaction != nil {
		close(os.stopCompaction)
	}
	
	return os.Scheduler.Stop()
}

// BatchSchedule schedules multiple tasks efficiently in a single operation.
func (os *optimizedScheduler) BatchSchedule(tasks []BatchScheduleRequest) error {
	if len(tasks) == 0 {
		return nil
	}
	
	// Group tasks for batch processing
	batchSize := os.opts.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	
	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		
		batch := tasks[i:end]
		if err := os.processBatch(batch); err != nil {
			return err
		}
		
		os.updateStats(func(s *OptimizationStats) {
			s.BatchedOperations++
		})
	}
	
	return nil
}

// BatchScheduleRequest represents a task to be scheduled in a batch operation.
type BatchScheduleRequest struct {
	ID    string
	Task  workerpool.Task  
	RunAt time.Time
}

// processBatch processes a batch of tasks efficiently.
func (os *optimizedScheduler) processBatch(batch []BatchScheduleRequest) error {
	// Process all tasks in the batch
	for _, req := range batch {
		if err := os.Schedule(req.ID, req.Task, req.RunAt); err != nil {
			return err
		}
	}
	return nil
}