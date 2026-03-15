package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    activeWorkflows = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "sdlc_active_workflows",
        Help: "Number of active workflow executions",
    })
    completedStages = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "sdlc_completed_stages_total",
        Help: "Total number of completed stages",
    })
    failedStages = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "sdlc_failed_stages_total",
        Help: "Total number of failed stages",
    })
    executionTime = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "sdlc_execution_time_seconds",
        Help:    "Execution time of workflows",
        Buckets: prometheus.DefBuckets,
    })
    retryAttempts = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "sdlc_retry_attempts_total",
        Help: "Total number of retry attempts",
    })
)

func init() {
    prometheus.MustRegister(activeWorkflows, completedStages, failedStages, executionTime, retryAttempts)
}

// Exported functions for metric updates
func IncActiveWorkflows() { activeWorkflows.Inc() }
func DecActiveWorkflows() { activeWorkflows.Dec() }
func IncCompletedStages() { completedStages.Inc() }
func IncFailedStages() { failedStages.Inc() }
func ObserveExecutionTime(seconds float64) { executionTime.Observe(seconds) }
func IncRetryAttempts() { retryAttempts.Inc() }
