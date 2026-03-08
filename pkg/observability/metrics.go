// Package observability provides Prometheus metrics and OpenTelemetry tracing
// instrumentation for KubePilot's AI actions, job executions, and CR code events.
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all KubePilot Prometheus metrics.
// Use the singleton Register() function to initialize and register metrics.
type Metrics struct {
	// AICommandsTotal counts natural language commands processed by the AI engine.
	AICommandsTotal *prometheus.CounterVec
	// AICommandDuration tracks AI engine latency in seconds.
	AICommandDuration *prometheus.HistogramVec
	// JobsTotal counts jobs by status (pending/running/done/failed/blocked).
	JobsTotal *prometheus.CounterVec
	// CRCodeAuthTotal counts CR code authorization attempts (success/failure).
	CRCodeAuthTotal *prometheus.CounterVec
	// CrashingPodsGauge tracks the current number of pods in a crash state.
	CrashingPodsGauge *prometheus.GaugeVec
	// NodePressureGauge tracks nodes with memory/disk/PID pressure.
	NodePressureGauge *prometheus.GaugeVec
}

var defaultMetrics *Metrics

// Register creates and registers all KubePilot metrics with the default Prometheus registry.
// Call once at startup before starting any components.
func Register() *Metrics {
	if defaultMetrics != nil {
		return defaultMetrics
	}

	m := &Metrics{
		AICommandsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kubepilot",
			Subsystem: "ai",
			Name:      "commands_total",
			Help:      "Total natural language commands processed by the AI engine.",
		}, []string{"status"}), // status: success | error

		AICommandDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "kubepilot",
			Subsystem: "ai",
			Name:      "command_duration_seconds",
			Help:      "Time taken for the AI engine to interpret a command.",
			Buckets:   prometheus.DefBuckets,
		}, []string{}),

		JobsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kubepilot",
			Subsystem: "jobs",
			Name:      "total",
			Help:      "Total KubePilot jobs by final status.",
		}, []string{"status", "environment"}),

		CRCodeAuthTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kubepilot",
			Subsystem: "security",
			Name:      "crcode_auth_total",
			Help:      "CR code authorization attempts.",
		}, []string{"result"}), // result: authorized | invalid | expired | not_found

		CrashingPodsGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "kubepilot",
			Subsystem: "cluster",
			Name:      "crashing_pods",
			Help:      "Current number of pods in CrashLoopBackOff, OOMKilled, or Error state.",
		}, []string{"namespace"}),

		NodePressureGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "kubepilot",
			Subsystem: "cluster",
			Name:      "node_pressure",
			Help:      "Number of nodes under resource pressure (memory, disk, or PID).",
		}, []string{"pressure_type"}),
	}

	defaultMetrics = m
	return m
}

// Get returns the initialized metrics singleton.
// Panics if Register() has not been called — ensures metrics are always available.
func Get() *Metrics {
	if defaultMetrics == nil {
		panic("observability.Register() must be called before observability.Get()")
	}
	return defaultMetrics
}
