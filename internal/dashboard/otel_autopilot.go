package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/kubepilot/kubepilot/pkg/otelautopilot"
)

func (s *Server) handleOTELAutopilotStatus(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	status, err := s.cfg.OTELAutopilot.Status(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleOTELAutopilotSetMode(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, fmt.Errorf("invalid JSON body"), http.StatusBadRequest)
		return
	}
	mode := otelautopilot.Mode(strings.ToLower(strings.TrimSpace(body.Mode)))
	if err := s.cfg.OTELAutopilot.SetMode(mode); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	status, err := s.cfg.OTELAutopilot.Status(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleOTELAutopilotPause(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	s.cfg.OTELAutopilot.Pause()
	status, err := s.cfg.OTELAutopilot.Status(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleOTELAutopilotResume(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	s.cfg.OTELAutopilot.Resume()
	status, err := s.cfg.OTELAutopilot.Status(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleOTELApps(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	apps, err := s.cfg.OTELAutopilot.Apps(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"apps": apps})
}

func (s *Server) handleOTELMetrics(w http.ResponseWriter, r *http.Request) {
	s.writeOTELSignal(w, r, "metrics")
}

func (s *Server) handleOTELLogs(w http.ResponseWriter, r *http.Request) {
	s.writeOTELSignal(w, r, "logs")
}

func (s *Server) handleOTELTraces(w http.ResponseWriter, r *http.Request) {
	s.writeOTELSignal(w, r, "traces")
}

func (s *Server) writeOTELSignal(w http.ResponseWriter, r *http.Request, kind string) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	store := s.cfg.OTELAutopilot.Store()
	if store == nil {
		httpError(w, fmt.Errorf("otel managed store unavailable"), http.StatusServiceUnavailable)
		return
	}
	plane := otelautopilot.Plane(strings.TrimSpace(r.URL.Query().Get("plane")))
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	switch kind {
	case "metrics":
		writeJSON(w, map[string]any{"plane": plane, "service": service, "metrics": store.QueryMetrics(plane, service, limit)})
	case "logs":
		writeJSON(w, map[string]any{"plane": plane, "service": service, "logs": store.QueryLogs(plane, service, limit)})
	case "traces":
		writeJSON(w, map[string]any{"plane": plane, "service": service, "traces": store.QueryTraces(plane, service, limit)})
	default:
		httpError(w, fmt.Errorf("unknown signal kind"), http.StatusBadRequest)
	}
}

func (s *Server) handleOTELService(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	id := mux.Vars(r)["id"]
	apps, err := s.cfg.OTELAutopilot.Apps(r.Context())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	var match *otelautopilot.AppCoverage
	for i := range apps {
		if apps[i].ID == id || apps[i].ServiceName == id || apps[i].Name == id {
			match = &apps[i]
			break
		}
	}
	store := s.cfg.OTELAutopilot.Store()
	service := id
	if match != nil {
		service = match.ServiceName
	}
	writeJSON(w, map[string]any{
		"app":     match,
		"metrics": store.QueryMetrics("", service, 50),
		"logs":    store.QueryLogs("", service, 50),
		"traces":  store.QueryTraces("", service, 50),
	})
}

// handleOTELIngest accepts a simple JSON push of metrics/logs/traces into the
// managed store (used by the in-cluster collector sidecar/agent).
func (s *Server) handleOTELIngest(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OTELAutopilot == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Metrics []otelautopilot.MetricSample `json:"metrics"`
		Logs    []otelautopilot.LogRecord    `json:"logs"`
		Traces  []otelautopilot.SpanRecord   `json:"traces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, fmt.Errorf("invalid JSON body"), http.StatusBadRequest)
		return
	}
	store := s.cfg.OTELAutopilot.Store()
	if len(body.Metrics) > 0 {
		store.IngestMetrics(body.Metrics...)
	}
	if len(body.Logs) > 0 {
		store.IngestLogs(body.Logs...)
	}
	if len(body.Traces) > 0 {
		store.IngestSpans(body.Traces...)
	}
	m, l, t, last := store.Status()
	writeJSON(w, map[string]any{
		"ok":             true,
		"metrics_count":  m,
		"logs_count":     l,
		"traces_count":   t,
		"last_ingest_at": last,
		"export_active":  s.cfg.OTELAutopilot.ExportActive(),
	})
}

// otelReady writes 503 and returns false when OTLP Autopilot is not wired.
func (s *Server) otelReady(w http.ResponseWriter) bool {
	if s.cfg.OTELAutopilot == nil || s.cfg.OTELAutopilot.Store() == nil {
		httpError(w, fmt.Errorf("otel autopilot is not configured"), http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (s *Server) handleOTLPTraces(w http.ResponseWriter, r *http.Request) {
	s.receiveOTLP(w, r, (*otelautopilot.Store).IngestOTLPTraces)
}

func (s *Server) handleOTLPMetrics(w http.ResponseWriter, r *http.Request) {
	s.receiveOTLP(w, r, (*otelautopilot.Store).IngestOTLPMetrics)
}

func (s *Server) handleOTLPLogs(w http.ResponseWriter, r *http.Request) {
	s.receiveOTLP(w, r, (*otelautopilot.Store).IngestOTLPLogs)
}

// receiveOTLP implements the OTLP/HTTP response contract: an empty
// Export*ServiceResponse in the request's encoding on success, 400 for a
// payload that will never decode (so the collector drops it rather than
// retrying forever).
func (s *Server) receiveOTLP(w http.ResponseWriter, r *http.Request, ingest func(*otelautopilot.Store, []byte, bool) (int, error)) {
	if !s.otelReady(w) {
		return
	}
	body, err := otelautopilot.ReadOTLPBody(r)
	if err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	isJSON := otelautopilot.IsOTLPJSON(r)
	if _, err := ingest(s.cfg.OTELAutopilot.Store(), body, isJSON); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
}

func parseWindow(r *http.Request, def time.Duration) time.Duration {
	if raw := r.URL.Query().Get("window"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 && d <= 24*time.Hour {
			return d
		}
	}
	return def
}

func (s *Server) handleOTELServiceGraph(w http.ResponseWriter, r *http.Request) {
	if !s.otelReady(w) {
		return
	}
	window := parseWindow(r, 15*time.Minute)
	ns := strings.TrimSpace(r.URL.Query().Get("namespace"))
	writeJSON(w, s.cfg.OTELAutopilot.ServiceGraph(r.Context(), window, ns))
}

func (s *Server) handleOTELTraceSummaries(w http.ResponseWriter, r *http.Request) {
	if !s.otelReady(w) {
		return
	}
	q := r.URL.Query()
	limit := 100
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	minMs, _ := strconv.ParseFloat(q.Get("min_ms"), 64)
	traces := s.cfg.OTELAutopilot.Store().QueryTraceSummaries(otelautopilot.TraceQuery{
		Since:      time.Now().UTC().Add(-parseWindow(r, 15*time.Minute)),
		Service:    strings.TrimSpace(q.Get("service")),
		Edge:       strings.TrimSpace(q.Get("edge")),
		ErrorsOnly: q.Get("errors") == "true",
		MinMs:      minMs,
		Limit:      limit,
	})
	writeJSON(w, map[string]any{"traces": traces})
}

func (s *Server) handleOTELTrace(w http.ResponseWriter, r *http.Request) {
	if !s.otelReady(w) {
		return
	}
	tr := s.cfg.OTELAutopilot.Store().GetTrace(mux.Vars(r)["traceId"])
	if tr == nil {
		httpError(w, fmt.Errorf("trace not found (it may have aged out of the managed store)"), http.StatusNotFound)
		return
	}
	writeJSON(w, tr)
}

func (s *Server) handleOTELInsights(w http.ResponseWriter, r *http.Request) {
	if !s.otelReady(w) {
		return
	}
	writeJSON(w, map[string]any{"insights": s.cfg.OTELAutopilot.Insights(r.Context())})
}

func (s *Server) handleOTELInsightAnalyze(w http.ResponseWriter, r *http.Request) {
	if !s.otelReady(w) {
		return
	}
	id := mux.Vars(r)["id"]
	if !s.cfg.OTELAutopilot.ReanalyzeInsight(id) {
		httpError(w, fmt.Errorf("insight not found"), http.StatusNotFound)
		return
	}
	writeJSON(w, s.cfg.OTELAutopilot.Insight(id))
}
