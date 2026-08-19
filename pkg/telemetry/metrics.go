package telemetry

import (
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds every instrument KubePilot records to.
//
// They are declared once at startup rather than looked up per call site: the
// OTel API allows repeated creation, but a single declaration keeps units and
// bucket boundaries in one place and makes the full metric surface readable at
// a glance. When telemetry is disabled these are backed by no-op instruments,
// so recording is safe and costs almost nothing.
type Metrics struct {
	// ── HTTP server (the dashboard + REST API) ──────────────────────────────
	HTTPDuration       metric.Float64Histogram
	HTTPActiveRequests metric.Int64UpDownCounter
	HTTPRequestSize    metric.Int64Histogram
	HTTPResponseSize   metric.Int64Histogram

	// ── Kubernetes API client ───────────────────────────────────────────────
	// KubePilot is mostly a client of the API server, so its latency and error
	// rate explain most of what the dashboard feels like to use.
	K8sDuration metric.Float64Histogram
	K8sErrors   metric.Int64Counter

	// ── AI backend (Ollama / OpenAI-compatible) ─────────────────────────────
	AIDuration metric.Float64Histogram
	AITokens   metric.Int64Counter
	AIErrors   metric.Int64Counter

	// ── Diagnosis and self-healing ──────────────────────────────────────────
	AnomaliesDetected metric.Int64Counter
	RCAReports        metric.Int64Counter
	RCADuration       metric.Float64Histogram
	AutopilotActions  metric.Int64Counter
}

// Latency buckets in seconds.
//
// Two scales are needed because the workloads differ by orders of magnitude: an
// API-server list returns in milliseconds, while a local LLM generating an RCA
// routinely runs for a minute or more. Sharing one set would leave every AI
// call piled into the overflow bucket.
var (
	fastBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}
	slowBuckets = []float64{0.5, 1, 2.5, 5, 10, 20, 30, 60, 120, 300, 600}
	sizeBuckets = []float64{64, 256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304}
)

// newMetrics declares the instruments against the given meter. Any failure is
// returned to the caller rather than swallowed: a malformed instrument is a
// programming error here, not an operational condition.
func newMetrics(m metric.Meter) (*Metrics, error) {
	var (
		out Metrics
		err error
	)

	if out.HTTPDuration, err = m.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of inbound HTTP requests to the dashboard and API."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(fastBuckets...),
	); err != nil {
		return nil, err
	}

	if out.HTTPActiveRequests, err = m.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of HTTP requests currently being served."),
		metric.WithUnit("{request}"),
	); err != nil {
		return nil, err
	}

	if out.HTTPRequestSize, err = m.Int64Histogram(
		"http.server.request.body.size",
		metric.WithDescription("Size of inbound HTTP request bodies."),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(sizeBuckets...),
	); err != nil {
		return nil, err
	}

	if out.HTTPResponseSize, err = m.Int64Histogram(
		"http.server.response.body.size",
		metric.WithDescription("Size of outbound HTTP response bodies."),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(sizeBuckets...),
	); err != nil {
		return nil, err
	}

	if out.K8sDuration, err = m.Float64Histogram(
		"kubepilot.k8s.client.duration",
		metric.WithDescription("Duration of requests KubePilot makes to the Kubernetes API server."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(fastBuckets...),
	); err != nil {
		return nil, err
	}

	if out.K8sErrors, err = m.Int64Counter(
		"kubepilot.k8s.client.errors",
		metric.WithDescription("Failed Kubernetes API requests, by HTTP status class."),
		metric.WithUnit("{error}"),
	); err != nil {
		return nil, err
	}

	if out.AIDuration, err = m.Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithDescription("Duration of calls to the AI backend, by operation and model."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(slowBuckets...),
	); err != nil {
		return nil, err
	}

	if out.AITokens, err = m.Int64Counter(
		"gen_ai.client.token.usage",
		metric.WithDescription("Tokens consumed by the AI backend, split by input and output."),
		metric.WithUnit("{token}"),
	); err != nil {
		return nil, err
	}

	if out.AIErrors, err = m.Int64Counter(
		"kubepilot.ai.errors",
		metric.WithDescription("Failed AI backend calls, by operation."),
		metric.WithUnit("{error}"),
	); err != nil {
		return nil, err
	}

	if out.AnomaliesDetected, err = m.Int64Counter(
		"kubepilot.anomalies.detected",
		metric.WithDescription("Cluster anomalies detected by the watcher, by severity and type."),
		metric.WithUnit("{anomaly}"),
	); err != nil {
		return nil, err
	}

	if out.RCAReports, err = m.Int64Counter(
		"kubepilot.rca.reports",
		metric.WithDescription("Root cause analysis reports produced, by outcome."),
		metric.WithUnit("{report}"),
	); err != nil {
		return nil, err
	}

	if out.RCADuration, err = m.Float64Histogram(
		"kubepilot.rca.duration",
		metric.WithDescription("Time taken to produce a root cause analysis report."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(slowBuckets...),
	); err != nil {
		return nil, err
	}

	if out.AutopilotActions, err = m.Int64Counter(
		"kubepilot.autopilot.actions",
		metric.WithDescription("Autopilot remediation decisions, by action, mode, and outcome."),
		metric.WithUnit("{action}"),
	); err != nil {
		return nil, err
	}

	return &out, nil
}
