package otelautopilot

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// MetricSample is a short-retention metric point for the managed store.
type MetricSample struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Plane     Plane             `json:"plane"`
	Service   string            `json:"service,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// LogRecord is a short-retention log line for the managed store.
type LogRecord struct {
	Timestamp time.Time         `json:"timestamp"`
	Severity  string            `json:"severity,omitempty"`
	Body      string            `json:"body"`
	Labels    map[string]string `json:"labels,omitempty"`
	Plane     Plane             `json:"plane"`
	Service   string            `json:"service,omitempty"`
}

// SpanRecord is a short-retention span for the managed store.
type SpanRecord struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Name      string            `json:"name"`
	Service   string            `json:"service,omitempty"`
	Status    string            `json:"status,omitempty"`
	Duration  time.Duration     `json:"duration"`
	Labels    map[string]string `json:"labels,omitempty"`
	Plane     Plane             `json:"plane"`
	Timestamp time.Time         `json:"timestamp"`
}

// Store is the KubePilot-managed short-retention signal store.
type Store struct {
	mu        sync.RWMutex
	retention time.Duration
	metrics   []MetricSample
	logs      []LogRecord
	spans     []SpanRecord
	lastIngest *time.Time
}

// NewStore creates a managed store with the given retention window.
func NewStore(retention time.Duration) *Store {
	if retention <= 0 {
		retention = 24 * time.Hour
	}
	return &Store{retention: retention}
}

// IngestMetrics appends metric samples and prunes expired ones.
func (s *Store) IngestMetrics(samples ...MetricSample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.lastIngest = &now
	for _, sample := range samples {
		if sample.Timestamp.IsZero() {
			sample.Timestamp = now
		}
		s.metrics = append(s.metrics, sample)
	}
	s.pruneLocked(now)
}

// IngestLogs appends log records and prunes expired ones.
func (s *Store) IngestLogs(records ...LogRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.lastIngest = &now
	for _, record := range records {
		if record.Timestamp.IsZero() {
			record.Timestamp = now
		}
		s.logs = append(s.logs, record)
	}
	s.pruneLocked(now)
}

// IngestSpans appends span records and prunes expired ones.
func (s *Store) IngestSpans(spans ...SpanRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.lastIngest = &now
	for _, span := range spans {
		if span.Timestamp.IsZero() {
			span.Timestamp = now
		}
		s.spans = append(s.spans, span)
	}
	s.pruneLocked(now)
}

func (s *Store) pruneLocked(now time.Time) {
	cutoff := now.Add(-s.retention)
	metrics := s.metrics[:0]
	for _, m := range s.metrics {
		if m.Timestamp.After(cutoff) {
			metrics = append(metrics, m)
		}
	}
	s.metrics = metrics

	logs := s.logs[:0]
	for _, l := range s.logs {
		if l.Timestamp.After(cutoff) {
			logs = append(logs, l)
		}
	}
	s.logs = logs

	spans := s.spans[:0]
	for _, sp := range s.spans {
		if sp.Timestamp.After(cutoff) {
			spans = append(spans, sp)
		}
	}
	s.spans = spans
}

// Status returns counts and last ingest time.
func (s *Store) Status() (metrics, logs, traces int, last *time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var lastCopy *time.Time
	if s.lastIngest != nil {
		t := *s.lastIngest
		lastCopy = &t
	}
	return len(s.metrics), len(s.logs), len(s.spans), lastCopy
}

// QueryMetrics returns recent metrics, optionally filtered by plane/service.
func (s *Store) QueryMetrics(plane Plane, service string, limit int) []MetricSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MetricSample, 0, limit)
	for i := len(s.metrics) - 1; i >= 0; i-- {
		m := s.metrics[i]
		if plane != "" && m.Plane != plane {
			continue
		}
		if service != "" && !strings.EqualFold(m.Service, service) {
			continue
		}
		out = append(out, m)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	return out
}

// QueryLogs returns recent logs, optionally filtered by plane/service.
func (s *Store) QueryLogs(plane Plane, service string, limit int) []LogRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LogRecord, 0, limit)
	for i := len(s.logs) - 1; i >= 0; i-- {
		l := s.logs[i]
		if plane != "" && l.Plane != plane {
			continue
		}
		if service != "" && !strings.EqualFold(l.Service, service) {
			continue
		}
		out = append(out, l)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// QueryTraces returns recent spans, optionally filtered by plane/service.
func (s *Store) QueryTraces(plane Plane, service string, limit int) []SpanRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SpanRecord, 0, limit)
	for i := len(s.spans) - 1; i >= 0; i-- {
		sp := s.spans[i]
		if plane != "" && sp.Plane != plane {
			continue
		}
		if service != "" && !strings.EqualFold(sp.Service, service) {
			continue
		}
		out = append(out, sp)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// EvidenceForResource returns compact OTEL snippets for RCA evidence chains.
func (s *Store) EvidenceForResource(namespace, name string, limit int) (metrics []MetricSample, logs []LogRecord, traces []SpanRecord) {
	if limit <= 0 {
		limit = 5
	}
	serviceHints := []string{name, namespace + "/" + name}
	for _, hint := range serviceHints {
		metrics = append(metrics, s.QueryMetrics("", hint, limit)...)
		logs = append(logs, s.QueryLogs("", hint, limit)...)
		traces = append(traces, s.QueryTraces("", hint, limit)...)
	}
	// Also match by label namespace/pod.
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.metrics) - 1; i >= 0 && len(metrics) < limit*2; i-- {
		m := s.metrics[i]
		if labelMatch(m.Labels, namespace, name) {
			metrics = append(metrics, m)
		}
	}
	for i := len(s.logs) - 1; i >= 0 && len(logs) < limit*2; i-- {
		l := s.logs[i]
		if labelMatch(l.Labels, namespace, name) {
			logs = append(logs, l)
		}
	}
	for i := len(s.spans) - 1; i >= 0 && len(traces) < limit*2; i-- {
		sp := s.spans[i]
		if labelMatch(sp.Labels, namespace, name) {
			traces = append(traces, sp)
		}
	}
	return trimMetrics(metrics, limit), trimLogs(logs, limit), trimSpans(traces, limit)
}

func labelMatch(labels map[string]string, namespace, name string) bool {
	if labels == nil {
		return false
	}
	ns := labels["k8s.namespace.name"]
	if ns == "" {
		ns = labels["namespace"]
	}
	pod := labels["k8s.pod.name"]
	if pod == "" {
		pod = labels["pod"]
	}
	svc := labels["service.name"]
	if svc == "" {
		svc = labels["service"]
	}
	if ns != "" && ns != namespace {
		return false
	}
	return strings.Contains(pod, name) || strings.EqualFold(svc, name) || strings.HasPrefix(pod, name)
}

func trimMetrics(in []MetricSample, n int) []MetricSample {
	if len(in) <= n {
		return in
	}
	return in[:n]
}
func trimLogs(in []LogRecord, n int) []LogRecord {
	if len(in) <= n {
		return in
	}
	return in[:n]
}
func trimSpans(in []SpanRecord, n int) []SpanRecord {
	if len(in) <= n {
		return in
	}
	return in[:n]
}
