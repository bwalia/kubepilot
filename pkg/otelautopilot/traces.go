package otelautopilot

import (
	"sort"
	"strings"
	"time"
)

// TraceSummary is one row in the trace explorer.
type TraceSummary struct {
	TraceID     string    `json:"trace_id"`
	RootService string    `json:"root_service"`
	RootName    string    `json:"root_name"`
	Start       time.Time `json:"start"`
	DurationMs  float64   `json:"duration_ms"`
	SpanCount   int       `json:"span_count"`
	ErrorCount  int       `json:"error_count"`
	Services    []string  `json:"services"`
	Error       string    `json:"error,omitempty"`
}

// TraceQuery filters the trace explorer.
type TraceQuery struct {
	Since      time.Time
	Service    string // node ID or bare service name
	Edge       string // "source→target" node IDs
	ErrorsOnly bool
	MinMs      float64
	Limit      int
}

// QueryTraceSummaries groups recent spans into traces, newest first.
func (s *Store) QueryTraceSummaries(q TraceQuery) []TraceSummary {
	spans := s.SpansSince(q.Since)
	byTrace := map[string][]SpanRecord{}
	for _, sp := range spans {
		byTrace[sp.TraceID] = append(byTrace[sp.TraceID], sp)
	}

	var edgeSrc, edgeDst string
	if q.Edge != "" {
		if parts := strings.SplitN(q.Edge, "→", 2); len(parts) == 2 {
			edgeSrc, edgeDst = parts[0], parts[1]
		}
	}

	out := make([]TraceSummary, 0, 64)
	for id, tr := range byTrace {
		sum := summarise(id, tr)
		if q.ErrorsOnly && sum.ErrorCount == 0 {
			continue
		}
		if q.MinMs > 0 && sum.DurationMs < q.MinMs {
			continue
		}
		if q.Service != "" && !traceTouches(tr, q.Service) {
			continue
		}
		if edgeSrc != "" && !traceCrosses(tr, edgeSrc, edgeDst) {
			continue
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.After(out[j].Start) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out
}

func summarise(id string, tr []SpanRecord) TraceSummary {
	present := make(map[string]bool, len(tr))
	for _, sp := range tr {
		present[sp.SpanID] = true
	}
	var start, end time.Time
	services := map[string]bool{}
	sum := TraceSummary{TraceID: id, SpanCount: len(tr), Services: []string{}}
	var root *SpanRecord
	for i := range tr {
		sp := &tr[i]
		if start.IsZero() || sp.Timestamp.Before(start) {
			start = sp.Timestamp
		}
		if e := sp.Timestamp.Add(sp.Duration); e.After(end) {
			end = e
		}
		if sp.Service != "" {
			services[sp.Service] = true
		}
		if sp.Status == "ERROR" {
			sum.ErrorCount++
			if sum.Error == "" {
				sum.Error = errorSignature(*sp)
			}
		}
		if sp.ParentID == "" || !present[sp.ParentID] {
			if root == nil || sp.Timestamp.Before(root.Timestamp) {
				root = sp
			}
		}
	}
	if root != nil {
		sum.RootService, sum.RootName = root.Service, root.Name
	}
	sum.Start = start
	sum.DurationMs = float64(end.Sub(start)) / float64(time.Millisecond)
	for svc := range services {
		sum.Services = append(sum.Services, svc)
	}
	sort.Strings(sum.Services)
	return sum
}

func traceTouches(tr []SpanRecord, service string) bool {
	for _, sp := range tr {
		if spanNodeID(sp) == service || sp.Service == service {
			return true
		}
		if vid, _ := virtualNode(sp); vid == service {
			return true
		}
	}
	return false
}

func traceCrosses(tr []SpanRecord, src, dst string) bool {
	byID := make(map[string]SpanRecord, len(tr))
	for _, sp := range tr {
		byID[sp.SpanID] = sp
	}
	for _, sp := range tr {
		if spanNodeID(sp) == src {
			if vid, _ := virtualNode(sp); vid == dst {
				return true
			}
		}
		if p, ok := byID[sp.ParentID]; ok && spanNodeID(p) == src && spanNodeID(sp) == dst {
			return true
		}
		if sp.ParentID == "" && src == "clients" && spanNodeID(sp) == dst {
			return true
		}
	}
	return false
}

// TraceDetail is a full trace for the waterfall view.
type TraceDetail struct {
	TraceSummary
	Spans []SpanRecord `json:"spans"`
	Logs  []LogRecord  `json:"logs,omitempty"`
}

// GetTrace returns one trace with its correlated logs.
func (s *Store) GetTrace(traceID string) *TraceDetail {
	spans := s.Trace(traceID)
	if len(spans) == 0 {
		return nil
	}
	return &TraceDetail{
		TraceSummary: summarise(traceID, spans),
		Spans:        spans,
		Logs:         s.LogsForTraces(map[string]bool{traceID: true}, 200),
	}
}
