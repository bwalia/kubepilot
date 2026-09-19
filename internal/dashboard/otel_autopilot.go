package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
