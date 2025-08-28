// Package metrics provides Prometheus instrumentation for goflow components.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Registry holds all metric instances for goflow components.
type Registry struct {
	// Rate Limiting Metrics
	RateLimitRequests  *prometheus.CounterVec
	RateLimitAllowed   *prometheus.CounterVec
	RateLimitDenied    *prometheus.CounterVec
	RateLimitWaitTime  *prometheus.HistogramVec
	RateLimitTokens    *prometheus.GaugeVec
	ConcurrencyActive  *prometheus.GaugeVec
	ConcurrencyWaiting *prometheus.GaugeVec

	// Task Scheduling Metrics
	TasksScheduled        *prometheus.CounterVec
	TasksExecuted         *prometheus.CounterVec
	TasksCompleted        *prometheus.CounterVec
	TasksFailed           *prometheus.CounterVec
	TaskExecutionDuration *prometheus.HistogramVec
	WorkerPoolSize        *prometheus.GaugeVec
	WorkerPoolActive      *prometheus.GaugeVec
	WorkerPoolQueued      *prometheus.GaugeVec

	// Streaming Metrics
	StreamOperations   *prometheus.CounterVec
	StreamItems        *prometheus.CounterVec
	StreamErrors       *prometheus.CounterVec
	StreamBufferSize   *prometheus.GaugeVec
	StreamBufferUsage  *prometheus.GaugeVec
	BackpressureEvents *prometheus.CounterVec
	WriterFlushes      *prometheus.CounterVec
	WriterBytesWritten *prometheus.CounterVec
}

// DefaultRegistry is the default metrics registry used by goflow components.
var DefaultRegistry *Registry

func init() {
	DefaultRegistry = NewRegistry(prometheus.DefaultRegisterer)
}

// NewRegistry creates a new metrics registry with the given Prometheus registerer.
func NewRegistry(reg prometheus.Registerer) *Registry {
	factory := promauto.With(reg)

	return &Registry{
		// Rate Limiting Metrics
		RateLimitRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "ratelimit",
				Name:      "requests_total",
				Help:      "Total number of rate limit requests",
			},
			[]string{"limiter_type", "limiter_name"},
		),

		RateLimitAllowed: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "ratelimit",
				Name:      "allowed_total",
				Help:      "Total number of allowed requests",
			},
			[]string{"limiter_type", "limiter_name"},
		),

		RateLimitDenied: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "ratelimit",
				Name:      "denied_total",
				Help:      "Total number of denied requests",
			},
			[]string{"limiter_type", "limiter_name"},
		),

		RateLimitWaitTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "goflow",
				Subsystem: "ratelimit",
				Name:      "wait_duration_seconds",
				Help:      "Time spent waiting for rate limit approval",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"limiter_type", "limiter_name"},
		),

		RateLimitTokens: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "ratelimit",
				Name:      "tokens_available",
				Help:      "Number of tokens currently available",
			},
			[]string{"limiter_type", "limiter_name"},
		),

		ConcurrencyActive: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "concurrency",
				Name:      "active",
				Help:      "Number of active concurrent operations",
			},
			[]string{"limiter_name"},
		),

		ConcurrencyWaiting: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "concurrency",
				Name:      "waiting",
				Help:      "Number of operations waiting for concurrency slot",
			},
			[]string{"limiter_name"},
		),

		// Task Scheduling Metrics
		TasksScheduled: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "scheduler",
				Name:      "tasks_scheduled_total",
				Help:      "Total number of tasks scheduled",
			},
			[]string{"scheduler_name"},
		),

		TasksExecuted: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "scheduler",
				Name:      "tasks_executed_total",
				Help:      "Total number of tasks executed",
			},
			[]string{"scheduler_name"},
		),

		TasksCompleted: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "scheduler",
				Name:      "tasks_completed_total",
				Help:      "Total number of tasks completed successfully",
			},
			[]string{"scheduler_name"},
		),

		TasksFailed: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "scheduler",
				Name:      "tasks_failed_total",
				Help:      "Total number of tasks that failed",
			},
			[]string{"scheduler_name"},
		),

		TaskExecutionDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "goflow",
				Subsystem: "scheduler",
				Name:      "task_duration_seconds",
				Help:      "Time spent executing tasks",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"scheduler_name"},
		),

		WorkerPoolSize: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "workerpool",
				Name:      "size",
				Help:      "Current worker pool size",
			},
			[]string{"pool_name"},
		),

		WorkerPoolActive: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "workerpool",
				Name:      "active_workers",
				Help:      "Number of active workers",
			},
			[]string{"pool_name"},
		),

		WorkerPoolQueued: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "workerpool",
				Name:      "queued_tasks",
				Help:      "Number of queued tasks",
			},
			[]string{"pool_name"},
		),

		// Streaming Metrics
		StreamOperations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "stream",
				Name:      "operations_total",
				Help:      "Total number of stream operations",
			},
			[]string{"operation", "stream_name"},
		),

		StreamItems: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "stream",
				Name:      "items_processed_total",
				Help:      "Total number of items processed by streams",
			},
			[]string{"operation", "stream_name"},
		),

		StreamErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "stream",
				Name:      "errors_total",
				Help:      "Total number of stream processing errors",
			},
			[]string{"operation", "stream_name"},
		),

		StreamBufferSize: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "stream",
				Name:      "buffer_size",
				Help:      "Stream buffer capacity",
			},
			[]string{"stream_name"},
		),

		StreamBufferUsage: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "goflow",
				Subsystem: "stream",
				Name:      "buffer_usage",
				Help:      "Current stream buffer usage",
			},
			[]string{"stream_name"},
		),

		BackpressureEvents: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "backpressure",
				Name:      "events_total",
				Help:      "Total number of backpressure events",
			},
			[]string{"strategy", "channel_name"},
		),

		WriterFlushes: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "writer",
				Name:      "flushes_total",
				Help:      "Total number of writer flushes",
			},
			[]string{"writer_name"},
		),

		WriterBytesWritten: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "goflow",
				Subsystem: "writer",
				Name:      "bytes_written_total",
				Help:      "Total bytes written",
			},
			[]string{"writer_name"},
		),
	}
}
